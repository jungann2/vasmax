package sync

import (
	"testing"

	"vasmax/internal/api"
	"vasmax/internal/user"
)

func TestManagedUsersEqualIgnoresOrder(t *testing.T) {
	limit := 100
	existing := []*user.UserEntry{
		{ID: 2, UUID: "uuid-2", SpeedLimit: 0, DeviceLimit: 0},
		{ID: 1, UUID: "uuid-1", SpeedLimit: 100, DeviceLimit: 0},
	}
	next := []api.User{
		{ID: 1, UUID: "uuid-1", SpeedLimit: &limit},
		{ID: 2, UUID: "uuid-2"},
	}

	if !managedUsersEqual(existing, next) {
		t.Fatal("expected user lists to be equal")
	}
}

func TestManagedUsersEqualDetectsLimitChange(t *testing.T) {
	oldLimit := 100
	newLimit := 200
	existing := []*user.UserEntry{
		{ID: 1, UUID: "uuid-1", SpeedLimit: oldLimit},
	}
	next := []api.User{
		{ID: 1, UUID: "uuid-1", SpeedLimit: &newLimit},
	}

	if managedUsersEqual(existing, next) {
		t.Fatal("expected speed limit change to be detected")
	}
}
