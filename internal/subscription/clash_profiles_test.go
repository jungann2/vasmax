package subscription

import "testing"

func TestBuildClashProxyGroupsAddsRealityFallback(t *testing.T) {
	groups := buildClashProxyGroups([]string{
		"node-anytls",
		"node-reality-apple",
		"node-reality-bing",
	}, "https://www.gstatic.com/generate_204")

	reality := findGroup(groups, "Reality智能")
	if reality == nil {
		t.Fatal("Reality智能 group was not generated")
	}
	if reality["type"] != "fallback" {
		t.Fatalf("Reality智能 type = %v, want fallback", reality["type"])
	}

	manual := findGroup(groups, "手动切换")
	if manual == nil {
		t.Fatal("手动切换 group was not generated")
	}
	proxies, ok := manual["proxies"].([]string)
	if !ok || len(proxies) == 0 || proxies[0] != "Reality智能" {
		t.Fatalf("manual proxies should start with Reality智能: %#v", manual["proxies"])
	}
}

func findGroup(groups []map[string]interface{}, name string) map[string]interface{} {
	for _, group := range groups {
		if group["name"] == name {
			return group
		}
	}
	return nil
}
