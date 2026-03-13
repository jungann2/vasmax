package api

import (
	"encoding/json"
	"fmt"
	"reflect"
	"testing"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
)

// genResourceUsage generates a random ResourceUsage with non-negative values.
func genResourceUsage() gopter.Gen {
	return gopter.CombineGens(
		gen.Int64Range(0, 1<<40),
		gen.Int64Range(0, 1<<40),
	).Map(func(vals []interface{}) ResourceUsage {
		return ResourceUsage{
			Total: vals[0].(int64),
			Used:  vals[1].(int64),
		}
	})
}

// genNetworkUsage generates a random NetworkUsage with non-negative values.
func genNetworkUsage() gopter.Gen {
	return gopter.CombineGens(
		gen.Int64Range(0, 1<<40),
		gen.Int64Range(0, 1<<40),
	).Map(func(vals []interface{}) NetworkUsage {
		return NetworkUsage{
			Sent: vals[0].(int64),
			Recv: vals[1].(int64),
		}
	})
}

// genDiskIOUsage generates a random DiskIOUsage with non-negative values.
func genDiskIOUsage() gopter.Gen {
	return gopter.CombineGens(
		gen.Int64Range(0, 1<<40),
		gen.Int64Range(0, 1<<40),
	).Map(func(vals []interface{}) DiskIOUsage {
		return DiskIOUsage{
			Read:  vals[0].(int64),
			Write: vals[1].(int64),
		}
	})
}

// genHostname generates a valid hostname string (alphanumeric, hyphens, dots).
func genHostname() gopter.Gen {
	return gen.RegexMatch(`[a-zA-Z0-9][a-zA-Z0-9\-_.]{0,30}`)
}

// genIPv4 generates a simple IPv4 address string.
func genIPv4() gopter.Gen {
	return gopter.CombineGens(
		gen.IntRange(1, 254),
		gen.IntRange(0, 255),
		gen.IntRange(0, 255),
		gen.IntRange(1, 254),
	).Map(func(vals []interface{}) string {
		return fmt.Sprintf("%d.%d.%d.%d",
			vals[0].(int), vals[1].(int), vals[2].(int), vals[3].(int))
	})
}

// genNodeStatus generates a random NodeStatus instance with all fields populated.
func genNodeStatus() gopter.Gen {
	return gopter.CombineGens(
		gen.Float64Range(0, 100), // CPU
		genResourceUsage(),       // Mem
		genResourceUsage(),       // Swap
		genResourceUsage(),       // Disk
		genNetworkUsage(),        // Network
		genDiskIOUsage(),         // DiskIO
		genHostname(),            // Hostname
		gen.Int64Range(0, 1<<30), // Uptime
		gen.AlphaString(),        // CPUModel
		genIPv4(),                // IPv4
		gen.IntRange(1, 10000),   // Goroutines
	).Map(func(vals []interface{}) NodeStatus {
		return NodeStatus{
			CPU:        vals[0].(float64),
			Mem:        vals[1].(ResourceUsage),
			Swap:       vals[2].(ResourceUsage),
			Disk:       vals[3].(ResourceUsage),
			Network:    vals[4].(NetworkUsage),
			DiskIO:     vals[5].(DiskIOUsage),
			Hostname:   vals[6].(string),
			Uptime:     vals[7].(int64),
			CPUModel:   vals[8].(string),
			IPv4:       vals[9].(string),
			Goroutines: vals[10].(int),
		}
	})
}

// Feature: node-monitoring-system, Property 4: NodeStatus 序列化往返一致性
// **Validates: Requirements 2.2**
func TestProperty4_NodeStatusSerializationRoundTrip(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100

	properties := gopter.NewProperties(parameters)

	properties.Property("NodeStatus JSON marshal/unmarshal round-trip produces identical struct", prop.ForAll(
		func(original NodeStatus) bool {
			// Marshal to JSON
			data, err := json.Marshal(original)
			if err != nil {
				t.Logf("Marshal error: %v", err)
				return false
			}

			// Unmarshal back
			var restored NodeStatus
			if err := json.Unmarshal(data, &restored); err != nil {
				t.Logf("Unmarshal error: %v", err)
				return false
			}

			// Compare using reflect.DeepEqual
			if !reflect.DeepEqual(original, restored) {
				t.Logf("Round-trip mismatch:\n  original: %+v\n  restored: %+v", original, restored)
				return false
			}

			return true
		},
		genNodeStatus(),
	))

	properties.TestingRun(t)
}
