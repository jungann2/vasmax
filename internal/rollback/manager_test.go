package rollback

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func withRollbackTestState(t *testing.T, cores []coreSnapshotTarget, services []serviceSnapshotTarget, configs []configSnapshotTarget, run func(string, ...string) error) {
	t.Helper()
	oldCores := coreSnapshotTargets
	oldServices := serviceSnapshotTargets
	oldConfigs := configSnapshotTargets
	oldRun := rollbackCommandRun
	oldOutput := rollbackCommandOutput
	coreSnapshotTargets = cores
	serviceSnapshotTargets = services
	configSnapshotTargets = configs
	rollbackCommandRun = run
	rollbackCommandOutput = func(string, ...string) ([]byte, error) { return nil, errors.New("no version") }
	t.Cleanup(func() {
		coreSnapshotTargets = oldCores
		serviceSnapshotTargets = oldServices
		configSnapshotTargets = oldConfigs
		rollbackCommandRun = oldRun
		rollbackCommandOutput = oldOutput
	})
}

func TestSnapshotItemsUseUniqueBackupPaths(t *testing.T) {
	m := NewManager(t.TempDir(), nil)
	items := m.snapshotItems(
		"/etc/vasmax/xray/conf/",
		"/etc/vasmax/sing-box/conf/",
	)

	seen := make(map[string]string)
	for _, item := range items {
		if previous, ok := seen[item.BackupPath]; ok {
			t.Fatalf("backup path collision: %s used for %s and %s", item.BackupPath, previous, item.Path)
		}
		seen[item.BackupPath] = item.Path
	}
}

func TestRestorePathReplacesDirectoryWithoutNesting(t *testing.T) {
	root := t.TempDir()
	backup := filepath.Join(root, "backup")
	target := filepath.Join(root, "target")

	if err := os.MkdirAll(filepath.Join(backup, "nested"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(backup, "nested", "new.txt"), []byte("new"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(target, "old"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(target, "old", "old.txt"), []byte("old"), 0644); err != nil {
		t.Fatal(err)
	}

	if err := restorePath(backup, target); err != nil {
		t.Fatalf("restore failed: %v", err)
	}

	if _, err := os.Stat(filepath.Join(target, "nested", "new.txt")); err != nil {
		t.Fatalf("expected restored file at target root: %v", err)
	}
	if _, err := os.Stat(filepath.Join(target, filepath.Base(backup), "nested", "new.txt")); !os.IsNotExist(err) {
		t.Fatalf("restore nested backup directory unexpectedly, stat err=%v", err)
	}
	if _, err := os.Stat(filepath.Join(target, "old", "old.txt")); !os.IsNotExist(err) {
		t.Fatalf("old target content should have been moved aside, stat err=%v", err)
	}
}

func TestCleanSnapshotRemovesBackups(t *testing.T) {
	root := t.TempDir()
	backup := filepath.Join(root, "config.yaml.bak")
	if err := os.WriteFile(backup, []byte("backup"), 0644); err != nil {
		t.Fatal(err)
	}
	snapFile := filepath.Join(root, "snapshot.json")
	if err := os.WriteFile(snapFile, []byte("{}"), 0644); err != nil {
		t.Fatal(err)
	}

	m := NewManager(root, nil)
	if err := m.CleanSnapshot(&Snapshot{
		ConfigBackups: []BackupItem{{Path: "/etc/vasmax/config.yaml", BackupPath: backup}},
	}); err != nil {
		t.Fatalf("clean failed: %v", err)
	}
	if _, err := os.Stat(backup); !os.IsNotExist(err) {
		t.Fatalf("expected backup removed, stat err=%v", err)
	}
	if _, err := os.Stat(snapFile); !os.IsNotExist(err) {
		t.Fatalf("expected snapshot metadata removed, stat err=%v", err)
	}
}

func TestRollbackRemovesCoreAndServiceCreatedAfterSnapshot(t *testing.T) {
	root := t.TempDir()
	binaryPath := filepath.Join(root, "xray")
	servicePath := filepath.Join(root, "xray.service")
	var commands []string
	withRollbackTestState(t,
		[]coreSnapshotTarget{{name: "xray", binaryPath: binaryPath}},
		[]serviceSnapshotTarget{{name: "xray.service", servicePath: servicePath}},
		nil,
		func(name string, args ...string) error {
			commands = append(commands, name+" "+strings.Join(args, " "))
			return nil
		},
	)

	m := NewManager(filepath.Join(root, "snap"), nil)
	snap, err := m.CreateSnapshot()
	if err != nil {
		t.Fatalf("snapshot failed: %v", err)
	}

	if err := os.WriteFile(binaryPath, []byte("new core"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(binaryPath+".bak", []byte("new backup"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(servicePath, []byte("new service"), 0644); err != nil {
		t.Fatal(err)
	}

	if err := m.Rollback(snap); err != nil {
		t.Fatalf("rollback failed: %v", err)
	}
	if _, err := os.Stat(binaryPath); !os.IsNotExist(err) {
		t.Fatalf("expected new binary removed, stat err=%v", err)
	}
	if _, err := os.Stat(binaryPath + ".bak"); !os.IsNotExist(err) {
		t.Fatalf("expected new backup binary removed, stat err=%v", err)
	}
	if _, err := os.Stat(servicePath); !os.IsNotExist(err) {
		t.Fatalf("expected new service removed, stat err=%v", err)
	}
	if !containsCommand(commands, "systemctl daemon-reload") {
		t.Fatalf("expected daemon-reload command, got %#v", commands)
	}
}

func TestRollbackRestoresExistingCoreServiceAndState(t *testing.T) {
	root := t.TempDir()
	binaryPath := filepath.Join(root, "xray")
	servicePath := filepath.Join(root, "xray.service")
	if err := os.WriteFile(binaryPath, []byte("old core"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(servicePath, []byte("old service"), 0644); err != nil {
		t.Fatal(err)
	}
	var commands []string
	withRollbackTestState(t,
		[]coreSnapshotTarget{{name: "xray", binaryPath: binaryPath}},
		[]serviceSnapshotTarget{{name: "xray.service", servicePath: servicePath}},
		nil,
		func(name string, args ...string) error {
			joined := name + " " + strings.Join(args, " ")
			commands = append(commands, joined)
			return nil
		},
	)

	m := NewManager(filepath.Join(root, "snap"), nil)
	snap, err := m.CreateSnapshot()
	if err != nil {
		t.Fatalf("snapshot failed: %v", err)
	}
	if err := os.WriteFile(binaryPath, []byte("new core"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(servicePath, []byte("new service"), 0644); err != nil {
		t.Fatal(err)
	}

	if err := m.Rollback(snap); err != nil {
		t.Fatalf("rollback failed: %v", err)
	}
	if got := readFileString(t, binaryPath); got != "old core" {
		t.Fatalf("expected old core restored, got %q", got)
	}
	if got := readFileString(t, servicePath); got != "old service" {
		t.Fatalf("expected old service restored, got %q", got)
	}
	for _, want := range []string{"systemctl enable xray.service", "systemctl restart xray.service"} {
		if !containsCommand(commands, want) {
			t.Fatalf("expected %q in commands %#v", want, commands)
		}
	}
}

func TestRollbackRestoresTLSDirectory(t *testing.T) {
	root := t.TempDir()
	tlsDir := filepath.Join(root, "tls")
	if err := os.MkdirAll(tlsDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tlsDir, "old.crt"), []byte("old"), 0644); err != nil {
		t.Fatal(err)
	}
	withRollbackTestState(t, nil, nil, []configSnapshotTarget{{name: "tls", path: tlsDir}}, func(string, ...string) error { return nil })

	m := NewManager(filepath.Join(root, "snap"), nil)
	snap, err := m.CreateSnapshot()
	if err != nil {
		t.Fatalf("snapshot failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tlsDir, "old.crt"), []byte("changed"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tlsDir, "new.key"), []byte("new"), 0600); err != nil {
		t.Fatal(err)
	}

	if err := m.Rollback(snap); err != nil {
		t.Fatalf("rollback failed: %v", err)
	}
	if got := readFileString(t, filepath.Join(tlsDir, "old.crt")); got != "old" {
		t.Fatalf("expected old cert restored, got %q", got)
	}
	if _, err := os.Stat(filepath.Join(tlsDir, "new.key")); !os.IsNotExist(err) {
		t.Fatalf("expected new cert removed, stat err=%v", err)
	}
}

func TestRollbackRemovesConfigDirectoryCreatedAfterSnapshot(t *testing.T) {
	root := t.TempDir()
	confDir := filepath.Join(root, "xray-conf")
	withRollbackTestState(t, nil, nil, []configSnapshotTarget{{name: "xray_conf", path: confDir}}, func(string, ...string) error { return nil })

	m := NewManager(filepath.Join(root, "snap"), nil)
	snap, err := m.CreateSnapshot()
	if err != nil {
		t.Fatalf("snapshot failed: %v", err)
	}
	if err := os.MkdirAll(confDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(confDir, "05_vless_reality_vision_inbounds.json"), []byte("{}"), 0644); err != nil {
		t.Fatal(err)
	}

	if err := m.Rollback(snap); err != nil {
		t.Fatalf("rollback failed: %v", err)
	}
	if _, err := os.Stat(confDir); !os.IsNotExist(err) {
		t.Fatalf("expected newly created config dir removed, stat err=%v", err)
	}
}

func TestRollbackDoesNotRestartActiveServiceWhenConfigRestoreFails(t *testing.T) {
	root := t.TempDir()
	servicePath := filepath.Join(root, "xray.service")
	if err := os.WriteFile(servicePath, []byte("new service"), 0644); err != nil {
		t.Fatal(err)
	}

	var commands []string
	withRollbackTestState(t, nil, nil, nil, func(name string, args ...string) error {
		commands = append(commands, name+" "+strings.Join(args, " "))
		return nil
	})

	m := NewManager(filepath.Join(root, "snap"), nil)
	err := m.Rollback(&Snapshot{
		ConfigBackups: []BackupItem{{
			Path:       filepath.Join(root, "config.yaml"),
			BackupPath: filepath.Join(root, "missing-config-backup"),
			Existed:    true,
		}},
		ServiceStates: []ServiceState{{
			Name:        "xray.service",
			ServicePath: servicePath,
			Existed:     false,
			Enabled:     false,
			Active:      true,
		}},
	})
	if err == nil {
		t.Fatal("expected rollback error from failed config restore")
	}
	if containsCommand(commands, "systemctl restart xray.service") {
		t.Fatalf("active service must not restart after critical restore failure, commands=%#v", commands)
	}
	if !containsCommand(commands, "systemctl stop xray.service") {
		t.Fatalf("expected service stop during rollback, commands=%#v", commands)
	}
}

func containsCommand(commands []string, want string) bool {
	for _, cmd := range commands {
		if cmd == want {
			return true
		}
	}
	return false
}

func readFileString(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}
