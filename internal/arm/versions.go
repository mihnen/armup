package arm

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"time"
)

// Normalize converts a user-supplied version string to its canonical form.
// ARM's website prints e.g. "15.2.Rel1" with a capital R, but the URLs and
// inner archive directories use lowercase. We just lowercase the whole
// thing — version strings only contain digits, dots, dashes, and "rel".
func Normalize(version string) string {
	return strings.ToLower(strings.TrimSpace(version))
}

// Curated holds the versions we know are reachable via the standard URL
// pattern. Newest first. Update when ARM ships a new release.
var Curated = []string{
	"15.2.rel1",
	"14.3.rel1",
	"14.2.rel1",
	"13.3.rel1",
	"13.2.rel1",
	"12.3.rel1",
	"12.2.rel1",
	"11.3.rel1",
}

const downloadsPage = "https://developer.arm.com/downloads/-/arm-gnu-toolchain-downloads"

var versionRE = regexp.MustCompile(`arm-gnu-toolchain-(\d+\.\d+\.rel\d+|\d+\.\d+-\d{4}\.\d+)-`)

// ErrPageBlocked is returned by Refresh when ARM's downloads page rejects
// the request (typically Akamai bot protection). Callers should fall back to
// ProbeCurated.
var ErrPageBlocked = errors.New("arm downloads page blocked or unparseable")

// Refresh fetches the ARM downloads page and extracts every version that
// appears in archive filenames. Returns ErrPageBlocked (wrapped) when ARM's
// CDN rejects the request, which is the common case from non-browser clients.
func Refresh(ctx context.Context) ([]string, error) {
	client := &http.Client{Timeout: 30 * time.Second}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, downloadsPage, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0 Safari/537.36")
	req.Header.Set("Accept", "text/html,application/xhtml+xml")
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrPageBlocked, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusForbidden || resp.StatusCode == http.StatusTooManyRequests {
		return nil, fmt.Errorf("%w: %s", ErrPageBlocked, resp.Status)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GET %s: %s", downloadsPage, resp.Status)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	matches := versionRE.FindAllSubmatch(body, -1)
	if len(matches) == 0 {
		return nil, fmt.Errorf("%w: no versions matched", ErrPageBlocked)
	}
	seen := make(map[string]struct{}, len(matches))
	out := make([]string, 0, len(matches))
	for _, m := range matches {
		v := string(m[1])
		if _, ok := seen[v]; ok {
			continue
		}
		seen[v] = struct{}{}
		out = append(out, v)
	}
	sortVersionsDesc(out)
	return out, nil
}

// ProbeCurated returns the curated versions that are installable on
// the given host. For versions in armup's per-version catalog the
// answer comes from PlatformsFor (no network); for versions the
// catalog doesn't know (e.g. a brand-new release we haven't tabulated
// yet) we fall back to HEAD-probing the archive URL.
//
// Used as a fallback when the downloads page is unreachable.
func ProbeCurated(ctx context.Context, host Host) ([]string, error) {
	hostKey := runtime.GOOS + "-" + runtime.GOARCH
	client := &http.Client{Timeout: 15 * time.Second}
	var out []string
	for _, v := range Curated {
		if known := PlatformsFor(v); known != nil {
			for _, p := range known {
				if p == hostKey {
					out = append(out, v)
					break
				}
			}
			continue
		}
		// Unknown to the catalog — HEAD-probe.
		url := host.ArchiveURL(v)
		req, err := http.NewRequestWithContext(ctx, http.MethodHead, url, nil)
		if err != nil {
			return nil, err
		}
		resp, err := client.Do(req)
		if err != nil {
			continue
		}
		resp.Body.Close()
		// Direct 200 or a redirect to the CDN both mean "reachable".
		if resp.StatusCode >= 200 && resp.StatusCode < 400 {
			out = append(out, v)
		}
	}
	sortVersionsDesc(out)
	return out, nil
}

// LoadCachedAvailable reads versions from a previously written cache file.
// Empty list and nil error means the file does not exist.
func LoadCachedAvailable(path string) ([]string, error) {
	b, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var out []string
	for _, line := range strings.Split(string(b), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		out = append(out, line)
	}
	return out, nil
}

// SaveAvailable writes versions one-per-line to path.
func SaveAvailable(path string, versions []string) error {
	return os.WriteFile(path, []byte(strings.Join(versions, "\n")+"\n"), 0o644)
}

// sortVersionsDesc sorts a list of ARM version strings newest-first.
// Handles both "13.3.rel1" and "11.2-2022.02" shapes.
func sortVersionsDesc(v []string) {
	sort.Slice(v, func(i, j int) bool { return cmpVersions(v[i], v[j]) > 0 })
}

// MergeAvailable folds the embedded gnu-rm versions for the running
// host into the given list of arm-gnu-toolchain versions and returns
// the merged set sorted newest-first. Used by `armup available` so the
// caller doesn't have to know about the two upstream catalogs.
func MergeAvailable(modern []string) []string {
	seen := make(map[string]struct{}, len(modern)+len(Legacy))
	out := make([]string, 0, len(modern)+len(Legacy))
	for _, v := range modern {
		if _, ok := seen[v]; ok {
			continue
		}
		seen[v] = struct{}{}
		out = append(out, v)
	}
	for _, v := range LegacyVersions() {
		if _, ok := seen[v]; ok {
			continue
		}
		seen[v] = struct{}{}
		out = append(out, v)
	}
	sortVersionsDesc(out)
	return out
}

func cmpVersions(a, b string) int {
	ai := sortKey(a)
	bi := sortKey(b)
	for k := 0; k < len(ai) && k < len(bi); k++ {
		if ai[k] != bi[k] {
			if ai[k] < bi[k] {
				return -1
			}
			return 1
		}
	}
	switch {
	case len(ai) < len(bi):
		return -1
	case len(ai) > len(bi):
		return 1
	}
	return 0
}

// sortKey produces a comparison vector that orders ARM version strings
// newest-first in true date order across both naming schemes:
//
//   - Modern versions (e.g. "14.3.rel1", "11.2-2022.02") share an
//     undated form that uses only the major.minor.rel digits. We
//     prepend a sentinel high year so they sort above legacy.
//   - Legacy versions ("9-2019-q4-major", "10.3-2021.10",
//     "5-2016-q1-update") all carry a 4-digit year token. We pull
//     it out as the primary key, then the quarter/month tokens that
//     follow the year, then the major-version tokens that precede it
//     (as a tiebreaker when two releases hit the same quarter).
//
// Without this, naïve element-wise compare puts e.g. "10-2020-q4-major"
// (split [10,2020,4]) above "10.3-2021.10" (split [10,3,2021,10])
// because 2020 > 3 at the second position — date-incorrect.
func sortKey(s string) []int {
	parts := splitVersion(s)
	yearIdx := -1
	for i, p := range parts {
		if p >= 2000 && p <= 2099 {
			yearIdx = i
			break
		}
	}
	if yearIdx < 0 {
		// Undated (modern) — sentinel year sorts above any real year.
		return append([]int{9999}, parts...)
	}
	out := make([]int, 0, len(parts)+1)
	out = append(out, parts[yearIdx])
	out = append(out, parts[yearIdx+1:]...)
	out = append(out, parts[:yearIdx]...)
	return out
}

var nonNumRE = regexp.MustCompile(`\D+`)

func splitVersion(s string) []int {
	parts := nonNumRE.Split(s, -1)
	out := make([]int, 0, len(parts))
	for _, p := range parts {
		if p == "" {
			continue
		}
		n := 0
		for _, c := range p {
			if c < '0' || c > '9' {
				break
			}
			n = n*10 + int(c-'0')
		}
		out = append(out, n)
	}
	return out
}
