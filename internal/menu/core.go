package menu

import (
	"context"
	"fmt"

	"github.com/sirupsen/logrus"

	"vasmax/internal/core"
)

// CoreMenu handles core management operations.
type CoreMenu struct {
	coreMgr *core.Manager
	logger  *logrus.Logger
}

// NewCoreMenu creates a new core management menu.
func NewCoreMenu(coreMgr *core.Manager, logger *logrus.Logger) *CoreMenu {
	return &CoreMenu{coreMgr: coreMgr, logger: logger}
}

// Show displays the core management menu.
func (m *CoreMenu) Show() {
	for {
		PrintTitle("核心管理")

		status := m.coreMgr.GetStatus()
		for name, s := range status {
			if s.Installed {
				state := Red("已停止")
				if s.Running {
					state = Green("运行中")
				}
				PrintInfo(fmt.Sprintf("%s: %s v%s", name, state, s.Version))
			}
		}
		PrintSeparator()

		PrintOption(1, "更新 Xray-core")
		PrintOption(2, "更新 sing-box")
		PrintOption(3, "回滚 Xray-core")
		PrintOption(4, "回滚 sing-box")
		PrintOption(5, "更新 GeoData")
		PrintOption(6, "重启所有核心")
		PrintOption(7, "停止所有核心")
		PrintOption(8, "自更新 VasmaX")
		PrintOptionStr("0", "返回上级菜单")

		choice := ReadChoice("请选择", []string{"1", "2", "3", "4", "5", "6", "7", "8"})
		switch choice {
		case "1":
			m.updateCore("xray")
		case "2":
			m.updateCore("singbox")
		case "3":
			m.rollbackCore("xray")
		case "4":
			m.rollbackCore("singbox")
		case "5":
			m.updateGeoData()
		case "6":
			if err := m.coreMgr.StartAll(); err != nil {
				PrintError(fmt.Sprintf("启动失败: %v", err))
			} else {
				PrintSuccess("所有核心已启动")
			}
		case "7":
			if err := m.coreMgr.StopAll(); err != nil {
				PrintError(fmt.Sprintf("停止失败: %v", err))
			} else {
				PrintSuccess("所有核心已停止")
			}
		case "8":
			PrintInfo("自更新功能 - TODO")
		case "0":
			return
		}
	}
}

func (m *CoreMenu) updateCore(coreType string) {
	if !Confirm(fmt.Sprintf("确认更新 %s?", coreType)) {
		return
	}
	PrintInfo(fmt.Sprintf("正在更新 %s...", coreType))
	ctx := context.Background()
	if err := m.coreMgr.UpdateCore(ctx, coreType); err != nil {
		PrintError(fmt.Sprintf("更新失败: %v", err))
	} else {
		PrintSuccess(fmt.Sprintf("%s 更新完成", coreType))
	}
}

func (m *CoreMenu) rollbackCore(coreType string) {
	if !Confirm(fmt.Sprintf("确认回滚 %s?", coreType)) {
		return
	}
	if err := m.coreMgr.RollbackCore(coreType); err != nil {
		PrintError(fmt.Sprintf("回滚失败: %v", err))
	} else {
		PrintSuccess(fmt.Sprintf("%s 已回滚", coreType))
	}
}

func (m *CoreMenu) updateGeoData() {
	PrintInfo("正在更新 GeoData...")
	ctx := context.Background()
	if err := m.coreMgr.UpdateGeoData(ctx); err != nil {
		PrintError(fmt.Sprintf("更新失败: %v", err))
	} else {
		PrintSuccess("GeoData 更新完成")
	}
}
