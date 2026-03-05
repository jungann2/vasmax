package menu

import (
	"crypto/rand"
	"fmt"

	"vasmax/internal/subscription"
	"vasmax/internal/user"
)

// AccountMenu handles user account management.
type AccountMenu struct {
	userMgr *user.Manager
	subMgr  *subscription.Manager
}

// NewAccountMenu creates a new account menu.
func NewAccountMenu(userMgr *user.Manager, subMgr *subscription.Manager) *AccountMenu {
	return &AccountMenu{userMgr: userMgr, subMgr: subMgr}
}

// Show displays the account management menu.
func (m *AccountMenu) Show() {
	for {
		PrintTitle("账号管理")
		PrintOption(1, "添加用户")
		PrintOption(2, "删除用户")
		PrintOption(3, "查看用户")
		PrintOptionStr("0", "返回上级菜单")

		choice := ReadChoice("请选择", []string{"1", "2", "3"})
		switch choice {
		case "1":
			m.addUser()
		case "2":
			m.removeUser()
		case "3":
			m.listUsers()
		case "0":
			return
		}
	}
}

func (m *AccountMenu) addUser() {
	PrintTitle("添加用户")

	// 生成或自定义 UUID
	uuid := ReadInput("请输入UUID（留空自动生成）")
	if uuid == "" {
		uuid = generateUUID()
	}

	email := ReadInput("请输入邮箱标识（留空使用默认）")
	if email == "" {
		email = fmt.Sprintf("user_local_%s", uuid[:8])
	}

	if err := m.userMgr.AddLocalUser(uuid, email); err != nil {
		PrintError(fmt.Sprintf("添加用户失败: %v", err))
		return
	}

	PrintSuccess(fmt.Sprintf("用户已添加: %s", uuid))

	// 重新生成订阅
	if m.subMgr != nil {
		if err := m.subMgr.GenerateAll(); err != nil {
			PrintWarning(fmt.Sprintf("重新生成订阅失败: %v", err))
		}
	}
}

func (m *AccountMenu) removeUser() {
	PrintTitle("删除用户")

	users := m.userMgr.GetAllUsers()
	if len(users) == 0 {
		PrintInfo("暂无用户")
		return
	}

	for i, u := range users {
		PrintOption(i+1, fmt.Sprintf("%s (%s)", u.Email, u.UUID))
	}

	input := ReadInput("请输入要删除的用户编号")
	var idx int
	if _, err := fmt.Sscanf(input, "%d", &idx); err != nil || idx < 1 || idx > len(users) {
		PrintError("无效编号")
		return
	}

	target := users[idx-1]
	if !Confirm(fmt.Sprintf("确认删除用户 %s?", target.Email)) {
		return
	}

	if err := m.userMgr.RemoveLocalUser(target.UUID); err != nil {
		PrintError(fmt.Sprintf("删除用户失败: %v", err))
		return
	}

	PrintSuccess(fmt.Sprintf("用户 %s 已删除", target.Email))
}

func (m *AccountMenu) listUsers() {
	PrintTitle("用户列表")

	users := m.userMgr.GetAllUsers()
	if len(users) == 0 {
		PrintInfo("暂无用户")
		return
	}

	for i, u := range users {
		speedInfo := "不限"
		if u.SpeedLimit > 0 {
			speedInfo = fmt.Sprintf("%d Mbps", u.SpeedLimit)
		}
		deviceInfo := "不限"
		if u.DeviceLimit > 0 {
			deviceInfo = fmt.Sprintf("%d", u.DeviceLimit)
		}
		PrintOption(i+1, fmt.Sprintf("%-20s UUID: %s  速率: %s  设备: %s",
			u.Email, u.UUID, speedInfo, deviceInfo))
	}
}

// generateUUID generates a random UUID v4.
func generateUUID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	b[6] = (b[6] & 0x0f) | 0x40 // Version 4
	b[8] = (b[8] & 0x3f) | 0x80 // Variant 10
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}
