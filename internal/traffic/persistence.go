package traffic

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"time"

	"vasmax/internal/security"
)

// persistedData 持久化文件格式
type persistedData struct {
	Timestamp int64               `json:"timestamp"`
	Data      map[string][2]int64 `json:"data"`
}

// SaveToFile 将当前流量数据持久化为 JSON 文件（原子写入）
func (c *Counter) SaveToFile(path string) error {
	c.mu.Lock()
	data := make(map[string][2]int64)
	for uid, ut := range c.counters {
		up := ut.Upload.Load()
		down := ut.Download.Load()
		if up > 0 || down > 0 {
			data[strconv.Itoa(uid)] = [2]int64{up, down}
		}
	}
	c.mu.Unlock()

	if len(data) == 0 {
		os.Remove(path)
		return nil
	}

	pd := persistedData{
		Timestamp: time.Now().Unix(),
		Data:      data,
	}

	b, err := json.MarshalIndent(pd, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化流量数据失败: %w", err)
	}

	return security.AtomicWrite(path, b, 0644)
}

// LoadFromFile 从文件恢复流量数据
// 文件损坏时记录警告从零开始
func (c *Counter) LoadFromFile(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("读取流量持久化文件失败: %w", err)
	}

	var pd persistedData
	if err := json.Unmarshal(data, &pd); err != nil {
		// 文件损坏，从零开始
		return fmt.Errorf("流量持久化文件损坏（将从零开始）: %w", err)
	}

	merged := make(map[int][2]int64, len(pd.Data))
	for uidStr, traffic := range pd.Data {
		uid, err := strconv.Atoi(uidStr)
		if err != nil {
			continue
		}
		merged[uid] = traffic
	}

	c.Merge(merged)
	return nil
}
