package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/sirupsen/logrus"

	"vasmax/internal/alive"
	"vasmax/internal/api"
	"vasmax/internal/audit"
	internalConfig "vasmax/internal/config"
	"vasmax/internal/core"
	"vasmax/internal/i18n"
	"vasmax/internal/protocol"
	"vasmax/internal/security"
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
	flag.Parse()

	if *showVersion {
		fmt.Printf("VasmaX %s\n", version)
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
		// TODO: 菜单系统将在 Task 26 实现
		fmt.Println("交互式菜单尚未实现")
		return
	}

	// 守护进程模式
	logger.WithField("version", version).Info("VasmaX 启动中")

	// 初始化各模块
	userMgr := user.NewManager()
	trafficCtr := traffic.NewCounter()
	aliveTrk := alive.NewTracker(cfg.NodeID)
	coreMgr := core.NewManager(cfg, logger)

	// 加载持久化流量数据
	trafficFile := filepath.Join(cfg.Paths.Cache, "traffic.json")
	if err := trafficCtr.LoadFromFile(trafficFile); err != nil {
		logger.WithError(err).Warn("加载流量缓存失败，从零开始")
	}

	// 启动核心
	if err := coreMgr.StartAll(); err != nil {
		logger.WithError(err).Warn("启动核心失败")
	}

	// 设置上下文和信号处理
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT)

	// 托管模式：启动 SyncLoop
	if !cfg.Standalone && cfg.APIHost != "" {
		apiClient := api.NewClient(cfg.APIHost, cfg.APIToken, cfg.NodeID, cfg.NodeType, logger)

		// 确保 Xray Stats API 配置存在（托管模式需要采集流量）
		if err := protocol.EnsureStatsConfig(cfg.Paths.XrayConf, true); err != nil {
			logger.WithError(err).Warn("配置 Xray Stats API 失败")
		}

		// 获取节点配置
		nodeCfg, err := fetchAndCacheConfig(apiClient, cfg, logger)
		if err != nil {
			logger.WithError(err).Warn("获取节点配置失败，使用默认间隔")
		}

		pullInterval := 60 * time.Second
		pushInterval := 60 * time.Second
		if nodeCfg != nil {
			if nodeCfg.BaseConfig.PullInterval > 0 {
				pullInterval = time.Duration(nodeCfg.BaseConfig.PullInterval) * time.Second
			}
			if nodeCfg.BaseConfig.PushInterval > 0 {
				pushInterval = time.Duration(nodeCfg.BaseConfig.PushInterval) * time.Second
			}
		}

		syncLoop := internalSync.NewLoop(apiClient, userMgr, trafficCtr, aliveTrk, coreMgr, protocol.DefaultRegistry(), cfg, logger, auditLog)

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
		snapshot := trafficCtr.Snapshot()
		if len(snapshot) > 0 {
			if err := apiClient.PushTraffic(snapshot); err != nil {
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
func fetchAndCacheConfig(client *api.Client, cfg *internalConfig.Config, logger *logrus.Logger) (*api.NodeConfig, error) {
	nodeCfg, err := client.FetchConfig()
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

func parseJSON(data []byte, v interface{}) error {
	return json.Unmarshal(data, v)
}
