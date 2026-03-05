package traffic

import (
	"bufio"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

// XrayStatsCollector 从 Xray Stats API 采集流量
type XrayStatsCollector struct {
	APIAddr  string // 默认 127.0.0.1:10085
	XrayPath string // xray 二进制路径
}

// NewXrayStatsCollector 创建采集器
func NewXrayStatsCollector(apiAddr, xrayPath string) *XrayStatsCollector {
	if apiAddr == "" {
		apiAddr = "127.0.0.1:10085"
	}
	if xrayPath == "" {
		xrayPath = "/usr/local/xray-core/xray"
	}
	return &XrayStatsCollector{APIAddr: apiAddr, XrayPath: xrayPath}
}

// Collect 采集并重置 Xray 流量统计
// 通过 xray api statsquery 命令获取统计数据
// 返回 map[email][upload, download]，email 格式为 "user_{id}"
func (x *XrayStatsCollector) Collect() (map[string][2]int64, error) {
	// 调用 xray api statsquery -reset 获取并清零统计
	cmd := exec.Command(x.XrayPath, "api", "statsquery",
		fmt.Sprintf("--server=%s", x.APIAddr), "-reset")

	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("调用 Xray Stats API 失败: %w", err)
	}

	return parseStatsOutput(string(output))
}

// parseStatsOutput 解析 xray api statsquery 输出
// 输出格式:
//
//	stat: <
//	  name: "user>>>user_1>>>traffic>>>uplink"
//	  value: 12345
//	>
func parseStatsOutput(output string) (map[string][2]int64, error) {
	result := make(map[string][2]int64)
	scanner := bufio.NewScanner(strings.NewReader(output))

	var currentName string
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		if strings.HasPrefix(line, "name:") {
			// 提取 name 字段，去掉引号
			currentName = strings.Trim(strings.TrimPrefix(line, "name:"), " \"")
		} else if strings.HasPrefix(line, "value:") && currentName != "" {
			valueStr := strings.TrimSpace(strings.TrimPrefix(line, "value:"))
			value, err := strconv.ParseInt(valueStr, 10, 64)
			if err != nil {
				currentName = ""
				continue
			}

			// 解析 name: "user>>>email>>>traffic>>>uplink/downlink"
			parts := strings.Split(currentName, ">>>")
			if len(parts) == 4 && parts[0] == "user" && parts[2] == "traffic" {
				email := parts[1]     // "user_{id}"
				direction := parts[3] // "uplink" 或 "downlink"

				entry := result[email]
				switch direction {
				case "uplink":
					entry[0] += value
				case "downlink":
					entry[1] += value
				}
				result[email] = entry
			}

			currentName = ""
		}
	}

	return result, nil
}
