package main

import (
	"bufio"
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"runtime"
	"strings"
	"syscall"

	"github.com/mihnen/armup/internal/arm"
	"github.com/mihnen/armup/internal/paths"
	"github.com/mihnen/armup/internal/pin"
	"github.com/mihnen/armup/internal/selfupdate"
	"github.com/mihnen/armup/internal/shell"
	"github.com/mihnen/armup/internal/store"
)

// version is set via -ldflags='-X main.version=...' by the release workflow.
// Local `go build` runs without the flag and report "dev".
var version = "dev"

//go:embed completion_bash.sh
var bashCompletion string

//go:embed completion_zsh.sh
var zshCompletion string

//go:embed completion_powershell.ps1
var powershellCompletion string

const usage = `armup - manage arm-none-eabi GCC toolchain versions

usage: armup <command> [options]

commands:
  init                       Create directories and add PATH entry to shell rc
  available [--refresh]      List versions available from ARM (cached or live)
  list                       List installed versions; '*' marks current
  install <version>          Download, verify, and extract a version
  use <version>              Switch the active version
  current                    Print the active version
  pinned                     Print the per-project pinned version (if any)
  uninstall <version> [-f]   Remove a version (-f to remove the active one)
  reset [-f] [--keep-shell]  Remove every installed version and armup data
  which                      Print the active toolchain's bin directory
  completion <shell>         Print a shell-completion script (bash, zsh, powershell)
  self-update [--nightly]    Replace the running binary with the latest release (or nightly)
  version                    Print armup's version
  help                       Show this help

The active version is exposed through a single PATH entry pointing at
` + "`<data>/current/bin`" + `, so switching versions takes effect immediately
in any new shell (and in existing shells, since PATH lookups follow the
symlink). Run ` + "`init`" + ` once to create the data dir and update your shell rc.
`

// main is intentionally a thin wrapper around run() so that any deferred
// cleanup (signal-handler cancel, etc.) actually executes — os.Exit skips
// pending defers, but a normal return doesn't.
func main() { os.Exit(run()) }

func run() int {
	if len(os.Args) < 2 {
		fmt.Fprint(os.Stderr, usage)
		return 2
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	cmd := os.Args[1]
	args := os.Args[2:]

	var err error
	switch cmd {
	case "init":
		err = cmdInit(args)
	case "available":
		err = cmdAvailable(ctx, args)
	case "list", "ls":
		err = cmdList(args)
	case "install":
		err = cmdInstall(ctx, args)
	case "use":
		err = cmdUse(args)
	case "current":
		err = cmdCurrent(args)
	case "pinned":
		err = cmdPinned(args)
	case "uninstall", "remove", "rm":
		err = cmdUninstall(args)
	case "reset":
		err = cmdReset(args)
	case "which":
		err = cmdWhich(args)
	case "completion":
		err = cmdCompletion(args)
	case "self-update":
		err = cmdSelfUpdate(ctx, args)
	case "__complete":
		err = cmdCompleteHidden(args)
	case "version", "--version", "-v":
		fmt.Println(version)
	case "help", "-h", "--help":
		fmt.Print(usage)
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n\n", cmd)
		fmt.Fprint(os.Stderr, usage)
		return 2
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}
	return 0
}

// newFlagSet builds an ExitOnError flag set whose -h / --help output is a
// friendly per-subcommand description plus the flag list, instead of the
// stdlib default which prints only the flags.
func newFlagSet(cmd, summary, doc string) *flag.FlagSet {
	fs := flag.NewFlagSet(cmd, flag.ExitOnError)
	fs.Usage = func() {
		out := fs.Output()
		fmt.Fprintf(out, "Usage: armup %s\n", summary)
		if d := strings.TrimSpace(doc); d != "" {
			fmt.Fprintln(out)
			fmt.Fprintln(out, d)
		}
		var hasFlags bool
		fs.VisitAll(func(*flag.Flag) { hasFlags = true })
		if hasFlags {
			fmt.Fprintln(out)
			fmt.Fprintln(out, "Flags:")
			fs.PrintDefaults()
		}
	}
	return fs
}

func cmdInit(args []string) error {
	fs := newFlagSet("init", "init",
		`One-time setup. Creates the data directory and adds armup's bin/
to your PATH (.zshrc/.bashrc on unix, HKCU\Environment\Path on
Windows). Idempotent — safe to re-run.`)
	fs.Parse(args)

	if err := store.EnsureLayout(); err != nil {
		return err
	}
	updated, err := shell.EnsureOnPath(paths.ActiveBinDir())
	if err != nil {
		if errors.Is(err, shell.ErrUnsupported) {
			fmt.Printf("Data directory ready at %s\n", paths.DataDir())
			fmt.Printf("Add %s to your PATH manually (shell integration not implemented for this OS)\n", paths.ActiveBinDir())
			return nil
		}
		return err
	}
	fmt.Printf("Data directory ready at %s\n", paths.DataDir())
	if len(updated) == 0 {
		fmt.Println("Shell rc files already configured")
	} else {
		for _, f := range updated {
			fmt.Printf("Added PATH entry to %s\n", f)
		}
		fmt.Println("Open a new shell or `source` the rc file to pick up the change")
	}
	return nil
}

func cmdAvailable(ctx context.Context, args []string) error {
	fs := newFlagSet("available", "available [--refresh] [--json]",
		`List ARM toolchain versions you can install. By default reads
from the local cache (or the curated list if no cache exists).
With --refresh, re-queries ARM and falls back to HEAD-probing
the curated versions if ARM's downloads page is blocked.

With --json, output the list as JSON with the source ("cached",
"curated", or "refresh") for scripting.`)
	refresh := fs.Bool("refresh", false, "fetch the latest list from ARM")
	asJSON := fs.Bool("json", false, "output as JSON")
	fs.Parse(args)

	var versions []string
	var source string

	if *refresh {
		if err := store.EnsureLayout(); err != nil {
			return err
		}
		fmt.Fprintln(os.Stderr, "Refreshing version list from developer.arm.com")
		v, err := arm.Refresh(ctx)
		if err != nil {
			if !errors.Is(err, arm.ErrPageBlocked) {
				return fmt.Errorf("refresh: %w", err)
			}
			fmt.Fprintf(os.Stderr, "Downloads page blocked (%v); probing curated versions instead\n", err)
			host, hErr := arm.CurrentHost()
			if hErr != nil {
				return hErr
			}
			v, err = arm.ProbeCurated(ctx, host)
			if err != nil {
				return fmt.Errorf("probe: %w", err)
			}
			if len(v) == 0 {
				return fmt.Errorf("no curated versions reachable")
			}
		}
		if err := arm.SaveAvailable(paths.AvailableFile(), v); err != nil {
			return err
		}
		versions = v
		source = "refresh"
	} else {
		cached, err := arm.LoadCachedAvailable(paths.AvailableFile())
		if err != nil {
			return err
		}
		if len(cached) > 0 {
			versions = cached
			source = "cached"
		} else {
			versions = arm.Curated
			source = "curated"
		}
	}

	if *asJSON {
		return writeJSON(struct {
			Source   string   `json:"source"`
			Versions []string `json:"versions"`
		}{source, versions})
	}
	printVersions(versions)
	return nil
}

func printVersions(v []string) {
	for _, x := range v {
		fmt.Println(x)
	}
}

// writeJSON writes v to stdout pretty-printed (2-space indent) with a
// trailing newline.
func writeJSON(v any) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

func cmdList(args []string) error {
	fs := newFlagSet("list", "list [--json]",
		`List the toolchain versions installed locally. The currently
active version is marked with a leading '*'.

With --json, output one record per version including its
absolute path and active-status flag.`)
	asJSON := fs.Bool("json", false, "output as JSON")
	fs.Parse(args)

	versions, err := store.List()
	if err != nil {
		return err
	}
	cur, _ := store.Current()

	if *asJSON {
		type entry struct {
			Version string `json:"version"`
			Current bool   `json:"current"`
			Path    string `json:"path"`
		}
		out := make([]entry, 0, len(versions))
		for _, v := range versions {
			out = append(out, entry{
				Version: v,
				Current: v == cur,
				Path:    paths.VersionDir(v),
			})
		}
		return writeJSON(out)
	}

	if len(versions) == 0 {
		fmt.Println("No versions installed. Run `armup available` to see options, then `armup install <version>`.")
		return nil
	}
	for _, v := range versions {
		mark := "  "
		if v == cur {
			mark = "* "
		}
		fmt.Printf("%s%s\n", mark, v)
	}
	return nil
}

func cmdInstall(ctx context.Context, args []string) error {
	fs := newFlagSet("install", "install [<version>]",
		`Download, verify, and extract a version of the arm-none-eabi
toolchain. Run 'armup available' for a list of versions.

With no <version> argument, install the version pinned in the
current project (.tool-versions or .armup-version) — see
'armup pinned'.

If no version is currently active, the newly-installed one becomes
the active version.`)
	fs.Parse(args)

	var version string
	switch fs.NArg() {
	case 0:
		v, err := pinnedOrError("install")
		if err != nil {
			return err
		}
		version = v
	case 1:
		version = arm.Normalize(fs.Arg(0))
	default:
		fs.Usage()
		return errors.New("too many arguments")
	}
	return store.Install(ctx, version, true)
}

func cmdUse(args []string) error {
	fs := newFlagSet("use", "use [<version>]",
		`Switch the active toolchain to <version>. Takes effect
immediately in any shell whose PATH includes armup's bin
directory — no shell reload needed, since PATH lookups follow
the symlink/junction.

With no <version> argument, switch to the version pinned in the
current project (.tool-versions or .armup-version) — see
'armup pinned'.`)
	fs.Parse(args)

	var v string
	switch fs.NArg() {
	case 0:
		got, err := pinnedOrError("use")
		if err != nil {
			return err
		}
		v = got
	case 1:
		v = arm.Normalize(fs.Arg(0))
	default:
		fs.Usage()
		return errors.New("too many arguments")
	}
	if err := store.Use(v); err != nil {
		return err
	}
	fmt.Printf("Now using %s\n", v)
	return nil
}

// pinnedOrError reads the per-project pin and returns the version, or a
// clear error explaining what `cmd` expected. Used by install/use to back
// the no-arg form.
func pinnedOrError(cmd string) (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	r, err := pin.Resolve(cwd)
	if err != nil {
		return "", fmt.Errorf("read project pin: %w", err)
	}
	if !r.Found() {
		return "", fmt.Errorf("no version pinned in this project; pass an explicit version (`armup %s <version>`) or create a .tool-versions / .armup-version file", cmd)
	}
	fmt.Fprintf(os.Stderr, "Resolved pinned version %s from %s\n", r.Version, r.Source)
	return r.Version, nil
}

// cmdPinned shows what's pinned for the current project (or none). It
// deliberately does NOT report the globally-active version — `armup
// current` is for that. The two answers can differ; conflating them
// confuses users.
func cmdPinned(args []string) error {
	fs := newFlagSet("pinned", "pinned [--json]",
		`Print the per-project pinned version, resolved by walking up
from the current directory looking for a .tool-versions or
.armup-version file. ARMUP_VERSION env var overrides the file
walk.

Distinct from 'armup current' — pinned is what the project asks
for, current is what's globally active. They may differ; pinned
does not change the active version on its own. See 'armup use'
(no args) to switch to the pinned version.

With --json, output {"version": "...", "source": "..."} or
{"version": null, "source": null} when nothing is pinned.`)
	asJSON := fs.Bool("json", false, "output as JSON")
	fs.Parse(args)

	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	r, err := pin.Resolve(cwd)
	if err != nil {
		return err
	}

	if *asJSON {
		type out struct {
			Version *string `json:"version"`
			Source  *string `json:"source"`
		}
		var v out
		if r.Found() {
			vs, src := r.Version, r.Source
			v.Version = &vs
			v.Source = &src
		}
		return writeJSON(v)
	}

	if !r.Found() {
		fmt.Println("none")
		return nil
	}
	fmt.Printf("%s (from %s)\n", r.Version, r.Source)
	return nil
}

func cmdCurrent(args []string) error {
	fs := newFlagSet("current", "current [--json]",
		`Print the currently active toolchain version, or 'none' if
no version is active.

With --json, output {"version": "...", "path": "..."} or
{"version": null} when none.`)
	asJSON := fs.Bool("json", false, "output as JSON")
	fs.Parse(args)

	cur, err := store.Current()
	if err != nil {
		return err
	}

	if *asJSON {
		type out struct {
			Version *string `json:"version"`
			Path    *string `json:"path,omitempty"`
		}
		var v out
		if cur != "" {
			path := paths.VersionDir(cur)
			v.Version = &cur
			v.Path = &path
		}
		return writeJSON(v)
	}

	if cur == "" {
		fmt.Println("none")
		return nil
	}
	fmt.Println(cur)
	return nil
}

func cmdUninstall(args []string) error {
	fs := newFlagSet("uninstall", "uninstall [-f] <version>",
		`Remove an installed toolchain version. Refuses to remove the
currently active version unless -f / --force is passed.`)
	force := fs.Bool("f", false, "remove even if it's the active version")
	fs.BoolVar(force, "force", false, "remove even if it's the active version")
	fs.Parse(args)
	if fs.NArg() != 1 {
		fs.Usage()
		return errors.New("missing <version> argument")
	}
	v := arm.Normalize(fs.Arg(0))
	if err := store.Uninstall(v, *force); err != nil {
		return err
	}
	fmt.Printf("Removed %s\n", v)
	return nil
}

func cmdWhich(args []string) error {
	fs := newFlagSet("which", "which [--json]",
		`Print the absolute path of the active toolchain's bin/ directory.
This is the directory armup adds to your PATH at init.

With --json, output {"path": "..."}.`)
	asJSON := fs.Bool("json", false, "output as JSON")
	fs.Parse(args)
	if *asJSON {
		return writeJSON(struct {
			Path string `json:"path"`
		}{paths.ActiveBinDir()})
	}
	fmt.Println(paths.ActiveBinDir())
	return nil
}

// cmdReset wipes everything armup has created: the data dir (every
// installed toolchain + cache + current link) and, by default, the shell
// PATH integration too. Prompts for confirmation unless -f is passed.
func cmdReset(args []string) error {
	fs := newFlagSet("reset", "reset [-f] [--keep-shell]",
		`Remove every installed toolchain version, the cache, and (by
default) armup's PATH entry from your shell rc / Windows registry.
Confirms before deleting unless -f / --force is passed. Does not
remove the armup binary itself.`)
	force := fs.Bool("f", false, "skip confirmation prompt")
	fs.BoolVar(force, "force", false, "skip confirmation prompt")
	keepShell := fs.Bool("keep-shell", false, "leave the shell rc / registry PATH entry alone")
	fs.Parse(args)

	dataDir := paths.DataDir()
	fmt.Printf("This will remove %s\n", dataDir)
	if !*keepShell {
		fmt.Println("and remove armup's PATH entry from your shell rc / Windows registry.")
	}
	if !*force {
		fmt.Print("Continue? [y/N] ")
		reader := bufio.NewReader(os.Stdin)
		ans, _ := reader.ReadString('\n')
		ans = strings.TrimSpace(strings.ToLower(ans))
		if ans != "y" && ans != "yes" {
			fmt.Println("aborted")
			return nil
		}
	}

	if err := store.Reset(); err != nil {
		return fmt.Errorf("remove %s: %w", dataDir, err)
	}
	fmt.Printf("Removed %s\n", dataDir)

	if !*keepShell {
		modified, err := shell.RemoveFromPath(paths.ActiveBinDir())
		if err != nil {
			if errors.Is(err, shell.ErrUnsupported) {
				fmt.Println("Shell cleanup not supported on this platform; remove the PATH entry manually.")
			} else {
				return fmt.Errorf("remove shell PATH entry: %w", err)
			}
		} else if len(modified) == 0 {
			fmt.Println("No shell PATH entry to remove.")
		} else {
			for _, f := range modified {
				fmt.Printf("Removed PATH entry from %s\n", f)
			}
			fmt.Println("Open a new shell for the change to take effect.")
		}
	}

	fmt.Println()
	fmt.Println("To finish removing armup, delete the binary itself, e.g.:")
	if runtime.GOOS == "windows" {
		// io.WriteString avoids fmt vet parsing the %USERPROFILE% literal.
		_, _ = os.Stdout.WriteString("    del %USERPROFILE%\\bin\\armup.exe\n")
	} else {
		fmt.Println("    rm ~/.local/bin/armup")
	}
	return nil
}

func cmdSelfUpdate(ctx context.Context, args []string) error {
	fs := newFlagSet("self-update", "self-update [--nightly]",
		`Replace the running armup binary with the latest release for the
current platform. SHA-256 verified.

By default, fetches the latest stable (semver-tagged) release.
With --nightly, fetches the rolling master build from the
'nightly' release.

Refuses to run on local dev builds (rebuild from source instead).`)
	nightly := fs.Bool("nightly", false, "fetch the rolling master build instead of the latest stable")
	fs.Parse(args)
	return selfupdate.Run(ctx, version, *nightly)
}

func cmdCompletion(args []string) error {
	fs := newFlagSet("completion", "completion <bash|zsh|powershell>",
		`Print a shell-completion script. Source the output in your
shell to enable tab-completion of subcommands and version
arguments. See README's "Shell completion" section for the
recommended install pattern per shell.`)
	fs.Parse(args)
	if fs.NArg() != 1 {
		fs.Usage()
		return errors.New("missing <shell> argument")
	}
	switch fs.Arg(0) {
	case "bash":
		fmt.Print(bashCompletion)
	case "zsh":
		fmt.Print(zshCompletion)
	case "powershell", "pwsh":
		fmt.Print(powershellCompletion)
	default:
		return fmt.Errorf("unsupported shell %q (supported: bash, zsh, powershell)", fs.Arg(0))
	}
	return nil
}

// cmdCompleteHidden is invoked by the shell-completion scripts to enumerate
// candidate values for the current word. Output is one candidate per line.
// Unknown kinds produce no output (and exit 0) so completion stays silent
// rather than spamming errors.
func cmdCompleteHidden(args []string) error {
	if len(args) == 0 {
		return nil
	}
	switch args[0] {
	case "subcommands":
		for _, c := range []string{
			"init", "available", "list", "install", "use", "current",
			"pinned", "uninstall", "reset", "which", "completion",
			"self-update", "version", "help",
		} {
			fmt.Println(c)
		}
	case "versions-installed":
		versions, _ := store.List()
		for _, v := range versions {
			fmt.Println(v)
		}
	case "versions-available":
		if cached, _ := arm.LoadCachedAvailable(paths.AvailableFile()); len(cached) > 0 {
			for _, v := range cached {
				fmt.Println(v)
			}
			return nil
		}
		for _, v := range arm.Curated {
			fmt.Println(v)
		}
	}
	return nil
}
