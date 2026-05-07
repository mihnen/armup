//go:build linux || darwin

package paths

import "os"

// MakeDirLink creates `link` as a symlink to `target`.
func MakeDirLink(target, link string) error { return os.Symlink(target, link) }

// SetCurrent atomically points `link` at `target`. Creates a temp symlink
// alongside, then renames over `link`, so any concurrent reader sees either
// the old or new target — never a missing path.
func SetCurrent(target, link string) error {
	tmp := link + ".tmp"
	os.Remove(tmp)
	if err := os.Symlink(target, tmp); err != nil {
		return err
	}
	if err := os.Rename(tmp, link); err != nil {
		os.Remove(tmp)
		return err
	}
	return nil
}
