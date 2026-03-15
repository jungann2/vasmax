package menu

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/sirupsen/logrus"

	"vasmax/internal/config"
	"vasmax/internal/protocol"
	"vasmax/internal/user"
)

// MonitorMenu 实时监控菜单
type MonitorMenu struct {
	config  *config.Config
	userMgr *user.Manager
	logger  *logrus.Logger
}

// NewMonitorMenu 创建监控菜单
func NewMonitorMenu(cfg *config.Config, userMgr *user.Manager, logger *logrus.Logger) *MonitorMenu {
	return &MonitorMenu{config: cfg, userMgr: userMgr, logger: logger}
}

// Show 显示监控菜单
func (m *MonitorMenu) Show() {
	for {
		PrintTitle("实时监控")
		PrintOption(1, "用户流量统计（Xray Stats API）")
		PrintOption(2, "当前活跃连接")
		PrintOption(3, "实时连接监控（持续刷新，Ctrl+C 退出）")
		PrintOptionStr("0", "返回上级菜单")

		choice := ReadChoice("请选择", []string{"1", "2", "3"})
		switch choice {
		case "1":
			m.showUserStats()
		case "2":
			m.showActiveConnections()
		case "3":
			m.liveMonitor()
		case "0":
			return
		}
	}
}

const xrayBinary = "/usr/local/xray-core/xray"
const statsAPIAddr = "127.0.0.1:10085"

// ensureStatsAPI 确保 Stats API 配置存在并重启 Xray
func (m *MonitorMenu) ensureStatsAPI() bool {
	confDir := m.config.Paths.XrayConf
	// 检查 01_api.json 是否存在
	apiFile := confDir + "/01_api.json"
	statsFile := confDir + "/06_stats.json"
	needRestart := false

	if !fileExists(apiFile) || !fileExists(statsFile) {
		PrintInfo("正在启用 Stats API 配置...")
		if err := protocol.GenerateStatsAPIConfig(confDir); err != nil {
			PrintError(fmt.Sprintf("生成 Stats API 配置失败: %v", err))
			return false
		}
		if err := protocol.GenerateStatsModuleConfig(confDir); err != nil {
			PrintError(fmt.Sprintf("生成 Stats 模块配置失败: %v", err))
			return false
		}
		needRestart = true
	}

	if needRestart {
		PrintInfo("正在重启 Xray 以加载 Stats API...")
		cmd := exec.Command("systemctl", "restart", "xray")
		if err := cmd.Run(); err != nil {
			PrintWarning(fmt.Sprintf("重启 Xray 失败: %v", err))
			return false
		}
		time.Sleep(2 * time.Second)
		PrintSuccess("Stats API 已启用")
	}
	return true
}

// xrayStatsQuery 查询 Xray Stats API
type statEntry struct {
	Name  string `json:"name"`
	Value int64  `json:"value"`
}

type statsResponse struct {
	Stat []statEntry `json:"stat"`
}

func (m *MonitorMenu) queryStats(pattern string) ([]statEntry, error) {
	args := []string{"api", "statsquery", "-s", statsAPIAddr}
	if pattern != "" {
		args = append(args, "-pattern", pattern)
	}
	out, err := exec.Command(xrayBinary, args...).CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("%s: %s", err, string(out))
	}
	// xray api statsquery 输出 JSON
	var resp statsResponse
	if err := json.Unmarshal(out, &resp); err != nil {
		// 可能输出为空（无数据）
		if strings.TrimSpace(string(out)) == "{}" || strings.TrimSpace(string(out)) == "" {
			return nil, nil
		}
		return nil, fmt.Errorf("解析统计数据失败: %v (原始: %s)", err, string(out))
	}
	return resp.Stat, nil
}

// showUserStats 显示用户流量统计
func (m *MonitorMenu) showUserStats() {
	PrintTitle("用户流量统计")

	if !m.ensureStatsAPI() {
		return
	}

	stats, err := m.queryStats("")
	if err != nil {
		PrintError(fmt.Sprintf("查询统计失败: %v", err))
		return
	}

	if len(stats) == 0 {
		PrintWarning("暂无流量统计数据（用户可能尚未产生流量）")
		fmt.Println()
		return
	}

	// 按用户聚合上行/下行
	type userTraffic struct {
		Upload   int64
		Download int64
	}
	userMap := make(map[string]*userTraffic)

	for _, s := range stats {
		// 格式: user>>>email>>>traffic>>>uplink 或 user>>>email>>>traffic>>>downlink
		// 或: inbound>>>tag>>>traffic>>>uplink
		parts := strings.Split(s.Name, ">>>")
		if len(parts) != 4 {
			continue
		}
		category := parts[0]
		name := parts[1]
		direction := parts[3]

		if category == "user" {
			if _, ok := userMap[name]; !ok {
				userMap[name] = &userTraffic{}
			}
			switch direction {
			case "uplink":
				userMap[name].Upload += s.Value
			case "downlink":
				userMap[name].Download += s.Value
			}
		}
	}

	// 构建用户 email -> 显示名映射
	users := m.userMgr.GetAllUsers()
	emailToDisplay := make(map[string]string)
	for _, u := range users {
		emailToDisplay[u.Email] = fmt.Sprintf("%s (%s)", u.Email, u.UUID[:8])
	}

	if len(userMap) == 0 {
		PrintWarning("暂无用户流量数据")
	} else {
		fmt.Printf("\n  %-30s %12s %12s %12s\n", "用户", "上行", "下行", "合计")
		fmt.Println("  " + strings.Repeat("─", 68))
		for email, t := range userMap {
			display := email
			if d, ok := emailToDisplay[email]; ok {
				display = d
			}
			total := t.Upload + t.Download
			fmt.Printf("  %-30s %12s %12s %12s\n",
				display,
				formatBytes(t.Upload),
				formatBytes(t.Download),
				formatBytes(total),
			)
		}
	}

	// 也显示入站流量
	fmt.Println()
	PrintInfo("入站流量统计:")
	inboundMap := make(map[string]*userTraffic)
	for _, s := range stats {
		parts := strings.Split(s.Name, ">>>")
		if len(parts) != 4 || parts[0] != "inbound" {
			continue
		}
		tag := parts[1]
		direction := parts[3]
		if _, ok := inboundMap[tag]; !ok {
			inboundMap[tag] = &userTraffic{}
		}
		switch direction {
		case "uplink":
			inboundMap[tag].Upload += s.Value
		case "downlink":
			inboundMap[tag].Download += s.Value
		}
	}

	if len(inboundMap) > 0 {
		fmt.Printf("  %-30s %12s %12s %12s\n", "入站标签", "上行", "下行", "合计")
		fmt.Println("  " + strings.Repeat("─", 68))
		for tag, t := range inboundMap {
			total := t.Upload + t.Download
			fmt.Printf("  %-30s %12s %12s %12s\n", tag, formatBytes(t.Upload), formatBytes(t.Download), formatBytes(total))
		}
	}

	fmt.Println()
}

// showActiveConnections 显示当前活跃连接
func (m *MonitorMenu) showActiveConnections() {
	PrintTitle("当前活跃连接")

	// 使用 ss 查看连接到 Xray 监听端口的 TCP 连接
	out, err := exec.Command("ss", "-tnp").Output()
	if err != nil {
		PrintError(fmt.Sprintf("获取连接信息失败: %v", err))
		return
	}

	lines := strings.Split(string(out), "\n")
	type connInfo struct {
		Source string
		Dest   string
		State  string
	}
	var conns []connInfo
	sourceCount := make(map[string]int)

	for _, line := range lines {
		if !strings.Contains(line, "xray") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 5 {
			continue
		}
		state := fields[0]
		local := fields[3]
		remote := fields[4]

		// 提取源 IP（去掉端口）
		srcIP := remote
		if idx := strings.LastIndex(remote, ":"); idx > 0 {
			srcIP = remote[:idx]
		}

		conns = append(conns, connInfo{Source: remote, Dest: local, State: state})
		sourceCount[srcIP]++
	}

	if len(conns) == 0 {
		PrintWarning("当前无活跃连接")
		fmt.Println()
		return
	}

	PrintInfo(fmt.Sprintf("活跃连接数: %d", len(conns)))
	fmt.Println()

	// 按源 IP 汇总
	fmt.Printf("  %-40s %s\n", "来源 IP", "连接数")
	fmt.Println("  " + strings.Repeat("─", 50))
	for ip, count := range sourceCount {
		fmt.Printf("  %-40s %d\n", ip, count)
	}

	// 显示详细连接（最多 30 条）
	fmt.Println()
	PrintInfo("连接详情（最多显示 30 条）:")
	fmt.Printf("  %-8s %-25s %-25s\n", "状态", "来源", "本地")
	fmt.Println("  " + strings.Repeat("─", 60))
	limit := len(conns)
	if limit > 30 {
		limit = 30
	}
	for i := 0; i < limit; i++ {
		c := conns[i]
		fmt.Printf("  %-8s %-25s %-25s\n", c.State, c.Source, c.Dest)
	}
	if len(conns) > 30 {
		PrintInfo(fmt.Sprintf("... 还有 %d 条连接未显示", len(conns)-30))
	}

	fmt.Println()
}

// liveMonitor 实时连接监控（持续刷新）
func (m *MonitorMenu) liveMonitor() {
	PrintInfo("实时监控已启动，每 3 秒刷新一次，按 Ctrl+C 返回菜单")
	fmt.Println()

	if !m.ensureStatsAPI() {
		return
	}

	// 使用 goroutine 监听用户输入来退出
	stopCh := make(chan struct{})
	go func() {
		ReadInput("按回车键退出监控")
		close(stopCh)
	}()

	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()

	// 先显示一次
	m.printLiveStatus()

	for {
		select {
		case <-ticker.C:
			m.printLiveStatus()
		case <-stopCh:
			fmt.Println()
			PrintSuccess("已退出实时监控")
			return
		}
	}
}

// printLiveStatus 打印一次实时状态
func (m *MonitorMenu) printLiveStatus() {
	// 清屏
	fmt.Print("\033[2J\033[H")

	now := time.Now().Format("2006-01-02 15:04:05")
	fmt.Printf("  %s 实时监控  %s  （按回车退出）\n\n", Cyan("▶"), now)

	// 1. 用户流量
	stats, _ := m.queryStats("")
	type userTraffic struct {
		Upload   int64
		Download int64
	}
	userMap := make(map[string]*userTraffic)
	for _, s := range stats {
		parts := strings.Split(s.Name, ">>>")
		if len(parts) != 4 || parts[0] != "user" {
			continue
		}
		name := parts[1]
		if _, ok := userMap[name]; !ok {
			userMap[name] = &userTraffic{}
		}
		switch parts[3] {
		case "uplink":
			userMap[name].Upload += s.Value
		case "downlink":
			userMap[name].Download += s.Value
		}
	}

	fmt.Printf("  %s 用户流量:\n", Yellow("━"))
	if len(userMap) == 0 {
		fmt.Println("    暂无数据")
	} else {
		fmt.Printf("    %-25s %10s %10s\n", "用户", "上行", "下行")
		for email, t := range userMap {
			fmt.Printf("    %-25s %10s %10s\n", email, formatBytes(t.Upload), formatBytes(t.Download))
		}
	}

	// 2. 活跃连接
	fmt.Println()
	out, err := exec.Command("ss", "-tnp").Output()
	connCount := 0
	sourceCount := make(map[string]int)
	if err == nil {
		for _, line := range strings.Split(string(out), "\n") {
			if !strings.Contains(line, "xray") {
				continue
			}
			connCount++
			fields := strings.Fields(line)
			if len(fields) >= 5 {
				remote := fields[4]
				if idx := strings.LastIndex(remote, ":"); idx > 0 {
					sourceCount[remote[:idx]]++
				}
			}
		}
	}

	fmt.Printf("  %s 活跃连接: %d\n", Yellow("━"), connCount)
	for ip, count := range sourceCount {
		fmt.Printf("    %-40s %d 连接\n", ip, count)
	}
}

// formatBytes 格式化字节数为人类可读格式
func formatBytes(b int64) string {
	const (
		KB = 1024
		MB = KB * 1024
		GB = MB * 1024
		TB = GB * 1024
	)
	switch {
	case b >= TB:
		return fmt.Sprintf("%.2f TB", float64(b)/float64(TB))
	case b >= GB:
		return fmt.Sprintf("%.2f GB", float64(b)/float64(GB))
	case b >= MB:
		return fmt.Sprintf("%.2f MB", float64(b)/float64(MB))
	case b >= KB:
		return fmt.Sprintf("%.2f KB", float64(b)/float64(KB))
	default:
		return fmt.Sprintf("%d B", b)
	}
}
