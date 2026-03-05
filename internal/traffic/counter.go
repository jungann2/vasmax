package traffic

import (
	"sync"
	"sync/atomic"
)

// UserTraffic 单用户流量（使用 atomic 无锁累加）
type UserTraffic struct {
	Upload   atomic.Int64
	Download atomic.Int64
}

// Counter 流量计数器（线程安全）
type Counter struct {
	mu       sync.Mutex
	counters map[int]*UserTraffic
}

// NewCounter 创建流量计数器
func NewCounter() *Counter {
	return &Counter{
		counters: make(map[int]*UserTraffic),
	}
}

// Add 累加流量（原子操作）
func (c *Counter) Add(userID int, upload, download int64) {
	c.mu.Lock()
	ut, ok := c.counters[userID]
	if !ok {
		ut = &UserTraffic{}
		c.counters[userID] = ut
	}
	c.mu.Unlock()

	ut.Upload.Add(upload)
	ut.Download.Add(download)
}

// Snapshot 获取快照并清零（用于上报）
// 返回所有非零流量数据
func (c *Counter) Snapshot() map[int][2]int64 {
	c.mu.Lock()
	defer c.mu.Unlock()

	snapshot := make(map[int][2]int64)
	for uid, ut := range c.counters {
		up := ut.Upload.Swap(0)
		down := ut.Download.Swap(0)
		if up > 0 || down > 0 {
			snapshot[uid] = [2]int64{up, down}
		}
	}

	// 清理零值条目
	for uid, ut := range c.counters {
		if ut.Upload.Load() == 0 && ut.Download.Load() == 0 {
			delete(c.counters, uid)
		}
	}

	return snapshot
}

// Merge 合并流量数据回计数器（上报失败时回滚）
func (c *Counter) Merge(data map[int][2]int64) {
	for uid, traffic := range data {
		c.Add(uid, traffic[0], traffic[1])
	}
}
