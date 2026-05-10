package main

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/mihnen/armup/internal/arm"
	"github.com/mihnen/armup/internal/paths"
	"github.com/mihnen/armup/internal/pin"
	"github.com/mihnen/armup/internal/store"
)

// cmdDoctor prints a self-diagnostic. The exit code is non-zero only
// when at least one check is FAIL — warnings (broken pin, stale staging,
// no PATH entry) print but don't fail scripts.
func cmdDoctor(ctx context.Context, args []string) error {
	fs := newFlagSet("doctor", "doctor",
		`Print a self-diagnostic of armup's state.

Reports OK / WARN / FAIL on each check: data directory layout,
PATH, active toolchain runnability, pinned version, installed
versions, leftover staging directories, and the cached available
list. Useful for triaging "armup says it's installed but the
toolchain can't run" issues.

Exits non-zero only when a check fails outright; warnings still
exit 0 so scripts can run doctor opportunistically.`)
	fs.Parse(args)

	d := &doctor{}
	d.printf("armup %s\n", version)
	d.printf("platform: %s/%s\n\n", runtime.GOOS, runtime.GOARCH)

	d.checkDataDir()
	d.checkPATH()
	d.checkCurrent(ctx)
	d.checkInstalled()
	d.checkStaging()
	d.checkPin()
	d.checkAvailableCache()
	d.checkCacheSize()

	fmt.Println()
	switch {
	case d.fails > 0:
		fmt.Printf("Doctor: %d issue(s), %d warning(s).\n", d.fails, d.warns)
		return fmt.Errorf("doctor reported %d failure(s)", d.fails)
	case d.warns > 0:
		fmt.Printf("Doctor: 0 issues, %d warning(s).\n", d.warns)
	default:
		fmt.Println("Doctor: clean.")
	}
	return nil
}

type doctor struct {
	fails int
	warns int
}

func (d *doctor) printf(f string, a ...any) { fmt.Printf(f, a...) }

func (d *doctor) ok(label, detail string) {
	if detail != "" {
		fmt.Printf("[OK]   %-40s %s\n", label, detail)
	} else {
		fmt.Printf("[OK]   %s\n", label)
	}
}

func (d *doctor) warn(label, detail, hint string) {
	d.warns++
	fmt.Printf("[WARN] %-40s %s\n", label, detail)
	if hint != "" {
		fmt.Printf("       %s\n", hint)
	}
}

func (d *doctor) fail(label, detail, hint string) {
	d.fails++
	fmt.Printf("[FAIL] %-40s %s\n", label, detail)
	if hint != "" {
		fmt.Printf("       %s\n", hint)
	}
}

func (d *doctor) info(label, detail string) {
	fmt.Printf("[INFO] %-40s %s\n", label, detail)
}

func (d *doctor) checkDataDir() {
	dir := paths.DataDir()
	if _, err := os.Stat(dir); err != nil {
		d.fail("data dir exists", err.Error(), "Run `armup init` to create it.")
		return
	}
	d.ok("data dir exists", dir)

	for _, sub := range []string{"versions", "cache"} {
		p := filepath.Join(dir, sub)
		if _, err := os.Stat(p); err != nil {
			d.warn(sub+"/ readable", err.Error(), "")
		}
	}
}

// checkPATH walks the user's PATH looking for either the live
// current/bin entry, or any entry matching <data>/current/bin
// (case-insensitive on Windows). A missing entry is a WARN — armup
// still works for `install`/`list`/`use`, but newly-spawned shells
// won't find arm-none-eabi-* on PATH.
func (d *doctor) checkPATH() {
	want := filepath.Join(paths.DataDir(), "current", "bin")
	got := os.Getenv("PATH")
	for _, p := range filepath.SplitList(got) {
		if pathsEqual(p, want) {
			d.ok("PATH includes current/bin", want)
			return
		}
	}
	d.warn("PATH includes current/bin", "not found in $PATH",
		"Run `armup init` (then open a new shell) to add "+want+".")
}

func pathsEqual(a, b string) bool {
	if runtime.GOOS == "windows" {
		return strings.EqualFold(filepath.Clean(a), filepath.Clean(b))
	}
	return filepath.Clean(a) == filepath.Clean(b)
}

func (d *doctor) checkCurrent(ctx context.Context) {
	cur, err := store.Current()
	if err != nil {
		d.fail("current symlink readable", err.Error(), "")
		return
	}
	if cur == "" {
		d.info("current symlink", "(no version active — run `armup use <version>`)")
		return
	}

	verDir := paths.VersionDir(cur)
	if _, err := os.Stat(verDir); err != nil {
		d.fail("current symlink target exists",
			fmt.Sprintf("%s → %s (missing)", cur, verDir),
			"Run `armup use <version>` to point at an installed version.")
		return
	}
	d.ok("current symlink", "→ "+cur)

	gcc := filepath.Join(verDir, "bin", "arm-none-eabi-gcc")
	if runtime.GOOS == "windows" {
		gcc += ".exe"
	}
	if _, err := os.Stat(gcc); err != nil {
		d.fail("current toolchain has gcc binary", err.Error(),
			"Re-install with `armup install "+cur+"`.")
		return
	}
	out, err := exec.CommandContext(ctx, gcc, "--version").Output()
	if err != nil {
		d.fail("current toolchain runs", err.Error(),
			"Try `"+gcc+" --version` directly to investigate.")
		return
	}
	banner := strings.SplitN(strings.TrimSpace(string(out)), "\n", 2)[0]
	d.ok("current toolchain runs", banner)
}

func (d *doctor) checkInstalled() {
	versions, err := store.List()
	if err != nil {
		d.fail("list installed versions", err.Error(), "")
		return
	}
	if len(versions) == 0 {
		d.info("installed versions", "(none — run `armup install <version>`)")
		return
	}
	cur, _ := store.Current()
	fmt.Printf("[INFO] %-40s %d\n", "installed versions", len(versions))
	for _, v := range versions {
		size := dirSize(paths.VersionDir(v))
		marker := "  "
		if v == cur {
			marker = " *"
		}
		fmt.Printf("       %s %-22s %s\n", marker, v, humanSize(size))
	}
}

// .staging-<name> directories are created during install and renamed
// to versions/<name> on success. Their presence indicates an
// interrupted install — an `armup uninstall <name>` (or manual
// removal) is the cleanup.
func (d *doctor) checkStaging() {
	versionsDir := paths.VersionsDir()
	entries, err := os.ReadDir(versionsDir)
	if err != nil {
		// Already reported by checkDataDir; don't double-warn.
		return
	}
	var stale []string
	for _, e := range entries {
		if e.IsDir() && strings.HasPrefix(e.Name(), ".staging-") {
			stale = append(stale, e.Name())
		}
	}
	if len(stale) == 0 {
		d.ok("no leftover .staging-* directories", "")
		return
	}
	d.warn("leftover .staging-* directories",
		fmt.Sprintf("%d found", len(stale)),
		"Remove with: rm -rf "+filepath.Join(versionsDir, ".staging-*"))
}

// checkPin reports the per-project pin (if any), notes whether the
// pinned version is installed, and whether it matches the active one.
// A missing pin is INFO, not WARN — most projects don't pin.
func (d *doctor) checkPin() {
	cwd, err := os.Getwd()
	if err != nil {
		d.warn("pin file lookup", err.Error(), "")
		return
	}
	r, err := pin.Resolve(cwd)
	if err != nil {
		d.warn("pin file readable", err.Error(), "")
		return
	}
	if !r.Found() {
		d.info("project pin", "(none — no .tool-versions, .armup-version, or $ARMUP_VERSION)")
		return
	}
	verDir := paths.VersionDir(r.Version)
	cur, _ := store.Current()
	detail := fmt.Sprintf("%s (from %s)", r.Version, r.Source)

	if _, err := os.Stat(verDir); err != nil {
		d.warn("project pin", detail,
			"Pinned version not installed — run `armup install`.")
		return
	}
	if cur != r.Version {
		d.warn("project pin", detail+" — does not match active "+cur,
			"Run `armup use` (no args) to switch to the pin.")
		return
	}
	d.ok("project pin", detail+" (matches active)")
}

func (d *doctor) checkAvailableCache() {
	path := paths.AvailableFile()
	st, err := os.Stat(path)
	if errors.Is(err, fs.ErrNotExist) {
		d.info("available list cache", "(empty — run `armup available --refresh`)")
		return
	}
	if err != nil {
		d.warn("available list cache", err.Error(), "")
		return
	}
	versions, _ := arm.LoadCachedAvailable(path)
	age := time.Since(st.ModTime()).Round(time.Hour)
	d.info("available list cache",
		fmt.Sprintf("%d versions, refreshed %s ago", len(versions), humanDuration(age)))
}

func (d *doctor) checkCacheSize() {
	dir := paths.CacheDir()
	size := dirSize(dir)
	count := 0
	_ = filepath.WalkDir(dir, func(_ string, e fs.DirEntry, err error) error {
		if err != nil || e.IsDir() {
			return nil
		}
		count++
		return nil
	})
	d.info("download cache", fmt.Sprintf("%d file(s), %s", count, humanSize(size)))
}

func dirSize(dir string) int64 {
	var total int64
	_ = filepath.WalkDir(dir, func(_ string, e fs.DirEntry, err error) error {
		if err != nil || e.IsDir() {
			return nil
		}
		info, err := e.Info()
		if err != nil {
			return nil
		}
		total += info.Size()
		return nil
	})
	return total
}

func humanSize(n int64) string {
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

func humanDuration(d time.Duration) string {
	switch {
	case d < time.Hour:
		return fmt.Sprintf("%d min", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%d hour(s)", int(d.Hours()))
	default:
		return fmt.Sprintf("%d day(s)", int(d.Hours()/24))
	}
}
