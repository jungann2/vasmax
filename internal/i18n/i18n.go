// Package i18n provides multi-language support for VasmaX.
// It supports Chinese (zh) and English (en) with Chinese as the default language.
// Language can be set via SetLang() or the VASMAX_LANG environment variable.
package i18n

import (
	"os"
	"sync"
)

var (
	currentLang string
	mu          sync.RWMutex
	messages    map[string]map[string]string // lang -> key -> text
)

func init() {
	messages = map[string]map[string]string{
		"zh": zhMessages,
		"en": enMessages,
	}

	// Default to Chinese; allow override via environment variable.
	currentLang = "zh"
	if lang := os.Getenv("VASMAX_LANG"); lang == "en" || lang == "zh" {
		currentLang = lang
	}
}

// T returns the translated text for the given key in the current language.
// If the key is not found, the key itself is returned as a fallback.
func T(key string) string {
	mu.RLock()
	lang := currentLang
	mu.RUnlock()

	if m, ok := messages[lang]; ok {
		if text, ok := m[key]; ok {
			return text
		}
	}
	return key
}

// SetLang sets the current language. Supported values: "zh", "en".
// Unrecognized values are ignored and the language remains unchanged.
func SetLang(lang string) {
	if lang != "zh" && lang != "en" {
		return
	}
	mu.Lock()
	currentLang = lang
	mu.Unlock()
}

// GetLang returns the current language code.
func GetLang() string {
	mu.RLock()
	defer mu.RUnlock()
	return currentLang
}
