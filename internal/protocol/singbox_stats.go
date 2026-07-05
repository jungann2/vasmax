package protocol

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"vasmax/internal/api"
)

const (
	// SingBoxStatsAPIAddr is the local V2Ray-compatible stats API endpoint used
	// for sing-box managed-mode traffic accounting.
	SingBoxStatsAPIAddr = "127.0.0.1:10086"
	singBoxStatsFile    = "03_v2ray_api.json"
)

// SingBoxStatsConfigPath returns the partial config path used for the optional
// sing-box V2Ray API stats service.
func SingBoxStatsConfigPath(confDir string) string {
	return filepath.Join(confDir, singBoxStatsFile)
}

// GenerateSingBoxStatsAPIConfigData creates a sing-box experimental.v2ray_api
// partial. The official sing-box V2Ray API counts only configured user names,
// so we include all managed user names used by the supported sing-box inbounds.
func GenerateSingBoxStatsAPIConfigData(users []*api.User) ([]byte, error) {
	userNames := managedSingBoxStatsUsers(users)
	config := map[string]interface{}{
		"experimental": map[string]interface{}{
			"v2ray_api": map[string]interface{}{
				"listen": SingBoxStatsAPIAddr,
				"stats": map[string]interface{}{
					"enabled": true,
					"users":   userNames,
				},
			},
		},
	}
	return json.MarshalIndent(config, "", "  ")
}

func managedSingBoxStatsUsers(users []*api.User) []string {
	seen := make(map[string]struct{}, len(users)*2)
	result := make([]string, 0, len(users)*2)
	for _, u := range users {
		if u == nil || u.ID <= 0 {
			continue
		}
		names := []string{
			fmt.Sprintf("user_%d", u.ID),
			fmt.Sprintf("user_%d-anytls", u.ID),
		}
		for _, name := range names {
			if _, ok := seen[name]; ok {
				continue
			}
			seen[name] = struct{}{}
			result = append(result, name)
		}
	}
	return result
}

// RemoveSingBoxStatsAPIConfig disables the optional stats partial. It is used
// when the installed sing-box binary was built without V2Ray API support.
func RemoveSingBoxStatsAPIConfig(confDir string) error {
	err := os.Remove(SingBoxStatsConfigPath(confDir))
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}
