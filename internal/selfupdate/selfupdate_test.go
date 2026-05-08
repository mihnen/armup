package selfupdate

import (
	"runtime"
	"strings"
	"testing"
)

// platformArchive must produce names that match what release.yml uploads.
// If this drifts from the workflow, self-update silently 404s.
func TestPlatformArchive(t *testing.T) {
	tag := "v1.2.3"
	archiveName, ext, binName := platformArchive(tag)

	wantBin := "armup"
	wantExt := ".tar.gz"
	if runtime.GOOS == "windows" {
		wantBin = "armup.exe"
		wantExt = ".zip"
	}
	if binName != wantBin {
		t.Errorf("binName = %q, want %q", binName, wantBin)
	}
	if ext != wantExt {
		t.Errorf("ext = %q, want %q", ext, wantExt)
	}

	want := "armup-" + tag + "-" + runtime.GOOS + "-" + runtime.GOARCH + wantExt
	if archiveName != want {
		t.Errorf("archiveName = %q, want %q", archiveName, want)
	}
}

func TestParseFirstStableTag(t *testing.T) {
	body := []byte(`[
		{"tag_name":"v0.2.0-beta2","draft":false,"prerelease":true},
		{"tag_name":"v0.2.0-beta1","draft":false,"prerelease":true},
		{"tag_name":"v0.1.0-beta1","draft":false,"prerelease":true}
	]`)
	got, err := parseFirstStableTag(body)
	if err != nil {
		t.Fatalf("parseFirstStableTag: %v", err)
	}
	if got != "v0.2.0-beta2" {
		t.Errorf("parseFirstStableTag = %q, want v0.2.0-beta2", got)
	}
}

func TestParseFirstStableTagSkipsDrafts(t *testing.T) {
	body := []byte(`[
		{"tag_name":"v1.0.0-draft","draft":true},
		{"tag_name":"v0.9.0","draft":false}
	]`)
	got, err := parseFirstStableTag(body)
	if err != nil {
		t.Fatalf("parseFirstStableTag: %v", err)
	}
	if got != "v0.9.0" {
		t.Errorf("parseFirstStableTag = %q, want v0.9.0 (drafts skipped)", got)
	}
}

func TestParseFirstStableTagEmpty(t *testing.T) {
	if _, err := parseFirstStableTag([]byte(`[]`)); err == nil {
		t.Error("parseFirstStableTag on empty list should error")
	}
}

func TestParseFirstStableTagAllDrafts(t *testing.T) {
	body := []byte(`[
		{"tag_name":"v1.0.0","draft":true},
		{"tag_name":"v0.9.0","draft":true}
	]`)
	if _, err := parseFirstStableTag(body); err == nil {
		t.Error("parseFirstStableTag with only drafts should error")
	}
}

func TestParseFirstStableTagBadJSON(t *testing.T) {
	if _, err := parseFirstStableTag([]byte(`not json`)); err == nil {
		t.Error("parseFirstStableTag on bad JSON should error")
	}
}

// The rolling 'nightly' release sits at the top of /releases (it gets
// republished on every push to master). parseFirstStableTag must skip it
// so users on a stable release don't accidentally jump onto the nightly
// channel via plain `armup self-update`.
func TestParseFirstStableTagSkipsNightly(t *testing.T) {
	body := []byte(`[
		{"tag_name":"nightly","draft":false,"prerelease":true},
		{"tag_name":"v1.0.0","draft":false,"prerelease":false}
	]`)
	got, err := parseFirstStableTag(body)
	if err != nil {
		t.Fatalf("parseFirstStableTag: %v", err)
	}
	if got != "v1.0.0" {
		t.Errorf("parseFirstStableTag = %q, want v1.0.0 (nightly should be skipped)", got)
	}
}

func TestFindSum(t *testing.T) {
	body := `1111111111111111111111111111111111111111111111111111111111111111  armup-v1-linux-amd64.tar.gz
2222222222222222222222222222222222222222222222222222222222222222 *armup-v1-windows-amd64.zip
3333333333333333333333333333333333333333333333333333333333333333  armup-v1-darwin-arm64.tar.gz
`
	cases := []struct {
		file, want string
	}{
		// Plain entry (gnu coreutils format)
		{"armup-v1-linux-amd64.tar.gz", "1111111111111111111111111111111111111111111111111111111111111111"},
		// Binary-mode entry (asterisk prefix on filename)
		{"armup-v1-windows-amd64.zip", "2222222222222222222222222222222222222222222222222222222222222222"},
		// Another plain entry
		{"armup-v1-darwin-arm64.tar.gz", "3333333333333333333333333333333333333333333333333333333333333333"},
	}
	for _, c := range cases {
		got, err := findSum(body, c.file)
		if err != nil {
			t.Errorf("findSum(%q): %v", c.file, err)
			continue
		}
		if got != c.want {
			t.Errorf("findSum(%q) = %q, want %q", c.file, got, c.want)
		}
	}
}

func TestFindSumMissing(t *testing.T) {
	body := "1111111111111111111111111111111111111111111111111111111111111111  some-other-file.tar.gz\n"
	_, err := findSum(body, "armup-vX-linux-amd64.tar.gz")
	if err == nil {
		t.Error("findSum on missing file should error")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("error %q should mention 'not found'", err)
	}
}
