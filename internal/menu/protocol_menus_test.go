package menu

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"vasmax/internal/config"
	"vasmax/internal/protocol"
)

func TestRealityTargetPoolSupportedOnlyForVision(t *testing.T) {
	tests := []struct {
		name      string
		protocols []string
		want      bool
	}{
		{name: "vision", protocols: []string{"vless_reality_vision"}, want: true},
		{name: "grpc only", protocols: []string{"vless_reality_grpc"}, want: false},
		{name: "xhttp only", protocols: []string{"vless_reality_xhttp"}, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &ProtocolMenus{config: &config.Config{Protocols: tt.protocols}}
			if got := m.realityTargetPoolSupported(); got != tt.want {
				t.Fatalf("expected %t, got %t", tt.want, got)
			}
		})
	}
}

func TestResolveDisplayServerIPUsesSubscriptionServerIP(t *testing.T) {
	cfg := &config.Config{Subscription: config.SubscriptionConfig{ServerIP: "203.0.113.10"}}
	got, err := resolveDisplayServerIP(cfg)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if got != "203.0.113.10" {
		t.Fatalf("expected configured IP, got %q", got)
	}
}

func TestResolveDisplayServerIPRejectsInvalidSubscriptionServerIP(t *testing.T) {
	cfg := &config.Config{Subscription: config.SubscriptionConfig{ServerIP: "not-an-ip"}}
	if got, err := resolveDisplayServerIP(cfg); err == nil || got != "" {
		t.Fatalf("expected invalid configured IP error, got ip=%q err=%v", got, err)
	}
}

func TestResolveDisplayServerIPReturnsPlaceholderWhenDetectionFails(t *testing.T) {
	oldPublic := detectPublicServerIPFunc
	oldLocal := detectOutboundLocalIPFunc
	defer func() {
		detectPublicServerIPFunc = oldPublic
		detectOutboundLocalIPFunc = oldLocal
	}()
	detectPublicServerIPFunc = func() (string, error) { return "", errors.New("public failed") }
	detectOutboundLocalIPFunc = func() (string, error) { return "", errors.New("local failed") }

	got, err := resolveDisplayServerIP(&config.Config{})
	if err == nil {
		t.Fatal("expected explicit error when all detection fails")
	}
	if got != displayServerIPPlaceholder {
		t.Fatalf("expected placeholder, got %q", got)
	}
}

func TestExpandRealityVisionServerInfosUsesTargetPool(t *testing.T) {
	cfg := &config.Config{
		Reality: config.RealityConfig{
			Targets: []config.RealityTarget{
				{Name: "one", ServerName: "one.example.com", Dest: "one.example.com:443", Port: 31305},
				{Name: "two", ServerName: "two.example.com", Dest: "two.example.com:443", Port: 31306},
			},
		},
	}
	info := &protocol.ServerInfo{Host: "203.0.113.10", Port: 31305, Reality: &cfg.Reality}

	got := expandRealityVisionServerInfos("vless_reality_vision", info, cfg)
	if len(got) != 2 {
		t.Fatalf("expected two expanded Reality infos, got %d", len(got))
	}
	if got[0].Port != 31305 || got[1].Port != 31306 {
		t.Fatalf("unexpected expanded ports: %d, %d", got[0].Port, got[1].Port)
	}
	if got[0].Reality.ServerName != "one.example.com" || got[1].Reality.ServerName != "two.example.com" {
		t.Fatalf("unexpected expanded server names: %s, %s", got[0].Reality.ServerName, got[1].Reality.ServerName)
	}
}

func TestProtocolsNeedDisplayIPOnlyForNoDomain(t *testing.T) {
	if protocolsNeedDisplayIP(&config.Config{
		Protocols:     []string{"vless_reality_vision"},
		ProtocolModes: map[string]string{"vless_reality_vision": "domain"},
	}) {
		t.Fatal("domain-mode Reality should not need public IP display")
	}
	if !protocolsNeedDisplayIP(&config.Config{
		Protocols:     []string{"anytls"},
		ProtocolModes: map[string]string{"anytls": "nodomain"},
	}) {
		t.Fatal("nodomain protocol should need public IP display")
	}
}

func TestInstallTransactionRestoresInMemoryConfigOnFailure(t *testing.T) {
	cfg := &config.Config{
		Protocols:     []string{"old"},
		ProtocolModes: map[string]string{"old": "domain"},
	}
	m := &InstallMenu{config: cfg}
	finish, ok := m.beginInstallTransaction("test install")
	if !ok {
		t.Fatal("expected transaction to start")
	}

	cfg.Protocols = append(cfg.Protocols, "new")
	cfg.ProtocolModes["new"] = "nodomain"
	finish(false)

	if len(cfg.Protocols) != 1 || cfg.Protocols[0] != "old" {
		t.Fatalf("expected protocols restored, got %#v", cfg.Protocols)
	}
	if _, ok := cfg.ProtocolModes["new"]; ok {
		t.Fatalf("expected new mode removed, got %#v", cfg.ProtocolModes)
	}
}

func TestApplyAndSyncRealityRuntimeDoesNotMutateConfigOnRuntimeFailure(t *testing.T) {
	cfg := &config.Config{
		Protocols: []string{"vless_reality_grpc"},
		Paths:     config.PathsConfig{XrayConf: t.TempDir()},
		Reality: config.RealityConfig{
			Dest:       "old.example.com:443",
			ServerName: "old.example.com",
		},
	}

	ok := applyAndSyncRealityRuntime(nil, cfg, nil, func(next *config.Config) {
		setSingleRealityTarget(next, "new.example.com:443", "new.example.com")
	})
	if ok {
		t.Fatal("expected runtime sync failure")
	}
	if cfg.Reality.Dest != "old.example.com:443" || cfg.Reality.ServerName != "old.example.com" {
		t.Fatalf("expected real cfg to remain unchanged, got dest=%q server=%q", cfg.Reality.Dest, cfg.Reality.ServerName)
	}
}

func TestNoDomainRealityVisionDefaultPortUsesPromptCopy(t *testing.T) {
	reg := protocol.DefaultRegistry()
	vision, ok := reg.Get("vless_reality_vision")
	if !ok {
		t.Fatal("missing vless_reality_vision protocol")
	}
	cfg := &config.Config{ProtocolPorts: map[string]int{}}
	promptCfg, err := cloneConfig(cfg)
	if err != nil {
		t.Fatal(err)
	}

	preferRealityVision443ForDirectOnly(promptCfg, []protocol.Protocol{vision})

	if cfg.ProtocolPorts["vless_reality_vision"] != 0 {
		t.Fatalf("real cfg should not be changed before transaction, got %d", cfg.ProtocolPorts["vless_reality_vision"])
	}
	if promptCfg.ProtocolPorts["vless_reality_vision"] != 443 {
		t.Fatalf("expected prompt cfg to default vision port to 443, got %d", promptCfg.ProtocolPorts["vless_reality_vision"])
	}
}

func TestPreferRealityVision443SkipsWhenNginxProxyExists(t *testing.T) {
	reg := protocol.DefaultRegistry()
	vision, ok := reg.Get("vless_reality_vision")
	if !ok {
		t.Fatal("missing vless_reality_vision protocol")
	}
	cfg := &config.Config{
		Protocols:     []string{"vless_ws_tls"},
		ProtocolPorts: map[string]int{},
	}

	preferRealityVision443ForDirectOnly(cfg, []protocol.Protocol{vision})

	if cfg.ProtocolPorts["vless_reality_vision"] != 0 {
		t.Fatalf("expected Reality Vision default port untouched when Nginx proxy exists, got %d", cfg.ProtocolPorts["vless_reality_vision"])
	}
}

func TestPreferRealityVision443SkipsWhenSubscriptionDomainUsesNginx(t *testing.T) {
	reg := protocol.DefaultRegistry()
	vision, ok := reg.Get("vless_reality_vision")
	if !ok {
		t.Fatal("missing vless_reality_vision protocol")
	}
	cfg := &config.Config{
		Subscription:  config.SubscriptionConfig{Domain: "sub.example.com"},
		ProtocolPorts: map[string]int{},
	}

	preferRealityVision443ForDirectOnly(cfg, []protocol.Protocol{vision})

	if cfg.ProtocolPorts["vless_reality_vision"] != 0 {
		t.Fatalf("expected Reality Vision default port untouched when subscription domain reserves 443, got %d", cfg.ProtocolPorts["vless_reality_vision"])
	}
	if !reservesNginxPublicPorts(cfg, []protocol.Protocol{vision}) {
		t.Fatal("expected subscription domain to reserve Nginx public ports")
	}
}

func TestSelectedProtocolRuntimeFlagsAnyTLSDoesNotRequireXrayOrReality(t *testing.T) {
	reg := protocol.DefaultRegistry()
	anytls, ok := reg.Get("anytls")
	if !ok {
		t.Fatal("missing anytls protocol")
	}
	hasSingBox, hasXray, hasReality := selectedProtocolRuntimeFlags([]protocol.Protocol{anytls})
	if !hasSingBox {
		t.Fatal("anytls should require sing-box")
	}
	if hasXray {
		t.Fatal("anytls-only install should not require Xray")
	}
	if hasReality {
		t.Fatal("anytls-only install should not generate Reality config")
	}
}

func TestDomainNeedsNginxProxy(t *testing.T) {
	reg := protocol.DefaultRegistry()
	ws, ok := reg.Get("vless_ws_tls")
	if !ok {
		t.Fatal("missing vless_ws_tls protocol")
	}
	if !domainNeedsNginxProxy([]protocol.Protocol{ws}, map[string]string{"vless_ws_tls": "node.example.com"}, "node.example.com") {
		t.Fatal("expected websocket TLS protocol to require Nginx for its domain")
	}
	if domainNeedsNginxProxy([]protocol.Protocol{ws}, map[string]string{"vless_ws_tls": "other.example.com"}, "node.example.com") {
		t.Fatal("protocol bound to a different domain should not require Nginx for this domain")
	}
}

func TestRewriteRealityVisionPoolToSingleCollapsesInbounds(t *testing.T) {
	confDir := t.TempDir()
	confPath := filepath.Join(confDir, "05_vless_reality_vision_inbounds.json")
	writeInboundDoc(t, confPath, []interface{}{
		testRealityInbound(31305, "vless_reality_vision_one", "old-one.example.com:443", "old-one.example.com"),
		testRealityInbound(31306, "vless_reality_vision_two", "old-two.example.com:443", "old-two.example.com"),
	})

	cfg := &config.Config{
		Protocols:     []string{"vless_reality_vision"},
		ProtocolPorts: map[string]int{"vless_reality_vision": 31305},
		Paths:         config.PathsConfig{XrayConf: confDir},
		Reality: config.RealityConfig{
			PrivateKey: "private",
			ShortID:    "abcd",
			Dest:       "new.example.com:8443",
			ServerName: "new.example.com",
		},
	}
	changed, err := rewriteRealityInboundConfigs(cfg)
	if err != nil {
		t.Fatalf("rewrite failed: %v", err)
	}
	if !changed {
		t.Fatal("expected rewrite to change target pool into single inbound")
	}

	inbounds := readInboundDoc(t, confPath)
	if len(inbounds) != 1 {
		t.Fatalf("expected exactly one inbound, got %d", len(inbounds))
	}
	inbound := inbounds[0]
	if inbound["tag"] != "vless_reality_vision" {
		t.Fatalf("expected single inbound tag, got %#v", inbound["tag"])
	}
	if port, _ := jsonNumberToInt(inbound["port"]); port != 31305 {
		t.Fatalf("expected port 31305, got %#v", inbound["port"])
	}
	reality := realitySettingsFromInbound(t, inbound)
	if reality["dest"] != "new.example.com:8443" {
		t.Fatalf("expected new dest, got %#v", reality["dest"])
	}
}

func TestRewriteRealityVisionSingleToPoolExpandsInbounds(t *testing.T) {
	confDir := t.TempDir()
	confPath := filepath.Join(confDir, "05_vless_reality_vision_inbounds.json")
	writeInboundDoc(t, confPath, []interface{}{
		testRealityInbound(31305, "vless_reality_vision", "old.example.com:443", "old.example.com"),
	})

	cfg := &config.Config{
		Protocols:     []string{"vless_reality_vision"},
		ProtocolPorts: map[string]int{"vless_reality_vision": 31305},
		Paths:         config.PathsConfig{XrayConf: confDir},
		Reality: config.RealityConfig{
			PrivateKey: "private",
			ShortID:    "abcd",
			Targets: []config.RealityTarget{
				{ServerName: "one.example.com", Dest: "one.example.com:443", Port: 31305},
				{ServerName: "two.example.com", Dest: "two.example.com:443", Port: 31306},
			},
		},
	}
	changed, err := rewriteRealityInboundConfigs(cfg)
	if err != nil {
		t.Fatalf("rewrite failed: %v", err)
	}
	if !changed {
		t.Fatal("expected rewrite to expand single inbound into target pool")
	}

	inbounds := readInboundDoc(t, confPath)
	if len(inbounds) != 2 {
		t.Fatalf("expected two target inbounds, got %d", len(inbounds))
	}
	for i, wantPort := range []int{31305, 31306} {
		if port, _ := jsonNumberToInt(inbounds[i]["port"]); port != wantPort {
			t.Fatalf("expected port %d at inbound %d, got %#v", wantPort, i, inbounds[i]["port"])
		}
	}
}

func TestMigrateRealityVisionTargetPoolDocumentPreservesTargets(t *testing.T) {
	doc := map[string]interface{}{
		"inbounds": []interface{}{
			testRealityInbound(31305, "vless_reality_vision_old", "old-one.example.com:443", "old-one.example.com"),
			testRealityInbound(31306, "vless_reality_vision_old2", "old-two.example.com:443", "old-two.example.com"),
		},
	}
	data, err := json.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	cfg := &config.Config{
		Reality: config.RealityConfig{
			PrivateKey: "private",
			ShortID:    "abcd",
			Targets: []config.RealityTarget{
				{ServerName: "one.example.com", Dest: "one.example.com:443", Port: 31305},
				{ServerName: "two.example.com", Dest: "two.example.com:443", Port: 31306},
			},
		},
	}
	newDoc, changed, newPort, err := migrateRealityVisionTargetPoolDocument(data, cfg, 32000, "vless_reality_vision")
	if err != nil {
		t.Fatalf("migrate failed: %v", err)
	}
	if !changed {
		t.Fatal("expected target pool migration to change document")
	}
	if newPort != 32000 {
		t.Fatalf("expected new base port 32000, got %d", newPort)
	}
	inbounds, ok := newDoc["inbounds"].([]interface{})
	if !ok {
		t.Fatalf("expected inbounds array, got %#v", newDoc["inbounds"])
	}
	if len(inbounds) != 2 {
		t.Fatalf("expected target count to stay 2, got %d", len(inbounds))
	}
	for i, raw := range inbounds {
		inbound := raw.(map[string]interface{})
		if port, _ := jsonNumberToInt(inbound["port"]); port != 32000+i {
			t.Fatalf("expected port %d, got %#v", 32000+i, inbound["port"])
		}
	}
	for i, target := range cfg.Reality.Targets {
		if target.Port != 32000+i {
			t.Fatalf("expected cfg target port %d, got %d", 32000+i, target.Port)
		}
	}
	if cfg.Reality.Port != 32000 {
		t.Fatalf("expected cfg reality base port 32000, got %d", cfg.Reality.Port)
	}
	if cfg.ProtocolPorts["vless_reality_vision"] != 32000 {
		t.Fatalf("expected protocol port 32000, got %d", cfg.ProtocolPorts["vless_reality_vision"])
	}
}

func TestRewriteRealityInboundConfigsErrorsOnMalformedRuntime(t *testing.T) {
	confDir := t.TempDir()
	confPath := filepath.Join(confDir, "05_vless_reality_grpc_inbounds.json")
	writeInboundDoc(t, confPath, []interface{}{
		map[string]interface{}{
			"port": float64(31306),
			"tag":  "vless_reality_grpc",
		},
	})

	cfg := &config.Config{
		Protocols: []string{"vless_reality_grpc"},
		Paths:     config.PathsConfig{XrayConf: confDir},
		Reality: config.RealityConfig{
			Dest:       "new.example.com:443",
			ServerName: "new.example.com",
		},
	}

	if _, err := rewriteRealityInboundConfigs(cfg); err == nil {
		t.Fatal("expected malformed runtime inbound to fail")
	}
}

func TestRewriteRealityInboundConfigsErrorsWhenRuntimeMissing(t *testing.T) {
	cfg := &config.Config{
		Protocols: []string{"vless_reality_grpc"},
		Paths:     config.PathsConfig{XrayConf: t.TempDir()},
		Reality: config.RealityConfig{
			Dest:       "new.example.com:443",
			ServerName: "new.example.com",
		},
	}

	if _, err := rewriteRealityInboundConfigs(cfg); err == nil {
		t.Fatal("expected missing runtime inbound to fail")
	}
}

func TestSyncRealityRuntimeRestoresPartialRewriteOnError(t *testing.T) {
	confDir := t.TempDir()
	visionPath := filepath.Join(confDir, "05_vless_reality_vision_inbounds.json")
	grpcPath := filepath.Join(confDir, "05_vless_reality_grpc_inbounds.json")
	writeInboundDoc(t, visionPath, []interface{}{
		testRealityInbound(31305, "vless_reality_vision", "old.example.com:443", "old.example.com"),
	})
	writeInboundDoc(t, grpcPath, []interface{}{
		map[string]interface{}{
			"port": float64(31306),
			"tag":  "vless_reality_grpc",
		},
	})

	cfg := &config.Config{
		Protocols: []string{"vless_reality_vision", "vless_reality_grpc"},
		Paths:     config.PathsConfig{XrayConf: confDir},
		Reality: config.RealityConfig{
			Dest:       "new.example.com:443",
			ServerName: "new.example.com",
		},
	}

	if _, err := SyncRealityRuntime(nil, cfg); err == nil {
		t.Fatal("expected malformed second runtime inbound to fail")
	}
	inbounds := readInboundDoc(t, visionPath)
	reality := realitySettingsFromInbound(t, inbounds[0])
	if reality["dest"] != "old.example.com:443" {
		t.Fatalf("expected first runtime file to be restored, got dest %#v", reality["dest"])
	}
}

func testRealityInbound(port int, tag, dest, serverName string) map[string]interface{} {
	return map[string]interface{}{
		"port": float64(port),
		"tag":  tag,
		"streamSettings": map[string]interface{}{
			"realitySettings": map[string]interface{}{
				"dest":        dest,
				"serverNames": []interface{}{serverName},
				"privateKey":  "old-private",
				"shortIds":    []interface{}{"old"},
			},
		},
	}
}

func writeInboundDoc(t *testing.T, path string, inbounds []interface{}) {
	t.Helper()
	data, err := json.Marshal(map[string]interface{}{"inbounds": inbounds})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatal(err)
	}
}

func readInboundDoc(t *testing.T, path string) []map[string]interface{} {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var doc struct {
		Inbounds []map[string]interface{} `json:"inbounds"`
	}
	if err := json.Unmarshal(data, &doc); err != nil {
		t.Fatal(err)
	}
	return doc.Inbounds
}

func realitySettingsFromInbound(t *testing.T, inbound map[string]interface{}) map[string]interface{} {
	t.Helper()
	stream, ok := inbound["streamSettings"].(map[string]interface{})
	if !ok {
		t.Fatalf("missing streamSettings: %#v", inbound)
	}
	reality, ok := stream["realitySettings"].(map[string]interface{})
	if !ok {
		t.Fatalf("missing realitySettings: %#v", stream)
	}
	return reality
}

func TestTLSConfigForDomainOnlyReusesMatchingConfiguredCertificate(t *testing.T) {
	base := config.TLSConfig{
		Domain:   "old.example.com",
		CertFile: "/etc/old.fullchain.crt",
		KeyFile:  "/etc/old.key",
		Provider: "letsencrypt",
	}
	mismatch := tlsConfigForDomain(base, "new.example.com")
	if mismatch.CertFile != "" || mismatch.KeyFile != "" {
		t.Fatalf("did not expect mismatched domain to reuse cert paths: %#v", mismatch)
	}
	if mismatch.Provider != base.Provider {
		t.Fatalf("expected provider copied for detection context")
	}

	match := tlsConfigForDomain(base, "old.example.com")
	if match.CertFile != base.CertFile || match.KeyFile != base.KeyFile {
		t.Fatalf("expected matching domain to reuse cert paths: %#v", match)
	}
}
