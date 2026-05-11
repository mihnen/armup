package arm

import "sort"

// modernPlatforms tracks the user-facing platform keys (`<os>-<arch>`)
// that ARM publishes for each modern arm-gnu-toolchain release.
// "windows-amd64" represents either the mingw-w64-x86_64 (14.2.rel1
// and later) or mingw-w64-i686 (every modern release) variant —
// Host.ResolveForVersion picks the right triple at install time, so
// from a user's perspective Windows is supported as long as either
// variant ships.
//
// Update this map when ARM ships a new release. Versions not present
// here fall through to ResolveForVersion's HEAD-probe behavior, so
// armup still works (just without the upfront platform check).
var modernPlatforms = map[string][]string{
	"15.2.rel1": {"linux-amd64", "linux-arm64", "darwin-arm64", "windows-amd64"},
	"14.3.rel1": {"linux-amd64", "linux-arm64", "darwin-arm64", "windows-amd64"},
	"14.2.rel1": {"linux-amd64", "linux-arm64", "darwin-amd64", "darwin-arm64", "windows-amd64"},
	"13.3.rel1": {"linux-amd64", "linux-arm64", "darwin-amd64", "darwin-arm64", "windows-amd64"},
	"13.2.rel1": {"linux-amd64", "linux-arm64", "darwin-amd64", "darwin-arm64", "windows-amd64"},
	"12.3.rel1": {"linux-amd64", "linux-arm64", "darwin-amd64", "darwin-arm64", "windows-amd64"},
	"12.2.rel1": {"linux-amd64", "linux-arm64", "darwin-amd64", "darwin-arm64", "windows-amd64"},
	"11.3.rel1": {"linux-amd64", "linux-arm64", "darwin-amd64", "windows-amd64"},
}

// PlatformsFor returns the (os-arch) platforms ARM publishes for
// version, drawing from the modern catalog and the gnu-rm legacy
// table. Returns nil if the version is unknown to armup — callers
// that still need an answer can HEAD-probe upstream.
//
// The returned slice is freshly allocated and sorted for stable output.
func PlatformsFor(version string) []string {
	if p, ok := modernPlatforms[version]; ok {
		out := make([]string, len(p))
		copy(out, p)
		sort.Strings(out)
		return out
	}
	if perPlat, ok := Legacy[version]; ok {
		out := make([]string, 0, len(perPlat))
		for plat := range perPlat {
			out = append(out, plat)
		}
		sort.Strings(out)
		return out
	}
	return nil
}

// SupportsPlatform reports whether ARM publishes `platform` for
// `version`. False for either "ARM doesn't publish it" or "armup
// doesn't know about this version yet" — the caller should HEAD-probe
// upstream if it cares about the distinction.
func SupportsPlatform(version, platform string) bool {
	for _, p := range PlatformsFor(version) {
		if p == platform {
			return true
		}
	}
	return false
}
