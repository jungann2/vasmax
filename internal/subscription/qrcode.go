package subscription

import (
	"fmt"
	"net/url"

	qrcode "github.com/skip2/go-qrcode"
)

// GenerateTerminalQR 使用 go-qrcode 库生成终端 UTF8 二维码
func GenerateTerminalQR(content string) string {
	qr, err := qrcode.New(content, qrcode.Medium)
	if err != nil {
		return fmt.Sprintf("failed to generate QR code: %v", err)
	}
	return qr.ToSmallString(false)
}

// GenerateOnlineQRURL 生成在线二维码 URL（api.qrserver.com）
func GenerateOnlineQRURL(content string) string {
	encoded := url.QueryEscape(content)
	return fmt.Sprintf("https://api.qrserver.com/v1/create-qr-code/?size=400x400&data=%s", encoded)
}
