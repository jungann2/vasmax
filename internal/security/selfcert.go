package security

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"os"
	"path/filepath"
	"time"
)

// SelfSignedCertPaths 自签证书文件路径
type SelfSignedCertPaths struct {
	CertFile string
	KeyFile  string
}

// EnsureSelfSignedCert 确保自签证书存在，如果已存在则跳过
// 返回证书和私钥文件路径
func EnsureSelfSignedCert(certDir string) (*SelfSignedCertPaths, error) {
	certFile := filepath.Join(certDir, "self-signed.crt")
	keyFile := filepath.Join(certDir, "self-signed.key")

	// 如果已存在则直接返回
	if fileExists(certFile) && fileExists(keyFile) {
		return &SelfSignedCertPaths{CertFile: certFile, KeyFile: keyFile}, nil
	}

	// 确保目录存在
	if err := os.MkdirAll(certDir, 0755); err != nil {
		return nil, err
	}

	// 生成 ECDSA P-256 私钥
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, err
	}

	// 生成证书模板（有效期 10 年）
	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			Organization: []string{"VasmaX Self-Signed"},
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(10 * 365 * 24 * time.Hour),
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
	}

	// 自签名
	certDER, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		return nil, err
	}

	// 写入证书文件
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})
	if err := AtomicWrite(certFile, certPEM, 0644); err != nil {
		return nil, err
	}

	// 写入私钥文件
	keyDER, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		return nil, err
	}
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})
	if err := AtomicWrite(keyFile, keyPEM, 0600); err != nil {
		return nil, err
	}

	return &SelfSignedCertPaths{CertFile: certFile, KeyFile: keyFile}, nil
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
