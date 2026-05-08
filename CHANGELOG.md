# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- Per-project version pinning. Drop a `.tool-versions` (asdf/mise
  format) or `.armup-version` file at the project root and armup
  walks up from the current directory to find it.
- `armup pinned` shows the resolved pin (version + source path), with
  `--json` for scripting. Distinct from `armup current` — `pinned` is
  what the project asks for; `current` is what's globally active.
- `armup install` and `armup use` with no version argument resolve to
  the project pin. With no pin, they error clearly.
- `ARMUP_VERSION` env var overrides the file lookup, for one-shot
  scripting.

## [1.0.0] — 2026-05-07

First stable release. Linux, macOS, and Windows are all supported with
the same workflow: `init`, `install`, `use`, `uninstall`, `list`,
`available`, `current`, `which`, `reset`, `completion`, `self-update`.

### Added

- `armup self-update` replaces the running binary with the latest
  GitHub release for the host platform. SHA-256 verified. Atomic on
  unix (temp + rename) and Windows (rename old + write new).
- `armup reset [-f] [--keep-shell]` removes every installed version,
  the cache, and (by default) the shell rc / Windows registry PATH
  entry. Confirms before deleting unless `-f` is passed.
- `--json` flag on `list`, `available`, `current`, and `which` for
  scripting. Pretty-printed JSON.
- Per-subcommand help: `armup <cmd> -h` prints a friendly usage +
  description block instead of bare flag output.
- `install.sh` and `install.ps1` one-liners hosted at
  `raw.githubusercontent.com/mihnen/armup/master/install.{sh,ps1}`.
  Pull the latest release, verify SHA-256, install to PATH.
- `SECURITY.md` with the responsible-disclosure path.
- README badges (CI status, latest release, license) plus a
  comprehensive Windows install/caveats section.
- Test coverage on the armup-specific logic (version normalization,
  comparison, archive layout promotion, atomic Use, refusal/force
  Uninstall, zip-slip defense, Windows PATH-list parsing). Wine
  compatibility for cross-platform testing on Linux.
- CI workflow: `go vet`, `gofmt -d -s`, `go test -race` on
  ubuntu/macos/windows-latest, plus cross-compile sanity for all 5
  shipped targets.
- golangci-lint in CI: errcheck, govet, ineffassign, staticcheck,
  unused, gocritic, misspell.
- Dependabot config for go modules + GitHub Actions.
- `.editorconfig` for IDE consistency.

### Changed

- Install errors are friendlier. Bad versions / unsupported
  platform combos now fail with "ARM does not publish a
  &lt;triple&gt; build for &lt;version&gt;" up front, instead of
  later on a confusing checksum 404.
- `Host.ResolveForVersion` probes the archive URL on unix too, not
  just Windows. One extra HEAD per install, much clearer error path.
- Install one-liners switched from `/releases/latest` (which excludes
  prereleases) to `/releases` (full list, take the most recent).
- Release workflow injects the tag into the binary via
  `-ldflags='-X main.version=...'`, so `armup version` reports the
  real version instead of `dev`.
- Refactored `main` into a `run() int` wrapper so deferred cleanup
  actually executes on error exit.

### Fixed

- ARM's older Windows toolchain archives use the i686 mingw triple,
  not x86_64. `armup install <pre-14.2 version>` on Windows now
  picks the right archive automatically.
- ARM's newer Windows zips (15.x+) don't have a wrapping directory.
  Install detects both layouts and promotes the correct directory.
- ARM's `.sha256` file is misnamed and contains MD5; the real
  SHA-256 is in `.sha256asc`. Install uses the latter.

## [0.2.0-beta2] — 2026-05-07

### Added

- `armup version`, `armup --version`, `armup -v` print the embedded
  release tag (set via `-ldflags='-X main.version=...'` at build time).

### Changed

- Release workflow injects the tag into the binary so released builds
  report a real version instead of `dev`.

## [0.2.0-beta1] — 2026-05-07

### Added

- Windows support. `armup init`, `install`, `use`, `uninstall`, `list`,
  `available`, `completion` work on Windows without admin or Developer
  Mode. PATH integration writes to `HKCU\Environment\Path`; the active
  toolchain is exposed through an NTFS junction at
  `%LOCALAPPDATA%\armup\current`.
- Windows builds are uploaded as `.zip` archives; SHA256SUMS covers
  every platform's archive.
- PowerShell shell completion (`armup completion powershell`) joins the
  existing bash and zsh.
- Self-extracting unwrapped Windows zips (15.x and later) install
  correctly.

## [0.1.0-beta1] — 2026-05-06

### Added

- Initial release. Linux and macOS support for installing, listing,
  switching, and uninstalling versions of ARM's `arm-none-eabi` GCC
  toolchain. Multi-threaded extraction via system `xz`+`tar` when
  available, single-threaded pure-Go fallback otherwise. Bash and zsh
  shell completion. SHA-256 verification of downloaded archives.
