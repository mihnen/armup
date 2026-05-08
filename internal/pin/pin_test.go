package pin

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveNoFiles(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("ARMUP_VERSION", "")
	r, err := Resolve(dir)
	if err != nil {
		t.Fatal(err)
	}
	if r.Found() {
		t.Errorf("Resolve in empty tree should be empty, got %+v", r)
	}
}

func TestResolveArmupVersion(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("ARMUP_VERSION", "")
	mustWrite(t, filepath.Join(dir, ".armup-version"), "14.3.rel1\n")

	r, err := Resolve(dir)
	if err != nil {
		t.Fatal(err)
	}
	if r.Version != "14.3.rel1" {
		t.Errorf("Version = %q, want 14.3.rel1", r.Version)
	}
	if r.Source != filepath.Join(dir, ".armup-version") {
		t.Errorf("Source = %q, want path to .armup-version", r.Source)
	}
}

func TestResolveToolVersions(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("ARMUP_VERSION", "")
	body := "# project pins\nnodejs 22.0.0\narmup 14.3.rel1\npython 3.13.0\n"
	mustWrite(t, filepath.Join(dir, ".tool-versions"), body)

	r, err := Resolve(dir)
	if err != nil {
		t.Fatal(err)
	}
	if r.Version != "14.3.rel1" {
		t.Errorf("Version = %q, want 14.3.rel1", r.Version)
	}
	if filepath.Base(r.Source) != ".tool-versions" {
		t.Errorf("Source = %q, want .tool-versions path", r.Source)
	}
}

// .tool-versions wins over .armup-version when both exist in the same dir
// (asdf/mise format is more explicit and supports multiple tools).
func TestResolvePrefersToolVersions(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("ARMUP_VERSION", "")
	mustWrite(t, filepath.Join(dir, ".tool-versions"), "armup 14.3.rel1\n")
	mustWrite(t, filepath.Join(dir, ".armup-version"), "12.3.rel1\n")

	r, err := Resolve(dir)
	if err != nil {
		t.Fatal(err)
	}
	if r.Version != "14.3.rel1" {
		t.Errorf("Version = %q, want 14.3.rel1 (.tool-versions should win)", r.Version)
	}
}

// .tool-versions without an armup line shouldn't block falling back to
// .armup-version in the same dir.
func TestResolveToolVersionsNoArmupLineFallsThrough(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("ARMUP_VERSION", "")
	mustWrite(t, filepath.Join(dir, ".tool-versions"), "nodejs 22.0.0\npython 3.13.0\n")
	mustWrite(t, filepath.Join(dir, ".armup-version"), "13.3.rel1\n")

	r, err := Resolve(dir)
	if err != nil {
		t.Fatal(err)
	}
	if r.Version != "13.3.rel1" {
		t.Errorf("Version = %q, want 13.3.rel1 (.armup-version fallback)", r.Version)
	}
}

func TestResolveWalksUp(t *testing.T) {
	t.Setenv("ARMUP_VERSION", "")
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, ".armup-version"), "12.3.rel1\n")
	deep := filepath.Join(root, "src", "platform", "board")
	if err := os.MkdirAll(deep, 0o755); err != nil {
		t.Fatal(err)
	}

	r, err := Resolve(deep)
	if err != nil {
		t.Fatal(err)
	}
	if r.Version != "12.3.rel1" {
		t.Errorf("Version = %q, want 12.3.rel1 (walk-up failed)", r.Version)
	}
}

func TestResolveEnvVarOverridesFile(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("ARMUP_VERSION", "15.2.rel1")
	mustWrite(t, filepath.Join(dir, ".armup-version"), "12.3.rel1\n")

	r, err := Resolve(dir)
	if err != nil {
		t.Fatal(err)
	}
	if r.Version != "15.2.rel1" {
		t.Errorf("Version = %q, want 15.2.rel1 (env should override file)", r.Version)
	}
	if r.Source != envSource {
		t.Errorf("Source = %q, want %q", r.Source, envSource)
	}
}

// Versions in pin files should run through arm.Normalize so casing
// mismatches like "14.3.Rel1" don't poison the install/use flow.
func TestResolveNormalizesCase(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("ARMUP_VERSION", "")
	mustWrite(t, filepath.Join(dir, ".armup-version"), "14.3.Rel1\n")

	r, err := Resolve(dir)
	if err != nil {
		t.Fatal(err)
	}
	if r.Version != "14.3.rel1" {
		t.Errorf("Version = %q, want 14.3.rel1 (normalized)", r.Version)
	}
}

func TestResolveCommentsInToolVersions(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("ARMUP_VERSION", "")
	body := "# header comment\n  # indented comment\narmup 14.3.rel1\n"
	mustWrite(t, filepath.Join(dir, ".tool-versions"), body)

	r, err := Resolve(dir)
	if err != nil {
		t.Fatal(err)
	}
	if r.Version != "14.3.rel1" {
		t.Errorf("Version = %q, want 14.3.rel1", r.Version)
	}
}

func mustWrite(t *testing.T, path, body string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}
