//go:build windows

package paths

import (
	"os"
	"path/filepath"
)

func DataDir() string {
	if x := os.Getenv("LOCALAPPDATA"); x != "" {
		return filepath.Join(x, appName)
	}
	if h, err := os.UserHomeDir(); err == nil {
		return filepath.Join(h, "AppData", "Local", appName)
	}
	return filepath.Join(os.Getenv("USERPROFILE"), "AppData", "Local", appName)
}
