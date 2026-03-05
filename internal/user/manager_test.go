package user

import (
	"fmt"
	"sync"
	"testing"

	"vasmax/internal/api"
)

func TestNewManager(t *testing.T) {
	m := NewManager()
	if m.Count() != 0 {
		t.Errorf("新管理器应有 0 用户，实际 %d", m.Count())
	}
	if users := m.GetAllUsers(); len(users) != 0 {
		t.Errorf("新管理器 GetAllUsers 应返回空列表")
	}
}

func TestUpdateUsers(t *testing.T) {
	m := NewManager()
	speed := 100
	device := 3

	users := []api.User{
		{ID: 1, UUID: "550e8400-e29b-41d4-a716-446655440001", SpeedLimit: &speed, DeviceLimit: &device},
		{ID: 2, UUID: "550e8400-e29b-41d4-a716-446655440002", SpeedLimit: nil, DeviceLimit: nil},
	}
	m.UpdateUsers(users)

	if m.Count() != 2 {
		t.Fatalf("期望 2 用户，实际 %d", m.Count())
	}

	u1 := m.GetUser(1)
	if u1 == nil || u1.UUID != users[0].UUID {
		t.Error("GetUser(1) 失败")
	}
	if u1.SpeedLimit != 100 || u1.DeviceLimit != 3 {
		t.Errorf("限速/设备限制不正确: speed=%d, device=%d", u1.SpeedLimit, u1.DeviceLimit)
	}
	if u1.Email != "user_1" {
		t.Errorf("Email 格式不正确: %s", u1.Email)
	}

	u2 := m.GetUser(2)
	if u2 == nil || u2.SpeedLimit != 0 || u2.DeviceLimit != 0 {
		t.Error("nil 指针应转为 0")
	}

	// UUID 查询
	byUUID := m.GetUserByUUID(users[0].UUID)
	if byUUID == nil || byUUID.ID != 1 {
		t.Error("GetUserByUUID 失败")
	}

	// 不存在的用户
	if m.GetUser(999) != nil {
		t.Error("不存在的 ID 应返回 nil")
	}
	if m.GetUserByUUID("nonexistent") != nil {
		t.Error("不存在的 UUID 应返回 nil")
	}
}

func TestAddLocalUser(t *testing.T) {
	m := NewManager()

	err := m.AddLocalUser("550e8400-e29b-41d4-a716-446655440001", "alice@test.com")
	if err != nil {
		t.Fatalf("添加用户失败: %v", err)
	}
	if m.Count() != 1 {
		t.Fatalf("期望 1 用户，实际 %d", m.Count())
	}

	u := m.GetUserByUUID("550e8400-e29b-41d4-a716-446655440001")
	if u == nil || u.Email != "alice@test.com" {
		t.Error("添加的用户信息不正确")
	}

	// 重复 UUID
	err = m.AddLocalUser("550e8400-e29b-41d4-a716-446655440001", "bob@test.com")
	if err == nil {
		t.Error("重复 UUID 应返回错误")
	}

	// 无效 UUID
	err = m.AddLocalUser("invalid-uuid", "test@test.com")
	if err == nil {
		t.Error("无效 UUID 应返回错误")
	}
}

func TestRemoveLocalUser(t *testing.T) {
	m := NewManager()
	uuid := "550e8400-e29b-41d4-a716-446655440001"

	m.AddLocalUser(uuid, "alice@test.com")
	if m.Count() != 1 {
		t.Fatal("添加后应有 1 用户")
	}

	err := m.RemoveLocalUser(uuid)
	if err != nil {
		t.Fatalf("删除用户失败: %v", err)
	}
	if m.Count() != 0 {
		t.Error("删除后应有 0 用户")
	}
	if m.GetUserByUUID(uuid) != nil {
		t.Error("删除后 UUID 查询应返回 nil")
	}

	// 删除不存在的用户
	err = m.RemoveLocalUser("nonexistent")
	if err == nil {
		t.Error("删除不存在的用户应返回错误")
	}
}

func TestConcurrentAccess(t *testing.T) {
	m := NewManager()
	var wg sync.WaitGroup

	// 并发读写
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			speed := 100
			users := []api.User{
				{ID: n, UUID: fmt.Sprintf("550e8400-e29b-41d4-a716-44665544%04d", n), SpeedLimit: &speed},
			}
			m.UpdateUsers(users)
		}(i)
	}

	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = m.GetAllUsers()
			_ = m.Count()
			_ = m.GetUser(1)
		}()
	}

	wg.Wait()
	// 无 panic 即通过
}

func TestUpdateUsersReplacesAll(t *testing.T) {
	m := NewManager()

	// 第一批用户
	m.UpdateUsers([]api.User{
		{ID: 1, UUID: "550e8400-e29b-41d4-a716-446655440001"},
		{ID: 2, UUID: "550e8400-e29b-41d4-a716-446655440002"},
	})
	if m.Count() != 2 {
		t.Fatal("第一批应有 2 用户")
	}

	// 第二批用户（完全替换）
	m.UpdateUsers([]api.User{
		{ID: 3, UUID: "550e8400-e29b-41d4-a716-446655440003"},
	})
	if m.Count() != 1 {
		t.Errorf("替换后应有 1 用户，实际 %d", m.Count())
	}
	if m.GetUser(1) != nil {
		t.Error("旧用户 ID=1 应不存在")
	}
	if m.GetUser(3) == nil {
		t.Error("新用户 ID=3 应存在")
	}
}
