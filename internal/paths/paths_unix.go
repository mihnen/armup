//go:build linux || darwin

package paths

import (
	"os"
	"path/filepath"
	"runtime"
)

func DataDir() string {
	if runtime.GOOS == "darwin" {
		return filepath.Join(homeDir(), "Library", "Application Support", appName)
	}
	if x := os.Getenv("XDG_DATA_HOME"); x != "" {
		return filepath.Join(x, appName)
	}
	return filepath.Join(homeDir(), ".local", "share", appName)
}

func homeDir() string {
	if h, err := os.UserHomeDir(); err == nil {
		return h
	}
	return os.Getenv("HOME")
}
