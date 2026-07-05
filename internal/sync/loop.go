// Package sync provides the main synchronization loop for xboard integration.
package sync

import (
	"context"
	"encoding/json"
	"errors"
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
	nodeConfig     *api.NodeConfig
	logger         *logrus.Logger
	auditLog       *audit.Logger
	xrayStats      *traffic.XrayStatsCollector
	emptyUserPulls int
	configDirty    bool
	configDirtyWhy string
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
	nodeCfg *api.NodeConfig,
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
		nodeConfig:     nodeCfg,
		logger:         logger,
		auditLog:       auditLog,
		xrayStats:      traffic.NewXrayStatsCollector("", ""),
	}
}

// MarkConfigDirty forces the next successful user sync to rebuild core configs
// even when the user list itself did not change.
func (l *Loop) MarkConfigDirty(reason string) {
	l.configDirty = true
	l.configDirtyWhy = reason
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

	l.loadCachedUsersIntoManager()

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
	l.loadCachedUsersIntoManager()
	var errs []error
	if err := l.pullUsers(ctx); err != nil {
		l.logger.WithError(err).Error("手动同步: 拉取用户失败")
		errs = append(errs, err)
	}
	if err := l.pushData(ctx); err != nil {
		l.logger.WithError(err).Error("手动同步: 上报数据失败")
		errs = append(errs, err)
	}
	if len(errs) > 0 {
		return errors.Join(errs...)
	}
	l.logger.Info("手动同步完成")
	return nil
}

// pullUsers 拉取用户列表 → 必要时重新生成配置 → 原子替换 UserTable → 重载核心
func (l *Loop) pullUsers(ctx context.Context) error {
	users, err := l.apiClient.FetchUsers(ctx)
	if err != nil {
		return err
	}
	if users == nil {
		// 304 未修改
		if !l.configDirty {
			return nil
		}
		cachedUsers := apiUsersFromEntries(l.userManager.GetAllUsers())
		if len(cachedUsers) == 0 {
			l.logger.WithField("reason", l.configDirtyWhy).Warn("节点配置已变化，但没有可用缓存用户，暂不重建核心配置")
			return nil
		}
		users = cachedUsers
		l.logger.WithFields(logrus.Fields{
			"count":  len(users),
			"reason": l.configDirtyWhy,
		}).Info("用户列表未变化但节点配置已变化，使用缓存用户重建核心配置")
	}

	if err := validateManagedUsers(users); err != nil {
		return err
	}

	existing := l.userManager.GetAllUsers()
	if l.shouldDeferEmptyUsers(existing, users) {
		return nil
	}

	if managedUsersEqual(existing, users) && !l.configDirty {
		l.cacheUsers(users)
		l.logger.WithField("count", len(users)).Debug("用户列表未变化，跳过配置重载")
		return nil
	}

	if managedCoreUsersEqual(existing, users) && !l.configDirty {
		l.userManager.UpdateUsers(users)
		l.cacheUsers(users)
		l.logger.WithField("count", len(users)).Info("用户状态字段已更新，核心用户未变化，跳过配置重载")
		return nil
	}

	if l.configDirty {
		l.logger.WithField("reason", l.configDirtyWhy).Info("节点配置已变化，强制重建核心配置")
	}

	// 先确认配置可完整生成，再切换内存用户表；避免配置生成失败时内存状态
	// 与当前运行核心不一致。
	if err := l.regenerateConfigs(users); err != nil {
		l.logger.WithError(err).Error("重新生成协议配置失败，跳过核心重载以保留当前运行配置")
		return err
	}

	// 原子替换用户表
	l.userManager.UpdateUsers(users)

	// 缓存用户列表
	l.cacheUsers(users)

	// 重载核心
	if err := l.reloadCores(); err != nil {
		l.logger.WithError(err).Error("重载核心失败")
		l.MarkConfigDirty("previous core reload failed")
		return err
	}
	l.configDirty = false
	l.configDirtyWhy = ""

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

func (l *Loop) loadCachedUsersIntoManager() {
	if len(l.userManager.GetAllUsers()) > 0 {
		return
	}
	users, err := l.LoadCachedUsers()
	if err != nil {
		l.logger.WithError(err).Debug("未加载到托管用户缓存")
		return
	}
	if err := validateManagedUsers(users); err != nil {
		l.logger.WithError(err).Warn("托管用户缓存无效，已忽略")
		return
	}
	if len(users) == 0 {
		return
	}
	l.userManager.UpdateUsers(users)
	l.logger.WithField("count", len(users)).Info("已加载托管用户缓存，避免启动时无变化重载")
}

func (l *Loop) shouldDeferEmptyUsers(existing []*user.UserEntry, next []api.User) bool {
	threshold := l.config.Sync.EmptyUsersApplyThreshold
	if threshold <= 0 || len(existing) == 0 || len(next) > 0 {
		l.emptyUserPulls = 0
		return false
	}

	l.emptyUserPulls++
	if l.emptyUserPulls < threshold {
		l.logger.WithFields(logrus.Fields{
			"existing_count": len(existing),
			"attempt":        l.emptyUserPulls,
			"threshold":      threshold,
		}).Warn("API 返回空用户列表，疑似临时异常，保留当前运行配置")
		return true
	}

	l.logger.WithFields(logrus.Fields{
		"existing_count": len(existing),
		"threshold":      threshold,
	}).Warn("API 连续返回空用户列表，达到阈值后应用空用户列表")
	l.emptyUserPulls = 0
	return false
}

// regenerateConfigs 根据当前用户列表重新生成所有协议配置文件
func (l *Loop) regenerateConfigs(users []api.User) error {
	apiUsers := make([]*api.User, len(users))
	for i := range users {
		apiUsers[i] = &users[i]
	}

	var errs []error
	writes := make([]configWrite, 0, len(l.config.Protocols))
	writeTargets := make(map[string]struct{}, len(l.config.Protocols))

	// 按核心类型分组已安装协议
	for _, protoName := range l.config.Protocols {
		p, ok := l.registry.Get(protoName)
		if !ok {
			continue
		}

		domain := l.config.GetProtocolDomain(protoName)
		if domain == "" {
			domain = l.config.TLS.Domain
		}
		certFile := l.config.TLS.CertFile
		keyFile := l.config.TLS.KeyFile
		if domain != "" {
			certFile, keyFile = config.DetectCertPath(&config.TLSConfig{
				Domain:   domain,
				CertFile: certFile,
				KeyFile:  keyFile,
			})
		}
		if p.CoreType() == "singbox" && p.Name() != "socks5" && (certFile == "" || keyFile == "") {
			certPaths, err := security.EnsureSelfSignedCert("/etc/vasmax/tls")
			if err != nil {
				l.logger.WithError(err).Error("生成 sing-box 自签证书失败")
				errs = append(errs, fmt.Errorf("%s: generate self-signed cert: %w", protoName, err))
				continue
			}
			certFile = certPaths.CertFile
			keyFile = certPaths.KeyFile
		}

		params := &protocol.InboundParams{
			Port:          protocol.EffectiveInboundPort(p, l.config),
			Domain:        domain,
			CertFile:      certFile,
			KeyFile:       keyFile,
			Users:         apiUsers,
			Tag:           protoName,
			Path:          protocol.DefaultWSPath(p),
			ServiceName:   protocol.DefaultGRPCServiceName(p),
			TLSMinVersion: l.config.TLS.MinVersion,
			TLSMaxVersion: l.config.TLS.MaxVersion,
			ALPN:          l.config.ALPN.ALPNList(),
			KeepAlive:     l.config.Connection,
		}
		if l.nodeConfig != nil && p.Name() == "anytls" {
			params.PaddingScheme = l.nodeConfig.PaddingScheme
		}
		if l.config.Reality.PrivateKey != "" && isRealityProtocol(protoName) {
			params.Reality = &l.config.Reality
		}
		if protoName == "hysteria2" {
			params.Hysteria2 = &l.config.Hysteria2
		}
		if protoName == "tuic" {
			params.Tuic = &l.config.Tuic
		}

		inboundJSONs, err := protocol.GenerateInboundMessages(p, params)
		if err != nil {
			l.logger.WithError(err).Errorf("生成 %s 入站配置失败", protoName)
			errs = append(errs, fmt.Errorf("%s: generate inbound: %w", protoName, err))
			continue
		}

		// 包装为 inbounds 数组格式
		wrapper := map[string]interface{}{
			"inbounds": inboundJSONs,
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
		if _, exists := writeTargets[confPath]; exists {
			errs = append(errs, fmt.Errorf("%s: duplicate config target %s", protoName, confPath))
			continue
		}
		writeTargets[confPath] = struct{}{}

		data, err := json.MarshalIndent(wrapper, "", "  ")
		if err != nil {
			l.logger.WithError(err).Errorf("序列化 %s 配置失败", protoName)
			errs = append(errs, fmt.Errorf("%s: marshal %s: %w", protoName, confPath, err))
			continue
		}
		if !json.Valid(data) {
			errs = append(errs, fmt.Errorf("%s: marshaled config is not valid JSON: %s", protoName, confPath))
			continue
		}

		writes = append(writes, configWrite{path: confPath, data: data, perm: 0644})
	}

	if len(errs) > 0 {
		return errors.Join(errs...)
	}

	if err := applyConfigWrites(writes); err != nil {
		return err
	}

	return nil
}

func managedUsersEqual(existing []*user.UserEntry, next []api.User) bool {
	if len(existing) != len(next) {
		return false
	}

	byID := make(map[int]*user.UserEntry, len(existing))
	for _, entry := range existing {
		if entry == nil {
			return false
		}
		if _, exists := byID[entry.ID]; exists {
			return false
		}
		byID[entry.ID] = entry
	}

	for _, apiUser := range next {
		entry, ok := byID[apiUser.ID]
		if !ok {
			return false
		}
		if entry.UUID != apiUser.UUID {
			return false
		}
		if entry.SpeedLimit != intPtrValue(apiUser.SpeedLimit) {
			return false
		}
		if entry.DeviceLimit != intPtrValue(apiUser.DeviceLimit) {
			return false
		}
	}

	return true
}

func managedCoreUsersEqual(existing []*user.UserEntry, next []api.User) bool {
	if len(existing) != len(next) {
		return false
	}

	byID := make(map[int]*user.UserEntry, len(existing))
	for _, entry := range existing {
		if entry == nil {
			return false
		}
		if _, exists := byID[entry.ID]; exists {
			return false
		}
		byID[entry.ID] = entry
	}

	for _, apiUser := range next {
		entry, ok := byID[apiUser.ID]
		if !ok {
			return false
		}
		if entry.UUID != apiUser.UUID {
			return false
		}
	}

	return true
}

func apiUsersFromEntries(entries []*user.UserEntry) []api.User {
	users := make([]api.User, 0, len(entries))
	for _, entry := range entries {
		if entry == nil {
			continue
		}
		u := api.User{
			ID:   entry.ID,
			UUID: entry.UUID,
		}
		if entry.SpeedLimit > 0 {
			limit := entry.SpeedLimit
			u.SpeedLimit = &limit
		}
		if entry.DeviceLimit > 0 {
			limit := entry.DeviceLimit
			u.DeviceLimit = &limit
		}
		users = append(users, u)
	}
	return users
}

type configWrite struct {
	path string
	data []byte
	perm os.FileMode
}

type configWriteBackup struct {
	path   string
	exists bool
	data   []byte
	perm   os.FileMode
}

func applyConfigWrites(writes []configWrite) error {
	backups := make(map[string]configWriteBackup, len(writes))
	for _, write := range writes {
		if write.path == "" {
			return fmt.Errorf("config write path must not be empty")
		}
		if _, exists := backups[write.path]; exists {
			return fmt.Errorf("duplicate config write target: %s", write.path)
		}

		backup := configWriteBackup{path: write.path}
		info, err := os.Stat(write.path)
		switch {
		case err == nil:
			if info.IsDir() {
				return fmt.Errorf("config write target is a directory: %s", write.path)
			}
			data, readErr := os.ReadFile(write.path)
			if readErr != nil {
				return fmt.Errorf("backup config %s: %w", write.path, readErr)
			}
			backup.exists = true
			backup.data = data
			backup.perm = info.Mode().Perm()
		case errors.Is(err, os.ErrNotExist):
			backup.exists = false
			backup.perm = write.perm
		default:
			return fmt.Errorf("stat config %s: %w", write.path, err)
		}

		backups[write.path] = backup
	}

	applied := make([]string, 0, len(writes))
	for _, write := range writes {
		if err := security.AtomicWrite(write.path, write.data, write.perm); err != nil {
			rollbackErr := rollbackConfigWrites(applied, backups)
			return errors.Join(fmt.Errorf("write config %s: %w", write.path, err), rollbackErr)
		}
		applied = append(applied, write.path)
	}

	return nil
}

func rollbackConfigWrites(paths []string, backups map[string]configWriteBackup) error {
	var errs []error
	for i := len(paths) - 1; i >= 0; i-- {
		backup := backups[paths[i]]
		if backup.exists {
			perm := backup.perm
			if perm == 0 {
				perm = 0644
			}
			if err := security.AtomicWrite(backup.path, backup.data, perm); err != nil {
				errs = append(errs, fmt.Errorf("restore %s: %w", backup.path, err))
			}
			continue
		}
		if err := os.Remove(backup.path); err != nil && !errors.Is(err, os.ErrNotExist) {
			errs = append(errs, fmt.Errorf("remove new config %s: %w", backup.path, err))
		}
	}
	return errors.Join(errs...)
}

func validateManagedUsers(users []api.User) error {
	ids := make(map[int]struct{}, len(users))
	uuids := make(map[string]struct{}, len(users))
	for i, u := range users {
		if u.ID <= 0 {
			return fmt.Errorf("user[%d].id must be > 0, got %d", i, u.ID)
		}
		if _, ok := ids[u.ID]; ok {
			return fmt.Errorf("duplicate user id: %d", u.ID)
		}
		ids[u.ID] = struct{}{}

		if err := security.ValidateUUID(u.UUID); err != nil {
			return fmt.Errorf("user[%d].uuid invalid: %w", i, err)
		}
		if _, ok := uuids[u.UUID]; ok {
			return fmt.Errorf("duplicate user uuid: %s", u.UUID)
		}
		uuids[u.UUID] = struct{}{}
	}
	return nil
}

func intPtrValue(v *int) int {
	if v == nil {
		return 0
	}
	return *v
}

func isRealityProtocol(protoName string) bool {
	return protoName == "vless_reality_vision" ||
		protoName == "vless_reality_grpc" ||
		protoName == "vless_reality_xhttp"
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

	var errs []error

	if err := l.coreManager.EnsureRuntimeBaseConfigs(); err != nil {
		l.logger.WithError(err).Warn("基础运行配置生成失败，跳过核心重载")
		return fmt.Errorf("ensure runtime base configs: %w", err)
	}

	if hasXray {
		if err := l.coreManager.TestXrayConfig(); err != nil {
			l.logger.WithError(err).Warn("Xray 配置预检失败，跳过重载")
			errs = append(errs, fmt.Errorf("xray config test: %w", err))
		} else if err := l.coreManager.ReloadXray(); err != nil {
			l.logger.WithError(err).Warn("Xray 热重载失败")
			if restartErr := l.coreManager.RestartXray(); restartErr != nil {
				l.logger.WithError(restartErr).Warn("Xray 重启失败")
				errs = append(errs, fmt.Errorf("xray reload: %w; restart: %w", err, restartErr))
			} else {
				l.logger.Info("Xray 重启成功")
			}
		} else {
			l.logger.Info("Xray 热重载成功")
		}
	}

	if hasSingbox {
		if err := l.coreManager.RestartSingBox(); err != nil {
			l.logger.WithError(err).Warn("sing-box 重启失败")
			errs = append(errs, fmt.Errorf("sing-box restart: %w", err))
		} else {
			l.logger.Info("sing-box 重启成功")
		}
	}

	return errors.Join(errs...)
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
	// 初始化静态信息（仅首次生效，内部使用 sync.Once）
	sysinfo.InitStaticInfo()

	// 0. 从 Xray Stats API 采集流量并累加到 trafficCounter
	l.collectXrayTraffic()

	// 1. 流量上报
	snapshot := l.trafficCounter.Snapshot()
	if len(snapshot) > 0 {
		if err := l.apiClient.PushTraffic(ctx, snapshot); err != nil {
			// 上报失败，回滚流量数据
			l.trafficCounter.Merge(snapshot)
			l.logger.WithError(err).Warn("流量上报失败，已回滚")
		}
	}

	// 2. 在线用户上报
	aliveSnapshot := l.aliveTracker.Snapshot()
	if len(aliveSnapshot) > 0 {
		if err := l.apiClient.PushAlive(ctx, aliveSnapshot); err != nil {
			l.logger.WithError(err).Warn("在线用户上报失败")
		}
	}

	// 3. 节点状态上报（受监控开关控制）
	if l.config.MonitoringEnabled {
		status, err := sysinfo.CollectStatus()
		if err != nil {
			l.logger.WithError(err).Warn("采集节点状态失败")
		} else {
			if err := l.apiClient.PushStatus(ctx, status); err != nil {
				l.logger.WithError(err).Warn("节点状态上报失败")
			}
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
