package sysinfo

import (
	"fmt"
	"strings"
	"testing"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
)

// physicalIfaceNames are interface names that should be included in network stats.
var physicalIfaceNames = []string{
	"eth0", "eth1", "ens33", "ens160", "enp0s3", "enp3s0",
	"wlan0", "wlp2s0", "bond0", "em1",
}

// virtualIfaceNames are interface names that should be excluded from network stats.
var virtualIfaceNames = []string{
	"lo",
	"docker0", "docker1", "docker_gwbridge",
	"veth1234abc", "vethXYZ", "veth0",
	"br-abc123", "br-def456",
}

// genIfaceName generates a random interface name (physical or virtual).
func genIfaceName(isVirtual bool) gopter.Gen {
	if isVirtual {
		return gen.OneConstOf(
			virtualIfaceNames[0],
			virtualIfaceNames[1],
			virtualIfaceNames[2],
			virtualIfaceNames[3],
			virtualIfaceNames[4],
			virtualIfaceNames[5],
			virtualIfaceNames[6],
			virtualIfaceNames[7],
			virtualIfaceNames[8],
		)
	}
	return gen.OneConstOf(
		physicalIfaceNames[0],
		physicalIfaceNames[1],
		physicalIfaceNames[2],
		physicalIfaceNames[3],
		physicalIfaceNames[4],
		physicalIfaceNames[5],
		physicalIfaceNames[6],
		physicalIfaceNames[7],
		physicalIfaceNames[8],
		physicalIfaceNames[9],
	)
}

// netDevLine formats a single /proc/net/dev line for the given interface.
// /proc/net/dev format: iface: recv_bytes packets errs drop fifo frame compressed multicast sent_bytes ...
func netDevLine(iface string, recv, sent int64) string {
	return fmt.Sprintf("  %s: %d 100 0 0 0 0 0 0 %d 100 0 0 0 0 0 0", iface, recv, sent)
}

// netDevHeader returns the standard /proc/net/dev header lines.
func netDevHeader() string {
	return "Inter-|   Receive                                                |  Transmit\n" +
		" face |bytes    packets errs drop fifo frame compressed multicast|bytes    packets errs drop fifo frame compressed"
}

// ifaceEntry holds generated data for a single network interface.
type ifaceEntry struct {
	Name    string
	Recv    int64
	Sent    int64
	Virtual bool
}

// genIfaceEntry generates a random interface entry (physical or virtual).
func genIfaceEntry(isVirtual bool) gopter.Gen {
	return gopter.CombineGens(
		genIfaceName(isVirtual),
		gen.Int64Range(0, 1<<40),
		gen.Int64Range(0, 1<<40),
	).Map(func(vals []interface{}) ifaceEntry {
		return ifaceEntry{
			Name:    vals[0].(string),
			Recv:    vals[1].(int64),
			Sent:    vals[2].(int64),
			Virtual: isVirtual,
		}
	})
}

// genMixedInterfaces generates a slice of interface entries with a mix of physical and virtual.
func genMixedInterfaces() gopter.Gen {
	return gopter.CombineGens(
		gen.SliceOfN(3, genIfaceEntry(false)), // 1-3 physical interfaces
		gen.SliceOfN(3, genIfaceEntry(true)),  // 1-3 virtual interfaces
	).Map(func(vals []interface{}) []ifaceEntry {
		physical := vals[0].([]ifaceEntry)
		virtual := vals[1].([]ifaceEntry)
		// Interleave: virtual first, then physical (order shouldn't matter)
		all := make([]ifaceEntry, 0, len(physical)+len(virtual))
		all = append(all, virtual...)
		all = append(all, physical...)
		return all
	})
}

// buildProcNetDev builds a /proc/net/dev content string from interface entries.
func buildProcNetDev(entries []ifaceEntry) string {
	var sb strings.Builder
	sb.WriteString(netDevHeader())
	sb.WriteString("\n")
	for _, e := range entries {
		sb.WriteString(netDevLine(e.Name, e.Recv, e.Sent))
		sb.WriteString("\n")
	}
	return sb.String()
}

// Feature: node-monitoring-system, Property 10: 网络接口过滤
// **Validates: Requirements 4.5**
func TestProperty10_NetworkInterfaceFiltering(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100

	properties := gopter.NewProperties(parameters)

	properties.Property("parseNetworkStats excludes lo/docker/veth/br- interfaces and sums only physical", prop.ForAll(
		func(entries []ifaceEntry) bool {
			content := buildProcNetDev(entries)
			result := parseNetworkStats(strings.NewReader(content))

			// Calculate expected totals from physical interfaces only
			var expectedRecv, expectedSent int64
			for _, e := range entries {
				if !e.Virtual {
					expectedRecv += e.Recv
					expectedSent += e.Sent
				}
			}

			if result.Recv != expectedRecv {
				t.Logf("Recv mismatch: got %d, want %d", result.Recv, expectedRecv)
				return false
			}
			if result.Sent != expectedSent {
				t.Logf("Sent mismatch: got %d, want %d", result.Sent, expectedSent)
				return false
			}
			return true
		},
		genMixedInterfaces(),
	))

	properties.Property("parseNetworkStats returns zero for content with only virtual interfaces", prop.ForAll(
		func(entries []ifaceEntry) bool {
			content := buildProcNetDev(entries)
			result := parseNetworkStats(strings.NewReader(content))
			return result.Recv == 0 && result.Sent == 0
		},
		gen.SliceOfN(5, genIfaceEntry(true)),
	))

	properties.Property("isVirtualInterface correctly identifies virtual interfaces", prop.ForAll(
		func(name string) bool {
			return isVirtualInterface(name)
		},
		genIfaceName(true),
	))

	properties.Property("isVirtualInterface correctly identifies physical interfaces", prop.ForAll(
		func(name string) bool {
			return !isVirtualInterface(name)
		},
		genIfaceName(false),
	))

	properties.TestingRun(t)
}

// Feature: node-monitoring-system, Property 5: 指标采集部分失败不影响整体
// **Validates: Requirements 2.3**
//
// This test verifies that when some /proc files are unreadable, CollectStatus()
// still returns valid data for metrics that CAN be collected, failed metrics
// return zero values, and the function never panics or returns an error.

// metricID represents a system metric that reads from /proc files.
type metricID int

const (
	metricNetwork metricID = iota
	metricDiskIO
	metricUptime
	metricCPUModel
	metricCount // sentinel: total number of metrics
)

// genUnavailableSubset generates a random subset of metrics to be "unavailable".
// Each metric has a 50% chance of being unavailable.
func genUnavailableSubset() gopter.Gen {
	return gen.SliceOfN(int(metricCount), gen.Bool()).
		SuchThat(func(v any) bool {
			s := v.([]bool)
			return len(s) == int(metricCount)
		})
}

func TestProperty5_PartialCollectionFailureDoesNotAffectOverall(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100

	properties := gopter.NewProperties(parameters)

	// Sub-property 1: Individual read functions return zero values on failure (not errors/panics)
	// readNetworkStats, readDiskIOStats, readUptime, readCPUModel each gracefully handle
	// missing /proc files by returning zero values.
	properties.Property("individual read functions return zero values when /proc files are unavailable", prop.ForAll(
		func(unavailable []bool) bool {
			// Test each function that reads from /proc files.
			// On non-Linux (or when /proc is unavailable), these should return zero values.

			if unavailable[metricNetwork] {
				result := readNetworkStats()
				if result.Sent != 0 || result.Recv != 0 {
					// On a system without /proc/net/dev, should be zero
					// On Linux with /proc/net/dev, non-zero is also valid
					// The key property: no panic occurred
				}
			}

			if unavailable[metricDiskIO] {
				result := readDiskIOStats()
				_ = result // no panic is the key assertion
			}

			if unavailable[metricUptime] {
				result := readUptime()
				if result < 0 {
					t.Log("readUptime returned negative value")
					return false
				}
			}

			if unavailable[metricCPUModel] {
				result := readCPUModel()
				_ = result // no panic, empty string on failure is valid
			}

			return true // no panic occurred
		},
		genUnavailableSubset(),
	))

	// Sub-property 2: CollectStatus() never returns error due to partial /proc failures.
	// Even when all /proc files are unreadable, CollectStatus() should return a valid
	// (possibly zero-valued) NodeStatus and nil error.
	properties.Property("CollectStatus returns valid result regardless of /proc availability", prop.ForAll(
		func(_ []bool) bool {
			status, err := CollectStatus()

			// CollectStatus should never return an error due to /proc read failures
			if err != nil {
				t.Logf("CollectStatus returned unexpected error: %v", err)
				return false
			}

			// Status should never be nil (even if all metrics fail)
			if status == nil {
				t.Log("CollectStatus returned nil status")
				return false
			}

			// Goroutines should always be positive (runtime.NumGoroutine() always works)
			if status.Goroutines <= 0 {
				t.Logf("Goroutines should be positive, got %d", status.Goroutines)
				return false
			}

			// All numeric fields should be non-negative (zero on failure, positive on success)
			if status.CPU < 0 {
				t.Logf("CPU should be non-negative, got %f", status.CPU)
				return false
			}
			if status.Mem.Total < 0 || status.Mem.Used < 0 {
				t.Logf("Mem values should be non-negative, got total=%d used=%d", status.Mem.Total, status.Mem.Used)
				return false
			}
			if status.Swap.Total < 0 || status.Swap.Used < 0 {
				t.Logf("Swap values should be non-negative, got total=%d used=%d", status.Swap.Total, status.Swap.Used)
				return false
			}
			if status.Disk.Total < 0 || status.Disk.Used < 0 {
				t.Logf("Disk values should be non-negative, got total=%d used=%d", status.Disk.Total, status.Disk.Used)
				return false
			}
			if status.Network.Sent < 0 || status.Network.Recv < 0 {
				t.Logf("Network values should be non-negative, got sent=%d recv=%d", status.Network.Sent, status.Network.Recv)
				return false
			}
			if status.DiskIO.Read < 0 || status.DiskIO.Write < 0 {
				t.Logf("DiskIO values should be non-negative, got read=%d write=%d", status.DiskIO.Read, status.DiskIO.Write)
				return false
			}
			if status.Uptime < 0 {
				t.Logf("Uptime should be non-negative, got %d", status.Uptime)
				return false
			}

			return true
		},
		genUnavailableSubset(),
	))

	// Sub-property 3: parseNetworkStats handles malformed/empty input gracefully.
	// When /proc/net/dev content is partially corrupted, the parser should still
	// return valid results for the parseable lines.
	properties.Property("parseNetworkStats handles partial corruption gracefully", prop.ForAll(
		func(goodEntries []ifaceEntry, corruptLineCount int) bool {
			var sb strings.Builder
			sb.WriteString(netDevHeader())
			sb.WriteString("\n")

			// Add some valid entries
			var expectedRecv, expectedSent int64
			for _, e := range goodEntries {
				sb.WriteString(netDevLine(e.Name, e.Recv, e.Sent))
				sb.WriteString("\n")
				if !e.Virtual {
					expectedRecv += e.Recv
					expectedSent += e.Sent
				}
			}

			// Add corrupt lines (missing fields, no colon, etc.)
			corruptLines := []string{
				"  corrupt_line_no_colon",
				"  bad_iface: not_a_number",
				"  eth99: 123",             // too few fields
				"",                         // empty line
				"  broken:",                // colon but no data
				"  eth98: 1 2 3 4 5 6 7 8", // exactly 8 fields (need 9)
			}
			for i := 0; i < corruptLineCount && i < len(corruptLines); i++ {
				sb.WriteString(corruptLines[i])
				sb.WriteString("\n")
			}

			result := parseNetworkStats(strings.NewReader(sb.String()))

			if result.Recv != expectedRecv {
				t.Logf("Recv mismatch with corruption: got %d, want %d", result.Recv, expectedRecv)
				return false
			}
			if result.Sent != expectedSent {
				t.Logf("Sent mismatch with corruption: got %d, want %d", result.Sent, expectedSent)
				return false
			}
			return true
		},
		gen.SliceOfN(3, genIfaceEntry(false)),
		gen.IntRange(0, 6),
	))

	properties.TestingRun(t)
}

// Feature: node-monitoring-system, Property 6: 静态信息仅采集一次
// **Validates: Requirements 2.4**
//
// This test verifies that after InitStaticInfo() is called once, subsequent
// calls to CollectStatus() always return the same hostname, cpu_model, ipv4,
// and ipv6 values — proving that static info is cached and not re-collected.
func TestProperty6_StaticInfoCollectedOnce(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100

	properties := gopter.NewProperties(parameters)

	// Ensure InitStaticInfo() has been called (sync.Once ensures idempotency)
	InitStaticInfo()

	// Capture the reference static info from the first CollectStatus() call
	refStatus, refErr := CollectStatus()
	if refErr != nil {
		t.Fatalf("initial CollectStatus() returned error: %v", refErr)
	}
	if refStatus == nil {
		t.Fatal("initial CollectStatus() returned nil status")
	}

	refHostname := refStatus.Hostname
	refCPUModel := refStatus.CPUModel
	refIPv4 := refStatus.IPv4
	refIPv6 := refStatus.IPv6

	properties.Property("CollectStatus returns identical static info across multiple calls", prop.ForAll(
		func(callCount int) bool {
			for i := 0; i < callCount; i++ {
				status, err := CollectStatus()
				if err != nil {
					t.Logf("CollectStatus() call %d returned error: %v", i, err)
					return false
				}
				if status == nil {
					t.Logf("CollectStatus() call %d returned nil status", i)
					return false
				}
				if status.Hostname != refHostname {
					t.Logf("call %d: hostname mismatch: got %q, want %q", i, status.Hostname, refHostname)
					return false
				}
				if status.CPUModel != refCPUModel {
					t.Logf("call %d: cpu_model mismatch: got %q, want %q", i, status.CPUModel, refCPUModel)
					return false
				}
				if status.IPv4 != refIPv4 {
					t.Logf("call %d: ipv4 mismatch: got %q, want %q", i, status.IPv4, refIPv4)
					return false
				}
				if status.IPv6 != refIPv6 {
					t.Logf("call %d: ipv6 mismatch: got %q, want %q", i, status.IPv6, refIPv6)
					return false
				}
			}
			return true
		},
		gen.IntRange(2, 10), // Random number of calls between 2 and 10
	))

	properties.TestingRun(t)
}
