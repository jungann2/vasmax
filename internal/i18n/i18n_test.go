package i18n

import (
	"testing"
)

func TestT_DefaultLanguageChinese(t *testing.T) {
	SetLang("zh")
	got := T("menu.title")
	if got != "VasmaX 管理菜单" {
		t.Errorf("T(\"menu.title\") = %q, want %q", got, "VasmaX 管理菜单")
	}
}

func TestT_EnglishLanguage(t *testing.T) {
	SetLang("en")
	defer SetLang("zh")

	got := T("menu.title")
	if got != "VasmaX Management Menu" {
		t.Errorf("T(\"menu.title\") = %q, want %q", got, "VasmaX Management Menu")
	}
}

func TestT_FallbackReturnsKey(t *testing.T) {
	SetLang("zh")
	key := "nonexistent.key"
	got := T(key)
	if got != key {
		t.Errorf("T(%q) = %q, want key itself %q", key, got, key)
	}
}

func TestSetLang_InvalidIgnored(t *testing.T) {
	SetLang("zh")
	SetLang("fr") // invalid, should be ignored
	got := GetLang()
	if got != "zh" {
		t.Errorf("GetLang() = %q after SetLang(\"fr\"), want \"zh\"", got)
	}
}

func TestSetLang_SwitchLanguages(t *testing.T) {
	SetLang("en")
	if GetLang() != "en" {
		t.Errorf("GetLang() = %q, want \"en\"", GetLang())
	}
	SetLang("zh")
	if GetLang() != "zh" {
		t.Errorf("GetLang() = %q, want \"zh\"", GetLang())
	}
}

func TestT_AllCategories(t *testing.T) {
	SetLang("zh")
	keys := []string{
		"menu.title", "menu.install", "menu.exit",
		"error.invalid_input", "error.not_installed",
		"status.running", "status.stopped",
		"common.yes", "common.no",
		"install.title", "account.title", "xboard.title",
		"core.title", "routing.title",
	}
	for _, key := range keys {
		got := T(key)
		if got == key {
			t.Errorf("T(%q) returned key itself, expected a translation", key)
		}
	}
}

func TestZhAndEnKeysMatch(t *testing.T) {
	for key := range zhMessages {
		if _, ok := enMessages[key]; !ok {
			t.Errorf("key %q exists in zhMessages but not in enMessages", key)
		}
	}
	for key := range enMessages {
		if _, ok := zhMessages[key]; !ok {
			t.Errorf("key %q exists in enMessages but not in zhMessages", key)
		}
	}
}
