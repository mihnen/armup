# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

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
