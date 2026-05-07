package store

import (
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"testing"

	"github.com/mihnen/armup/internal/paths"
)

// underWine reports whether we appear to be running on top of wine. wine
// stubs out FSCTL_SET_REPARSE_POINT so junction creation fails; tests that
// depend on Use()/SetCurrent should bail out early under wine. Native
// Windows runs these fine.
func underWine() bool {
	return os.Getenv("WINELOADER") != "" || os.Getenv("WINEPREFIX") != ""
}

// withTempDataDir redirects paths.DataDir() at a per-test temp location.
// Sets the env vars that each platform's DataDir consults so the same tests
// exercise the same behavior on Linux, macOS, and Windows.
func withTempDataDir(t *testing.T) string {
	t.Helper()
	if runtime.GOOS == "windows" && underWine() {
		t.Skip("Use()/SetCurrent require FSCTL_SET_REPARSE_POINT, not supported by wine")
	}
	tmp := t.TempDir()
	switch runtime.GOOS {
	case "windows":
		t.Setenv("LOCALAPPDATA", tmp)
	case "darwin":
		t.Setenv("HOME", tmp)
	default:
		t.Setenv("HOME", tmp)
		t.Setenv("XDG_DATA_HOME", filepath.Join(tmp, "share"))
	}
	return paths.DataDir()
}

// TestPromoteExtractionWrapped covers ARM's typical layout: archive contains
// a single top-level directory named arm-gnu-toolchain-<ver>-<triple>/ with
// bin/, lib/, share/ inside. promoteExtraction should rename the inner dir
// to the version slot and clean up the now-empty staging dir.
func TestPromoteExtractionWrapped(t *testing.T) {
	root := t.TempDir()
	staging := filepath.Join(root, ".staging-13.3.rel1")
	inner := "arm-gnu-toolchain-13.3.rel1-x86_64-arm-none-eabi"
	verDir := filepath.Join(root, "13.3.rel1")

	mustMkdir(t, filepath.Join(staging, inner, "bin"))
	mustMkdir(t, filepath.Join(staging, inner, "lib"))
	mustWrite(t, filepath.Join(staging, inner, "bin", "arm-none-eabi-gcc"), "binary")

	if err := promoteExtraction(staging, inner, verDir); err != nil {
		t.Fatalf("promoteExtraction: %v", err)
	}
	mustExist(t, filepath.Join(verDir, "bin", "arm-none-eabi-gcc"))
	mustNotExist(t, staging)
}

// TestPromoteExtractionUnwrapped covers ARM's newer Windows zip layout (15.x):
// no wrapping directory, bin/ and lib/ sit at the top of the archive. The
// staging dir itself becomes the version slot.
func TestPromoteExtractionUnwrapped(t *testing.T) {
	root := t.TempDir()
	staging := filepath.Join(root, ".staging-15.2.rel1")
	inner := "arm-gnu-toolchain-15.2.rel1-mingw-w64-x86_64-arm-none-eabi"
	verDir := filepath.Join(root, "15.2.rel1")

	mustMkdir(t, filepath.Join(staging, "bin"))
	mustMkdir(t, filepath.Join(staging, "arm-none-eabi"))
	mustWrite(t, filepath.Join(staging, "bin", "arm-none-eabi-gcc.exe"), "binary")

	if err := promoteExtraction(staging, inner, verDir); err != nil {
		t.Fatalf("promoteExtraction: %v", err)
	}
	mustExist(t, filepath.Join(verDir, "bin", "arm-none-eabi-gcc.exe"))
	mustExist(t, filepath.Join(verDir, "arm-none-eabi"))
	mustNotExist(t, staging)
}

// TestListEmpty covers the "fresh install, nothing yet" path.
func TestListEmpty(t *testing.T) {
	withTempDataDir(t)
	got, err := List()
	if err != nil {
		t.Fatalf("List on missing versions dir: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("List on missing versions dir = %v, want empty", got)
	}
}

// TestListAndCurrent: with two versions present and `current` pointing at one,
// List returns both newest-first and Current returns the symlink target's
// basename.
func TestListAndCurrent(t *testing.T) {
	withTempDataDir(t)
	if err := EnsureLayout(); err != nil {
		t.Fatal(err)
	}
	mustMkdir(t, paths.VersionDir("12.3.rel1"))
	mustMkdir(t, paths.VersionDir("14.3.rel1"))

	got, err := List()
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"14.3.rel1", "12.3.rel1"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("List:\ngot  %v\nwant %v", got, want)
	}

	cur, err := Current()
	if err != nil || cur != "" {
		t.Errorf("Current with no symlink = (%q, %v), want (\"\", nil)", cur, err)
	}

	if err := Use("14.3.rel1"); err != nil {
		t.Fatalf("Use: %v", err)
	}
	cur, err = Current()
	if err != nil || cur != "14.3.rel1" {
		t.Errorf("Current after Use = (%q, %v), want (\"14.3.rel1\", nil)", cur, err)
	}
}

// TestUseRefusesMissing: switching to an uninstalled version errors with
// the version name in the message.
func TestUseRefusesMissing(t *testing.T) {
	withTempDataDir(t)
	if err := EnsureLayout(); err != nil {
		t.Fatal(err)
	}
	err := Use("99.9.rel1")
	if err == nil {
		t.Fatal("Use of missing version should error")
	}
	if !contains(err.Error(), "99.9.rel1") {
		t.Errorf("error %q should mention the version name", err)
	}
}

// TestUseSwapsCurrentAtomically: switching from one installed version to
// another retargets the symlink without leaving it broken in between.
// (We can't observe atomicity directly in a unit test, but at minimum the
// endpoints must work and there should be no leftover .tmp symlink.)
func TestUseSwapsCurrent(t *testing.T) {
	withTempDataDir(t)
	if err := EnsureLayout(); err != nil {
		t.Fatal(err)
	}
	mustMkdir(t, paths.VersionDir("12.3.rel1"))
	mustMkdir(t, paths.VersionDir("14.3.rel1"))

	if err := Use("12.3.rel1"); err != nil {
		t.Fatal(err)
	}
	if cur, _ := Current(); cur != "12.3.rel1" {
		t.Fatalf("Current = %q, want 12.3.rel1", cur)
	}
	if err := Use("14.3.rel1"); err != nil {
		t.Fatal(err)
	}
	if cur, _ := Current(); cur != "14.3.rel1" {
		t.Fatalf("Current = %q, want 14.3.rel1", cur)
	}
	mustNotExist(t, paths.CurrentLink()+".tmp")
}

// TestUninstallRefusesActive: removing the active version requires force.
func TestUninstallRefusesActive(t *testing.T) {
	withTempDataDir(t)
	if err := EnsureLayout(); err != nil {
		t.Fatal(err)
	}
	mustMkdir(t, paths.VersionDir("12.3.rel1"))
	if err := Use("12.3.rel1"); err != nil {
		t.Fatal(err)
	}

	err := Uninstall("12.3.rel1", false)
	if err == nil {
		t.Fatal("Uninstall of active version without force should error")
	}
	if !contains(err.Error(), "current") && !contains(err.Error(), "force") {
		t.Errorf("error %q should mention 'current' or 'force'", err)
	}
	mustExist(t, paths.VersionDir("12.3.rel1"))
}

// TestUninstallForce: -f removes the dir AND the current link.
func TestUninstallForce(t *testing.T) {
	withTempDataDir(t)
	if err := EnsureLayout(); err != nil {
		t.Fatal(err)
	}
	mustMkdir(t, paths.VersionDir("12.3.rel1"))
	if err := Use("12.3.rel1"); err != nil {
		t.Fatal(err)
	}

	if err := Uninstall("12.3.rel1", true); err != nil {
		t.Fatalf("Uninstall force: %v", err)
	}
	mustNotExist(t, paths.VersionDir("12.3.rel1"))
	if cur, _ := Current(); cur != "" {
		t.Errorf("Current after force-uninstall = %q, want empty", cur)
	}
}

// TestUninstallNonActive: removing a non-active version succeeds without -f
// and leaves current pointing at the still-installed version.
func TestUninstallNonActive(t *testing.T) {
	withTempDataDir(t)
	if err := EnsureLayout(); err != nil {
		t.Fatal(err)
	}
	mustMkdir(t, paths.VersionDir("12.3.rel1"))
	mustMkdir(t, paths.VersionDir("14.3.rel1"))
	if err := Use("14.3.rel1"); err != nil {
		t.Fatal(err)
	}

	if err := Uninstall("12.3.rel1", false); err != nil {
		t.Fatalf("Uninstall non-active: %v", err)
	}
	mustNotExist(t, paths.VersionDir("12.3.rel1"))
	if cur, _ := Current(); cur != "14.3.rel1" {
		t.Errorf("Current after Uninstall = %q, want 14.3.rel1", cur)
	}
}

// TestReset wipes everything armup created under DataDir. With multiple
// versions installed and `current` linked, after Reset the data dir is gone.
func TestReset(t *testing.T) {
	dataDir := withTempDataDir(t)
	if err := EnsureLayout(); err != nil {
		t.Fatal(err)
	}
	mustMkdir(t, paths.VersionDir("12.3.rel1"))
	mustMkdir(t, paths.VersionDir("14.3.rel1"))
	mustWrite(t, filepath.Join(paths.CacheDir(), "stale.tar.xz"), "junk")
	if err := Use("14.3.rel1"); err != nil {
		t.Fatal(err)
	}
	mustExist(t, dataDir)

	if err := Reset(); err != nil {
		t.Fatalf("Reset: %v", err)
	}
	mustNotExist(t, dataDir)
}

// helpers

func mustMkdir(t *testing.T, dir string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
}

func mustWrite(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func mustExist(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); err != nil {
		t.Errorf("expected %s to exist: %v", path, err)
	}
}

func mustNotExist(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Lstat(path); err == nil {
		t.Errorf("expected %s to not exist", path)
	}
}

func contains(s, substr string) bool {
	for i := 0; i+len(substr) <= len(s); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
