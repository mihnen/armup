package arm

import (
	"reflect"
	"testing"
)

func TestPlatformsForModern(t *testing.T) {
	cases := []struct {
		version string
		want    []string
	}{
		{"14.3.rel1", []string{"darwin-arm64", "linux-amd64", "linux-arm64", "windows-amd64"}},
		{"14.2.rel1", []string{"darwin-amd64", "darwin-arm64", "linux-amd64", "linux-arm64", "windows-amd64"}},
		{"11.3.rel1", []string{"darwin-amd64", "linux-amd64", "linux-arm64", "windows-amd64"}},
	}
	for _, c := range cases {
		got := PlatformsFor(c.version)
		if !reflect.DeepEqual(got, c.want) {
			t.Errorf("PlatformsFor(%q) = %v, want %v", c.version, got, c.want)
		}
	}
}

func TestPlatformsForLegacy(t *testing.T) {
	// 5-2016-q1-update only has Linux + macOS Intel; no Windows in our
	// catalog (ARM's pre-2017 Windows was a Nullsoft installer we don't
	// support), no Linux ARM, no Apple Silicon.
	got := PlatformsFor("5-2016-q1-update")
	want := []string{"darwin-amd64", "linux-amd64"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("PlatformsFor(5-2016-q1-update) = %v, want %v", got, want)
	}
}

func TestPlatformsForUnknown(t *testing.T) {
	if got := PlatformsFor("99.99.rel1"); got != nil {
		t.Errorf("PlatformsFor(unknown) = %v, want nil", got)
	}
}

func TestSupportsPlatform(t *testing.T) {
	if !SupportsPlatform("14.3.rel1", "darwin-arm64") {
		t.Error("14.3.rel1 should support darwin-arm64")
	}
	if SupportsPlatform("14.3.rel1", "darwin-amd64") {
		t.Error("14.3.rel1 should NOT support darwin-amd64 (ARM dropped Intel-Mac after 14.2)")
	}
	if SupportsPlatform("99.99.rel1", "linux-amd64") {
		t.Error("unknown version should return false (no positive claim)")
	}
}

// Every modern release's platform list is a subset of SupportedPlatforms.
// Catches typos in the catalog map.
func TestModernPlatformsUseSupportedKeys(t *testing.T) {
	known := make(map[string]bool, len(SupportedPlatforms))
	for _, p := range SupportedPlatforms {
		known[p] = true
	}
	for version, plats := range modernPlatforms {
		for _, p := range plats {
			if !known[p] {
				t.Errorf("modern release %q references unknown platform %q", version, p)
			}
		}
	}
}
