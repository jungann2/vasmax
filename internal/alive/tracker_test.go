package alive

import (
	"testing"
	"time"
)

func TestTrackAndSnapshot(t *testing.T) {
	tr := NewTracker(42)
	tr.Track(1, "192.168.1.1")
	tr.Track(1, "10.0.0.1")
	tr.Track(2, "172.16.0.1")

	snap := tr.Snapshot()
	if len(snap) != 2 {
		t.Fatalf("期望 2 个用户，实际 %d", len(snap))
	}
	if len(snap[1]) != 2 {
		t.Errorf("用户 1 应有 2 个 IP，实际 %d", len(snap[1]))
	}

	// 验证后缀
	for _, ip := range snap[1] {
		if ip != "192.168.1.1_42" && ip != "10.0.0.1_42" {
			t.Errorf("IP 后缀不正确: %s", ip)
		}
	}
}

func TestRemove(t *testing.T) {
	tr := NewTracker(1)
	tr.Track(1, "192.168.1.1")
	tr.Track(1, "10.0.0.1")

	tr.Remove(1, "192.168.1.1")
	snap := tr.Snapshot()
	if len(snap[1]) != 1 {
		t.Errorf("移除后应有 1 个 IP，实际 %d", len(snap[1]))
	}

	tr.Remove(1, "10.0.0.1")
	snap = tr.Snapshot()
	if len(snap) != 0 {
		t.Error("全部移除后应为空")
	}

	// 移除不存在的
	tr.Remove(999, "1.1.1.1") // 不应 panic
}

func TestCleanExpired(t *testing.T) {
	tr := NewTracker(1)
	tr.Track(1, "192.168.1.1")

	// 手动设置过期时间
	tr.mu.Lock()
	tr.online[1]["192.168.1.1"] = time.Now().Add(-10 * time.Minute)
	tr.mu.Unlock()

	tr.Track(1, "10.0.0.1") // 这个是新的

	tr.CleanExpired(5 * time.Minute)

	snap := tr.Snapshot()
	if len(snap[1]) != 1 {
		t.Errorf("清理后应有 1 个 IP，实际 %d", len(snap[1]))
	}
}

func TestEmptySnapshot(t *testing.T) {
	tr := NewTracker(1)
	snap := tr.Snapshot()
	if len(snap) != 0 {
		t.Error("空追踪器快照应为空")
	}
}
