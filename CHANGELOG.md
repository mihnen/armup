# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- Built-in support for legacy ARM toolchain releases (the pre-2022
  `gnu-rm` line). `armup install 10.3-2021.10`, `armup install
  9-2019-q4-major`, etc. now work directly. armup ships a curated
  table of legacy versions with their URLs and embedded SHA-256s
  (computed locally after verifying ARM's published MD5 — legacy
  archives don't ship a SHA-256 sidecar).
- `armup available --legacy` lists legacy versions for the running
  platform.
- Legacy entries reuse the same install pipeline as `--from`:
  download, verify SHA-256, extract, register. Idempotent and
  refuses to clobber existing slots.

## [1.3.0] — 2026-05-09

### Added

- `armup install --from <SRC>` installs a toolchain from any URL or
  local archive instead of the canonical developer.arm.com URL pattern.
  Solves three real-world cases at once: legacy ARM releases (the
  pre-2022 `gnu-rm` line, whose URLs don't fit a clean pattern),
  internal corporate mirrors, and custom GCC builds.
- `<SRC>` accepts HTTPS URLs, `file://` URIs, bare local paths, and
  Windows UNC shares. Local archives are read in place — no copy to
  the cache.
- `--as <name>` overrides the version slot name (default: derived from
  the source filename with archive extension stripped).
- `--sha256 <hex>` verifies the archive before extraction; absent and
  the install warns but proceeds (legacy archives don't ship checksum
  sidecars).
- Refuses to clobber an existing `versions/<name>` with a clear error
  pointing the user at `--as` or `armup uninstall`.
- `.tar.bz2` extraction support added (legacy ARM archives are bz2).

### Changed

- `promoteExtraction` now handles three archive layouts: wrapped with
  the modern expected name, wrapped with any single arbitrary subdir
  (legacy / custom builds), and unwrapped (newer Windows zips). Same
  function, generalized.

### Fixed

- `armup list` no longer surfaces internal `.staging-<name>`
  directories left over from interrupted installs.

## [1.2.0] — 2026-05-08

### Added

- `armup which <version>` and `armup which --pinned` print the bin
  directory of a specific or pinned version without touching the
  global active version. Designed for direnv / per-shell PATH
  isolation: `PATH_add "$(armup which --pinned)"` in a .envrc lets
  each shell pin its own toolchain independently of other open
  shells.

## [1.1.0] — 2026-05-08

### Added

- Fish shell support. `armup init` writes a `set -gx PATH ...` block to
  `~/.config/fish/config.fish` (in addition to .zshrc/.bashrc) when fish
  is installed. `armup reset` strips it. `armup completion fish` emits a
  fish completion script — drop it into `~/.config/fish/completions/`
  for fish to auto-load.
- Per-project version pinning. Drop a `.tool-versions` (asdf/mise
  format) or `.armup-version` file at the project root and armup
  walks up from the current directory to find it.
- `armup pinned` shows the resolved pin (version + source path), with
  `--json` for scripting. Distinct from `armup current` — `pinned` is
  what the project asks for; `current` is what's globally active.
- `armup install` and `armup use` with no version argument resolve to
  the project pin. With no pin, they error clearly.
- `ARMUP_VERSION` env var overrides the pin-file lookup, for one-shot
  scripting.
- Nightly rolling pre-release. Every push to master rebuilds all five
  platform archives and force-replaces a tag named `nightly`. Asset
  URLs are stable; binaries embed `nightly+<sha>` for traceability.
- `armup self-update --nightly` opts into the rolling build. Plain
  `armup self-update` continues to follow only semver-tagged stable
  releases — the `nightly` tag is filtered out so users can't
  accidentally jump channels.
- pre-commit config (`.pre-commit-config.yaml`) using TekWizely's Go
  hooks for cross-platform local enforcement of gofmt and go vet,
  with golangci-lint deferred to the pre-push stage.
- `CONTRIBUTING.md` documenting the contribution flow.

### Changed

- `armup install <version>` is now idempotent — re-running on an
  already-installed version is a no-op success rather than an error.
  Important for the per-project pin workflow where the same install
  command may run repeatedly.
- Install errors are friendlier on unix too: `Host.ResolveForVersion`
  probes the archive URL up front, so bad versions / unsupported
  triple combinations fail with a clear "ARM does not publish a
  &lt;triple&gt; build for &lt;version&gt;" message instead of dying
  later on a confusing checksum 404.

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
