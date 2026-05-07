//go:build linux || darwin

package shell

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// rcFiles returns the shell rc files armup may have written to.
func rcFiles() []string {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil
	}
	return []string{
		filepath.Join(home, ".zshrc"),
		filepath.Join(home, ".bashrc"),
	}
}

// EnsureOnPath appends a marked block to ~/.zshrc and ~/.bashrc that prepends
// dir to PATH. If a marked block already exists in a file, it's left alone
// (idempotent). Files that don't exist are created. Reports which files were
// updated and which already had the block.
func EnsureOnPath(dir string) ([]string, error) {
	rcs := rcFiles()
	if rcs == nil {
		return nil, fmt.Errorf("locate home directory")
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

// RemoveFromPath removes the marker block (BeginMarker..EndMarker) from
// each rc file that contains it. The `dir` argument is unused on unix —
// the marker block is the source of truth. Returns the list of files
// modified.
func RemoveFromPath(_ string) ([]string, error) {
	rcs := rcFiles()
	if rcs == nil {
		return nil, fmt.Errorf("locate home directory")
	}
	var modified []string
	for _, rc := range rcs {
		body, err := os.ReadFile(rc)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return modified, err
		}
		next, removed := stripBlock(string(body))
		if !removed {
			continue
		}
		if err := os.WriteFile(rc, []byte(next), 0o644); err != nil {
			return modified, err
		}
		modified = append(modified, rc)
	}
	return modified, nil
}

// stripBlock removes the marker block (BeginMarker line through EndMarker
// line, inclusive of both, and the surrounding newlines) from content.
// Returns the new content and whether anything was removed.
func stripBlock(content string) (string, bool) {
	begin := strings.Index(content, BeginMarker)
	if begin == -1 {
		return content, false
	}
	rel := strings.Index(content[begin:], EndMarker)
	if rel == -1 {
		return content, false
	}
	end := begin + rel + len(EndMarker)
	if end < len(content) && content[end] == '\n' {
		end++
	}
	if begin > 0 && content[begin-1] == '\n' {
		begin--
	}
	return content[:begin] + content[end:], true
}
