# Contributing

## Setup

Clone the repo and install pre-commit hooks (one-time):

```sh
git clone https://github.com/mihnen/armup
cd armup
pre-commit install
pre-commit install --hook-type pre-push
```

`pre-commit` is the Python tool from [pre-commit.com](https://pre-commit.com/).
Install it with `brew install pre-commit` (macOS), `pipx install pre-commit`
(Linux/Windows), or `pip install pre-commit`.

The hooks include `golangci-lint`, which must be on your `PATH`. If you
don't already have it:

```sh
go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest
# Make sure `$(go env GOPATH)/bin` is on your PATH.
```

## Before pushing

The pre-push hook runs `golangci-lint`. The pre-commit hook runs
`gofmt`, `go vet`, and a few generic file-cleanliness checks. If you
want to run everything manually:

```sh
gofmt -w -s .          # auto-format Go source
go vet ./...
go test -race ./...
golangci-lint run
```

To run all hooks across the entire repo (handy after pulling a big
change):

```sh
pre-commit run --all-files
pre-commit run --all-files --hook-stage pre-push
```

## Tests

```sh
go test ./...
```

Windows-tagged code (the registry-based PATH integration and NTFS
junction creation) can be exercised cross-platform on Linux via wine:

```sh
GOOS=windows GOARCH=amd64 go test -exec wine ./...
```

Tests that need real `FSCTL_SET_REPARSE_POINT` (junction creation) skip
under wine; CI's `windows-latest` runner exercises them natively.

## CI

Every push and pull request runs:

- `go vet`, `gofmt -d`, and `go test -race` on `ubuntu-latest`,
  `macos-latest`, and `windows-latest`.
- Cross-compile sanity for all five shipped targets (linux/amd64,
  linux/arm64, darwin/amd64, darwin/arm64, windows/amd64).
- `golangci-lint` on `ubuntu-latest`.

If the pre-commit hooks pass locally, CI generally passes too.

## Releases

- Tag `v<MAJOR>.<MINOR>.<PATCH>` (or `v<...>-<PRERELEASE>` for betas).
  The release workflow builds and uploads archives for all five
  targets, plus a `SHA256SUMS` file.
- Every push to `master` also produces a rolling pre-release at the
  `nightly` tag. Asset URLs are stable, e.g.
  `releases/download/nightly/armup-nightly-linux-amd64.tar.gz`.
- Update `CHANGELOG.md` (top `[Unreleased]` section) as you go;
  rename it to the new version on tag.
