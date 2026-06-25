package subscription

import "testing"

func TestClashRulesIncludeCommonAIServiceDomains(t *testing.T) {
	rules := buildClashRules()
	expected := []string{
		"DOMAIN-SUFFIX,openai.com,OpenAI",
		"DOMAIN-SUFFIX,chatgpt.com,OpenAI",
		"DOMAIN-SUFFIX,oaistatic.com,OpenAI",
		"DOMAIN-SUFFIX,oaiusercontent.com,OpenAI",
		"DOMAIN-SUFFIX,anthropic.com,OpenAI",
		"DOMAIN-SUFFIX,claude.ai,OpenAI",
		"DOMAIN-SUFFIX,claude.com,OpenAI",
	}

	for _, want := range expected {
		if !containsRule(rules, want) {
			t.Fatalf("missing AI service rule %q in %#v", want, rules)
		}
	}
}

func containsRule(rules []string, want string) bool {
	for _, rule := range rules {
		if rule == want {
			return true
		}
	}
	return false
}
