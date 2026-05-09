package store

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/mihnen/armup/internal/archive"
	"github.com/mihnen/armup/internal/arm"
	"github.com/mihnen/armup/internal/download"
	"github.com/mihnen/armup/internal/paths"
)

// promoteExtraction moves the toolchain root out of stagingDir and into
// verDir. Handles three archive shapes that show up in practice:
//
//  1. "wrapped, expected name" — the modern arm-gnu-toolchain path.
//     stagingDir/<innerName>/{bin,lib,...} exists and we rename that
//     subdir to verDir. innerName comes from arm.Host.InnerDirName.
//  2. "wrapped, arbitrary name" — legacy ARM (gnu-rm) ships
//     gcc-arm-none-eabi-<ver>/, custom builds may use anything else.
//     We detect: if the staging dir contains exactly one subdir
//     (top-level files like a .version sidecar are tolerated), promote
//     that subdir.
//  3. "unwrapped" — newer Windows zips (15.x) put bin/, lib/, etc. at
//     the archive root with no wrapping dir. The staging dir itself is
//     the toolchain root; rename it to verDir.
//
// Pass innerName="" to skip step 1 (used by InstallFromSource where the
// inner name isn't known).
func promoteExtraction(stagingDir, innerName, verDir string) error {
	// Step 1: expected wrapping name.
	if innerName != "" {
		innerDir := filepath.Join(stagingDir, innerName)
		if _, err := os.Stat(innerDir); err == nil {
			if err := os.Rename(innerDir, verDir); err != nil {
				return fmt.Errorf("rename to %s: %w", verDir, err)
			}
			os.RemoveAll(stagingDir)
			return nil
		} else if !os.IsNotExist(err) {
			return err
		}
	}

	// Step 2: detect a single-subdir layout regardless of name.
	entries, err := os.ReadDir(stagingDir)
	if err != nil {
		return err
	}
	var subdirs []string
	for _, e := range entries {
		if e.IsDir() {
			subdirs = append(subdirs, e.Name())
		}
	}
	if len(subdirs) == 1 {
		// One top-level dir; treat any non-dir siblings (.version files,
		// LICENSE.txt, etc.) as ignorable since they wouldn't break the
		// promote-and-cleanup either way.
		from := filepath.Join(stagingDir, subdirs[0])
		if err := os.Rename(from, verDir); err != nil {
			return fmt.Errorf("rename to %s: %w", verDir, err)
		}
		os.RemoveAll(stagingDir)
		return nil
	}

	// Step 3: unwrapped — staging IS the toolchain root.
	if err := os.Rename(stagingDir, verDir); err != nil {
		return fmt.Errorf("rename to %s: %w", verDir, err)
	}
	return nil
}

// Reset removes everything armup has created under DataDir(): all installed
// versions, the current link, the cache, and any ancillary state. Does not
// touch the armup binary itself or any shell rc / registry config — those
// are the caller's responsibility.
func Reset() error {
	return os.RemoveAll(paths.DataDir())
}

// EnsureLayout creates the directory layout if missing.
func EnsureLayout() error {
	for _, d := range []string{paths.DataDir(), paths.VersionsDir(), paths.CacheDir()} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			return err
		}
	}
	return nil
}

// List returns the installed versions, sorted descending.
func List() ([]string, error) {
	entries, err := os.ReadDir(paths.VersionsDir())
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var out []string
	for _, e := range entries {
		if e.IsDir() {
			out = append(out, e.Name())
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i] > out[j] })
	return out, nil
}

// Current returns the version `current` resolves to, or "" if unset.
func Current() (string, error) {
	target, err := os.Readlink(paths.CurrentLink())
	if os.IsNotExist(err) {
		return "", nil
	}
	if err != nil {
		var pErr *os.PathError
		if errors.As(err, &pErr) {
			return "", nil
		}
		return "", err
	}
	return filepath.Base(target), nil
}

// Use atomically retargets `current` to point at the given version.
// Errors if version is not installed.
func Use(version string) error {
	dir := paths.VersionDir(version)
	st, err := os.Stat(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("version %s is not installed", version)
		}
		return err
	}
	if !st.IsDir() {
		return fmt.Errorf("%s is not a directory", dir)
	}
	return paths.SetCurrent(dir, paths.CurrentLink())
}

// Uninstall removes a version's directory. Refuses to remove the current
// version unless force is true; with force, the current symlink is also
// removed.
func Uninstall(version string, force bool) error {
	cur, _ := Current()
	if cur == version && !force {
		return fmt.Errorf("%s is the current version; pass --force to uninstall it", version)
	}
	if cur == version && force {
		os.Remove(paths.CurrentLink())
	}
	dir := paths.VersionDir(version)
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return fmt.Errorf("version %s is not installed", version)
	}
	return os.RemoveAll(dir)
}

// InstallSourceOpts configures InstallFromSource.
type InstallSourceOpts struct {
	// Source is an HTTPS/HTTP URL, a file:// URI, or a local filesystem
	// path. Required.
	Source string
	// As is the version slot name to install under. If empty, derived
	// from the source's basename with archive extensions stripped.
	As string
	// SHA256, if non-empty, must match the archive's hex SHA-256 digest.
	// If empty, the install proceeds with a stderr warning.
	SHA256 string
	// SetCurrentIfFirst: when true and no version is currently active,
	// the freshly-installed one becomes active. Same semantics as Install.
	SetCurrentIfFirst bool
}

// InstallFromSource installs a toolchain from an arbitrary source — used
// for legacy ARM versions that don't fit the developer.arm.com URL
// pattern, internal mirrors, and custom GCC builds. The source can be a
// remote URL or a local path; see CLI docs.
//
// On success, the toolchain ends up at versions/<As>/.
func InstallFromSource(ctx context.Context, opts InstallSourceOpts) error {
	if opts.Source == "" {
		return errors.New("source is required")
	}
	if err := EnsureLayout(); err != nil {
		return err
	}

	srcPath, srcFile, isLocal, err := resolveSource(opts.Source)
	if err != nil {
		return err
	}

	name := opts.As
	if name == "" {
		name = deriveAsName(srcFile)
	}
	if err := validateAsName(name); err != nil {
		return err
	}

	verDir := paths.VersionDir(name)
	if _, err := os.Stat(verDir); err == nil {
		return fmt.Errorf("version %s is already installed at %s; pass --as <name> to install under a different name (or `armup uninstall %s` to replace it)",
			name, verDir, name)
	}

	// Resolve the on-disk path of the archive (downloading if remote).
	var archivePath string
	switch {
	case isLocal:
		archivePath = srcPath
		fmt.Printf("Using local archive %s\n", archivePath)
	default:
		archivePath = filepath.Join(paths.CacheDir(), srcFile)
		fmt.Printf("Downloading %s\n", opts.Source)
		if err := download.ToFile(ctx, opts.Source, archivePath); err != nil {
			return err
		}
	}

	// Verify or warn.
	if opts.SHA256 != "" {
		if err := arm.VerifyFile(archivePath, opts.SHA256); err != nil {
			if !isLocal {
				os.Remove(archivePath)
			}
			return err
		}
	} else {
		fmt.Fprintln(os.Stderr, "warning: --sha256 not provided; archive integrity is not verified")
	}

	stagingDir := filepath.Join(paths.VersionsDir(), ".staging-"+name)
	os.RemoveAll(stagingDir)
	if err := os.MkdirAll(stagingDir, 0o755); err != nil {
		return err
	}

	fmt.Printf("Extracting %s\n", filepath.Base(archivePath))
	extractStart := time.Now()
	if err := archive.Extract(ctx, archivePath, stagingDir); err != nil {
		os.RemoveAll(stagingDir)
		return err
	}
	fmt.Printf("Extracted in %s\n", time.Since(extractStart).Round(time.Millisecond))

	// innerName="" — we don't know the wrapping dir name for arbitrary
	// sources; detection falls through to step 2/3.
	if err := promoteExtraction(stagingDir, "", verDir); err != nil {
		os.RemoveAll(stagingDir)
		return err
	}

	if opts.SetCurrentIfFirst {
		if cur, _ := Current(); cur == "" {
			if err := Use(name); err != nil {
				return fmt.Errorf("set current: %w", err)
			}
		}
	}
	return nil
}

// resolveSource normalizes a --from value into (path-on-disk, basename,
// is-local). For remote sources, path-on-disk is "" (caller downloads).
func resolveSource(src string) (path, basename string, isLocal bool, err error) {
	switch {
	case strings.HasPrefix(src, "http://") || strings.HasPrefix(src, "https://"):
		u, err := url.Parse(src)
		if err != nil {
			return "", "", false, fmt.Errorf("parse %s: %w", src, err)
		}
		base := filepath.Base(u.Path)
		if base == "." || base == "/" || base == "" {
			return "", "", false, fmt.Errorf("cannot derive a filename from URL %s", src)
		}
		return "", base, false, nil
	case strings.HasPrefix(src, "file://"):
		u, err := url.Parse(src)
		if err != nil {
			return "", "", false, fmt.Errorf("parse %s: %w", src, err)
		}
		p := u.Path
		// Windows file URIs are file:///C:/path; url.Parse leaves the Path
		// as "/C:/path". Strip the leading slash so filepath.Abs handles
		// it correctly.
		if runtime.GOOS == "windows" && len(p) >= 3 && p[0] == '/' && p[2] == ':' {
			p = p[1:]
		}
		abs, err := filepath.Abs(p)
		if err != nil {
			return "", "", false, err
		}
		if _, err := os.Stat(abs); err != nil {
			return "", "", false, err
		}
		return abs, filepath.Base(abs), true, nil
	default:
		// Bare local path. Resolve relative paths to absolute so error
		// messages and the cache layout are stable. UNC paths on Windows
		// pass through unchanged.
		abs, err := filepath.Abs(src)
		if err != nil {
			return "", "", false, err
		}
		if _, err := os.Stat(abs); err != nil {
			return "", "", false, err
		}
		return abs, filepath.Base(abs), true, nil
	}
}

// archiveExts are stripped (in order) from the source basename when
// deriving a default --as name. Strip the longest match first.
var archiveExts = []string{".tar.bz2", ".tar.gz", ".tar.xz", ".zip"}

func deriveAsName(basename string) string {
	low := strings.ToLower(basename)
	for _, ext := range archiveExts {
		if strings.HasSuffix(low, ext) {
			return basename[:len(basename)-len(ext)]
		}
	}
	return basename
}

// reservedAsNames are subdirs of DataDir that armup uses for its own
// state. Refusing them prevents a --as that would collide with internal
// layout.
var reservedAsNames = map[string]bool{
	"current": true, "versions": true, "cache": true,
}

func validateAsName(name string) error {
	if name == "" {
		return errors.New("--as name cannot be empty (could not derive from source filename)")
	}
	if reservedAsNames[name] {
		return fmt.Errorf("--as name %q is reserved by armup; pick something else", name)
	}
	if strings.ContainsAny(name, `/\`) {
		return fmt.Errorf("--as name %q must not contain path separators", name)
	}
	if name == "." || name == ".." {
		return fmt.Errorf("--as name %q is invalid", name)
	}
	return nil
}

// Install downloads, verifies, and extracts a version.
// If setCurrentIfFirst is true and no current version is set, this becomes
// the active one.
func Install(ctx context.Context, version string, setCurrentIfFirst bool) error {
	host, err := arm.CurrentHost()
	if err != nil {
		return err
	}
	if err := EnsureLayout(); err != nil {
		return err
	}

	verDir := paths.VersionDir(version)
	if _, err := os.Stat(verDir); err == nil {
		// Idempotent: re-running install on an installed version is not
		// an error. Skip the network round-trip and report the no-op.
		fmt.Printf("Version %s is already installed at %s\n", version, verDir)
		return nil
	}

	host, err = host.ResolveForVersion(ctx, version)
	if err != nil {
		return err
	}

	archiveName := host.ArchiveFilename(version)
	archivePath := filepath.Join(paths.CacheDir(), archiveName)
	archiveURL := host.ArchiveURL(version)
	checksumURL := host.ChecksumURL(version)

	fmt.Printf("Fetching checksum for %s\n", version)
	expected, err := arm.FetchChecksum(ctx, checksumURL)
	if err != nil {
		return fmt.Errorf("fetch checksum: %w", err)
	}

	needDownload := true
	if _, err := os.Stat(archivePath); err == nil {
		if err := arm.VerifyFile(archivePath, expected); err == nil {
			fmt.Printf("Using cached %s\n", archiveName)
			needDownload = false
		} else {
			fmt.Printf("Cached archive failed verification, re-downloading\n")
			os.Remove(archivePath)
		}
	}
	if needDownload {
		fmt.Printf("Downloading %s\n", archiveURL)
		if err := download.ToFile(ctx, archiveURL, archivePath); err != nil {
			return err
		}
		if err := arm.VerifyFile(archivePath, expected); err != nil {
			os.Remove(archivePath)
			return err
		}
	}

	stagingDir := filepath.Join(paths.VersionsDir(), ".staging-"+version)
	os.RemoveAll(stagingDir)
	if err := os.MkdirAll(stagingDir, 0o755); err != nil {
		return err
	}

	fmt.Printf("Extracting %s\n", archiveName)
	extractStart := time.Now()
	if err := archive.Extract(ctx, archivePath, stagingDir); err != nil {
		os.RemoveAll(stagingDir)
		return err
	}
	fmt.Printf("Extracted in %s\n", time.Since(extractStart).Round(time.Millisecond))

	if err := promoteExtraction(stagingDir, host.InnerDirName(version), verDir); err != nil {
		os.RemoveAll(stagingDir)
		return err
	}

	if setCurrentIfFirst {
		if cur, _ := Current(); cur == "" {
			if err := Use(version); err != nil {
				return fmt.Errorf("set current: %w", err)
			}
		}
	}
	return nil
}
