//go:build linux || darwin

package shell

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// EnsureOnPath appends a marked block to ~/.zshrc and ~/.bashrc that prepends
// dir to PATH. If a marked block already exists in a file, it's left alone
// (idempotent). Files that don't exist are created. Reports which files were
// updated and which already had the block.
func EnsureOnPath(dir string) ([]string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("locate home directory: %w", err)
	}
	rcs := []string{
		filepath.Join(home, ".zshrc"),
		filepath.Join(home, ".bashrc"),
	}
	block := fmt.Sprintf("\n%s\nexport PATH=\"%s:$PATH\"\n%s\n",
		BeginMarker, dir, EndMarker)

	var updated []string
	for _, rc := range rcs {
		existing, err := os.ReadFile(rc)
		if err != nil && !os.IsNotExist(err) {
			return updated, fmt.Errorf("read %s: %w", rc, err)
		}
		if strings.Contains(string(existing), BeginMarker) {
			continue
		}
		f, err := os.OpenFile(rc, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
		if err != nil {
			return updated, fmt.Errorf("open %s: %w", rc, err)
		}
		if _, err := f.WriteString(block); err != nil {
			f.Close()
			return updated, fmt.Errorf("write %s: %w", rc, err)
		}
		if err := f.Close(); err != nil {
			return updated, err
		}
		updated = append(updated, rc)
	}
	return updated, nil
}
