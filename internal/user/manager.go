package user

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
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
	users     atomic.Value // *UserTable（原子替换，无锁读取）
	localSeq  int          // 本地用户自增 ID（负数，避免与 API 用户冲突）
	localFile string       // 本地用户持久化文件路径
}

// DefaultLocalUsersFile 默认本地用户文件路径
const DefaultLocalUsersFile = "/etc/vasmax/local_users.json"

// localUserJSON 本地用户 JSON 序列化结构
type localUserJSON struct {
	UUID        string `json:"uuid"`
	Email       string `json:"email"`
	SpeedLimit  int    `json:"speed_limit,omitempty"`
	DeviceLimit int    `json:"device_limit,omitempty"`
}

// NewManager 创建用户管理器并自动加载本地用户
func NewManager() *Manager {
	m := &Manager{localFile: DefaultLocalUsersFile}
	m.users.Store(&UserTable{
		byID:    make(map[int]*UserEntry),
		byUUID:  make(map[string]*UserEntry),
		entries: make([]*UserEntry, 0),
	})
	// 自动加载持久化的本地用户
	_ = m.loadLocalUsers()
	return m
}

// loadLocalUsers 从文件加载本地用户
func (m *Manager) loadLocalUsers() error {
	data, err := os.ReadFile(m.localFile)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	var users []localUserJSON
	if err := json.Unmarshal(data, &users); err != nil {
		return err
	}
	for _, u := range users {
		m.localSeq--
		entry := &UserEntry{
			ID:          m.localSeq,
			UUID:        u.UUID,
			Email:       u.Email,
			SpeedLimit:  u.SpeedLimit,
			DeviceLimit: u.DeviceLimit,
		}
		old := m.users.Load().(*UserTable)
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
	}
	return nil
}

func (m *Manager) saveLocalUsersTable(table *UserTable) error {
	var users []localUserJSON
	for _, e := range table.entries {
		if e.ID < 0 { // 只保存本地用户（ID 为负数）
			users = append(users, localUserJSON{
				UUID:        e.UUID,
				Email:       e.Email,
				SpeedLimit:  e.SpeedLimit,
				DeviceLimit: e.DeviceLimit,
			})
		}
	}
	data, err := json.MarshalIndent(users, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(m.localFile), 0700); err != nil {
		return err
	}
	return security.AtomicWrite(m.localFile, data, 0600)
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

	// 复制旧表并添加新用户。先持久化候选表，成功后再切换内存状态。
	newID := m.localSeq - 1
	entry := &UserEntry{
		ID:    newID,
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

	if err := m.saveLocalUsersTable(table); err != nil {
		return err
	}
	m.localSeq = newID
	m.users.Store(table)
	return nil
}

// UpdateLocalUser 更新本地用户的速率/设备限制（独立模式）
// speedLimit: Mbps，0 表示不限；deviceLimit: 设备数，0 表示不限
func (m *Manager) UpdateLocalUser(uuid string, speedLimit, deviceLimit int) error {
	old := m.users.Load().(*UserTable)
	entry, exists := old.byUUID[uuid]
	if !exists {
		return fmt.Errorf("用户不存在: %s", uuid)
	}
	if entry.ID >= 0 {
		return fmt.Errorf("托管模式用户不可在本地编辑")
	}

	// 构建新表（copy-on-write）
	table := &UserTable{
		byID:    make(map[int]*UserEntry, len(old.byID)),
		byUUID:  make(map[string]*UserEntry, len(old.byUUID)),
		entries: make([]*UserEntry, 0, len(old.entries)),
	}
	for k, v := range old.byID {
		table.byID[k] = v
	}
	for k, v := range old.byUUID {
		table.byUUID[k] = v
	}
	table.entries = append(table.entries, old.entries...)

	// 更新目标用户（替换指针）
	updated := &UserEntry{
		ID:          entry.ID,
		UUID:        entry.UUID,
		Email:       entry.Email,
		SpeedLimit:  speedLimit,
		DeviceLimit: deviceLimit,
	}
	table.byID[updated.ID] = updated
	table.byUUID[updated.UUID] = updated
	for i, e := range table.entries {
		if e.UUID == uuid {
			table.entries[i] = updated
			break
		}
	}

	if err := m.saveLocalUsersTable(table); err != nil {
		return err
	}
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

	if err := m.saveLocalUsersTable(table); err != nil {
		return err
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

// RecoverFromXrayConfigs 从 Xray 入站配置文件中恢复用户（迁移用）
// 当 local_users.json 不存在但 Xray 配置中有用户时，自动恢复
func (m *Manager) RecoverFromXrayConfigs(xrayConfDir string) error {
	if m.Count() > 0 {
		return nil // 已有用户，无需恢复
	}

	entries, err := os.ReadDir(xrayConfDir)
	if err != nil {
		return err
	}

	recovered := 0
	for _, entry := range entries {
		if entry.IsDir() || !isInboundFile(entry.Name()) {
			continue
		}
		data, err := os.ReadFile(filepath.Join(xrayConfDir, entry.Name()))
		if err != nil {
			continue
		}
		uuids := extractUUIDs(data)
		for _, uuid := range uuids {
			if m.GetUserByUUID(uuid) != nil {
				continue
			}
			short := uuid
			if len(short) > 8 {
				short = short[:8]
			}
			email := fmt.Sprintf("user_%s", short)
			_ = m.AddLocalUser(uuid, email)
			recovered++
		}
	}
	return nil
}

func isInboundFile(name string) bool {
	return len(name) > 5 && name[len(name)-5:] == ".json"
}

// extractUUIDs 从 Xray 配置 JSON 中提取所有用户 UUID
func extractUUIDs(data []byte) []string {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil
	}
	inboundsRaw, ok := raw["inbounds"]
	if !ok {
		return nil
	}
	var inbounds []json.RawMessage
	if err := json.Unmarshal(inboundsRaw, &inbounds); err != nil {
		return nil
	}
	var uuids []string
	for _, ib := range inbounds {
		var inbound struct {
			Settings struct {
				Clients []struct {
					ID string `json:"id"`
				} `json:"clients"`
			} `json:"settings"`
		}
		if err := json.Unmarshal(ib, &inbound); err != nil {
			continue
		}
		for _, c := range inbound.Settings.Clients {
			if c.ID != "" {
				uuids = append(uuids, c.ID)
			}
		}
	}
	return uuids
}
