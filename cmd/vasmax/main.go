package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/sirupsen/logrus"

	"vasmax/internal/alive"
	"vasmax/internal/api"
	"vasmax/internal/audit"
	internalConfig "vasmax/internal/config"
	"vasmax/internal/core"
	"vasmax/internal/firewall"
	"vasmax/internal/i18n"
	menuPkg "vasmax/internal/menu"
	"vasmax/internal/nginx"
	"vasmax/internal/protocol"
	"vasmax/internal/rollback"
	"vasmax/internal/route"
	"vasmax/internal/security"
	"vasmax/internal/subscription"
	internalSync "vasmax/internal/sync"
	"vasmax/internal/sysinfo"
	"vasmax/internal/traffic"
	"vasmax/internal/user"
)

var version = "dev"

func main() {
	// 命令行参数
	configPath := flag.String("c", internalConfig.DefaultConfigPath, "配置文件路径")
	showVersion := flag.Bool("version", false, "显示版本号")
	showMenu := flag.Bool("menu", false, "显示交互式菜单")
	runHealth := flag.Bool("health", false, "运行健康检查")
	writeConfigReference := flag.Bool("write-config-reference", false, "写入配置参考文件")
	updateGeoData := flag.Bool("update-geodata", false, "更新 Xray/sing-box GeoData")
	flag.Parse()

	if *showVersion {
		fmt.Printf("VasmaX %s\n", version)
		return
	}

	if *writeConfigReference {
		if err := internalConfig.WriteReferenceConfig(*configPath); err != nil {
			fmt.Fprintf(os.Stderr, "写入配置参考文件失败: %v\n", err)
			os.Exit(1)
		}
		return
	}

	if *runHealth {
		os.Exit(sysinfo.RunHealthCheck(*configPath))
	}

	// 加载配置
	cfg, err := internalConfig.LoadConfig(*configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "加载配置失败: %v\n", err)
		os.Exit(1)
	}

	// 校验配置
	if err := cfg.Validate(); err != nil {
		fmt.Fprintf(os.Stderr, "配置校验失败: %v\n", err)
		os.Exit(1)
	}

	// 初始化日志
	logger, logCleanup, logErr := internalConfig.InitLogger(&cfg.Log)
	if logErr != nil {
		fmt.Fprintf(os.Stderr, "初始化日志失败: %v\n", logErr)
		os.Exit(1)
	}
	defer logCleanup()
	if err := internalConfig.WriteReferenceConfig(*configPath); err != nil {
		logger.WithError(err).Warn("写入配置参考文件失败")
	}

	if *updateGeoData {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
		defer cancel()
		if err := core.NewManager(cfg, logger).UpdateGeoData(ctx); err != nil {
			logger.WithError(err).Error("GeoData 更新失败")
			os.Exit(1)
		}
		logger.Info("GeoData 更新完成")
		return
	}

	// 设置语言
	i18n.SetLang(cfg.Lang)

	// 初始化审计日志
	var auditLog *audit.Logger
	if cfg.Audit.Enabled {
		auditLog, err = audit.NewLogger(cfg.Audit.FilePath, int64(cfg.Audit.MaxSize)*1024*1024, cfg.Audit.MaxFiles)
		if err != nil {
			logger.WithError(err).Warn("初始化审计日志失败")
		}
	}

	// 交互式菜单模式
	if *showMenu {
		coreMgr := core.NewManager(cfg, logger)
		reg := protocol.DefaultRegistry()
		rbMgr := rollback.NewManager("/etc/vasmax/snapshots", logger)
		userMgr := user.NewManager()
		// 迁移：从 Xray 配置中恢复用户（旧版本未持久化用户的兼容）
		if err := userMgr.RecoverFromXrayConfigs(cfg.Paths.XrayConf); err != nil {
			logger.WithError(err).Debug("恢复 Xray 用户失败")
		}
		subMgr, _ := subscription.NewManager(cfg, reg, userMgr, logger)
		routeMgr := route.NewManager(cfg.Paths.XrayConf, cfg.Paths.SingBoxConf, logger)
		btMgr := route.NewBTManager(routeMgr)
		blMgr := route.NewBlacklistManager(routeMgr)
		warpMgr := route.NewWARPManager(logger)
		nginxMgr := nginx.NewManager(cfg.Paths.NginxConf, logger)
		fwMgr := firewall.NewManager(logger)

		mainMenu := menuPkg.NewMainMenu(
			cfg, coreMgr, reg, rbMgr,
			userMgr, subMgr,
			routeMgr, btMgr, blMgr, warpMgr,
			nginxMgr, fwMgr,
			logger,
		)
		mainMenu.Show()
		return
	}

	// 守护进程模式
	logger.WithField("version", version).Info("VasmaX 启动中")

	// 初始化各模块
	userMgr := user.NewManager()
	if err := userMgr.RecoverFromXrayConfigs(cfg.Paths.XrayConf); err != nil {
		logger.WithError(err).Debug("恢复 Xray 用户失败")
	}
	trafficCtr := traffic.NewCounter()
	aliveTrk := alive.NewTracker(cfg.NodeID)
	coreMgr := core.NewManager(cfg, logger)
	reg := protocol.DefaultRegistry()
	subMgr, err := subscription.NewManager(cfg, reg, userMgr, logger)
	if err != nil {
		logger.WithError(err).Warn("初始化订阅管理器失败，启动时不会自动刷新订阅")
	}

	// 加载持久化流量数据
	trafficFile := filepath.Join(cfg.Paths.Cache, "traffic.json")
	if err := trafficCtr.LoadFromFile(trafficFile); err != nil {
		logger.WithError(err).Warn("加载流量缓存失败，从零开始")
	}

	// 设置上下文和信号处理
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT)

	var syncLoop *internalSync.Loop
	var pullInterval = 60 * time.Second
	var pushInterval = 60 * time.Second

	// 托管模式：启动 SyncLoop
	if !cfg.Standalone && cfg.APIHost != "" {
		apiClient := api.NewClient(cfg.APIHost, cfg.APIToken, cfg.NodeID, cfg.NodeType, logger)
		if cfg.APIPrefix != "" {
			apiClient.SetAPIPrefix(cfg.APIPrefix)
		}

		// 确保 Xray Stats API 配置存在（托管模式需要采集流量）
		if err := protocol.EnsureStatsConfig(cfg.Paths.XrayConf, true); err != nil {
			logger.WithError(err).Warn("配置 Xray Stats API 失败")
		}

		// 获取节点配置
		nodeCfg, err := fetchAndCacheConfig(ctx, apiClient, cfg, logger)
		if err != nil {
			logger.WithError(err).Warn("获取节点配置失败，使用默认间隔")
		}

		nodeConfigChanged := false
		if nodeCfg != nil {
			if nodeCfg.BaseConfig.PullInterval > 0 {
				pullInterval = time.Duration(nodeCfg.BaseConfig.PullInterval) * time.Second
			}
			if nodeCfg.BaseConfig.PushInterval > 0 {
				pushInterval = time.Duration(nodeCfg.BaseConfig.PushInterval) * time.Second
			}
			if applyManagedNodeConfig(cfg, nodeCfg, reg, logger) {
				nodeConfigChanged = true
				if err := internalConfig.SaveConfig(*configPath, cfg); err != nil {
					logger.WithError(err).Warn("保存 Xboard 下发配置失败")
				}
			}
		}
		pullInterval = clampManagedInterval("pull_interval", pullInterval, cfg.Sync.MinPullIntervalSeconds, logger)
		pushInterval = clampManagedInterval("push_interval", pushInterval, cfg.Sync.MinPushIntervalSeconds, logger)

		syncLoop = internalSync.NewLoop(apiClient, userMgr, trafficCtr, aliveTrk, coreMgr, reg, cfg, nodeCfg, logger, auditLog)
		if nodeConfigChanged {
			syncLoop.MarkConfigDirty("xboard node config changed")
			if err := syncLoop.RunOnce(ctx); err != nil {
				logger.WithError(err).Warn("启动前同步失败，将继续启动现有核心配置")
			}
		}
	}

	if changed, err := menuPkg.SyncRealityRuntime(nil, cfg); err != nil {
		logger.WithError(err).Warn("启动前同步 Reality 入站配置失败")
	} else if changed {
		logger.Info("启动前已根据 config.yaml 同步 Reality 入站配置")
	}

	// 启动核心。托管模式若启动前同步成功，这里会启动已修正后的配置；
	// 若同步失败，则继续启动现有配置，避免服务完全不可用。
	if err := coreMgr.StartAll(); err != nil {
		logger.WithError(err).Warn("启动核心失败")
	}

	if cfg.Standalone && subMgr != nil {
		if err := subMgr.GenerateAll(); err != nil {
			logger.WithError(err).Warn("启动时重新生成订阅失败")
		} else {
			logger.Info("启动时已根据当前配置重新生成订阅")
		}
	}

	if syncLoop != nil {

		go func() {
			defer func() {
				if r := recover(); r != nil {
					logger.Errorf("SyncLoop panic: %v", r)
				}
			}()
			syncLoop.Start(ctx, pullInterval, pushInterval)
		}()

		logger.Info("托管模式已启动")
	} else {
		logger.Info("独立模式运行中")
	}

	// 等待退出信号
	sig := <-sigCh
	logger.WithField("signal", sig).Info("收到退出信号，开始优雅关闭")

	// 优雅关闭
	cancel()

	// 上报未提交流量
	if !cfg.Standalone && cfg.APIHost != "" {
		apiClient := api.NewClient(cfg.APIHost, cfg.APIToken, cfg.NodeID, cfg.NodeType, logger)
		if cfg.APIPrefix != "" {
			apiClient.SetAPIPrefix(cfg.APIPrefix)
		}
		snapshot := trafficCtr.Snapshot()
		if len(snapshot) > 0 {
			shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer shutdownCancel()
			if err := apiClient.PushTraffic(shutdownCtx, snapshot); err != nil {
				logger.WithError(err).Warn("关闭时上报流量失败")
				trafficCtr.Merge(snapshot)
			}
		}
	}

	// 持久化剩余流量
	if err := trafficCtr.SaveToFile(trafficFile); err != nil {
		logger.WithError(err).Error("持久化流量数据失败")
	}

	// 关闭审计日志
	if auditLog != nil {
		auditLog.Close()
	}

	logger.Info("VasmaX 已退出")
}

// fetchAndCacheConfig fetches node config from API and caches it locally.
// Falls back to cached config if API is unreachable.
func fetchAndCacheConfig(ctx context.Context, client *api.Client, cfg *internalConfig.Config, logger *logrus.Logger) (*api.NodeConfig, error) {
	nodeCfg, err := client.FetchConfig(ctx)
	if err != nil {
		// Try loading from cache.
		logger.WithError(err).Warn("API 不可达，尝试加载缓存配置")
		cached, cacheErr := loadCachedConfig(filepath.Join(cfg.Paths.Cache, "node_config.json"))
		if cacheErr != nil {
			return nil, fmt.Errorf("API 不可达且无缓存: %w", err)
		}
		return cached, nil
	}
	if nodeCfg == nil {
		// 304 not modified, load from cache.
		return loadCachedConfig(filepath.Join(cfg.Paths.Cache, "node_config.json"))
	}

	// Apply server_port override.
	if nodeCfg.ServerPort > 0 {
		logger.WithField("server_port", nodeCfg.ServerPort).Info("使用 API 下发端口")
	}

	// Cache the config.
	cacheNodeConfig(filepath.Join(cfg.Paths.Cache, "node_config.json"), nodeCfg, logger)

	return nodeCfg, nil
}

// nodeConfigCache wraps NodeConfig with timestamp for caching.
type nodeConfigCache struct {
	Timestamp int64           `json:"timestamp"`
	Config    *api.NodeConfig `json:"config"`
}

func loadCachedConfig(path string) (*api.NodeConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var cache nodeConfigCache
	if err := parseJSON(data, &cache); err != nil {
		return nil, err
	}
	return cache.Config, nil
}

func cacheNodeConfig(path string, cfg *api.NodeConfig, logger *logrus.Logger) {
	cache := nodeConfigCache{
		Timestamp: time.Now().Unix(),
		Config:    cfg,
	}
	if err := security.AtomicWriteJSON(path, cache, 0600); err != nil {
		logger.WithError(err).Warn("写入节点配置缓存失败")
	}
}

func applyManagedNodeConfig(cfg *internalConfig.Config, nodeCfg *api.NodeConfig, reg *protocol.Registry, logger *logrus.Logger) bool {
	if cfg == nil || nodeCfg == nil {
		return false
	}

	changed := false
	if nodeCfg.ServerName != "" && cfg.TLS.Domain != nodeCfg.ServerName {
		cfg.TLS.Domain = nodeCfg.ServerName
		changed = true
	}

	if nodeCfg.ServerPort <= 0 {
		return changed
	}

	matches := managedDirectProtocols(cfg, reg)
	if len(matches) != 1 {
		if len(matches) > 1 {
			logger.WithFields(logrus.Fields{
				"node_type": cfg.NodeType,
				"protocols": matches,
			}).Warn("Xboard 下发端口匹配到多个直连协议，跳过自动覆盖以避免端口冲突")
		}
		return changed
	}

	if cfg.ProtocolPorts == nil {
		cfg.ProtocolPorts = make(map[string]int)
	}
	if cfg.ProtocolPorts[matches[0]] != nodeCfg.ServerPort {
		cfg.ProtocolPorts[matches[0]] = nodeCfg.ServerPort
		logger.WithFields(logrus.Fields{
			"protocol": matches[0],
			"port":     nodeCfg.ServerPort,
		}).Info("已应用 Xboard 下发监听端口")
		changed = true
	}

	return changed
}

func managedDirectProtocols(cfg *internalConfig.Config, reg *protocol.Registry) []string {
	var matches []string
	for _, protoName := range cfg.Protocols {
		p, ok := reg.Get(protoName)
		if !ok || protocol.NeedsNginxProxy(p) {
			continue
		}
		if protocolMatchesNodeType(protoName, cfg.NodeType) {
			matches = append(matches, protoName)
		}
	}
	return matches
}

func protocolMatchesNodeType(protoName, nodeType string) bool {
	switch strings.ToLower(nodeType) {
	case "anytls", "tuic", "naive":
		return protoName == nodeType
	case "hysteria", "hysteria2":
		return protoName == "hysteria2"
	case "socks", "socks5":
		return protoName == "socks5"
	case "vless":
		return strings.HasPrefix(protoName, "vless_")
	case "vmess":
		return strings.HasPrefix(protoName, "vmess_")
	case "trojan":
		return strings.HasPrefix(protoName, "trojan_")
	default:
		return protoName == nodeType
	}
}

func clampManagedInterval(name string, interval time.Duration, minSeconds int, logger *logrus.Logger) time.Duration {
	if minSeconds <= 0 {
		return interval
	}
	minInterval := time.Duration(minSeconds) * time.Second
	if interval >= minInterval {
		return interval
	}
	if logger != nil {
		logger.WithFields(logrus.Fields{
			"name":      name,
			"requested": interval,
			"minimum":   minInterval,
		}).Warn("Xboard 下发同步间隔过短，已使用本地最小间隔保护")
	}
	return minInterval
}

func parseJSON(data []byte, v interface{}) error {
	return json.Unmarshal(data, v)
}
