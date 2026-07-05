package menu

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"

	"vasmax/internal/config"
	"vasmax/internal/core"
	"vasmax/internal/security"
)

type realityRuntimeFileBackup struct {
	path string
	data []byte
}

func syncRealityRuntime(coreMgr *core.Manager, cfg *config.Config) (bool, error) {
	backups, backupErr := snapshotRealityRuntimeFiles(cfg)
	if backupErr != nil {
		return false, backupErr
	}
	changed, err := rewriteRealityInboundConfigs(cfg)
	if err != nil {
		restoreErr := restoreRealityRuntimeFiles(backups)
		return changed, fmt.Errorf("rewrite Reality runtime failed: %w; runtime rollback: %v", err, restoreErr)
	}
	if changed && coreMgr != nil {
		if err := coreMgr.RestartXray(); err != nil {
			restoreErr := restoreRealityRuntimeFiles(backups)
			var restartErr error
			if restoreErr == nil {
				restartErr = coreMgr.RestartXray()
			}
			return changed, fmt.Errorf("restart Xray after Reality runtime sync failed: %w; runtime rollback: %v", err, errors.Join(restoreErr, restartErr))
		}
	}
	return changed, nil
}

func snapshotRealityRuntimeFiles(cfg *config.Config) ([]realityRuntimeFileBackup, error) {
	if cfg == nil {
		return nil, nil
	}
	confDir := cfg.Paths.XrayConf
	if confDir == "" {
		confDir = "/etc/vasmax/xray/conf/"
	}
	backups := make([]realityRuntimeFileBackup, 0)
	seen := make(map[string]struct{})
	for _, protoName := range cfg.Protocols {
		if !strings.Contains(protoName, "reality") {
			continue
		}
		confPath := filepath.Join(confDir, fmt.Sprintf("05_%s_inbounds.json", protoName))
		if _, ok := seen[confPath]; ok {
			continue
		}
		seen[confPath] = struct{}{}
		data, err := os.ReadFile(confPath)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, err
		}
		backups = append(backups, realityRuntimeFileBackup{path: confPath, data: append([]byte(nil), data...)})
	}
	return backups, nil
}

func restoreRealityRuntimeFiles(backups []realityRuntimeFileBackup) error {
	var errs []error
	for _, backup := range backups {
		if err := security.AtomicWrite(backup.path, backup.data, 0644); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

func rewriteRealityInboundConfigs(cfg *config.Config) (bool, error) {
	if cfg == nil || (cfg.Reality.Dest == "" && cfg.Reality.ServerName == "" && len(cfg.Reality.Targets) == 0) {
		return false, nil
	}

	confDir := cfg.Paths.XrayConf
	if confDir == "" {
		confDir = "/etc/vasmax/xray/conf/"
	}

	changed := false
	for _, protoName := range cfg.Protocols {
		if !strings.Contains(protoName, "reality") {
			continue
		}

		confPath := filepath.Join(confDir, fmt.Sprintf("05_%s_inbounds.json", protoName))
		data, err := os.ReadFile(confPath)
		if err != nil {
			if os.IsNotExist(err) {
				return changed, fmt.Errorf("%s runtime inbound config not found: %s", protoName, confPath)
			}
			return changed, err
		}

		var doc map[string]interface{}
		if err := json.Unmarshal(data, &doc); err != nil {
			return changed, fmt.Errorf("parse %s: %w", confPath, err)
		}

		inbounds, ok := doc["inbounds"].([]interface{})
		if !ok || len(inbounds) == 0 {
			return changed, fmt.Errorf("%s runtime inbound config has no inbounds: %s", protoName, confPath)
		}

		fileChanged := false
		if protoName == "vless_reality_vision" && len(inbounds) > 0 {
			basePort := configuredProtocolPort(cfg, protoName, defaultProtocolPort(protoName))
			var targetInbounds []interface{}
			var err error
			if len(cfg.Reality.Targets) > 0 {
				targetInbounds, err = buildRealityTargetPoolInbounds(inbounds[0], cfg, basePort, protoName)
			} else {
				targetInbounds, err = buildSingleRealityInbound(inbounds[0], cfg, basePort, protoName)
			}
			if err != nil {
				return changed, err
			}
			if !reflect.DeepEqual(inbounds, targetInbounds) {
				doc["inbounds"] = targetInbounds
				fileChanged = true
			}
			if fileChanged {
				if err := security.AtomicWriteJSON(confPath, doc, 0644); err != nil {
					return changed, err
				}
				changed = true
			}
			continue
		}

		for _, rawInbound := range inbounds {
			inbound, ok := rawInbound.(map[string]interface{})
			if !ok {
				return changed, fmt.Errorf("%s inbound is not an object", protoName)
			}
			stream, ok := inbound["streamSettings"].(map[string]interface{})
			if !ok {
				return changed, fmt.Errorf("%s inbound missing streamSettings", protoName)
			}
			reality, ok := stream["realitySettings"].(map[string]interface{})
			if !ok {
				return changed, fmt.Errorf("%s inbound missing realitySettings", protoName)
			}

			if setMapValue(reality, "dest", cfg.Reality.Dest) {
				fileChanged = true
			}
			if cfg.Reality.PrivateKey != "" && setMapValue(reality, "privateKey", cfg.Reality.PrivateKey) {
				fileChanged = true
			}
			if setMapValue(reality, "serverNames", []interface{}{cfg.Reality.ServerName}) {
				fileChanged = true
			}
			if cfg.Reality.ShortID != "" && setMapValue(reality, "shortIds", []interface{}{cfg.Reality.ShortID}) {
				fileChanged = true
			}
		}

		if fileChanged {
			if err := security.AtomicWriteJSON(confPath, doc, 0644); err != nil {
				return changed, err
			}
			changed = true
		}
	}

	return changed, nil
}

func buildSingleRealityInbound(template interface{}, cfg *config.Config, basePort int, protoName string) ([]interface{}, error) {
	targets := cfg.Reality.EffectiveTargets(basePort)
	if len(targets) == 0 {
		return nil, nil
	}
	next, err := cloneJSONMap(template)
	if err != nil {
		return nil, err
	}
	target := targets[0]
	next["port"] = float64(target.Port)
	next["tag"] = protoName
	if err := applyRealityTargetToInbound(next, cfg, target); err != nil {
		return nil, err
	}
	return []interface{}{next}, nil
}

func buildRealityTargetPoolInbounds(template interface{}, cfg *config.Config, basePort int, protoName string) ([]interface{}, error) {
	targets := cfg.Reality.EffectiveTargets(basePort)
	result := make([]interface{}, 0, len(targets))
	for _, target := range targets {
		next, err := cloneJSONMap(template)
		if err != nil {
			return nil, err
		}
		next["port"] = float64(target.Port)
		next["tag"] = fmt.Sprintf("%s_%s", protoName, config.RealityTargetName(target.ServerName))
		if err := applyRealityTargetToInbound(next, cfg, target); err != nil {
			return nil, err
		}
		result = append(result, next)
	}
	return result, nil
}

func cloneJSONMap(v interface{}) (map[string]interface{}, error) {
	data, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	var out map[string]interface{}
	if err := json.Unmarshal(data, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func applyRealityTargetToInbound(inbound map[string]interface{}, cfg *config.Config, target config.RealityTarget) error {
	stream, ok := inbound["streamSettings"].(map[string]interface{})
	if !ok {
		return fmt.Errorf("missing streamSettings in Reality inbound")
	}
	reality, ok := stream["realitySettings"].(map[string]interface{})
	if !ok {
		return fmt.Errorf("missing realitySettings in Reality inbound")
	}
	reality["dest"] = target.Dest
	reality["serverNames"] = []interface{}{target.ServerName}
	if cfg.Reality.PrivateKey != "" {
		reality["privateKey"] = cfg.Reality.PrivateKey
	}
	if cfg.Reality.ShortID != "" {
		reality["shortIds"] = []interface{}{cfg.Reality.ShortID}
	}
	return nil
}

func setMapValue(m map[string]interface{}, key string, value interface{}) bool {
	if reflect.DeepEqual(m[key], value) {
		return false
	}
	m[key] = value
	return true
}
