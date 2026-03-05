package user

import (
	"fmt"
	"sync/atomic"
	"vasmax/internal/api"
	"vasmax/internal/security"
)

// UserEntry 用户条目
type UserEntry struct {
	ID          int
	UUID        string
	Email       string // 格式 "user_{id}" 或自定义
	SpeedLimit  int    // Mbps, 0=不限
	DeviceLimit int    // 0=不限
}

// ToAPIUser 将 UserEntry 转换为 api.User
func (e *UserEntry) ToAPIUser() *api.User {
	u := &api.User{
		ID:   e.ID,
		UUID: e.UUID,
	}
	if e.SpeedLimit > 0 {
		sl := e.SpeedLimit
		u.SpeedLimit = &sl
	}
	if e.DeviceLimit > 0 {
		dl := e.DeviceLimit
		u.DeviceLimit = &dl
	}
	return u
}

// UserTable 用户表（不可变，整体替换）
type UserTable struct {
	byID    map[int]*UserEntry
	byUUID  map[string]*UserEntry
	entries []*UserEntry
}

// Manager 用户管理器
type Manager struct {
	users    atomic.Value // *UserTable（原子替换，无锁读取）
	localSeq int          // 本地用户自增 ID（负数，避免与 API 用户冲突）
}

// NewManager 创建用户管理器
func NewManager() *Manager {
	m := &Manager{}
	m.users.Store(&UserTable{
		byID:    make(map[int]*UserEntry),
		byUUID:  make(map[string]*UserEntry),
		entries: make([]*UserEntry, 0),
	})
	return m
}

// UpdateUsers 原子替换用户表（托管模式，从 API 用户列表）
func (m *Manager) UpdateUsers(users []api.User) {
	table := &UserTable{
		byID:    make(map[int]*UserEntry, len(users)),
		byUUID:  make(map[string]*UserEntry, len(users)),
		entries: make([]*UserEntry, 0, len(users)),
	}

	for _, u := range users {
		entry := &UserEntry{
			ID:    u.ID,
			UUID:  u.UUID,
			Email: fmt.Sprintf("user_%d", u.ID),
		}
		if u.SpeedLimit != nil {
			entry.SpeedLimit = *u.SpeedLimit
		}
		if u.DeviceLimit != nil {
			entry.DeviceLimit = *u.DeviceLimit
		}
		table.byID[u.ID] = entry
		table.byUUID[u.UUID] = entry
		table.entries = append(table.entries, entry)
	}

	m.users.Store(table)
}

// AddLocalUser 添加本地用户（独立模式）
func (m *Manager) AddLocalUser(uuid, email string) error {
	if err := security.ValidateUUID(uuid); err != nil {
		return fmt.Errorf("无效 UUID: %w", err)
	}

	old := m.users.Load().(*UserTable)

	// 检查 UUID 是否已存在
	if _, exists := old.byUUID[uuid]; exists {
		return fmt.Errorf("UUID 已存在: %s", uuid)
	}

	// 复制旧表并添加新用户
	m.localSeq--
	entry := &UserEntry{
		ID:    m.localSeq,
		UUID:  uuid,
		Email: email,
	}

	table := &UserTable{
		byID:    make(map[int]*UserEntry, len(old.byID)+1),
		byUUID:  make(map[string]*UserEntry, len(old.byUUID)+1),
		entries: make([]*UserEntry, 0, len(old.entries)+1),
	}
	for k, v := range old.byID {
		table.byID[k] = v
	}
	for k, v := range old.byUUID {
		table.byUUID[k] = v
	}
	table.entries = append(table.entries, old.entries...)

	table.byID[entry.ID] = entry
	table.byUUID[entry.UUID] = entry
	table.entries = append(table.entries, entry)

	m.users.Store(table)
	return nil
}

// RemoveLocalUser 删除本地用户（独立模式）
func (m *Manager) RemoveLocalUser(uuid string) error {
	old := m.users.Load().(*UserTable)

	entry, exists := old.byUUID[uuid]
	if !exists {
		return fmt.Errorf("用户不存在: %s", uuid)
	}

	table := &UserTable{
		byID:    make(map[int]*UserEntry, len(old.byID)-1),
		byUUID:  make(map[string]*UserEntry, len(old.byUUID)-1),
		entries: make([]*UserEntry, 0, len(old.entries)-1),
	}
	for k, v := range old.byID {
		if k != entry.ID {
			table.byID[k] = v
		}
	}
	for k, v := range old.byUUID {
		if k != uuid {
			table.byUUID[k] = v
		}
	}
	for _, e := range old.entries {
		if e.UUID != uuid {
			table.entries = append(table.entries, e)
		}
	}

	m.users.Store(table)
	return nil
}

// GetUser 根据 ID 获取用户
func (m *Manager) GetUser(id int) *UserEntry {
	table := m.users.Load().(*UserTable)
	return table.byID[id]
}

// GetUserByUUID 根据 UUID 获取用户
func (m *Manager) GetUserByUUID(uuid string) *UserEntry {
	table := m.users.Load().(*UserTable)
	return table.byUUID[uuid]
}

// GetAllUsers 获取所有用户列表
func (m *Manager) GetAllUsers() []*UserEntry {
	table := m.users.Load().(*UserTable)
	return table.entries
}

// Count 获取用户数量
func (m *Manager) Count() int {
	table := m.users.Load().(*UserTable)
	return len(table.entries)
}
