package menu

import (
	"fmt"
	"path/filepath"

	"github.com/sirupsen/logrus"

	"vasmax/internal/config"
	"vasmax/internal/subscription"
	"vasmax/internal/user"
)

// SubscriptionMenu handles subscription management.
type SubscriptionMenu struct {
	config  *config.Config
	subMgr  *subscription.Manager
	userMgr *user.Manager
	logger  *logrus.Logger
}

// NewSubscriptionMenu creates a new subscription menu.
func NewSubscriptionMenu(cfg *config.Config, subMgr *subscription.Manager, userMgr *user.Manager, logger *logrus.Logger) *SubscriptionMenu {
	return &SubscriptionMenu{config: cfg, subMgr: subMgr, userMgr: userMgr, logger: logger}
}

// Show displays the subscription management menu.
func (m *SubscriptionMenu) Show() {
	for {
		PrintTitle("订阅管理")
		subDomain := m.config.Subscription.Domain
		if subDomain == "" {
			subDomain = m.config.TLS.Domain
		}
		if subDomain != "" {
			PrintInfo(fmt.Sprintf("订阅域名: %s", subDomain))
		} else {
			PrintWarning("未配置订阅域名")
		}
		PrintSeparator()
		PrintOption(1, "查看订阅链接")
		PrintOption(2, "重新生成订阅")
		PrintOption(3, "设置订阅域名")
		PrintOption(4, "远程订阅管理")
		PrintOptionStr("0", "返回上级菜单")

		choice := ReadChoice("请选择", []string{"1", "2", "3", "4"})
		switch choice {
		case "1":
			m.showLinks()
		case "2":
			m.regenerate()
		case "3":
			m.setDomain()
		case "4":
			m.remoteSubMenu()
		case "0":
			return
		}
	}
}

func (m *SubscriptionMenu) showLinks() {
	PrintTitle("订阅链接")
	subDomain := m.config.Subscription.Domain
	if subDomain == "" {
		subDomain = m.config.TLS.Domain
	}
	if subDomain == "" {
		PrintWarning("未配置订阅域名，无法生成链接")
		return
	}

	users := m.userMgr.GetAllUsers()
	if len(users) == 0 {
		PrintInfo("暂无用户")
		return
	}

	for _, u := range users {
		emailMd5 := subscription.GenerateSubscribePath(u.Email, m.config.Subscription.Salt)
		base := fmt.Sprintf("https://%s/s/%s", subDomain, emailMd5)
		PrintInfo(fmt.Sprintf("用户: %s", u.Email))
		PrintInfo(fmt.Sprintf("  通用:    %s/default", base))
		PrintInfo(fmt.Sprintf("  Clash:   %s/clash", base))
		PrintInfo(fmt.Sprintf("  SingBox: %s/singbox", base))
		fmt.Println()
	}
}

func (m *SubscriptionMenu) regenerate() {
	PrintInfo("正在重新生成订阅...")
	if err := m.subMgr.GenerateAll(); err != nil {
		PrintError(fmt.Sprintf("生成失败: %v", err))
		return
	}
	PrintSuccess("订阅已重新生成")
}

func (m *SubscriptionMenu) setDomain() {
	PrintTitle("设置订阅域名")
	PrintInfo(fmt.Sprintf("当前域名: %s", m.config.Subscription.Domain))
	PrintSuccess("  直接回车取消")
	domain := ReadInput("请输入新的订阅域名")
	if domain == "" {
		return
	}
	m.config.Subscription.Domain = domain
	if err := config.SaveConfig(config.DefaultConfigPath, m.config); err != nil {
		PrintError(fmt.Sprintf("保存失败: %v", err))
		return
	}
	PrintSuccess(fmt.Sprintf("订阅域名已设置为: %s", domain))
}

func (m *SubscriptionMenu) remoteSubMenu() {
	baseDir := filepath.Dir(m.config.Paths.Subscribe)
	for {
		PrintTitle("远程订阅管理")
		subs, _ := subscription.LoadRemoteSubscriptions(baseDir)
		if len(subs) == 0 {
			PrintInfo("暂无远程订阅")
		} else {
			for i, s := range subs {
				PrintOption(i+1, fmt.Sprintf("%-20s  %s:%d", s.Alias, s.Domain, s.Port))
			}
		}
		PrintSeparator()
		PrintOption(len(subs)+1, "添加远程订阅")
		PrintOption(len(subs)+2, "删除远程订阅")
		PrintOptionStr("0", "返回")

		choices := []string{"0"}
		for i := 1; i <= len(subs)+2; i++ {
			choices = append(choices, fmt.Sprintf("%d", i))
		}
		choice := ReadChoice("请选择", choices)
		switch {
		case choice == "0":
			return
		case choice == fmt.Sprintf("%d", len(subs)+1):
			input := ReadInput("请输入格式: 域名:端口:别名")
			if input == "" {
				continue
			}
			sub, err := subscription.ParseRemoteSubscription(input)
			if err != nil {
				PrintError(fmt.Sprintf("格式错误: %v", err))
				continue
			}
			subs = append(subs, *sub)
			if err := subscription.SaveRemoteSubscriptions(baseDir, subs); err != nil {
				PrintError(fmt.Sprintf("保存失败: %v", err))
			} else {
				PrintSuccess(fmt.Sprintf("已添加: %s", sub.Alias))
			}
		case choice == fmt.Sprintf("%d", len(subs)+2):
			if len(subs) == 0 {
				PrintInfo("暂无远程订阅")
				continue
			}
			idx := ReadInput("请输入要删除的编号")
			var n int
			if _, err := fmt.Sscanf(idx, "%d", &n); err != nil || n < 1 || n > len(subs) {
				PrintError("无效编号")
				continue
			}
			alias := subs[n-1].Alias
			subs = append(subs[:n-1], subs[n:]...)
			if err := subscription.SaveRemoteSubscriptions(baseDir, subs); err != nil {
				PrintError(fmt.Sprintf("保存失败: %v", err))
			} else {
				PrintSuccess(fmt.Sprintf("已删除: %s", alias))
			}
		}
	}
}
