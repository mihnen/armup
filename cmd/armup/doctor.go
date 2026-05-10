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
//
// Optional cleanup flags apply selected fixes after the read-only walk:
//
//	--fix             remove .staging-* dirs and dangling current symlink
//	--clean-cache     wipe the download cache
//	--remove-broken   delete installed versions whose gcc can't run
func cmdDoctor(ctx context.Context, args []string) error {
	fs := newFlagSet("doctor", "doctor [--fix] [--clean-cache] [--remove-broken]",
		`Print a self-diagnostic of armup's state.

Reports OK / WARN / FAIL on data directory layout, PATH, active
toolchain runnability, pinned version, installed versions (each
verified to run), leftover staging directories, orphan files,
and the cached available list. Useful for triaging "armup says
it's installed but the toolchain can't run" issues.

Cleanup flags (applied after the read-only walk):

  --fix              Remove leftover .staging-<name> directories
                     and a dangling 'current' symlink. Safe — these
                     are pure broken-state artifacts.
  --clean-cache      Wipe <data>/cache/. Removes downloaded archives
                     left over from before armup auto-cleaned them
                     or from --keep-archive installs.
  --remove-broken    Delete installed versions whose
                     bin/arm-none-eabi-gcc is missing or won't run.

Flags can be combined. Exits non-zero when a FAIL remains after
any selected fixes; warnings exit 0.`)
	fixFlag := fs.Bool("fix", false, "remove .staging-* dirs and a dangling current symlink")
	cleanCacheFlag := fs.Bool("clean-cache", false, "wipe the download cache")
	removeBrokenFlag := fs.Bool("remove-broken", false, "delete installed versions whose gcc won't run")
	fs.Parse(args)

	d := &doctor{ctx: ctx}
	d.printf("armup %s\n", version)
	d.printf("platform: %s/%s\n\n", runtime.GOOS, runtime.GOARCH)

	d.checkDataDir()
	d.checkPATH()
	d.checkCurrent()
	d.checkInstalled()
	d.checkOrphans()
	d.checkStaging()
	d.checkPin()
	d.checkAvailableCache()
	d.checkCacheSize()

	if *fixFlag || *cleanCacheFlag || *removeBrokenFlag {
		fmt.Println()
		fmt.Println("Applying fixes:")
		if *fixFlag {
			d.fixStaging()
			d.fixDanglingCurrent()
		}
		if *cleanCacheFlag {
			d.fixCleanCache()
		}
		if *removeBrokenFlag {
			d.fixRemoveBroken()
		}
	}

	fmt.Println()
	remaining := d.fails - d.fixed
	switch {
	case remaining > 0:
		fmt.Printf("Doctor: %d issue(s) remaining, %d warning(s), %d fixed.\n", remaining, d.warns, d.fixed)
		return fmt.Errorf("doctor reported %d unresolved failure(s)", remaining)
	case d.fixed > 0:
		fmt.Printf("Doctor: %d issue(s) fixed, %d warning(s).\n", d.fixed, d.warns)
	case d.warns > 0:
		fmt.Printf("Doctor: 0 issues, %d warning(s).\n", d.warns)
	default:
		fmt.Println("Doctor: clean.")
	}
	return nil
}

type doctor struct {
	ctx context.Context

	fails int
	warns int
	fixed int

	// Actionable state collected during the read-only walk, consumed
	// by the fix-* methods.
	staleStaging    []string // full paths to .staging-* dirs
	danglingCurrent string   // path of dangling current symlink, or ""
	brokenVersions  []string // installed versions whose gcc won't run
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

func (d *doctor) checkCurrent() {
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
			"Run with --fix to clear it, or `armup use <version>` to repoint.")
		d.danglingCurrent = filepath.Join(paths.DataDir(), "current")
		return
	}
	d.ok("current symlink", "→ "+cur)
}

// checkInstalled lists installed versions and verifies each one's
// arm-none-eabi-gcc actually runs. Broken installs (missing binary
// or exec failure) are collected so --remove-broken can act on them.
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
		status, banner := d.verifyVersion(v)
		head := fmt.Sprintf("%s %-22s %s", marker, v, humanSize(size))
		switch status {
		case "OK":
			if v == cur {
				fmt.Printf("       %-44s OK   %s\n", head, banner)
			} else {
				fmt.Printf("       %-44s OK\n", head)
			}
		default:
			fmt.Printf("       %-44s BROKEN  %s\n", head, banner)
			d.fails++
			d.brokenVersions = append(d.brokenVersions, v)
		}
	}
}

// verifyVersion runs <verDir>/bin/arm-none-eabi-gcc --version and
// returns ("OK", banner) on success or ("BROKEN", reason) on
// failure (missing binary, exec error, non-zero exit).
func (d *doctor) verifyVersion(v string) (status, detail string) {
	gcc := filepath.Join(paths.VersionDir(v), "bin", "arm-none-eabi-gcc")
	if runtime.GOOS == "windows" {
		gcc += ".exe"
	}
	if _, err := os.Stat(gcc); err != nil {
		return "BROKEN", "gcc binary missing"
	}
	out, err := exec.CommandContext(d.ctx, gcc, "--version").Output()
	if err != nil {
		return "BROKEN", "gcc failed to run: " + err.Error()
	}
	return "OK", strings.SplitN(strings.TrimSpace(string(out)), "\n", 2)[0]
}

// checkOrphans flags files (not directories) at the top of versions/.
// Every legitimate entry there is either an install dir or a
// .staging-* dir. Loose files indicate manual fiddling and won't
// hurt anything but shouldn't be there.
func (d *doctor) checkOrphans() {
	versionsDir := paths.VersionsDir()
	entries, err := os.ReadDir(versionsDir)
	if err != nil {
		return
	}
	var orphans []string
	for _, e := range entries {
		if !e.IsDir() {
			orphans = append(orphans, e.Name())
		}
	}
	if len(orphans) == 0 {
		d.ok("no orphan files in versions/", "")
		return
	}
	d.warn("orphan files in versions/",
		fmt.Sprintf("%d found: %s", len(orphans), strings.Join(orphans, ", ")),
		"Remove by hand — armup never creates files here.")
}

// .staging-<name> directories are created during install and renamed
// to versions/<name> on success. Their presence indicates an
// interrupted install — `armup doctor --fix` removes them.
func (d *doctor) checkStaging() {
	versionsDir := paths.VersionsDir()
	entries, err := os.ReadDir(versionsDir)
	if err != nil {
		// Already reported by checkDataDir; don't double-warn.
		return
	}
	for _, e := range entries {
		if e.IsDir() && strings.HasPrefix(e.Name(), ".staging-") {
			d.staleStaging = append(d.staleStaging, filepath.Join(versionsDir, e.Name()))
		}
	}
	if len(d.staleStaging) == 0 {
		d.ok("no leftover .staging-* directories", "")
		return
	}
	d.warn("leftover .staging-* directories",
		fmt.Sprintf("%d found", len(d.staleStaging)),
		"Run `armup doctor --fix` to remove.")
}

// fixStaging removes every .staging-* directory collected during the
// read-only walk. Counts toward d.fixed (which offsets d.fails for the
// summary).
func (d *doctor) fixStaging() {
	for _, p := range d.staleStaging {
		if err := os.RemoveAll(p); err != nil {
			fmt.Printf("[FAIL] removing %s: %v\n", p, err)
			continue
		}
		fmt.Printf("[FIXED] removed %s\n", p)
		// .staging-* started as a WARN, not a FAIL — fixing it just
		// clears the warning. Don't increment d.fixed (which would
		// double-count against d.fails in the summary).
	}
	d.staleStaging = nil
}

// fixDanglingCurrent removes a 'current' symlink whose target version
// no longer exists. After the fix, the data dir is in a consistent
// "no version active" state; `armup use <X>` repoints it.
func (d *doctor) fixDanglingCurrent() {
	if d.danglingCurrent == "" {
		return
	}
	if err := os.Remove(d.danglingCurrent); err != nil {
		fmt.Printf("[FAIL] removing %s: %v\n", d.danglingCurrent, err)
		return
	}
	fmt.Printf("[FIXED] removed dangling current symlink %s\n", d.danglingCurrent)
	d.fixed++
	d.danglingCurrent = ""
}

// fixCleanCache wipes every file under cache/. Logs the freed bytes
// for visibility. Counts as INFO, not FIXED — cache contents weren't
// a "FAIL" to begin with.
func (d *doctor) fixCleanCache() {
	dir := paths.CacheDir()
	before := dirSize(dir)
	entries, err := os.ReadDir(dir)
	if err != nil {
		fmt.Printf("[FAIL] reading cache dir %s: %v\n", dir, err)
		return
	}
	removed := 0
	for _, e := range entries {
		p := filepath.Join(dir, e.Name())
		if err := os.RemoveAll(p); err != nil {
			fmt.Printf("[FAIL] removing %s: %v\n", p, err)
			continue
		}
		removed++
	}
	fmt.Printf("[FIXED] wiped %d cache item(s), freed %s\n", removed, humanSize(before))
}

// fixRemoveBroken deletes every installed version that checkInstalled
// flagged as BROKEN. Each removal is a FIXED for that version's FAIL.
func (d *doctor) fixRemoveBroken() {
	for _, v := range d.brokenVersions {
		verDir := paths.VersionDir(v)
		if err := os.RemoveAll(verDir); err != nil {
			fmt.Printf("[FAIL] removing %s: %v\n", verDir, err)
			continue
		}
		fmt.Printf("[FIXED] removed broken version %s\n", v)
		d.fixed++
	}
	d.brokenVersions = nil
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
