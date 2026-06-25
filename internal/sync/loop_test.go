package sync

import (
	"os"
	"path/filepath"
	"testing"

	"vasmax/internal/api"
	"vasmax/internal/config"
	"vasmax/internal/security"
	"vasmax/internal/user"

	"github.com/sirupsen/logrus"
)

func TestManagedUsersEqualIgnoresOrder(t *testing.T) {
	limit := 100
	existing := []*user.UserEntry{
		{ID: 2, UUID: "550e8400-e29b-41d4-a716-446655440002", SpeedLimit: 0, DeviceLimit: 0},
		{ID: 1, UUID: "550e8400-e29b-41d4-a716-446655440001", SpeedLimit: 100, DeviceLimit: 0},
	}
	next := []api.User{
		{ID: 1, UUID: "550e8400-e29b-41d4-a716-446655440001", SpeedLimit: &limit},
		{ID: 2, UUID: "550e8400-e29b-41d4-a716-446655440002"},
	}

	if !managedUsersEqual(existing, next) {
		t.Fatal("expected user lists to be equal")
	}
}

func TestManagedUsersEqualDetectsLimitChange(t *testing.T) {
	oldLimit := 100
	newLimit := 200
	existing := []*user.UserEntry{
		{ID: 1, UUID: "550e8400-e29b-41d4-a716-446655440001", SpeedLimit: oldLimit},
	}
	next := []api.User{
		{ID: 1, UUID: "550e8400-e29b-41d4-a716-446655440001", SpeedLimit: &newLimit},
	}

	if managedUsersEqual(existing, next) {
		t.Fatal("expected speed limit change to be detected")
	}
}

func TestManagedCoreUsersEqualIgnoresLimitChange(t *testing.T) {
	oldLimit := 100
	newLimit := 200
	existing := []*user.UserEntry{
		{ID: 1, UUID: "550e8400-e29b-41d4-a716-446655440001", SpeedLimit: oldLimit, DeviceLimit: 1},
	}
	next := []api.User{
		{ID: 1, UUID: "550e8400-e29b-41d4-a716-446655440001", SpeedLimit: &newLimit},
	}

	if !managedCoreUsersEqual(existing, next) {
		t.Fatal("expected limit-only change to keep core users equal")
	}
}

func TestManagedCoreUsersEqualDetectsUUIDChange(t *testing.T) {
	existing := []*user.UserEntry{
		{ID: 1, UUID: "550e8400-e29b-41d4-a716-446655440001"},
	}
	next := []api.User{
		{ID: 1, UUID: "550e8400-e29b-41d4-a716-446655440002"},
	}

	if managedCoreUsersEqual(existing, next) {
		t.Fatal("expected UUID change to require core reload")
	}
}

func TestAPIUsersFromEntriesPreservesLimits(t *testing.T) {
	users := apiUsersFromEntries([]*user.UserEntry{
		nil,
		{
			ID:          1,
			UUID:        "550e8400-e29b-41d4-a716-446655440001",
			SpeedLimit:  100,
			DeviceLimit: 2,
		},
	})

	if len(users) != 1 {
		t.Fatalf("expected 1 converted user, got %d", len(users))
	}
	if users[0].ID != 1 || users[0].UUID != "550e8400-e29b-41d4-a716-446655440001" {
		t.Fatalf("unexpected converted user: %#v", users[0])
	}
	if users[0].SpeedLimit == nil || *users[0].SpeedLimit != 100 {
		t.Fatalf("expected speed limit 100, got %#v", users[0].SpeedLimit)
	}
	if users[0].DeviceLimit == nil || *users[0].DeviceLimit != 2 {
		t.Fatalf("expected device limit 2, got %#v", users[0].DeviceLimit)
	}
}

func TestValidateManagedUsersRejectsBadInput(t *testing.T) {
	validUUID := "550e8400-e29b-41d4-a716-446655440001"
	if err := validateManagedUsers([]api.User{{ID: 1, UUID: validUUID}}); err != nil {
		t.Fatalf("expected valid user to pass: %v", err)
	}

	tests := []struct {
		name  string
		users []api.User
	}{
		{name: "zero id", users: []api.User{{ID: 0, UUID: validUUID}}},
		{name: "bad uuid", users: []api.User{{ID: 1, UUID: "bad-uuid"}}},
		{name: "duplicate id", users: []api.User{{ID: 1, UUID: validUUID}, {ID: 1, UUID: "550e8400-e29b-41d4-a716-446655440002"}}},
		{name: "duplicate uuid", users: []api.User{{ID: 1, UUID: validUUID}, {ID: 2, UUID: validUUID}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := validateManagedUsers(tt.users); err == nil {
				t.Fatal("expected validation error")
			}
		})
	}
}

func TestShouldDeferEmptyUsersUntilThreshold(t *testing.T) {
	l := &Loop{
		config: &config.Config{
			Sync: config.SyncConfig{EmptyUsersApplyThreshold: 3},
		},
		logger: logrus.New(),
	}
	existing := []*user.UserEntry{{ID: 1, UUID: "550e8400-e29b-41d4-a716-446655440001"}}

	if !l.shouldDeferEmptyUsers(existing, nil) {
		t.Fatal("expected first empty list to be deferred")
	}
	if !l.shouldDeferEmptyUsers(existing, nil) {
		t.Fatal("expected second empty list to be deferred")
	}
	if l.shouldDeferEmptyUsers(existing, nil) {
		t.Fatal("expected third empty list to be applied")
	}
}

func TestLoadCachedUsersIntoManager(t *testing.T) {
	dir := t.TempDir()
	cacheDir := filepath.Join(dir, "cache")
	if err := os.MkdirAll(cacheDir, 0700); err != nil {
		t.Fatal(err)
	}
	cachePath := filepath.Join(cacheDir, "users.json")
	data := []byte(`{"timestamp":1,"users":[{"id":1,"uuid":"550e8400-e29b-41d4-a716-446655440001"}]}`)
	if err := security.AtomicWrite(cachePath, data, 0600); err != nil {
		t.Fatal(err)
	}

	userMgr := user.NewManager()
	l := &Loop{
		userManager: userMgr,
		config:      &config.Config{Paths: config.PathsConfig{Cache: cacheDir}},
		logger:      logrus.New(),
	}
	l.loadCachedUsersIntoManager()

	if got := userMgr.Count(); got != 1 {
		t.Fatalf("expected cached user loaded, got %d", got)
	}
}

func TestApplyConfigWritesWritesAllFiles(t *testing.T) {
	dir := t.TempDir()
	first := filepath.Join(dir, "first.json")
	second := filepath.Join(dir, "second.json")

	err := applyConfigWrites([]configWrite{
		{path: first, data: []byte(`{"first":true}`), perm: 0600},
		{path: second, data: []byte(`{"second":true}`), perm: 0600},
	})
	if err != nil {
		t.Fatalf("applyConfigWrites failed: %v", err)
	}

	firstData, err := os.ReadFile(first)
	if err != nil {
		t.Fatal(err)
	}
	if string(firstData) != `{"first":true}` {
		t.Fatalf("unexpected first file content: %s", firstData)
	}

	secondData, err := os.ReadFile(second)
	if err != nil {
		t.Fatal(err)
	}
	if string(secondData) != `{"second":true}` {
		t.Fatalf("unexpected second file content: %s", secondData)
	}
}

func TestApplyConfigWritesRollsBackOnFailure(t *testing.T) {
	dir := t.TempDir()
	existing := filepath.Join(dir, "existing.json")
	newFile := filepath.Join(dir, "new.json")
	blocker := filepath.Join(dir, "blocker")
	badTarget := filepath.Join(blocker, "bad.json")

	if err := security.AtomicWrite(existing, []byte("old"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := security.AtomicWrite(blocker, []byte("not a directory"), 0600); err != nil {
		t.Fatal(err)
	}

	err := applyConfigWrites([]configWrite{
		{path: existing, data: []byte("new"), perm: 0600},
		{path: newFile, data: []byte("new file"), perm: 0600},
		{path: badTarget, data: []byte("bad"), perm: 0600},
	})
	if err == nil {
		t.Fatal("expected applyConfigWrites to fail")
	}

	existingData, err := os.ReadFile(existing)
	if err != nil {
		t.Fatal(err)
	}
	if string(existingData) != "old" {
		t.Fatalf("expected existing file to be restored, got %q", string(existingData))
	}

	if _, err := os.Stat(newFile); !os.IsNotExist(err) {
		t.Fatalf("expected new file to be removed after rollback, got %v", err)
	}
}
