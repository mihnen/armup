package arm

import (
	"regexp"
	"runtime"
	"strings"
	"testing"
)

// Sanity check on the shape of every embedded legacy entry: every entry
// has a plausible URL and a 64-hex-char SHA-256.
func TestLegacyTableShape(t *testing.T) {
	if len(Legacy) == 0 {
		t.Fatal("Legacy table is empty — should be populated by the helper")
	}
	sha256RE := regexp.MustCompile(`^[0-9a-f]{64}$`)
	for version, perPlatform := range Legacy {
		if len(perPlatform) == 0 {
			t.Errorf("legacy version %q has no platform entries", version)
		}
		for platform, e := range perPlatform {
			if !strings.HasPrefix(e.URL, "https://developer.arm.com/") {
				t.Errorf("%s/%s: URL doesn't look like an ARM URL: %s", version, platform, e.URL)
			}
			if !sha256RE.MatchString(e.SHA256) {
				t.Errorf("%s/%s: SHA256 %q is not 64 hex chars", version, platform, e.SHA256)
			}
			// Sanity: platform key shape "os-arch".
			if !strings.Contains(platform, "-") {
				t.Errorf("%s: platform key %q should be '<os>-<arch>'", version, platform)
			}
		}
	}
}

// LegacyLookup returns the running host's entry, or (zero, false).
func TestLegacyLookup(t *testing.T) {
	// We don't assume any specific version exists — just that whatever
	// IS in the table for this host returns sensibly.
	host := runtime.GOOS + "-" + runtime.GOARCH
	for version, perPlatform := range Legacy {
		want, hostHas := perPlatform[host]
		got, ok := LegacyLookup(version)
		if hostHas != ok {
			t.Errorf("%s: LegacyLookup ok=%v, table has=%v", version, ok, hostHas)
		}
		if hostHas && got != want {
			t.Errorf("%s: LegacyLookup mismatch", version)
		}
	}

	// Unknown version returns (zero, false).
	if e, ok := LegacyLookup("nonexistent.99.rel99"); ok || e != (LegacyEntry{}) {
		t.Errorf("LegacyLookup of unknown returned (%+v, %v); want (zero, false)", e, ok)
	}
}

// LegacyVersions returns names sorted descending (newest first).
func TestLegacyVersionsSortedDescending(t *testing.T) {
	versions := LegacyVersions()
	for i := 1; i < len(versions); i++ {
		if cmpVersions(versions[i-1], versions[i]) < 0 {
			t.Errorf("LegacyVersions not descending: %v", versions)
			break
		}
	}
}

// LegacyAllVersions returns every version regardless of platform.
func TestLegacyAllVersionsCoversTable(t *testing.T) {
	all := LegacyAllVersions()
	if len(all) != len(Legacy) {
		t.Errorf("LegacyAllVersions len = %d, Legacy table = %d", len(all), len(Legacy))
	}
	seen := make(map[string]bool, len(all))
	for _, v := range all {
		seen[v] = true
	}
	for v := range Legacy {
		if !seen[v] {
			t.Errorf("LegacyAllVersions missing %q", v)
		}
	}
}
