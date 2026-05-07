package archive

import (
	"path/filepath"
	"testing"
)

// safeJoin is armup's defense against zip-slip / tar-slip. The stdlib
// readers happily hand us archive entries whose names contain `..` or a
// leading `/`; safeJoin normalizes those into paths anchored inside the
// destination so an entry named `../etc/passwd` lands at `<dst>/etc/passwd`
// rather than overwriting `/etc/passwd`. The invariant being tested: the
// returned path always sits under `base`.
func TestSafeJoin(t *testing.T) {
	base := t.TempDir()

	cases := []struct {
		in     string
		wantIn string // expected path tail inside base
	}{
		// Plain entries pass through untouched.
		{"foo/bar.txt", "foo/bar.txt"},
		{"./foo/bar.txt", "foo/bar.txt"},
		{"foo/./bar.txt", "foo/bar.txt"},
		// Dangerous entries get neutralized into base-relative paths.
		{"../escape.txt", "escape.txt"},
		{"foo/../../escape.txt", "escape.txt"},
		{"/abs/escape.txt", "abs/escape.txt"},
	}
	for _, c := range cases {
		got, err := safeJoin(base, c.in)
		if err != nil {
			t.Errorf("safeJoin(base, %q) errored: %v", c.in, err)
			continue
		}
		want := filepath.Join(base, c.wantIn)
		if got != want {
			t.Errorf("safeJoin(base, %q) = %q, want %q", c.in, got, want)
		}
		// Belt-and-suspenders: assert the result really is inside base.
		rel, err := filepath.Rel(base, got)
		if err != nil || rel == ".." || filepath.IsAbs(rel) || (len(rel) >= 3 && rel[:3] == ".."+string(filepath.Separator)) {
			t.Errorf("safeJoin(base, %q) = %q escapes base (rel=%q)", c.in, got, rel)
		}
	}
}

func TestHumanBytes(t *testing.T) {
	cases := []struct {
		in   int64
		want string
	}{
		{0, "0 B"},
		{512, "512 B"},
		{1023, "1023 B"},
		{1024, "1.0 KiB"},
		{1536, "1.5 KiB"},
		{1024 * 1024, "1.0 MiB"},
		{1024 * 1024 * 1024, "1.0 GiB"},
	}
	for _, c := range cases {
		if got := humanBytes(c.in); got != c.want {
			t.Errorf("humanBytes(%d) = %q, want %q", c.in, got, c.want)
		}
	}
}
