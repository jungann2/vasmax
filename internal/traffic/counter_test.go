package traffic

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
)

func TestNewCounter(t *testing.T) {
	c := NewCounter()
	snap := c.Snapshot()
	if len(snap) != 0 {
		t.Error("新计数器快照应为空")
	}
}

func TestAddAndSnapshot(t *testing.T) {
	c := NewCounter()
	c.Add(1, 100, 200)
	c.Add(1, 50, 30)
	c.Add(2, 1000, 2000)

	snap := c.Snapshot()
	if len(snap) != 2 {
		t.Fatalf("期望 2 个用户，实际 %d", len(snap))
	}
	if snap[1] != [2]int64{150, 230} {
		t.Errorf("用户 1 流量不正确: %v", snap[1])
	}
	if snap[2] != [2]int64{1000, 2000} {
		t.Errorf("用户 2 流量不正确: %v", snap[2])
	}

	// Snapshot 后应清零
	snap2 := c.Snapshot()
	if len(snap2) != 0 {
		t.Errorf("二次快照应为空，实际 %d 条", len(snap2))
	}
}

func TestMerge(t *testing.T) {
	c := NewCounter()
	data := map[int][2]int64{
		1: {100, 200},
		2: {300, 400},
	}
	c.Merge(data)

	snap := c.Snapshot()
	if snap[1] != [2]int64{100, 200} {
		t.Errorf("Merge 后用户 1 不正确: %v", snap[1])
	}
	if snap[2] != [2]int64{300, 400} {
		t.Errorf("Merge 后用户 2 不正确: %v", snap[2])
	}
}

func TestSnapshotThenMergeRollback(t *testing.T) {
	c := NewCounter()
	c.Add(1, 100, 200)

	snap := c.Snapshot()
	// 模拟上报失败，回滚
	c.Merge(snap)

	snap2 := c.Snapshot()
	if snap2[1] != [2]int64{100, 200} {
		t.Errorf("回滚后数据不正确: %v", snap2[1])
	}
}

func TestConcurrentAdd(t *testing.T) {
	c := NewCounter()
	var wg sync.WaitGroup
	n := 100

	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			c.Add(1, 10, 20)
		}()
	}
	wg.Wait()

	snap := c.Snapshot()
	expected := [2]int64{int64(n * 10), int64(n * 20)}
	if snap[1] != expected {
		t.Errorf("并发累加不正确: 期望 %v, 实际 %v", expected, snap[1])
	}
}

func TestPersistence(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "traffic.json")

	c := NewCounter()
	c.Add(1, 100, 200)
	c.Add(2, 300, 400)

	if err := c.SaveToFile(path); err != nil {
		t.Fatalf("保存失败: %v", err)
	}

	// 验证文件存在
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("文件不存在: %v", err)
	}

	// 加载到新计数器
	c2 := NewCounter()
	if err := c2.LoadFromFile(path); err != nil {
		t.Fatalf("加载失败: %v", err)
	}

	snap := c2.Snapshot()
	if snap[1] != [2]int64{100, 200} {
		t.Errorf("恢复后用户 1 不正确: %v", snap[1])
	}
	if snap[2] != [2]int64{300, 400} {
		t.Errorf("恢复后用户 2 不正确: %v", snap[2])
	}
}

func TestLoadFromFileNotExist(t *testing.T) {
	c := NewCounter()
	err := c.LoadFromFile("/nonexistent/path/traffic.json")
	if err != nil {
		t.Error("不存在的文件应返回 nil")
	}
}

func TestLoadFromFileCorrupted(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "traffic.json")
	os.WriteFile(path, []byte("invalid json"), 0644)

	c := NewCounter()
	err := c.LoadFromFile(path)
	if err == nil {
		t.Error("损坏文件应返回错误")
	}
}

func TestSaveEmptyCounter(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "traffic.json")

	c := NewCounter()
	if err := c.SaveToFile(path); err != nil {
		t.Fatalf("保存空计数器失败: %v", err)
	}

	// 空计数器不应创建文件
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Error("空计数器不应创建文件")
	}
}

func TestParseStatsOutput(t *testing.T) {
	output := `stat: <
  name: "user>>>user_1>>>traffic>>>uplink"
  value: 12345
>
stat: <
  name: "user>>>user_1>>>traffic>>>downlink"
  value: 67890
>
stat: <
  name: "user>>>user_2>>>traffic>>>uplink"
  value: 100
>
`
	result, err := parseStatsOutput(output)
	if err != nil {
		t.Fatalf("解析失败: %v", err)
	}
	if result["user_1"] != [2]int64{12345, 67890} {
		t.Errorf("user_1 不正确: %v", result["user_1"])
	}
	if result["user_2"] != [2]int64{100, 0} {
		t.Errorf("user_2 不正确: %v", result["user_2"])
	}
}
