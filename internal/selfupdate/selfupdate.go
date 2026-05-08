// Package selfupdate replaces the running armup binary with the latest
// release from GitHub. The flow:
//
//  1. Query the GitHub Releases API for the most recent release tag
//     (including prereleases — /releases, not /releases/latest).
//  2. Compare with the running binary's embedded version. If they match,
//     no-op. If the running binary is "dev" (locally built), refuse —
//     a developer rebuild is the right move there.
//  3. Build the platform archive name, download it, fetch SHA256SUMS,
//     verify the matching entry.
//  4. Extract just the new armup binary out of the archive.
//  5. Atomically replace the running binary. On Linux/macOS that's a
//     simple rename. On Windows the running .exe holds an open handle,
//     so we rename it to .old first and write the new file at the
//     original path; the running process keeps using the .old via its
//     open handle.
package selfupdate

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"

	"github.com/mihnen/armup/internal/archive"
	"github.com/mihnen/armup/internal/download"
)

// nightlyTag is the fixed tag name produced by .github/workflows/nightly.yml.
const nightlyTag = "nightly"

const (
	owner = "mihnen"
	repo  = "armup"
)

// Run replaces the running armup binary with the latest release. Errors
// out for "dev" builds (rebuild from source instead) and is a no-op when
// already on the latest release.
//
// When `nightly` is true, the rolling master-tracking pre-release is
// fetched instead of the latest semver release. Nightly fetches always
// proceed (no version-equality short-circuit) since the embedded version
// includes a commit SHA that's unlikely to match the just-built one.
func Run(ctx context.Context, currentVersion string, nightly bool) error {
	if currentVersion == "dev" {
		return errors.New("self-update is not available on local dev builds; rebuild from source")
	}

	var tag string
	if nightly {
		tag = nightlyTag
	} else {
		t, err := latestStableTag(ctx)
		if err != nil {
			return fmt.Errorf("query releases: %w", err)
		}
		tag = t
		if tag == currentVersion {
			fmt.Printf("armup %s is already the latest release\n", currentVersion)
			return nil
		}
	}
	fmt.Printf("Current: %s\nLatest:  %s\n", currentVersion, tag)

	archiveName, ext, binName := platformArchive(tag)
	archiveURL := fmt.Sprintf("https://github.com/%s/%s/releases/download/%s/%s", owner, repo, tag, archiveName)
	sumsURL := fmt.Sprintf("https://github.com/%s/%s/releases/download/%s/SHA256SUMS", owner, repo, tag)

	tmp, err := os.MkdirTemp("", "armup-selfupdate-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmp)

	archivePath := filepath.Join(tmp, archiveName)
	fmt.Printf("Downloading %s\n", archiveURL)
	if err := download.ToFile(ctx, archiveURL, archivePath); err != nil {
		return err
	}

	fmt.Println("Verifying checksum")
	expected, err := fetchExpectedSum(ctx, sumsURL, archiveName)
	if err != nil {
		return fmt.Errorf("fetch checksum: %w", err)
	}
	if err := verifyFile(archivePath, expected); err != nil {
		return err
	}

	fmt.Println("Extracting")
	if err := archive.Extract(ctx, archivePath, tmp); err != nil {
		return err
	}

	innerDir := strings.TrimSuffix(archiveName, ext)
	newBinary := filepath.Join(tmp, innerDir, binName)
	if _, err := os.Stat(newBinary); err != nil {
		return fmt.Errorf("expected new binary at %s: %w", newBinary, err)
	}

	self, err := os.Executable()
	if err != nil {
		return err
	}
	if resolved, err := filepath.EvalSymlinks(self); err == nil {
		self = resolved
	}

	if err := replaceBinary(self, newBinary); err != nil {
		return fmt.Errorf("replace running binary at %s: %w", self, err)
	}

	fmt.Printf("\nUpdated to %s. Re-run armup to use the new version.\n", tag)
	return nil
}

// platformArchive returns the archive filename, its extension, and the
// expected binary name inside the archive for the running OS/arch.
func platformArchive(tag string) (archiveName, ext, binName string) {
	binName = "armup"
	if runtime.GOOS == "windows" {
		ext = ".zip"
		binName = "armup.exe"
	} else {
		ext = ".tar.gz"
	}
	archiveName = fmt.Sprintf("armup-%s-%s-%s%s", tag, runtime.GOOS, runtime.GOARCH, ext)
	return
}

// latestStableTag returns the tag of the most recent published *semver*
// release. Uses /releases (not /releases/latest) so prereleases like
// v1.0.0-beta1 still count. Filters out non-semver tags so the rolling
// `nightly` release doesn't accidentally show up as "latest".
func latestStableTag(ctx context.Context) (string, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/releases", owner, repo)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Accept", "application/vnd.github+json")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("GET %s: %w", url, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("GET %s: %s", url, resp.Status)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	return parseFirstStableTag(body)
}

// semverTagRE matches v0.1.0, v1.2.3, v1.2.3-rc1, v1.0.0-beta.2 — the
// shape produced by release.yml's tag filter. Anything else (e.g.
// `nightly`) is skipped.
var semverTagRE = regexp.MustCompile(`^v\d+\.\d+\.\d+(-[A-Za-z0-9.]+)?$`)

func parseFirstStableTag(body []byte) (string, error) {
	type release struct {
		TagName string `json:"tag_name"`
		Draft   bool   `json:"draft"`
	}
	var releases []release
	if err := json.Unmarshal(body, &releases); err != nil {
		return "", fmt.Errorf("parse releases JSON: %w", err)
	}
	for _, r := range releases {
		if r.Draft {
			continue
		}
		if !semverTagRE.MatchString(r.TagName) {
			continue
		}
		return r.TagName, nil
	}
	return "", errors.New("no published stable releases found")
}

// fetchExpectedSum downloads the SHA256SUMS file and returns the hex digest
// for the line matching `file`.
func fetchExpectedSum(ctx context.Context, url, file string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("GET %s: %w", url, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("GET %s: %s", url, resp.Status)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	return findSum(string(body), file)
}

// findSum scans a SHA256SUMS-formatted body for the entry matching `file`.
// Format per line: "<hex>  <file>" or "<hex>  *<file>" (binary mode).
func findSum(body, file string) (string, error) {
	for _, line := range strings.Split(body, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		name := strings.TrimPrefix(fields[1], "*")
		if name == file {
			return strings.ToLower(fields[0]), nil
		}
	}
	return "", fmt.Errorf("file %q not found in SHA256SUMS", file)
}

// verifyFile checks the SHA-256 of `path` against `expected` (hex).
func verifyFile(path, expected string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return err
	}
	got := hex.EncodeToString(h.Sum(nil))
	if !strings.EqualFold(got, expected) {
		return fmt.Errorf("checksum mismatch for %s: expected %s, got %s", path, expected, got)
	}
	return nil
}

// replaceBinary atomically replaces the file at `target` with the contents
// of `newPath`.
//
//   - On unix, write a sibling temp file then rename. The kernel keeps the
//     old inode alive for the running process; new launches see the new
//     binary.
//   - On Windows, the running .exe holds an exclusive handle that blocks
//     in-place writes but allows rename. So we move the running file to
//     `target.old`, then write the new bytes to `target`. The .old is
//     released when the current process exits and the OS reaps it.
func replaceBinary(target, newPath string) error {
	if runtime.GOOS == "windows" {
		return replaceWindows(target, newPath)
	}
	return replaceUnix(target, newPath)
}

func replaceUnix(target, newPath string) error {
	tmp := target + ".new"
	if err := copyExecutable(newPath, tmp); err != nil {
		return err
	}
	if err := os.Rename(tmp, target); err != nil {
		os.Remove(tmp)
		return err
	}
	return nil
}

func replaceWindows(target, newPath string) error {
	old := target + ".old"
	// Best-effort cleanup of a previous self-update's leftover.
	_ = os.Remove(old)
	if err := os.Rename(target, old); err != nil {
		return fmt.Errorf("rename running binary out of the way: %w", err)
	}
	if err := copyExecutable(newPath, target); err != nil {
		// Try to put the original back; if rename fails the user is left
		// with target.old containing the working binary, which is recoverable.
		if rerr := os.Rename(old, target); rerr != nil {
			return fmt.Errorf("write new binary failed (%v); restore also failed (%v); recover by renaming %s back to %s",
				err, rerr, old, target)
		}
		return err
	}
	return nil
}

func copyExecutable(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o755)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		os.Remove(dst)
		return err
	}
	return out.Close()
}
