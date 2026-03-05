package subscription

import (
	"crypto/md5"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"vasmax/internal/security"
)

const (
	// SaltMinLength Salt 最小长度
	SaltMinLength = 8
	// SaltFilePath Salt 持久化文件相对路径
	SaltFilePath = "subscribe_local/subscribeSalt"
)

// GenerateSubscribePath 使用 MD5(email + salt) 生成订阅路径
func GenerateSubscribePath(email, salt string) string {
	h := md5.Sum([]byte(email + salt))
	return hex.EncodeToString(h[:])
}

// GenerateSalt 生成至少 8 字符的随机 Salt
func GenerateSalt() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// LoadOrCreateSalt 从文件加载 Salt，不存在则生成并保存
func LoadOrCreateSalt(baseDir string) (string, error) {
	saltPath := filepath.Join(baseDir, SaltFilePath)
	data, err := os.ReadFile(saltPath)
	if err == nil {
		salt := strings.TrimSpace(string(data))
		if len(salt) >= SaltMinLength {
			return salt, nil
		}
	}
	salt := GenerateSalt()
	if err := os.MkdirAll(filepath.Dir(saltPath), 0755); err != nil {
		return salt, fmt.Errorf("failed to create salt directory: %w", err)
	}
	if err := security.AtomicWrite(saltPath, []byte(salt), 0600); err != nil {
		return salt, fmt.Errorf("failed to save salt: %w", err)
	}
	return salt, nil
}

// SubscribeURL 生成完整订阅 URL
func SubscribeURL(domain, format, emailMd5 string) string {
	return fmt.Sprintf("https://%s/s/%s/%s", domain, format, emailMd5)
}
