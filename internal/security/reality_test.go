package security

import "testing"

func TestDefaultRealityDestAvoidsKnownProblematicTargets(t *testing.T) {
	if IsKnownProblematicRealityDest(DefaultRealityServerName) {
		t.Fatalf("default Reality target must not use known problematic target: %s", DefaultRealityServerName)
	}
	if DefaultRealityDest != DefaultRealityServerName+":443" {
		t.Fatalf("default Reality dest should match default server name: %q", DefaultRealityDest)
	}
}

func TestDefaultRealityTargetServerNames(t *testing.T) {
	targets := DefaultRealityTargetServerNames()
	want := []string{
		"www.nvidia.com",
		"www.samsung.com",
		"www.tesla.com",
		"www.amazon.com",
		"www.mozilla.org",
	}
	if len(targets) != len(want) {
		t.Fatalf("default Reality target pool size = %d, want %d", len(targets), len(want))
	}
	for i := range want {
		if targets[i] != want[i] {
			t.Fatalf("target %d = %q, want %q", i, targets[i], want[i])
		}
		if IsKnownProblematicRealityDest(targets[i]) {
			t.Fatalf("target %d uses known problematic Reality target: %q", i, targets[i])
		}
	}
}

func TestMicrosoftRealityTargetGetsWarning(t *testing.T) {
	for _, warning := range knownRealityDestWarnings("www.microsoft.com") {
		if warning != "" {
			return
		}
	}
	t.Fatal("expected www.microsoft.com to produce a Reality warning")
}

func TestAppleRealityTargetGetsWarning(t *testing.T) {
	for _, warning := range knownRealityDestWarnings("www.apple.com") {
		if warning != "" {
			return
		}
	}
	t.Fatal("expected www.apple.com to produce a Reality warning")
}
