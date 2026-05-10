# armup

[![ci](https://github.com/mihnen/armup/actions/workflows/ci.yml/badge.svg)](https://github.com/mihnen/armup/actions/workflows/ci.yml)
[![release](https://img.shields.io/github/v/release/mihnen/armup?include_prereleases&sort=semver)](https://github.com/mihnen/armup/releases/latest)
[![license](https://img.shields.io/github/license/mihnen/armup)](LICENSE)

A version manager for ARM's official `arm-none-eabi` GCC toolchain — the one
used for STM32 and other Cortex-M / Cortex-R / Cortex-A bare-metal work.
Think `rustup` or `nvm`, but for arm-none-eabi.

```sh
armup install 14.3.rel1
armup install 15.2.rel1
armup use 14.3.rel1     # switch the active toolchain in one command
arm-none-eabi-gcc --version
```

## Why

ARM publishes the toolchain as a tarball or zip on developer.arm.com. There's
no package manager that reliably ships current builds across distros, and
hand-managing two or three installed versions side-by-side is fiddly. `armup`:

- Downloads the official ARM binary for your platform directly, with SHA-256
  verification.
- Keeps multiple versions side-by-side under a per-user data directory.
- Exposes the active one through a single PATH entry — switching versions is
  a `rename` of one symlink (or junction on Windows), instant in any shell.
- Lists installed versions and ARM's catalog of available versions.
- No `sudo` required on any platform.

## Install

### Linux / macOS

One-liner — pulls the latest release, verifies SHA-256, drops `armup` into
`~/.local/bin/`:

```sh
curl -sSL https://raw.githubusercontent.com/mihnen/armup/master/install.sh | sh
armup init
```

Open a fresh shell after `init` so the new PATH is loaded. From there:

```sh
armup available
armup install 14.3.rel1
armup use 14.3.rel1
```

Override the install location with `ARMUP_INSTALL_DIR=...` before piping to
`sh`. Or grab the tarball manually from the
[Releases page](../../releases/latest) — archives are
`armup-<version>-{linux-amd64, linux-arm64, darwin-amd64, darwin-arm64}.tar.gz`.

### Windows

PowerShell one-liner — pulls the latest release, verifies SHA-256, drops
`armup.exe` into `%USERPROFILE%\bin\`:

```powershell
iwr https://raw.githubusercontent.com/mihnen/armup/master/install.ps1 | iex
armup init
```

Override the install location with `$env:ARMUP_INSTALL_DIR = '...'` before
running the line. Or grab the `.zip` manually from the
[Releases page](../../releases/latest).

> **First-run note:** the `armup.exe` binary isn't code-signed yet, so on
> some Windows configurations SmartScreen will warn before the first launch
> (More info → Run anyway). Code signing is on the roadmap for a future
> release.

`init` writes the toolchain `bin` directory into `HKCU\Environment\Path`
(no admin required). **Open a new terminal** so the new PATH is loaded —
already-running shells won't see it. Every other command works the same as
on Linux/macOS.

A few Windows-specific things to know:

- `armup use` swaps the active toolchain via an NTFS junction. Junctions
  don't need admin or Developer Mode, but they require the active version
  and the `current` link to be on the same volume — they will be by default,
  both under `%LOCALAPPDATA%\armup\`.
- ARM published 32-bit (`mingw-w64-i686`) Windows builds for every release;
  64-bit (`mingw-w64-x86_64`) builds only start at 14.2.rel1. `armup` picks
  the 64-bit variant when available and falls back automatically.
- Some toolchain include paths can exceed the legacy 260-character limit.
  If you hit `path too long` errors during a build, enable
  [`LongPathsEnabled`](https://learn.microsoft.com/en-us/windows/win32/fileio/maximum-file-path-limitation)
  in the registry or Group Policy Editor.

### Build from source

Requires Go 1.25+:

```sh
git clone https://github.com/mihnen/armup
cd armup
go build -o armup ./cmd/armup
```

Cross-compile for another platform:

```sh
GOOS=linux   GOARCH=amd64 go build -o armup       ./cmd/armup
GOOS=darwin  GOARCH=arm64 go build -o armup       ./cmd/armup
GOOS=windows GOARCH=amd64 go build -o armup.exe   ./cmd/armup
```

## Commands

```
armup init                       one-time setup (creates data dir, updates PATH)
armup available [--refresh]      list versions you can install
armup install <version>          download from developer.arm.com
armup install --from <SRC>       install from a custom URL or local archive
armup list                       list installed versions; * marks active
armup use <version>              switch the active version
armup current                    print the active version
armup pinned                     print the project-pinned version (or 'none')
armup which                      print the active toolchain's bin directory
armup which <version>            print a specific installed version's bin directory
armup which --pinned             print the project-pinned version's bin directory
armup uninstall <version> [-f]   remove a version (-f to remove the active one)
armup reset [-f] [--keep-shell]  remove all versions and armup data
armup completion <shell>         print a shell-completion script (bash, zsh, fish, powershell)
armup self-update [--nightly]    replace the running binary with the latest release (or nightly build)
armup version                    print armup's version
```

Run `armup <command> -h` for command-specific help.

`armup list`, `armup available`, `armup current`, `armup pinned`, and
`armup which` all accept `--json` for scripting.

## Custom installs

`armup install --from <SRC>` installs a toolchain from any source you
hand it. Useful for:

- **Internal mirrors** behind a corporate firewall.
- **Custom GCC builds** with vendor patches.
- **Any archive that isn't in `armup available`** but is laid out the
  same way ARM ships theirs.

`<SRC>` can be a remote URL, a `file://` URI, a bare local path, or a
Windows UNC share. Examples:

```sh
# Internal mirror
armup install --from 'https://mirror.example.com/14.3.rel1.tar.xz' --as 14.3.rel1-mirror

# Local archive (already on disk)
armup install --from ~/Downloads/gcc-arm-none-eabi-12.3-custom.tar.bz2 --as 12.3-custom

# file:// URI for a network share
armup install --from 'file:///mnt/share/archives/12.3.rel1.tar.xz' --as 12.3.rel1
```

Flags:

- `--as <name>` — version slot name. Defaults to the source filename
  with archive extension stripped. You'll usually want to override it
  with something short.
- `--sha256 <hex>` — verify the archive's SHA-256 before extraction.
  If absent, a warning prints but the install proceeds. Skip it if
  you don't have a checksum to verify against.

armup refuses to clobber an existing `versions/<name>` slot — pick a
different `--as` or `armup uninstall <name>` first.

Supported archive formats: `.tar.gz`, `.tar.xz`, `.tar.bz2`, `.zip`.
Local archives are read in place — no copy to the cache.

## Nightly builds

Every push to `master` produces a rolling pre-release at
[releases/tag/nightly](../../releases/tag/nightly). The assets there have
stable URLs (e.g.
`releases/download/nightly/armup-nightly-linux-amd64.tar.gz`) and the
binary's embedded version reports `nightly+<short-sha>` so it's traceable
to the commit that built it.

To opt into nightly:

```sh
armup self-update --nightly
```

Plain `armup self-update` keeps tracking semver-tagged releases — the
nightly tag is filtered out.

## Per-project pinning

Drop a `.tool-versions` file (asdf/mise format) at your project root:

```
armup 14.3.rel1
```

Or a single-line `.armup-version`:

```
14.3.rel1
```

Then anywhere inside the project tree:

```sh
armup pinned          # 14.3.rel1 (from /path/to/project/.tool-versions)
armup install         # installs the pinned version
armup use             # switches the active version to the pinned one
```

The `ARMUP_VERSION` environment variable overrides the file lookup:

```sh
ARMUP_VERSION=15.2.rel1 armup install   # one-shot install of 15.2.rel1
```

`armup current` and `armup which` (with no arguments) always report the
**globally active** version — pinning doesn't change them on its own.
They diverge from `armup pinned` until you run `armup use` (no args)
to apply the pin.

### Per-shell PATH with direnv

`armup use` is global: it retargets a shared symlink, so every shell
sees the change. If you want each shell's toolchain to follow the
project it `cd`'d into — independently of other open shells —
combine `armup which --pinned` with [direnv](https://direnv.net):

```bash
# .envrc at the project root
PATH_add "$(armup which --pinned)"
```

This sets `PATH` per shell without touching the global `current`
symlink. Two shells in two different projects can run different
toolchain versions simultaneously.

For a specific version (no project pin file needed):

```bash
PATH_add "$(armup which 14.3.rel1)"
```

`use` updates a single link; the switch is visible immediately in any shell
whose PATH includes the toolchain directory.

## Shell completion

`armup use <TAB>` lists installed versions; `armup install <TAB>` lists
available versions. Candidates are queried from the binary at completion
time, so they always reflect current state.

### zsh

```sh
mkdir -p ~/.zsh/completions
armup completion zsh > ~/.zsh/completions/_armup
```

Add this to `~/.zshrc` **before** `compinit` runs (in oh-my-zsh setups,
before `source $ZSH/oh-my-zsh.sh`):

```sh
fpath=(~/.zsh/completions $fpath)
```

Or, simpler but slightly slower at shell startup:

```sh
echo 'source <(armup completion zsh)' >> ~/.zshrc
```

### bash

```sh
echo 'source <(armup completion bash)' >> ~/.bashrc
```

### fish

```sh
armup completion fish > ~/.config/fish/completions/armup.fish
```

(fish auto-loads completions from that directory on startup. No
sourcing or fpath setup required.)

### PowerShell

Append to your profile so it loads in every session:

```powershell
if (-not (Test-Path $PROFILE)) { New-Item -Type File -Force -Path $PROFILE }
Add-Content -Path $PROFILE -Value "`narmup completion powershell | Out-String | Invoke-Expression"
```

To load in the current session without restarting:

```powershell
armup completion powershell | Out-String | Invoke-Expression
```

## Extraction performance

`install` extracts the toolchain using the fastest path available on the
host:

1. **`xz` + `tar` on PATH** (Linux/macOS): pipes `xz -T 0 -dc` into `tar -x`,
   so decompression is multi-threaded across all cores. Typical install of a
   ~150 MiB archive completes in a few seconds.
2. **`tar` only**: `tar -xJf -`, single-threaded.
3. **Neither**: pure-Go fallback via `github.com/ulikunitz/xz`,
   single-threaded; ~3× slower than the multi-threaded path.

Windows installs are zip archives, extracted via the Go standard library.

`ARMTOOLCHAIN_PURE_GO=1` forces the pure-Go fallback (useful for testing).

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md) for setup, pre-commit hooks, and
the test/lint expectations.

## License

[MIT](LICENSE).
