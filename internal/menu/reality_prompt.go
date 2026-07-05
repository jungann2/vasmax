package menu

import (
	"fmt"
	"strings"
	"time"

	"vasmax/internal/config"
	"vasmax/internal/core"
	"vasmax/internal/security"
)

const realityProbeTimeout = 5 * time.Second

func printRealityTargetGuidance() {
	PrintInfo("Reality 伪装目标只填域名或 域名:端口，不要带 http:// 或 https://")
	PrintInfo(fmt.Sprintf("推荐目标: %s", strings.Join(security.RecommendedRealityDests(), " / ")))
	PrintWarning("不建议使用 Apple / iCloud / Microsoft / Bing 相关域名：容易出现握手异常或被重点识别")
}

func readRealityDestInput() (string, string, bool) {
	printRealityTargetGuidance()
	dest := strings.TrimSpace(ReadInput("请输入 Reality 伪装目标（直接回车取消，如 www.nvidia.com 或 www.nvidia.com:443）"))
	if dest == "" {
		return "", "", false
	}
	if strings.Contains(dest, "://") {
		PrintError("伪装目标不要带 http:// 或 https://")
		return "", "", false
	}
	if strings.ContainsAny(dest, "/ \t") {
		PrintError("伪装目标只填写域名或 域名:端口，不要填写路径、空格或其他内容")
		return "", "", false
	}
	normalizedDest, serverName, err := security.NormalizeRealityDest(dest)
	if err != nil {
		PrintError(fmt.Sprintf("伪装目标无效: %v", err))
		return "", "", false
	}
	printRealityDestWarnings(serverName)
	return normalizedDest, serverName, true
}

func printRealityTargetPoolDetails(cfg *config.Config) {
	PrintInfo(fmt.Sprintf("Reality Vision 目标池: %s", formatRealityTargetPool(cfg)))
	if cfg != nil && len(cfg.Reality.Targets) > 0 {
		PrintWarning("仅 vless_reality_vision 使用目标池；客户端订阅会生成多个 Reality Vision 节点，并额外生成 Reality智能 分组")
	}
}

func setSingleRealityTarget(cfg *config.Config, dest, serverName string) {
	cfg.Reality.Dest = dest
	cfg.Reality.ServerName = serverName
	cfg.Reality.Targets = nil
}

func resetDefaultRealityTargetPool(cfg *config.Config) {
	basePort := configuredProtocolPort(cfg, "vless_reality_vision", defaultProtocolPort("vless_reality_vision"))
	cfg.Reality.Targets = nil
	cfg.Reality.EnsureDefaultTargets(basePort)
}

func detectRealityTargetPool(cfg *config.Config) []security.RealityProbeResult {
	candidates := realityProbeCandidates(cfg)
	PrintInfo(fmt.Sprintf(
		"正在检测 %d 个 Reality 伪站候选（固定推荐 %d 个 + 扩展池随机最多 %d 个；每个目标最多等待 %s）...",
		len(candidates),
		len(security.RecommendedRealityDests()),
		security.DefaultRealityExtendedProbeSampleSize,
		realityProbeTimeout,
	))
	results := security.ProbeRealityDests(candidates, realityProbeTimeout)
	printRealityProbeResults(results)
	return results
}

func realityProbeCandidates(cfg *config.Config) []string {
	candidates := make([]string, 0, 16)
	if cfg != nil {
		if cfg.Reality.Dest != "" {
			candidates = append(candidates, cfg.Reality.Dest)
		} else if cfg.Reality.ServerName != "" {
			candidates = append(candidates, cfg.Reality.ServerName)
		}
		for _, target := range cfg.Reality.Targets {
			if target.Disabled {
				continue
			}
			if target.Dest != "" {
				candidates = append(candidates, target.Dest)
				continue
			}
			if target.ServerName != "" {
				candidates = append(candidates, target.ServerName)
			}
		}
	}
	candidates = append(candidates, security.RecommendedRealityDests()...)
	candidates = append(candidates, security.SampleExtendedRealityDests(security.DefaultRealityExtendedProbeSampleSize)...)
	return candidates
}

func printRealityProbeResults(results []security.RealityProbeResult) {
	if len(results) == 0 {
		PrintWarning("没有可检测的 Reality 伪站候选")
		return
	}
	PrintSeparator()
	for i, result := range results {
		PrintInfo(fmt.Sprintf(
			"%d. %s  %s  延时: %s  TLS1.3: %s  H2: %s",
			i+1,
			result.ServerName,
			realityProbeStatus(result),
			formatRealityProbeLatency(result.Latency),
			formatProbeBool(result.SupportsTLS13),
			formatProbeBool(result.SupportsH2),
		))
		if result.IsCloudflare {
			PrintWarning("   命中 Cloudflare CDN，不建议作为 Reality 伪站")
		}
		if result.Error != "" {
			PrintWarning(fmt.Sprintf("   错误: %s", result.Error))
		}
		for _, warning := range result.Warnings {
			PrintWarning(fmt.Sprintf("   提示: %s", warning))
		}
	}
	PrintSeparator()
}

func applyBestRealityTarget(coreMgr *core.Manager, cfg *config.Config, subMgr subscriptionRegenerator) (*security.RealityProbeResult, bool) {
	results := detectRealityTargetPool(cfg)
	best, ok := security.BestRealityProbeResult(results)
	if !ok {
		PrintError("未找到同时满足 TLS1.3、H2、非 Cloudflare、非高风险域名的可用伪站")
		PrintInfo("可以稍后重试，或手动输入你验证过的伪站")
		return nil, false
	}

	PrintSuccess(fmt.Sprintf("最佳匹配: %s（延时 %s）", best.ServerName, formatRealityProbeLatency(best.Latency)))
	if !Confirm(fmt.Sprintf("将 Reality 切换为单伪站 %s 并自动刷新订阅，确认?", best.Dest)) {
		return nil, false
	}

	if !applyAndSyncRealityRuntime(coreMgr, cfg, subMgr, func(next *config.Config) {
		setSingleRealityTarget(next, best.Dest, best.ServerName)
	}) {
		return nil, false
	}
	PrintSuccess(fmt.Sprintf("已切换 Reality 伪站: %s", best.ServerName))
	return best, true
}

func realityProbeStatus(result security.RealityProbeResult) string {
	if result.Available() {
		return "可用"
	}
	if result.Error != "" {
		return "失败"
	}
	return "不推荐"
}

func formatProbeBool(ok bool) string {
	if ok {
		return "是"
	}
	return "否"
}

func formatRealityProbeLatency(latency time.Duration) string {
	if latency <= 0 {
		return "-"
	}
	return fmt.Sprintf("%dms", latency.Milliseconds())
}

type subscriptionRegenerator interface {
	GenerateAll() error
}

func SyncRealityRuntime(coreMgr *core.Manager, cfg *config.Config) (bool, error) {
	return syncRealityRuntime(coreMgr, cfg)
}

func applyAndSyncRealityRuntime(coreMgr *core.Manager, cfg *config.Config, subMgr subscriptionRegenerator, mutate func(*config.Config)) bool {
	if cfg == nil {
		PrintError("配置为空，无法更新 Reality 参数")
		return false
	}
	oldCfg, err := cloneConfig(cfg)
	if err != nil {
		PrintError(fmt.Sprintf("创建 Reality 配置快照失败: %v", err))
		return false
	}
	nextCfg, err := cloneConfig(cfg)
	if err != nil {
		PrintError(fmt.Sprintf("复制 Reality 配置失败: %v", err))
		return false
	}
	if mutate != nil {
		mutate(nextCfg)
	}

	if changed, err := SyncRealityRuntime(coreMgr, nextCfg); err != nil {
		PrintWarning(fmt.Sprintf("同步 Xray Reality 入站配置失败: %v", err))
		PrintWarning("配置未保存，订阅未刷新，已保留旧 Reality 参数")
		return false
	} else if changed {
		PrintSuccess("已同步 Xray Reality 入站配置并重启 Xray")
	}

	*cfg = *nextCfg
	if err := config.SaveConfig(config.DefaultConfigPath, cfg); err != nil {
		PrintError(fmt.Sprintf("保存失败: %v", err))
		*cfg = *oldCfg
		rollbackRealityRuntime(coreMgr, cfg, nil)
		return false
	}

	if subMgr != nil {
		if err := subMgr.GenerateAll(); err != nil {
			PrintWarning(fmt.Sprintf("重新生成订阅失败: %v", err))
			PrintWarning("正在回滚 Reality 配置和运行配置，避免客户端与服务端不一致")
			*cfg = *oldCfg
			rollbackRealityRuntime(coreMgr, cfg, subMgr)
			return false
		}
		PrintSuccess("已重新生成订阅，客户端参数与当前 Reality 配置保持一致")
	}
	return true
}

func rollbackRealityRuntime(coreMgr *core.Manager, cfg *config.Config, subMgr subscriptionRegenerator) {
	if _, err := SyncRealityRuntime(coreMgr, cfg); err != nil {
		PrintWarning(fmt.Sprintf("回滚 Xray Reality 入站配置失败，请手动检查运行配置: %v", err))
	}
	if err := config.SaveConfig(config.DefaultConfigPath, cfg); err != nil {
		PrintWarning(fmt.Sprintf("恢复旧 Reality 配置文件失败: %v", err))
	}
	if subMgr != nil {
		if err := subMgr.GenerateAll(); err != nil {
			PrintWarning(fmt.Sprintf("恢复旧订阅失败，请手动重新生成订阅: %v", err))
		}
	}
}
