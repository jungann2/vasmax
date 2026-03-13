package menu

import (
	"fmt"

	"github.com/sirupsen/logrus"

	"vasmax/internal/config"
)

// ALPNMenu handles ALPN switching for TLS protocols.
type ALPNMenu struct {
	config *config.Config
	logger *logrus.Logger
}

// NewALPNMenu creates a new ALPN menu.
func NewALPNMenu(cfg *config.Config, logger *logrus.Logger) *ALPNMenu {
	return &ALPNMenu{config: cfg, logger: logger}
}

// Show displays the ALPN switching menu.
func (m *ALPNMenu) Show() {
	for {
		PrintTitle("ALPN 切换")
		PrintInfo(fmt.Sprintf("当前模式: %s", m.currentModeLabel()))
		PrintInfo("ALPN 影响 TLS 握手协议协商，修改后需重启核心生效")
		PrintSeparator()
		PrintOption(1, "h2 + http/1.1（默认，兼容性最好）")
		PrintOption(2, "仅 h2（HTTP/2 专用，性能更好）")
		PrintOption(3, "仅 http/1.1（旧客户端兼容）")
		PrintOption(4, "仅 h3（QUIC 协议专用，如 Hysteria2/TUIC）")
		PrintOption(5, "h2 + http/1.1 + h3（全开，客户端自动协商最优）")
		PrintInfo("")
		PrintInfo("注: ALPN 在 TLS 握手时由客户端从服务端列表中选择，")
		PrintInfo("    h3 仅对 QUIC 类协议有效，TCP 协议写入 h3 无实际作用")
		PrintOptionStr("0", "返回上级菜单")

		choice := ReadChoice("请选择", []string{"1", "2", "3", "4", "5"})
		switch choice {
		case "1":
			m.setMode("h2_http11")
		case "2":
			m.setMode("h2_only")
		case "3":
			m.setMode("http11_only")
		case "4":
			m.setMode("h3_only")
		case "5":
			m.setMode("all")
		case "0":
			return
		}
	}
}

func (m *ALPNMenu) setMode(mode string) {
	if m.config.ALPN.Mode == mode {
		PrintInfo("已是当前模式，无需修改")
		return
	}
	m.config.ALPN.Mode = mode
	if err := config.SaveConfig(config.DefaultConfigPath, m.config); err != nil {
		PrintError(fmt.Sprintf("保存配置失败: %v", err))
		return
	}
	PrintSuccess(fmt.Sprintf("ALPN 已切换为: %s", m.currentModeLabel()))
	PrintInfo("请前往「核心管理」重启核心使配置生效")
}

func (m *ALPNMenu) currentModeLabel() string {
	switch m.config.ALPN.Mode {
	case "h2_only":
		return "仅 h2"
	case "http11_only":
		return "仅 http/1.1"
	case "h3_only":
		return "仅 h3"
	case "all":
		return "全部支持（h2 + http/1.1 + h3）"
	default:
		return "h2 + http/1.1（默认）"
	}
}

// ALPNList 根据配置返回当前 ALPN 列表（供协议生成时使用）
func ALPNList(cfg *config.Config) []string {
	switch cfg.ALPN.Mode {
	case "h2_only":
		return []string{"h2"}
	case "http11_only":
		return []string{"http/1.1"}
	case "h3_only":
		return []string{"h3"}
	case "all":
		return []string{"h2", "http/1.1", "h3"}
	default:
		return []string{"h2", "http/1.1"}
	}
}
