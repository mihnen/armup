package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/mihnen/armup/internal/arm"
	"github.com/mihnen/armup/internal/paths"
	"github.com/mihnen/armup/internal/shell"
	"github.com/mihnen/armup/internal/store"
)

const usage = `armup - manage arm-none-eabi GCC toolchain versions

usage: armup <command> [options]

commands:
  init                       Create directories and add PATH entry to shell rc
  available [--refresh]      List versions available from ARM (cached or live)
  list                       List installed versions; '*' marks current
  install <version>          Download, verify, and extract a version
  use <version>              Switch the active version
  current                    Print the active version
  uninstall <version> [-f]   Remove a version (-f to remove the active one)
  which                      Print the active toolchain's bin directory
  help                       Show this help

The active version is exposed through a single PATH entry pointing at
` + "`<data>/current/bin`" + `, so switching versions takes effect immediately
in any new shell (and in existing shells, since PATH lookups follow the
symlink). Run ` + "`init`" + ` once to create the data dir and update your shell rc.
`

func main() {
	if len(os.Args) < 2 {
		fmt.Fprint(os.Stderr, usage)
		os.Exit(2)
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
	case "uninstall", "remove", "rm":
		err = cmdUninstall(args)
	case "which":
		err = cmdWhich(args)
	case "help", "-h", "--help":
		fmt.Print(usage)
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n\n", cmd)
		fmt.Fprint(os.Stderr, usage)
		os.Exit(2)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func cmdInit(args []string) error {
	fs := flag.NewFlagSet("init", flag.ExitOnError)
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
	fs := flag.NewFlagSet("available", flag.ExitOnError)
	refresh := fs.Bool("refresh", false, "fetch the latest list from ARM")
	fs.Parse(args)

	if *refresh {
		if err := store.EnsureLayout(); err != nil {
			return err
		}
		fmt.Fprintln(os.Stderr, "Refreshing version list from developer.arm.com")
		versions, err := arm.Refresh(ctx)
		if err != nil {
			if !errors.Is(err, arm.ErrPageBlocked) {
				return fmt.Errorf("refresh: %w", err)
			}
			fmt.Fprintf(os.Stderr, "Downloads page blocked (%v); probing curated versions instead\n", err)
			host, hErr := arm.CurrentHost()
			if hErr != nil {
				return hErr
			}
			versions, err = arm.ProbeCurated(ctx, host)
			if err != nil {
				return fmt.Errorf("probe: %w", err)
			}
			if len(versions) == 0 {
				return fmt.Errorf("no curated versions reachable")
			}
		}
		if err := arm.SaveAvailable(paths.AvailableFile(), versions); err != nil {
			return err
		}
		printVersions(versions)
		return nil
	}

	cached, err := arm.LoadCachedAvailable(paths.AvailableFile())
	if err != nil {
		return err
	}
	if len(cached) > 0 {
		printVersions(cached)
		return nil
	}
	printVersions(arm.Curated)
	return nil
}

func printVersions(v []string) {
	for _, x := range v {
		fmt.Println(x)
	}
}

func cmdList(args []string) error {
	fs := flag.NewFlagSet("list", flag.ExitOnError)
	fs.Parse(args)

	versions, err := store.List()
	if err != nil {
		return err
	}
	if len(versions) == 0 {
		fmt.Println("No versions installed. Run `armup available` to see options, then `armup install <version>`.")
		return nil
	}
	cur, _ := store.Current()
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
	fs := flag.NewFlagSet("install", flag.ExitOnError)
	fs.Parse(args)
	if fs.NArg() != 1 {
		return errors.New("usage: armup install <version>")
	}
	return store.Install(ctx, arm.Normalize(fs.Arg(0)), true)
}

func cmdUse(args []string) error {
	fs := flag.NewFlagSet("use", flag.ExitOnError)
	fs.Parse(args)
	if fs.NArg() != 1 {
		return errors.New("usage: armup use <version>")
	}
	v := arm.Normalize(fs.Arg(0))
	if err := store.Use(v); err != nil {
		return err
	}
	fmt.Printf("Now using %s\n", v)
	return nil
}

func cmdCurrent(args []string) error {
	fs := flag.NewFlagSet("current", flag.ExitOnError)
	fs.Parse(args)

	cur, err := store.Current()
	if err != nil {
		return err
	}
	if cur == "" {
		fmt.Println("none")
		return nil
	}
	fmt.Println(cur)
	return nil
}

func cmdUninstall(args []string) error {
	fs := flag.NewFlagSet("uninstall", flag.ExitOnError)
	force := fs.Bool("f", false, "remove even if it's the active version")
	fs.BoolVar(force, "force", false, "remove even if it's the active version")
	fs.Parse(args)
	if fs.NArg() != 1 {
		return errors.New("usage: armup uninstall [-f] <version>")
	}
	v := arm.Normalize(fs.Arg(0))
	if err := store.Uninstall(v, *force); err != nil {
		return err
	}
	fmt.Printf("Removed %s\n", v)
	return nil
}

func cmdWhich(args []string) error {
	fs := flag.NewFlagSet("which", flag.ExitOnError)
	fs.Parse(args)
	fmt.Println(paths.ActiveBinDir())
	return nil
}
