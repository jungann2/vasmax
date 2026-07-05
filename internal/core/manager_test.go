package core

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"vasmax/internal/config"
)

func TestSingBoxConfigFileForConfDir(t *testing.T) {
	tests := []struct {
		name    string
		confDir string
		want    string
	}{
		{
			name:    "default",
			confDir: "/etc/vasmax/sing-box/conf/config/",
			want:    filepath.Join(filepath.Clean("/etc/vasmax/sing-box/conf"), "config.json"),
		},
		{
			name:    "without trailing slash",
			confDir: "/etc/vasmax/sing-box/conf/config",
			want:    filepath.Join(filepath.Clean("/etc/vasmax/sing-box/conf"), "config.json"),
		},
		{
			name:    "custom",
			confDir: "/opt/vasmax/sing-box/partials/",
			want:    filepath.Join(filepath.Clean("/opt/vasmax/sing-box"), "config.json"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := singBoxConfigFileForConfDir(tt.confDir); got != tt.want {
				t.Fatalf("expected %q, got %q", tt.want, got)
			}
		})
	}
}

func TestUninstallCoreIgnoresStopDisableFailures(t *testing.T) {
	root := t.TempDir()
	binaryPath := filepath.Join(root, "xray")
	if err := os.WriteFile(binaryPath, []byte("core"), 0755); err != nil {
		t.Fatal(err)
	}
	oldRun := coreCommandRun
	coreCommandRun = func(name string, args ...string) error {
		if name == "systemctl" && len(args) > 0 {
			switch args[0] {
			case "stop", "disable":
				return errors.New("transient systemctl failure")
			}
		}
		return nil
	}
	t.Cleanup(func() { coreCommandRun = oldRun })

	m := NewManager(&config.Config{}, nil)
	if err := m.uninstallCore("vasmax-test-does-not-exist.service", binaryPath); err != nil {
		t.Fatalf("stop/disable failures should not fail uninstall: %v", err)
	}
	if _, err := os.Stat(binaryPath); !os.IsNotExist(err) {
		t.Fatalf("expected binary removed, stat err=%v", err)
	}
}

func TestSyncConfiguredPathsUpdatesSingBoxConfigFile(t *testing.T) {
	cfg := &config.Config{
		Paths: config.PathsConfig{
			SingBoxConf: "/custom/sing-box/conf/config/",
		},
	}
	m := NewManager(cfg, nil)

	want := singBoxConfigFileForConfDir(cfg.Paths.SingBoxConf)
	if m.singbox.ConfigFile != want {
		t.Fatalf("expected sing-box config file %q, got %q", want, m.singbox.ConfigFile)
	}
}

func TestInstallGeoDataCronWritesValidCronDEntry(t *testing.T) {
	oldPath := geoDataCronPath
	oldLogDir := geoDataLogDir
	geoDataCronPath = filepath.Join(t.TempDir(), "VasmaX-geodata")
	geoDataLogDir = filepath.Join(t.TempDir(), "log")
	t.Cleanup(func() {
		geoDataCronPath = oldPath
		geoDataLogDir = oldLogDir
	})

	if err := InstallGeoDataCron(); err != nil {
		t.Fatalf("install geodata cron failed: %v", err)
	}
	if _, err := os.Stat(geoDataLogDir); err != nil {
		t.Fatalf("expected geodata log dir created: %v", err)
	}
	data, err := os.ReadFile(geoDataCronPath)
	if err != nil {
		t.Fatal(err)
	}
	content := string(data)
	if !strings.Contains(content, " root /usr/local/bin/VasmaX -c /etc/vasmax/config.yaml --update-geodata ") {
		t.Fatalf("cron.d entry must include user field and update command, got:\n%s", content)
	}
	if !strings.Contains(content, "geodata-update.log") {
		t.Fatalf("expected cron output log path, got:\n%s", content)
	}
}

func TestSingBoxServiceRunsMergedConfigFile(t *testing.T) {
	cfg := &config.Config{
		Paths: config.PathsConfig{
			SingBoxConf: "/custom/sing-box/conf/config/",
		},
	}
	m := NewManager(cfg, nil)
	content := m.singBoxServiceContent()

	if !strings.Contains(content, " run -c "+m.singbox.ConfigFile) {
		t.Fatalf("service should run merged config file %q:\n%s", m.singbox.ConfigFile, content)
	}
	if strings.Contains(content, " run -C ") {
		t.Fatalf("service must not run partial config directory:\n%s", content)
	}
}

func TestUnusedCoreServices(t *testing.T) {
	tests := []struct {
		name   string
		needed map[string]bool
		want   []string
	}{
		{name: "xray only", needed: map[string]bool{"xray": true}, want: []string{"sing-box.service"}},
		{name: "singbox only", needed: map[string]bool{"singbox": true}, want: []string{"xray.service"}},
		{name: "both", needed: map[string]bool{"xray": true, "singbox": true}, want: nil},
		{name: "none", needed: map[string]bool{}, want: []string{"xray.service", "sing-box.service"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := unusedCoreServices(tt.needed, "xray.service", "sing-box.service")
			if strings.Join(got, ",") != strings.Join(tt.want, ",") {
				t.Fatalf("expected %#v, got %#v", tt.want, got)
			}
		})
	}
}

func TestCoreNeededFollowsConfiguredProtocols(t *testing.T) {
	tests := []struct {
		name      string
		protocols []string
		coreType  string
		want      bool
	}{
		{name: "xray reality", protocols: []string{"vless_reality_vision"}, coreType: "xray", want: true},
		{name: "xray not needed", protocols: []string{"anytls"}, coreType: "xray", want: false},
		{name: "singbox anytls", protocols: []string{"anytls"}, coreType: "singbox", want: true},
		{name: "singbox not needed", protocols: []string{"vless_reality_vision"}, coreType: "singbox", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := NewManager(&config.Config{Protocols: tt.protocols}, nil)
			if got := m.coreNeeded(tt.coreType); got != tt.want {
				t.Fatalf("expected %s needed=%t, got %t", tt.coreType, tt.want, got)
			}
		})
	}
}
