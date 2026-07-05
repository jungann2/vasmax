package subscription

import (
	"encoding/json"
	"testing"
)

func TestSingBoxFullProfileSplitsDomesticDirectRules(t *testing.T) {
	data, err := GenerateSingBoxFullProfile(nil)
	if err != nil {
		t.Fatal(err)
	}

	var profile map[string]interface{}
	if err := json.Unmarshal(data, &profile); err != nil {
		t.Fatal(err)
	}

	route := profile["route"].(map[string]interface{})
	rules := route["rules"].([]interface{})

	var hasGeositeRule, hasGeoipRule bool
	for _, raw := range rules {
		rule := raw.(map[string]interface{})
		_, hasGeosite := rule["geosite"]
		_, hasGeoip := rule["geoip"]
		if hasGeosite && hasGeoip {
			t.Fatalf("domestic direct rule must not combine geosite and geoip: %#v", rule)
		}
		if hasGeosite && rule["outbound"] == "direct" {
			hasGeositeRule = true
		}
		if hasGeoip && rule["outbound"] == "direct" {
			hasGeoipRule = true
		}
	}

	if !hasGeositeRule {
		t.Fatal("expected separate geosite direct rule")
	}
	if !hasGeoipRule {
		t.Fatal("expected separate geoip direct rule")
	}
}
