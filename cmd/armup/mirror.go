package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/mihnen/armup/internal/arm"
	"github.com/mihnen/armup/internal/download"
)

// mirrorTask is a single (version, platform) pair the mirror command
// will fetch.
type mirrorTask struct{ version, platform string }

// probeSizes HEAD-probes each task's archive in parallel and returns
// the summed Content-Length. Probe failures count as 0 — they're
// non-fatal here; the real download is what surfaces the error.
func probeSizes(ctx context.Context, tasks []mirrorTask, concurrency int) int64 {
	in := make(chan mirrorTask, len(tasks))
	for _, t := range tasks {
		in <- t
	}
	close(in)
	var total atomic.Int64
	var wg sync.WaitGroup
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for t := range in {
				if sz, err := probeSize(ctx, t); err == nil {
					total.Add(sz)
				}
			}
		}()
	}
	wg.Wait()
	return total.Load()
}

// probeSize returns the Content-Length of the archive at (version,
// platform) without downloading the body. For local sources (file://
// or bare path) it stats the file.
func probeSize(ctx context.Context, t mirrorTask) (int64, error) {
	var src string
	if entry, ok := arm.Legacy[t.version][t.platform]; ok {
		src = entry.URL
	} else {
		host, err := arm.HostFor(t.platform)
		if err != nil {
			return 0, err
		}
		h, err := host.ResolveForVersion(ctx, t.version)
		if err != nil {
			return 0, err
		}
		src = h.ArchiveURL(t.version)
	}

	if download.IsLocal(src) {
		fi, err := os.Stat(download.LocalPath(src))
		if err != nil {
			return 0, err
		}
		return fi.Size(), nil
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodHead, src, nil)
	if err != nil {
		return 0, err
	}
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return 0, err
	}
	resp.Body.Close()
	if resp.StatusCode >= 400 {
		return 0, fmt.Errorf("HEAD %s: %s", src, resp.Status)
	}
	if resp.ContentLength < 0 {
		return 0, nil
	}
	return resp.ContentLength, nil
}

// progressTracker aggregates byte counts across concurrent downloads.
// Safe for concurrent use; all counters are atomics.
type progressTracker struct {
	transferred   atomic.Int64
	active        atomic.Int32
	expectedTotal int64 // bytes we expect to transfer; set once before workers start
	start         time.Time
}

func (p *progressTracker) add(delta int64) { p.transferred.Add(delta) }
func (p *progressTracker) startTask()      { p.active.Add(1) }
func (p *progressTracker) endTask()        { p.active.Add(-1) }

func (p *progressTracker) summary(done, total int) string {
	bytes := p.transferred.Load()
	active := p.active.Load()
	elapsed := time.Since(p.start).Seconds()
	rate := int64(0)
	if elapsed > 0 {
		rate = int64(float64(bytes) / elapsed)
	}
	// Fixed-width columns so the progress line doesn't jitter as
	// values cross unit boundaries. The done counter pads to the
	// total's digit width. Transferred bytes pads to 10 chars —
	// the universal max width humanBytes can produce ("1023.9 MiB"
	// or "1023.9 KiB" at unit-boundary moments). Total uses its
	// natural width since it's fixed once probed. Rate sits at
	// the right edge — it can flex without shifting anything else.
	w := len(strconv.Itoa(total))
	const transferredWidth = 10
	if p.expectedTotal > 0 {
		pct := float64(bytes) / float64(p.expectedTotal) * 100
		return fmt.Sprintf("[ %*d/%d done | %2d active | %*s / %s (%3.0f%%) | %s/s ]",
			w, done, total, active,
			transferredWidth, humanBytes(bytes), humanBytes(p.expectedTotal),
			pct, humanBytes(rate))
	}
	return fmt.Sprintf("[ %*d/%d done | %2d active | %*s | %s/s ]",
		w, done, total, active, transferredWidth, humanBytes(bytes), humanBytes(rate))
}

func humanBytes(n int64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	div, exp := int64(unit), 0
	for x := n / unit; x >= unit; x /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB", float64(n)/float64(div), "KMGTPE"[exp])
}

// isStderrTTY reports whether stderr is attached to a terminal.
// When stderr is a pipe or file we drop the in-place progress line
// to avoid spamming \r-escaped garbage into log captures.
func isStderrTTY() bool {
	fi, err := os.Stderr.Stat()
	if err != nil {
		return false
	}
	return (fi.Mode() & os.ModeCharDevice) != 0
}

// cmdMirror dispatches `armup mirror <subcommand>`.
func cmdMirror(ctx context.Context, args []string) error {
	if len(args) < 1 {
		return errors.New("usage: armup mirror create <dest> --platforms <list> [--versions <list>]")
	}
	switch args[0] {
	case "create":
		return cmdMirrorCreate(ctx, args[1:])
	default:
		return fmt.Errorf("unknown subcommand %q (try `armup mirror create`)", args[0])
	}
}

// cmdMirrorCreate populates a local directory with the ARM URL path
// structure so it can be served as an `ARMUP_MIRROR` target. For each
// (version, platform) pair, downloads the archive (and the SHA-256
// hash file for modern releases), verifies, and writes into <dest>
// under the same path ARM uses on developer.arm.com.
func cmdMirrorCreate(ctx context.Context, args []string) error {
	fs := newFlagSet("mirror create",
		"mirror create <dest> --platforms <list> [--versions <list>]",
		`Build a local mirror of ARM's toolchain catalog under <dest>.

The resulting directory tree matches developer.arm.com's URL path
structure, so it can be served by any web server (nginx, caddy,
'python -m http.server', etc.) and then pointed at via:

  ARMUP_MIRROR=https://internal.example.com/arm armup install <ver>

Or accessed directly via file://:

  armup install <ver> --mirror file:///srv/arm-mirror

Required:
  --platforms <list>   Comma-separated platforms to mirror, e.g.
                       linux-amd64,linux-arm64,windows-amd64. Use 'all'
                       to include every platform armup knows about.

Optional:
  --versions <list>    Comma-separated versions to mirror. Defaults to
                       every version in the unified catalog (modern
                       arm-gnu-toolchain releases + the gnu-rm legacy
                       table).

Downloads are skipped when the destination file is already present
and its SHA-256 matches; safe to re-run incrementally. Failures are
collected and reported at the end — a partial run produces a usable
mirror covering everything that succeeded.

Downloads run in parallel; the default concurrency is 4 (per-archive
progress bars are suppressed in favor of a per-task summary line).
Bump it up on a fat link or down on a metered/flaky one.`)
	platformsFlag := fs.String("platforms", "", "comma-separated platforms (required); use 'all' for every supported platform")
	versionsFlag := fs.String("versions", "", "comma-separated versions (defaults to all)")
	concurrencyFlag := fs.Int("concurrency", 4, "number of parallel downloads")
	fs.Parse(args)

	if fs.NArg() != 1 {
		fs.Usage()
		return errors.New("mirror create takes one positional argument: <dest>")
	}
	dest, err := filepath.Abs(fs.Arg(0))
	if err != nil {
		return err
	}

	platforms, err := parsePlatforms(*platformsFlag)
	if err != nil {
		return err
	}
	versions := parseVersions(*versionsFlag)
	concurrency := *concurrencyFlag
	if concurrency < 1 {
		return errors.New("--concurrency must be at least 1")
	}

	if err := os.MkdirAll(dest, 0o755); err != nil {
		return err
	}

	// Build the cross-product, filtering out combos we know ARM
	// doesn't publish so we don't waste a download attempt on a 404.
	// For unknown versions (not in the catalog yet), PlatformsFor
	// returns nil and we let the download path HEAD-probe upstream.
	var attempt []mirrorTask
	var skipped []mirrorTask
	for _, v := range versions {
		known := arm.PlatformsFor(v)
		for _, p := range platforms {
			if known != nil && !sliceContains(known, p) {
				skipped = append(skipped, mirrorTask{v, p})
				continue
			}
			attempt = append(attempt, mirrorTask{v, p})
		}
	}

	fmt.Printf("Mirror target: %s\n", dest)
	fmt.Printf("Platforms:     %s\n", strings.Join(platforms, ", "))
	fmt.Printf("Versions:      %d (%s)\n", len(versions), describeVersions(versions))
	fmt.Printf("Concurrency:   %d\n", concurrency)
	fmt.Printf("Tasks:         %d (%d combo(s) ARM doesn't publish skipped)\n", len(attempt), len(skipped))
	if len(skipped) > 0 {
		for _, t := range skipped {
			fmt.Printf("    - %s / %s\n", t.version, t.platform)
		}
	}
	fmt.Println()

	// Pre-pass: probe each archive's Content-Length in parallel so
	// the live progress display has a denominator. Failures are
	// non-fatal — a 0 size just contributes nothing to the total.
	tracker := &progressTracker{start: time.Now()}
	if len(attempt) > 0 {
		fmt.Printf("Probing archive sizes... ")
		tracker.expectedTotal = probeSizes(ctx, attempt, concurrency)
		fmt.Printf("%s total to fetch\n\n", humanBytes(tracker.expectedTotal))
	}

	type result struct {
		mirrorTask
		err error
	}

	total := len(attempt)
	tasks := make(chan mirrorTask, total)
	results := make(chan result, total)

	for _, t := range attempt {
		tasks <- t
	}
	close(tasks)

	hook := func(delta int64) { tracker.add(delta) }

	var wg sync.WaitGroup
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for t := range tasks {
				tracker.startTask()
				err := mirrorOne(ctx, dest, t.version, t.platform, hook)
				tracker.endTask()
				results <- result{t, err}
			}
		}()
	}
	go func() {
		wg.Wait()
		close(results)
	}()

	// Single-threaded display loop. Selects between the result
	// channel (one completion per receive) and a ticker that
	// refreshes the aggregate progress line. Both clear the
	// in-place progress line before printing.
	stderrIsTTY := isStderrTTY()
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	done := 0
	var failed []string
	clearLine := func() {
		if stderrIsTTY {
			fmt.Fprint(os.Stderr, "\r\033[K")
		}
	}
	printProgress := func() {
		if !stderrIsTTY {
			return
		}
		fmt.Fprintf(os.Stderr, "\r%s", tracker.summary(done, total))
	}

drain:
	for done < total {
		select {
		case <-ticker.C:
			printProgress()
		case r, ok := <-results:
			if !ok {
				break drain
			}
			done++
			clearLine()
			label := fmt.Sprintf("%s / %s", r.version, r.platform)
			prefix := fmt.Sprintf("(%d/%d)", done, total)
			switch {
			case r.err == nil:
				fmt.Printf("%s [OK]   %s\n", prefix, label)
			case errors.Is(r.err, errPlatformUnavailable):
				fmt.Printf("%s [SKIP] %-40s %v\n", prefix, label, r.err)
			default:
				fmt.Printf("%s [FAIL] %-40s %v\n", prefix, label, r.err)
				failed = append(failed, label+": "+r.err.Error())
			}
			printProgress()
		}
	}
	clearLine()
	fmt.Println()
	if len(failed) > 0 {
		fmt.Printf("Mirror: %d failure(s).\n", len(failed))
		return fmt.Errorf("%d download(s) failed", len(failed))
	}
	fmt.Println("Mirror: complete.")
	return nil
}

// errPlatformUnavailable signals that this (version, platform) combo
// simply doesn't exist upstream (ARM never published it). It's not a
// failure — the mirror skips it.
var errPlatformUnavailable = errors.New("not published for this platform")

// mirrorOne fetches a single (version, platform) archive (and the
// SHA-256 hash file for modern releases) into <dest>. Returns
// errPlatformUnavailable when the combo doesn't exist upstream. The
// hook receives byte-count deltas as the download streams, for
// aggregate progress reporting; it may be nil.
func mirrorOne(ctx context.Context, dest, version, platform string, hook download.ProgressHook) error {
	host, err := arm.HostFor(platform)
	if err != nil {
		return err
	}

	if entry, ok := arm.Legacy[version][platform]; ok {
		// Legacy entries are self-contained: URL + SHA-256 baked in.
		// Download from URL (the form that's actually reachable on
		// developer.arm.com, mangled query-string and all) but write
		// under the Mirror path when set (canonical filename — what
		// armup expects to find when it resolves the mirror URL).
		writeURL := entry.URL
		if entry.Mirror != "" {
			writeURL = entry.Mirror
		}
		rel, err := urlToRelative(writeURL)
		if err != nil {
			return err
		}
		return fetchArchiveAs(ctx, entry.URL, entry.SHA256, filepath.Join(dest, rel), hook)
	}
	if _, ok := arm.Legacy[version]; ok {
		// Legacy table has this version, just not this platform.
		return errPlatformUnavailable
	}

	// Modern path. Resolve Windows i686/x86_64 against upstream first.
	h, err := host.ResolveForVersion(ctx, version)
	if err != nil {
		// ResolveForVersion's error message is shaped for end-users
		// of `armup install`. Map it to errPlatformUnavailable so
		// the mirror loop treats it as a skip, not a failure.
		return errPlatformUnavailable
	}

	archiveURL := h.ArchiveURL(version)
	checksumURL := h.ChecksumURL(version)

	// Fetch the SHA-256 first; we both verify the archive against it
	// AND drop it into the mirror tree so armup's install-time fetch
	// can find it.
	sha, err := arm.FetchChecksum(ctx, checksumURL)
	if err != nil {
		return fmt.Errorf("fetch checksum: %w", err)
	}
	rel, err := urlToRelative(archiveURL)
	if err != nil {
		return err
	}
	if err := fetchArchiveAs(ctx, archiveURL, sha, filepath.Join(dest, rel), hook); err != nil {
		return err
	}
	return fetchSidecar(ctx, checksumURL, dest)
}

// fetchArchiveAs downloads `src` to `out`, but only if the destination
// doesn't already match the expected SHA-256. The src URL and the
// on-disk path are decoupled so the mangled-URL legacy entries can
// download from one URL and be written under their canonical name.
func fetchArchiveAs(ctx context.Context, src, sha, out string, hook download.ProgressHook) error {
	if _, err := os.Stat(out); err == nil {
		if err := arm.VerifyFile(out, sha); err == nil {
			return nil // already mirrored and verified
		}
		_ = os.Remove(out)
	}

	if err := os.MkdirAll(filepath.Dir(out), 0o755); err != nil {
		return err
	}
	if err := download.ToFileWithProgress(ctx, src, out, hook); err != nil {
		return err
	}
	if err := arm.VerifyFile(out, sha); err != nil {
		_ = os.Remove(out)
		return fmt.Errorf("checksum mismatch: %w", err)
	}
	return nil
}

// fetchSidecar writes the raw body of `url` to <dest>/<path-of-url>.
// Used for the .sha256asc files; they're tiny enough not to need
// the streaming download path.
func fetchSidecar(ctx context.Context, url, dest string) error {
	rel, err := urlToRelative(url)
	if err != nil {
		return err
	}
	out := filepath.Join(dest, rel)
	if _, err := os.Stat(out); err == nil {
		return nil // already present
	}
	if err := os.MkdirAll(filepath.Dir(out), 0o755); err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("GET %s: %s", url, resp.Status)
	}
	f, err := os.Create(out)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = io.Copy(f, resp.Body)
	return err
}

// urlToRelative turns a developer.arm.com URL into the path-from-mirror-root
// that we want to write the file to. Strips the scheme + host and any
// query string (the latter only matters for pre-2017 mangled URLs).
func urlToRelative(u string) (string, error) {
	parsed, err := url.Parse(u)
	if err != nil {
		return "", err
	}
	if parsed.Path == "" {
		return "", fmt.Errorf("URL %s has no path", u)
	}
	// On Windows, forward-slash URL paths translate to backslashes
	// for filesystem ops; filepath.FromSlash handles that.
	return filepath.FromSlash(strings.TrimPrefix(parsed.Path, "/")), nil
}

func parsePlatforms(spec string) ([]string, error) {
	if spec == "" {
		return nil, errors.New("--platforms is required (e.g. --platforms linux-amd64,windows-amd64, or --platforms all)")
	}
	if spec == "all" {
		out := make([]string, len(arm.SupportedPlatforms))
		copy(out, arm.SupportedPlatforms)
		return out, nil
	}
	parts := strings.Split(spec, ",")
	out := make([]string, 0, len(parts))
	known := make(map[string]bool, len(arm.SupportedPlatforms))
	for _, p := range arm.SupportedPlatforms {
		known[p] = true
	}
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if !known[p] {
			return nil, fmt.Errorf("unknown platform %q (supported: %s)", p, strings.Join(arm.SupportedPlatforms, ", "))
		}
		out = append(out, p)
	}
	return out, nil
}

// parseVersions returns the requested versions list. Empty input
// expands to the full unified catalog (Curated + LegacyAllVersions),
// sorted newest-first.
func parseVersions(spec string) []string {
	if strings.TrimSpace(spec) == "" {
		return arm.MergeAvailable(arm.Curated)
	}
	parts := strings.Split(spec, ",")
	out := make([]string, 0, len(parts))
	for _, v := range parts {
		v = strings.TrimSpace(v)
		if v == "" {
			continue
		}
		out = append(out, arm.Normalize(v))
	}
	return out
}

func sliceContains(s []string, v string) bool {
	for _, x := range s {
		if x == v {
			return true
		}
	}
	return false
}

func describeVersions(versions []string) string {
	switch {
	case len(versions) == 0:
		return "none"
	case len(versions) <= 3:
		return strings.Join(versions, ", ")
	default:
		return fmt.Sprintf("%s, %s, ... %s", versions[0], versions[1], versions[len(versions)-1])
	}
}
