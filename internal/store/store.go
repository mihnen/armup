package store

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/mihnen/armup/internal/archive"
	"github.com/mihnen/armup/internal/arm"
	"github.com/mihnen/armup/internal/download"
	"github.com/mihnen/armup/internal/paths"
)

// promoteExtraction moves the toolchain root out of stagingDir and into
// verDir. ARM ships two layouts and we have to handle both:
//
//   - "wrapped"   (most archives): top of the archive is a single directory
//     named innerName, with bin/, lib/, etc. inside it.
//   - "unwrapped" (newer Windows zips, 15.x): bin/, lib/, etc. sit at the
//     top of the archive directly, with no wrapping directory.
//
// If stagingDir/<innerName> exists, rename it to verDir and remove the now-
// empty stagingDir. Otherwise rename stagingDir itself to verDir.
func promoteExtraction(stagingDir, innerName, verDir string) error {
	innerDir := filepath.Join(stagingDir, innerName)
	src := innerDir
	if _, err := os.Stat(innerDir); err != nil {
		if !os.IsNotExist(err) {
			return err
		}
		src = stagingDir
	}
	if err := os.Rename(src, verDir); err != nil {
		return fmt.Errorf("rename to %s: %w", verDir, err)
	}
	if src == innerDir {
		os.RemoveAll(stagingDir)
	}
	return nil
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
		return fmt.Errorf("version %s is already installed at %s", version, verDir)
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
