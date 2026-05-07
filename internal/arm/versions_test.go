package arm

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestNormalize(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"15.2.rel1", "15.2.rel1"},
		{"15.2.Rel1", "15.2.rel1"},
		{"15.2.REL1", "15.2.rel1"},
		{"  15.2.Rel1  ", "15.2.rel1"},
		{"11.2-2022.02", "11.2-2022.02"},
		{"", ""},
	}
	for _, c := range cases {
		if got := Normalize(c.in); got != c.want {
			t.Errorf("Normalize(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestCmpVersions(t *testing.T) {
	cases := []struct {
		a, b string
		want int
	}{
		{"15.2.rel1", "15.2.rel1", 0},
		{"15.2.rel1", "14.3.rel1", 1},
		{"14.3.rel1", "15.2.rel1", -1},
		{"14.2.rel1", "14.3.rel1", -1},
		{"12.3.rel1", "12.2.rel1", 1},
		// rel suffix: same major.minor, different rel — rel digit decides
		{"12.3.rel2", "12.3.rel1", 1},
		// digit-count safety: "10" > "9" numerically
		{"10.0.rel1", "9.0.rel1", 1},
		// older/newer ARM filename schemes
		{"11.2-2022.02", "11.3.rel1", -1},
	}
	for _, c := range cases {
		if got := cmpVersions(c.a, c.b); got != c.want {
			t.Errorf("cmpVersions(%q, %q) = %d, want %d", c.a, c.b, got, c.want)
		}
	}
}

func TestSortVersionsDesc(t *testing.T) {
	in := []string{
		"12.3.rel1",
		"15.2.rel1",
		"14.3.rel1",
		"11.3.rel1",
		"14.2.rel1",
	}
	want := []string{
		"15.2.rel1",
		"14.3.rel1",
		"14.2.rel1",
		"12.3.rel1",
		"11.3.rel1",
	}
	sortVersionsDesc(in)
	if !reflect.DeepEqual(in, want) {
		t.Errorf("sortVersionsDesc:\ngot:  %v\nwant: %v", in, want)
	}
}

func TestSplitVersion(t *testing.T) {
	cases := []struct {
		in   string
		want []int
	}{
		{"15.2.rel1", []int{15, 2, 1}},
		{"11.2-2022.02", []int{11, 2, 2022, 2}},
		{"v1.0.0", []int{1, 0, 0}},
	}
	for _, c := range cases {
		if got := splitVersion(c.in); !reflect.DeepEqual(got, c.want) {
			t.Errorf("splitVersion(%q) = %v, want %v", c.in, got, c.want)
		}
	}
}

func TestHostURLBuilders(t *testing.T) {
	h := Host{Triple: "x86_64-arm-none-eabi", Ext: ".tar.xz"}
	v := "14.3.rel1"

	wantFile := "arm-gnu-toolchain-14.3.rel1-x86_64-arm-none-eabi.tar.xz"
	if got := h.ArchiveFilename(v); got != wantFile {
		t.Errorf("ArchiveFilename = %q, want %q", got, wantFile)
	}

	wantURL := "https://developer.arm.com/-/media/Files/downloads/gnu/14.3.rel1/binrel/" + wantFile
	if got := h.ArchiveURL(v); got != wantURL {
		t.Errorf("ArchiveURL = %q, want %q", got, wantURL)
	}
	if got := h.ChecksumURL(v); got != wantURL+".sha256asc" {
		t.Errorf("ChecksumURL = %q, want %q", got, wantURL+".sha256asc")
	}

	wantInner := "arm-gnu-toolchain-14.3.rel1-x86_64-arm-none-eabi"
	if got := h.InnerDirName(v); got != wantInner {
		t.Errorf("InnerDirName = %q, want %q", got, wantInner)
	}
}

func TestAvailableRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "available.txt")

	// Loading from a missing file should be a clean (nil, nil).
	got, err := LoadCachedAvailable(path)
	if err != nil {
		t.Fatalf("LoadCachedAvailable on missing file: %v", err)
	}
	if got != nil {
		t.Fatalf("LoadCachedAvailable on missing file = %v, want nil", got)
	}

	want := []string{"15.2.rel1", "14.3.rel1", "12.3.rel1"}
	if err := SaveAvailable(path, want); err != nil {
		t.Fatalf("SaveAvailable: %v", err)
	}
	got, err = LoadCachedAvailable(path)
	if err != nil {
		t.Fatalf("LoadCachedAvailable after save: %v", err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("round trip mismatch:\ngot:  %v\nwant: %v", got, want)
	}

	// Empty + whitespace lines should be skipped.
	if err := os.WriteFile(path, []byte("15.2.rel1\n\n  \n14.3.rel1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err = LoadCachedAvailable(path)
	if err != nil {
		t.Fatal(err)
	}
	want = []string{"15.2.rel1", "14.3.rel1"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("LoadCachedAvailable with blank lines:\ngot:  %v\nwant: %v", got, want)
	}
}
