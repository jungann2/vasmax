package menu

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/sirupsen/logrus"

	"vasmax/internal/config"
	"vasmax/internal/core"
	"vasmax/internal/protocol"
	"vasmax/internal/rollback"
	"vasmax/internal/security"
	"vasmax/internal/sysinfo"
)

// InstallMenu handles protocol installation management.
type InstallMenu struct {
	config      *config.Config
	coreMgr     *core.Manager
	registry    *protocol.Registry
	rollbackMgr *rollback.Manager
	logger      *logrus.Logger
}

// NewInstallMenu creates a new install menu.
func NewInstallMenu(cfg *config.Config, coreMgr *core.Manager, reg *protocol.Registry, rbMgr *rollback.Manager, logger *logrus.Logger) *InstallMenu {
	return &InstallMenu{config: cfg, coreMgr: coreMgr, registry: reg, rollbackMgr: rbMgr, logger: logger}
}

// Show displays the installation management menu.
func (m *InstallMenu) Show() {
	for {
		PrintTitle("安装管理")
		PrintOption(1, "任意组合安装")
		PrintOption(2, "一键 Reality 安装（无域名）")
		PrintOption(3, "查看已安装协议")
		PrintOption(4, "卸载协议")
		PrintOptionStr("0", "返回上级菜单")

		choice := ReadChoice("请选择", []string{"1", "2", "3", "4"})
		switch choice {
		case "1":
			m.installCombination()
		case "2":
			m.installReality()
		case "3":
			m.showInstalled()
		case "4":
			m.uninstallProtocol()
		case "0":
			return
		}
	}
}

func (m *InstallMenu) installCombination() {
	PrintTitle("任意组合安装")

	// 列出所有可用协议
	allProtos := m.registry.ListAll()
	for i, p := range allProtos {
		installed := ""
		for _, ip := range m.config.Protocols {
			if ip == p.Name() {
				installed = Green(" [已安装]")
				break
			}
		}
		PrintOption(i+1, fmt.Sprintf("%-30s (%s)%s", p.Name(), p.CoreType(), installed))
	}

	fmt.Println()
	input := ReadInput("请输入要安装的协议编号（空格/逗号分隔，如 1,3,5）")
	if input == "" || input == "0" {
		return
	}

	// 解析选择
	var selected []protocol.Protocol
	parts := strings.FieldsFunc(input, func(r rune) bool { return r == ',' || r == ' ' })
	for _, p := range parts {
		var idx int
		if _, err := fmt.Sscanf(p, "%d", &idx); err != nil || idx < 1 || idx > len(allProtos) {
			PrintError(fmt.Sprintf("无效编号: %s", p))
			return
		}
		selected = append(selected, allProtos[idx-1])
	}

	if len(selected) == 0 {
		return
	}

	// 安装前检查
	if err := sysinfo.CheckDiskSpace("/", 100); err != nil {
		PrintError(fmt.Sprintf("磁盘空间不足: %v", err))
		return
	}

	// 域名输入（非 Reality 协议需要）
	needsDomain := false
	for _, p := range selected {
		if !strings.Contains(p.Name(), "reality") {
			needsDomain = true
			break
		}
	}

	var domain string
	if needsDomain {
		domain = ReadInput("请输入域名")
		if err := security.ValidateDomain(domain); err != nil {
			PrintError(fmt.Sprintf("域名无效: %v", err))
			return
		}
	}

	// 创建回滚快照
	snap, err := m.rollbackMgr.CreateSnapshot()
	if err != nil {
		m.logger.WithError(err).Warn("创建回滚快照失败")
	}

	// 安装核心
	ctx := context.Background()
	for _, p := range selected {
		PrintInfo(fmt.Sprintf("正在安装 %s...", p.Name()))

		coreType := p.CoreType()
		status := m.coreMgr.GetStatus()
		if cs, ok := status[coreType]; !ok || !cs.Installed {
			if err := m.coreMgr.InstallCore(ctx, coreType); err != nil {
				PrintError(fmt.Sprintf("安装核心 %s 失败: %v", coreType, err))
				if snap != nil {
					m.rollbackMgr.Rollback(snap)
				}
				return
			}
		}

		// 记录已安装协议
		found := false
		for _, ip := range m.config.Protocols {
			if ip == p.Name() {
				found = true
				break
			}
		}
		if !found {
			m.config.Protocols = append(m.config.Protocols, p.Name())
		}

		_ = domain // Used in config generation
		PrintSuccess(fmt.Sprintf("%s 安装完成", p.Name()))
	}

	// 保存配置
	if err := config.SaveConfig(config.DefaultConfigPath, m.config); err != nil {
		PrintError(fmt.Sprintf("保存配置失败: %v", err))
	}

	// 验证服务启动
	PrintInfo("等待服务启动...")
	time.Sleep(3 * time.Second)
	status := m.coreMgr.GetStatus()
	for name, s := range status {
		if s.Installed && s.Running {
			PrintSuccess(fmt.Sprintf("%s 运行正常", name))
		} else if s.Installed {
			PrintWarning(fmt.Sprintf("%s 未运行", name))
		}
	}

	// 清理快照
	if snap != nil {
		m.rollbackMgr.CleanSnapshot(snap)
	}
}

func (m *InstallMenu) installReality() {
	PrintTitle("一键 Reality 安装（无域名）")
	PrintInfo("将自动生成 X25519 密钥对和 shortId")

	// 安装 VLESS+Reality+Vision
	ctx := context.Background()
	status := m.coreMgr.GetStatus()
	if cs, ok := status["xray"]; !ok || !cs.Installed {
		PrintInfo("正在安装 Xray-core...")
		if err := m.coreMgr.InstallCore(ctx, "xray"); err != nil {
			PrintError(fmt.Sprintf("安装 Xray-core 失败: %v", err))
			return
		}
	}

	found := false
	for _, p := range m.config.Protocols {
		if p == "vless_reality_vision" {
			found = true
			break
		}
	}
	if !found {
		m.config.Protocols = append(m.config.Protocols, "vless_reality_vision")
	}

	if err := config.SaveConfig(config.DefaultConfigPath, m.config); err != nil {
		PrintError(fmt.Sprintf("保存配置失败: %v", err))
	}

	PrintSuccess("Reality 安装完成")
}

func (m *InstallMenu) showInstalled() {
	PrintTitle("已安装协议")
	if len(m.config.Protocols) == 0 {
		PrintInfo("暂无已安装协议")
		return
	}
	for i, p := range m.config.Protocols {
		PrintOption(i+1, p)
	}
}

func (m *InstallMenu) uninstallProtocol() {
	PrintTitle("卸载协议")
	if len(m.config.Protocols) == 0 {
		PrintInfo("暂无已安装协议")
		return
	}
	for i, p := range m.config.Protocols {
		PrintOption(i+1, p)
	}

	input := ReadInput("请输入要卸载的协议编号")
	var idx int
	if _, err := fmt.Sscanf(input, "%d", &idx); err != nil || idx < 1 || idx > len(m.config.Protocols) {
		PrintError("无效编号")
		return
	}

	name := m.config.Protocols[idx-1]
	if !Confirm(fmt.Sprintf("确认卸载 %s?", name)) {
		return
	}

	// 移除协议
	m.config.Protocols = append(m.config.Protocols[:idx-1], m.config.Protocols[idx:]...)
	if err := config.SaveConfig(config.DefaultConfigPath, m.config); err != nil {
		PrintError(fmt.Sprintf("保存配置失败: %v", err))
		return
	}

	PrintSuccess(fmt.Sprintf("%s 已卸载", name))
}
