//go:build windows

package shell

import "testing"

// alreadyOnPath decides whether init's PATH entry is a no-op. It scans the
// current PATH value (semicolon-separated, REG_EXPAND_SZ on Windows) and
// returns true if any entry matches case-insensitively after filepath.Clean.
// This guards against duplicate entries when `armup init` is re-run.
func TestAlreadyOnPath(t *testing.T) {
	dir := `C:\opt\armup\current\bin`

	cases := []struct {
		name    string
		pathVar string
		dir     string
		want    bool
	}{
		{"empty path", "", dir, false},
		{"single match", dir, dir, true},
		{"match among others", `C:\Windows\System32;` + dir + `;C:\Other`, dir, true},
		{"case-insensitive", `c:\opt\armup\current\bin`, dir, true},
		{"trailing slash on entry", dir + `\`, dir, true},
		{"trailing slash on dir", dir, dir + `\`, true},
		{"double semicolons skipped", `;;` + dir + `;;`, dir, true},
		{"unrelated entries", `C:\Windows\System32;C:\Python\Scripts`, dir, false},
		{"prefix-only match must not count", `C:\opt\armup\current\bi`, dir, false},
		{"superstring must not count", dir + `extra`, dir, false},
	}
	for _, c := range cases {
		got := alreadyOnPath(c.pathVar, c.dir)
		if got != c.want {
			t.Errorf("%s: alreadyOnPath(%q, %q) = %v, want %v",
				c.name, c.pathVar, c.dir, got, c.want)
		}
	}
}
