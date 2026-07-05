package protocol

import (
	"encoding/json"
	"testing"

	"vasmax/internal/api"
)

func TestGenerateSingBoxStatsAPIConfigDataIncludesManagedNames(t *testing.T) {
	users := []*api.User{{ID: 1}, {ID: 2}}
	data, err := GenerateSingBoxStatsAPIConfigData(users)
	if err != nil {
		t.Fatal(err)
	}

	var cfg map[string]map[string]map[string]interface{}
	if err := json.Unmarshal(data, &cfg); err != nil {
		t.Fatal(err)
	}
	stats := cfg["experimental"]["v2ray_api"]["stats"].(map[string]interface{})
	got := stats["users"].([]interface{})
	if len(got) != 4 {
		t.Fatalf("stats users = %v, want 4 entries", got)
	}
	want := map[string]bool{
		"user_1": true, "user_1-anytls": true,
		"user_2": true, "user_2-anytls": true,
	}
	for _, item := range got {
		name, _ := item.(string)
		if !want[name] {
			t.Fatalf("unexpected stats user %q in %v", name, got)
		}
	}
}
