package security

import (
	"strings"
	"testing"
)

// --- ValidateDomain tests ---

func TestValidateDomain_Valid(t *testing.T) {
	valid := []string{
		"example.com",
		"sub.example.com",
		"a.b.c.d.example.com",
		"my-domain.org",
		"x1.y2.z3",
		"EXAMPLE.COM",
		"Example.Com",
		"a1.b2",
		"example.com.", // trailing dot (FQDN)
	}
	for _, d := range valid {
		if err := ValidateDomain(d); err != nil {
			t.Errorf("ValidateDomain(%q) returned error: %v", d, err)
		}
	}
}

func TestValidateDomain_Invalid(t *testing.T) {
	invalid := []struct {
		domain string
		desc   string
	}{
		{"", "empty"},
		{".", "just a dot"},
		{"localhost", "single label"},
		{"-example.com", "label starts with hyphen"},
		{"example-.com", "label ends with hyphen"},
		{"exam ple.com", "contains space"},
		{"exam_ple.com", "contains underscore"},
		{"example..com", "empty label"},
		{strings.Repeat("a", 64) + ".com", "label exceeds 63 chars"},
		{strings.Repeat("a.", 127) + "com", "domain exceeds 253 chars"},
		{"example.com/path", "contains slash"},
	}
	for _, tc := range invalid {
		if err := ValidateDomain(tc.domain); err == nil {
			t.Errorf("ValidateDomain(%q) [%s] expected error, got nil", tc.domain, tc.desc)
		}
	}
}

// --- ValidatePort tests ---

func TestValidatePort_Valid(t *testing.T) {
	valid := []int{1, 80, 443, 8080, 65535}
	for _, p := range valid {
		if err := ValidatePort(p); err != nil {
			t.Errorf("ValidatePort(%d) returned error: %v", p, err)
		}
	}
}

func TestValidatePort_Invalid(t *testing.T) {
	invalid := []int{0, -1, 65536, 100000, -100}
	for _, p := range invalid {
		if err := ValidatePort(p); err == nil {
			t.Errorf("ValidatePort(%d) expected error, got nil", p)
		}
	}
}

// --- ValidateUUID tests ---

func TestValidateUUID_Valid(t *testing.T) {
	valid := []string{
		"550e8400-e29b-41d4-a716-446655440000",
		"6ba7b810-9dad-11d1-80b4-00c04fd430c8",
		"f47ac10b-58cc-4372-a567-0e02b2c3d479",
		"ABCDEF01-2345-6789-ABCD-EF0123456789",
		"abcdef01-2345-6789-abcd-ef0123456789",
	}
	for _, u := range valid {
		if err := ValidateUUID(u); err != nil {
			t.Errorf("ValidateUUID(%q) returned error: %v", u, err)
		}
	}
}

func TestValidateUUID_Invalid(t *testing.T) {
	invalid := []struct {
		uuid string
		desc string
	}{
		{"", "empty"},
		{"not-a-uuid", "random string"},
		{"550e8400-e29b-41d4-a716", "too short"},
		{"550e8400-e29b-41d4-a716-446655440000-extra", "too long"},
		{"550e8400e29b41d4a716446655440000", "no hyphens"},
		{"550e8400-e29b-41d4-a716-44665544000g", "invalid hex char"},
		{"550e8400-e29b-41d4-a716-44665544000", "35 chars"},
		{"550e8400-e29b-41d4-a716-4466554400000", "37 chars"},
	}
	for _, tc := range invalid {
		if err := ValidateUUID(tc.uuid); err == nil {
			t.Errorf("ValidateUUID(%q) [%s] expected error, got nil", tc.uuid, tc.desc)
		}
	}
}

// --- ValidatePath tests ---

func TestValidatePath_Valid(t *testing.T) {
	valid := []struct {
		path string
		dirs []string
	}{
		{"/etc/nginx/conf.d/default.conf", []string{"/etc/nginx/"}},
		{"/etc/vasmax/config.yaml", []string{"/etc/vasmax/"}},
		{"/var/log/test.log", []string{"/var/log/", "/etc/"}},
		{"/some/path/file.txt", nil}, // no allowed dirs = skip dir check
	}
	for _, tc := range valid {
		if err := ValidatePath(tc.path, tc.dirs); err != nil {
			t.Errorf("ValidatePath(%q, %v) returned error: %v", tc.path, tc.dirs, err)
		}
	}
}

func TestValidatePath_Invalid(t *testing.T) {
	invalid := []struct {
		path string
		dirs []string
		desc string
	}{
		{"", nil, "empty path"},
		{"/etc/../passwd", []string{"/etc/"}, "path traversal with .."},
		{"../../../etc/passwd", []string{"/etc/"}, "relative path traversal"},
		{"/etc/nginx/../../shadow", []string{"/etc/nginx/"}, "traversal in middle"},
		{"/var/log/test.log", []string{"/etc/"}, "outside allowed dirs"},
		{strings.Repeat("a", MaxPathLength+1), nil, "exceeds max length"},
	}
	for _, tc := range invalid {
		if err := ValidatePath(tc.path, tc.dirs); err == nil {
			t.Errorf("ValidatePath(%q, %v) [%s] expected error, got nil", tc.path, tc.dirs, tc.desc)
		}
	}
}

// --- ValidateURL tests ---

func TestValidateURL_Valid(t *testing.T) {
	valid := []string{
		"https://example.com",
		"https://example.com/path",
		"https://example.com:8443/api/v1",
		"https://sub.domain.example.com/path?query=1",
		"https://192.168.1.1:443/test",
	}
	for _, u := range valid {
		if err := ValidateURL(u); err != nil {
			t.Errorf("ValidateURL(%q) returned error: %v", u, err)
		}
	}
}

func TestValidateURL_Invalid(t *testing.T) {
	invalid := []struct {
		url  string
		desc string
	}{
		{"", "empty"},
		{"http://example.com", "http not https"},
		{"ftp://example.com", "ftp scheme"},
		{"example.com", "no scheme"},
		{"https://", "no host"},
		{strings.Repeat("a", MaxURLLength+1), "exceeds max length"},
	}
	for _, tc := range invalid {
		if err := ValidateURL(tc.url); err == nil {
			t.Errorf("ValidateURL(%q) [%s] expected error, got nil", tc.url, tc.desc)
		}
	}
}

// --- Length limit tests ---

func TestValidateDomain_LengthLimit(t *testing.T) {
	// 253 chars is the max
	long := strings.Repeat("a", 62) + "." + strings.Repeat("b", 62) + "." + strings.Repeat("c", 62) + "." + strings.Repeat("d", 62)
	if len(long) > MaxDomainLength {
		t.Skipf("constructed domain is %d chars, adjusting", len(long))
	}
	if err := ValidateDomain(long); err != nil {
		t.Errorf("ValidateDomain with %d chars returned error: %v", len(long), err)
	}
}

func TestValidateUUID_ExactLength(t *testing.T) {
	// UUID must be exactly 36 characters
	short := "550e8400-e29b-41d4-a716-44665544000"
	if err := ValidateUUID(short); err == nil {
		t.Error("ValidateUUID with 35 chars expected error, got nil")
	}
}
