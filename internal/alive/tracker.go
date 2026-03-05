package alive

import (
	"fmt"
	"sync"
	"time"
)

// Tracker 在线设备追踪器
type Tracker struct {
	mu     sync.RWMutex
	online map[int]map[string]time.Time // user_id -> {ip -> last_seen}
	nodeID int
}

// NewTracker 创建追踪器
func NewTracker(nodeID int) *Tracker {
	return &Tracker{
		online: make(map[int]map[string]time.Time),
		nodeID: nodeID,
	}
}

// Track 记录用户在线
func (t *Tracker) Track(userID int, ip string) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.online[userID] == nil {
		t.online[userID] = make(map[string]time.Time)
	}
	t.online[userID][ip] = time.Now()
}

// Remove 移除用户连接
func (t *Tracker) Remove(userID int, ip string) {
	t.mu.Lock()
	defer t.mu.Unlock()

	ips := t.online[userID]
	if ips == nil {
		return
	}
	delete(ips, ip)
	if len(ips) == 0 {
		delete(t.online, userID)
	}
}

// Snapshot 获取在线用户快照（IP 附加 _{nodeID} 后缀）
func (t *Tracker) Snapshot() map[int][]string {
	t.mu.RLock()
	defer t.mu.RUnlock()

	result := make(map[int][]string, len(t.online))
	suffix := fmt.Sprintf("_%d", t.nodeID)
	for userID, ips := range t.online {
		list := make([]string, 0, len(ips))
		for ip := range ips {
			list = append(list, ip+suffix)
		}
		result[userID] = list
	}
	return result
}

// CleanExpired 清理超时连接（默认 5 分钟无活动）
func (t *Tracker) CleanExpired(timeout time.Duration) {
	t.mu.Lock()
	defer t.mu.Unlock()

	now := time.Now()
	for userID, ips := range t.online {
		for ip, lastSeen := range ips {
			if now.Sub(lastSeen) > timeout {
				delete(ips, ip)
			}
		}
		if len(ips) == 0 {
			delete(t.online, userID)
		}
	}
}
