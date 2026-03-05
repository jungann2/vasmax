// Package sync provides the main synchronization loop for xboard integration.
package sync

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/sirupsen/logrus"

	"vasmax/internal/alive"
	"vasmax/internal/api"
	"vasmax/internal/audit"
	"vasmax/internal/config"
	"vasmax/internal/core"
	"vasmax/internal/protocol"
	"vasmax/internal/security"
	"vasmax/internal/sysinfo"
	"vasmax/internal/traffic"
	"vasmax/internal/user"
)

// Loop 同步循环
type Loop struct {
	apiClient      *api.Client
	userManager    *user.Manager
	trafficCounter *traffic.Counter
	aliveTracker   *alive.Tracker
	coreManager    *core.Manager
	registry       *protocol.Registry
	config         *config.Config
	logger         *logrus.Logger
	auditLog       *audit.Logger
	xrayStats      *traffic.XrayStatsCollector
}

// NewLoop 创建同步循环
func NewLoop(
	apiClient *api.Client,
	userMgr *user.Manager,
	trafficCtr *traffic.Counter,
	aliveTrk *alive.Tracker,
	coreMgr *core.Manager,
	reg *protocol.Registry,
	cfg *config.Config,
	logger *logrus.Logger,
	auditLog *audit.Logger,
) *Loop {
	return &Loop{
		apiClient:      apiClient,
		userManager:    userMgr,
		trafficCounter: trafficCtr,
		aliveTracker:   aliveTrk,
		coreManager:    coreMgr,
		registry:       reg,
		config:         cfg,
		logger:         logger,
		auditLog:       auditLog,
		xrayStats:      traffic.NewXrayStatsCollector("", ""),
	}
}

// Start 启动同步循环，使用 time.Ticker 按间隔执行
func (l *Loop) Start(ctx context.Context, pullInterval, pushInterval time.Duration) {
	pullTicker := time.NewTicker(pullInterval)
	pushTicker := time.NewTicker(pushInterval)
	defer pullTicker.Stop()
	defer pushTicker.Stop()

	l.logger.WithFields(logrus.Fields{
		"pull_interval": pullInterval,
		"push_interval": pushInterval,
	}).Info("同步循环已启动")

	// 启动时立即执行一次同步，不等待第一个 tick
	if err := l.pullUsers(ctx); err != nil {
		l.logger.WithError(err).Error("初始拉取用户失败")
	}
	if err := l.pushData(ctx); err != nil {
		l.logger.WithError(err).Error("初始上报数据失败")
	}

	for {
		select {
		case <-ctx.Done():
			l.logger.Info("同步循环已停止")
			return
		case <-pullTicker.C:
			if err := l.pullUsers(ctx); err != nil {
				l.logger.WithError(err).Error("拉取用户失败")
			}
		case <-pushTicker.C:
			if err := l.pushData(ctx); err != nil {
				l.logger.WithError(err).Error("上报数据失败")
			}
		}
	}
}

// RunOnce 执行一次完整同步（手动触发）
func (l *Loop) RunOnce(ctx context.Context) error {
	if err := l.pullUsers(ctx); err != nil {
		l.logger.WithError(err).Error("手动同步: 拉取用户失败")
	}
	if err := l.pushData(ctx); err != nil {
		l.logger.WithError(err).Error("手动同步: 上报数据失败")
		return err
	}
	l.logger.Info("手动同步完成")
	return nil
}

// pullUsers 拉取用户列表 → 原子替换 UserTable → 重新生成配置 → 重载核心
func (l *Loop) pullUsers(ctx context.Context) error {
	users, err := l.apiClient.FetchUsers()
	if err != nil {
		return err
	}
	if users == nil {
		// 304 未修改
		return nil
	}

	// 原子替换用户表
	l.userManager.UpdateUsers(users)

	// 缓存用户列表
	l.cacheUsers(users)

	// 重新生成各协议用户配置并写入 ConfigPath
	if err := l.regenerateConfigs(users); err != nil {
		l.logger.WithError(err).Error("重新生成协议配置失败")
		// 不 return，仍然尝试重载核心（可能旧配置仍可用）
	}

	// 重载核心
	if err := l.reloadCores(); err != nil {
		l.logger.WithError(err).Error("重载核心失败")
	}

	l.logger.WithField("count", len(users)).Info("用户列表已同步")

	if l.auditLog != nil {
		_ = l.auditLog.Log(&audit.AuditEntry{
			Action:  "user_sync",
			Details: fmt.Sprintf("用户列表已同步，共 %d 用户", len(users)),
			Result:  "success",
			Source:  "syncloop",
		})
	}

	return nil
}

// regenerateConfigs 根据当前用户列表重新生成所有协议配置文件
func (l *Loop) regenerateConfigs(users []api.User) error {
	apiUsers := make([]*api.User, len(users))
	for i := range users {
		apiUsers[i] = &users[i]
	}

	// 按核心类型分组已安装协议
	for _, protoName := range l.config.Protocols {
		p, ok := l.registry.Get(protoName)
		if !ok {
			continue
		}

		params := &protocol.InboundParams{
			Port:     p.DefaultPort(),
			Domain:   l.config.TLS.Domain,
			CertFile: l.config.TLS.CertFile,
			KeyFile:  l.config.TLS.KeyFile,
			Users:    apiUsers,
			Tag:      protoName,
		}
		if l.config.Reality.PrivateKey != "" {
			params.Reality = &l.config.Reality
		}
		if l.config.Hysteria2.Port > 0 {
			params.Hysteria2 = &l.config.Hysteria2
		}
		if l.config.Tuic.Port > 0 {
			params.Tuic = &l.config.Tuic
		}

		inboundJSON, err := p.GenerateInbound(params)
		if err != nil {
			l.logger.WithError(err).Errorf("生成 %s 入站配置失败", protoName)
			continue
		}

		// 包装为 inbounds 数组格式
		wrapper := map[string]interface{}{
			"inbounds": []json.RawMessage{inboundJSON},
		}

		var confDir string
		var fileName string
		switch p.CoreType() {
		case "xray":
			confDir = l.config.Paths.XrayConf
			fileName = fmt.Sprintf("05_%s_inbounds.json", protoName)
		case "singbox":
			confDir = l.config.Paths.SingBoxConf
			fileName = fmt.Sprintf("10_%s_inbounds.json", protoName)
		default:
			continue
		}

		confPath := filepath.Join(confDir, fileName)
		if err := security.AtomicWriteJSON(confPath, wrapper, 0644); err != nil {
			l.logger.WithError(err).Errorf("写入 %s 配置失败", confPath)
		}
	}

	return nil
}

// reloadCores 重载所有已安装的核心
// Xray: SIGUSR1 热重载; sing-box: 先合并配置再重启
func (l *Loop) reloadCores() error {
	hasXray := false
	hasSingbox := false
	for _, protoName := range l.config.Protocols {
		if p, ok := l.registry.Get(protoName); ok {
			switch p.CoreType() {
			case "xray":
				hasXray = true
			case "singbox":
				hasSingbox = true
			}
		}
	}

	if hasXray {
		if err := l.coreManager.ReloadXray(); err != nil {
			l.logger.WithError(err).Warn("Xray 热重载失败")
		} else {
			l.logger.Info("Xray 热重载成功")
		}
	}

	if hasSingbox {
		// sing-box 不支持多文件配置，需先合并为单一 config.json
		if err := l.coreManager.MergeSingBoxConfig(); err != nil {
			l.logger.WithError(err).Warn("sing-box 配置合并失败，跳过重启")
		} else if err := l.coreManager.RestartSingBox(); err != nil {
			l.logger.WithError(err).Warn("sing-box 重启失败")
		} else {
			l.logger.Info("sing-box 重启成功")
		}
	}

	return nil
}

// collectXrayTraffic 从 Xray Stats API 采集流量并累加到 trafficCounter
// 同时根据有流量的用户更新 alive tracker（有流量即视为在线）
func (l *Loop) collectXrayTraffic() {
	// 检查是否有 Xray 协议
	hasXray := false
	for _, protoName := range l.config.Protocols {
		if p, ok := l.registry.Get(protoName); ok && p.CoreType() == "xray" {
			hasXray = true
			break
		}
	}
	if !hasXray {
		return
	}

	stats, err := l.xrayStats.Collect()
	if err != nil {
		l.logger.WithError(err).Debug("采集 Xray 流量失败")
		return
	}

	// stats 格式: map["user_{id}"][upload, download]
	// 需要解析 email 中的 user_id
	for email, trafficData := range stats {
		// email 格式: "user_{id}"
		var uid int
		if _, err := fmt.Sscanf(email, "user_%d", &uid); err != nil {
			continue
		}
		if trafficData[0] > 0 || trafficData[1] > 0 {
			l.trafficCounter.Add(uid, trafficData[0], trafficData[1])
			// 有流量即视为在线，使用占位 IP 标记
			// xboard alive 接口主要关心在线用户数量
			l.aliveTracker.Track(uid, "127.0.0.1")
		}
	}

	// 清理超过 2 个 push 周期无活动的用户
	l.aliveTracker.CleanExpired(5 * time.Minute)
}

// pushData 上报流量、在线用户、节点状态
func (l *Loop) pushData(ctx context.Context) error {
	// 0. 从 Xray Stats API 采集流量并累加到 trafficCounter
	l.collectXrayTraffic()

	// 1. 流量上报
	snapshot := l.trafficCounter.Snapshot()
	if len(snapshot) > 0 {
		if err := l.apiClient.PushTraffic(snapshot); err != nil {
			// 上报失败，回滚流量数据
			l.trafficCounter.Merge(snapshot)
			l.logger.WithError(err).Warn("流量上报失败，已回滚")
		}
	}

	// 2. 在线用户上报
	aliveSnapshot := l.aliveTracker.Snapshot()
	if len(aliveSnapshot) > 0 {
		if err := l.apiClient.PushAlive(aliveSnapshot); err != nil {
			l.logger.WithError(err).Warn("在线用户上报失败")
		}
	}

	// 3. 节点状态上报
	status, err := sysinfo.CollectStatus()
	if err != nil {
		l.logger.WithError(err).Warn("采集节点状态失败")
	} else {
		if err := l.apiClient.PushStatus(status); err != nil {
			l.logger.WithError(err).Warn("节点状态上报失败")
		}
	}

	return nil
}

// userCache 用户缓存结构
type userCache struct {
	Timestamp int64      `json:"timestamp"`
	ETag      string     `json:"etag"`
	Users     []api.User `json:"users"`
}

// cacheUsers 缓存用户列表到本地文件
func (l *Loop) cacheUsers(users []api.User) {
	cachePath := filepath.Join(l.config.Paths.Cache, "users.json")
	cache := userCache{
		Timestamp: time.Now().Unix(),
		Users:     users,
	}
	if err := security.AtomicWriteJSON(cachePath, cache, 0600); err != nil {
		l.logger.WithError(err).Warn("缓存用户列表失败")
	}
}

// LoadCachedUsers 从缓存加载用户列表（API 不可达时使用）
func (l *Loop) LoadCachedUsers() ([]api.User, error) {
	cachePath := filepath.Join(l.config.Paths.Cache, "users.json")
	data, err := os.ReadFile(cachePath)
	if err != nil {
		return nil, err
	}
	var cache userCache
	if err := json.Unmarshal(data, &cache); err != nil {
		return nil, err
	}
	return cache.Users, nil
}
