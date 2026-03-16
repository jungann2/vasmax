package config

import (
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// TLS certificate paths for various panel integrations.
const (
	DefaultTLSDir = "/etc/vasmax/tls/"
	BTCertDir     = "/www/server/panel/vhost/cert/"
	OnePanelDir   = "/opt/1panel/apps/openresty/openresty/www/sites/"
)

// CertInfo holds information about a TLS certificate.
type CertInfo struct {
	CertFile string
	KeyFile  string
	Domain   string
	NotAfter time.Time
	DaysLeft int
	IsValid  bool
}

// CheckCertificate reads and validates a TLS certificate file.
func CheckCertificate(certFile string) (*CertInfo, error) {
	data, err := os.ReadFile(certFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read certificate: %w", err)
	}

	block, _ := pem.Decode(data)
	if block == nil {
		return nil, fmt.Errorf("failed to decode PEM block")
	}

	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse certificate: %w", err)
	}

	daysLeft := int(time.Until(cert.NotAfter).Hours() / 24)

	info := &CertInfo{
		CertFile: certFile,
		Domain:   cert.Subject.CommonName,
		NotAfter: cert.NotAfter,
		DaysLeft: daysLeft,
		IsValid:  daysLeft > 0,
	}

	return info, nil
}

// DetectCertPath auto-detects TLS certificate paths from various sources.
// Priority: config > VasmaX default > acme.sh > BT panel > 1Panel > fullchain fallback.
func DetectCertPath(cfg *TLSConfig) (certFile, keyFile string) {
	// 1. Config file paths.
	if cfg.CertFile != "" && cfg.KeyFile != "" {
		if fileExistsCheck(cfg.CertFile) && fileExistsCheck(cfg.KeyFile) {
			return cfg.CertFile, cfg.KeyFile
		}
	}

	domain := cfg.Domain
	if domain == "" {
		return "", ""
	}

	// 2. VasmaX default path (fullchain preferred).
	defFullchain := filepath.Join(DefaultTLSDir, domain+".fullchain.crt")
	defKey := filepath.Join(DefaultTLSDir, domain+".key")
	if fileExistsCheck(defFullchain) && fileExistsCheck(defKey) {
		return defFullchain, defKey
	}
	defCert := filepath.Join(DefaultTLSDir, domain+".crt")
	if fileExistsCheck(defCert) && fileExistsCheck(defKey) {
		return defCert, defKey
	}

	// 3. acme.sh ECC cert directory.
	acmeECCDir := filepath.Join("/root/.acme.sh", domain+"_ecc")
	acmeCert := filepath.Join(acmeECCDir, domain+".cer")
	acmeFullchain := filepath.Join(acmeECCDir, "fullchain.cer")
	acmeKey := filepath.Join(acmeECCDir, domain+".key")
	if fileExistsCheck(acmeFullchain) && fileExistsCheck(acmeKey) {
		return acmeFullchain, acmeKey
	}
	if fileExistsCheck(acmeCert) && fileExistsCheck(acmeKey) {
		return acmeCert, acmeKey
	}

	// 4. acme.sh RSA cert directory.
	acmeRSADir := filepath.Join("/root/.acme.sh", domain)
	acmeRSACert := filepath.Join(acmeRSADir, domain+".cer")
	acmeRSAFullchain := filepath.Join(acmeRSADir, "fullchain.cer")
	acmeRSAKey := filepath.Join(acmeRSADir, domain+".key")
	if fileExistsCheck(acmeRSAFullchain) && fileExistsCheck(acmeRSAKey) {
		return acmeRSAFullchain, acmeRSAKey
	}
	if fileExistsCheck(acmeRSACert) && fileExistsCheck(acmeRSAKey) {
		return acmeRSACert, acmeRSAKey
	}

	// 5. BT panel paths.
	btCert := filepath.Join(BTCertDir, domain, "fullchain.pem")
	btKey := filepath.Join(BTCertDir, domain, "privkey.pem")
	if fileExistsCheck(btCert) && fileExistsCheck(btKey) {
		return btCert, btKey
	}

	// 6. 1Panel paths.
	oneCert := filepath.Join(OnePanelDir, domain, "ssl", "fullchain.pem")
	oneKey := filepath.Join(OnePanelDir, domain, "ssl", "privkey.pem")
	if fileExistsCheck(oneCert) && fileExistsCheck(oneKey) {
		return oneCert, oneKey
	}

	return "", ""
}

// DetectAcmeCert checks if acme.sh has a cert for the domain (ECC or RSA).
// Returns true and the acme.sh cert directory if found.
func DetectAcmeCert(domain string) (found bool, certDir string) {
	eccDir := filepath.Join("/root/.acme.sh", domain+"_ecc")
	if fileExistsCheck(filepath.Join(eccDir, domain+".key")) {
		return true, eccDir
	}
	rsaDir := filepath.Join("/root/.acme.sh", domain)
	if fileExistsCheck(filepath.Join(rsaDir, domain+".key")) {
		return true, rsaDir
	}
	return false, ""
}

// EnsureKeyPermissions ensures the private key file has 0600 permissions.
func EnsureKeyPermissions(keyFile string) error {
	return os.Chmod(keyFile, 0600)
}

func fileExistsCheck(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
