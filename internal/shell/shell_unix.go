//go:build linux || darwin

package shell

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// rcKind picks which syntax to use when writing a PATH-export block.
type rcKind int

const (
	rcBash rcKind = iota // POSIX-ish: export PATH="..."
	rcFish               // fish:    set -gx PATH ...
)

// rcFile names a shell rc file armup may have written to and which syntax
// to use when writing a fresh block. The list always includes .zshrc and
// .bashrc; fish's config is included only when ~/.config/fish exists.
type rcFile struct {
	path string
	kind rcKind
}

func rcFiles() []rcFile {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil
	}
	files := []rcFile{
		{filepath.Join(home, ".zshrc"), rcBash},
		{filepath.Join(home, ".bashrc"), rcBash},
	}
	fishDir := filepath.Join(home, ".config", "fish")
	if info, err := os.Stat(fishDir); err == nil && info.IsDir() {
		files = append(files, rcFile{filepath.Join(fishDir, "config.fish"), rcFish})
	}
	return files
}

// block returns the marked PATH-prepending block to write to this rc file.
func (r rcFile) block(dir string) string {
	switch r.kind {
	case rcFish:
		return fmt.Sprintf("\n%s\nset -gx PATH \"%s\" $PATH\n%s\n",
			BeginMarker, dir, EndMarker)
	default:
		return fmt.Sprintf("\n%s\nexport PATH=\"%s:$PATH\"\n%s\n",
			BeginMarker, dir, EndMarker)
	}
}

// EnsureOnPath appends a marked block prepending dir to PATH in ~/.zshrc,
// ~/.bashrc, and ~/.config/fish/config.fish (if fish is installed). If a
// marked block already exists in a file, it's left alone (idempotent).
// Files that don't exist are created. Returns the list of files modified.
func EnsureOnPath(dir string) ([]string, error) {
	rcs := rcFiles()
	if rcs == nil {
		return nil, fmt.Errorf("locate home directory")
	}

	var updated []string
	for _, rc := range rcs {
		existing, err := os.ReadFile(rc.path)
		if err != nil && !os.IsNotExist(err) {
			return updated, fmt.Errorf("read %s: %w", rc.path, err)
		}
		if strings.Contains(string(existing), BeginMarker) {
			continue
		}
		f, err := os.OpenFile(rc.path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
		if err != nil {
			return updated, fmt.Errorf("open %s: %w", rc.path, err)
		}
		if _, err := f.WriteString(rc.block(dir)); err != nil {
			f.Close()
			return updated, fmt.Errorf("write %s: %w", rc.path, err)
		}
		if err := f.Close(); err != nil {
			return updated, err
		}
		updated = append(updated, rc.path)
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
		body, err := os.ReadFile(rc.path)
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
		if err := os.WriteFile(rc.path, []byte(next), 0o644); err != nil {
			return modified, err
		}
		modified = append(modified, rc.path)
	}
	return modified, nil
}

// stripBlock removes the marker block (BeginMarker line through EndMarker
// line, inclusive of both, and the surrounding newlines) from content.
// Returns the new content and whether anything was removed. Works for
// both POSIX and fish syntax — the markers are the same.
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
