// Package pin resolves a per-project armup version pin.
//
// Resolution order (first match wins):
//
//  1. ARMUP_VERSION environment variable, if set.
//  2. The first .tool-versions file found while walking from cwd up to
//     the filesystem root, parsed as `armup <version>` (asdf/mise format,
//     comments and other tools tolerated).
//  3. The first .armup-version file found in the same walk, with the
//     version on a single line.
//
// A successful resolution returns the version string and the Source that
// produced it (a file path, or the literal env-var label). When nothing is
// pinned, Resolve returns a zero Result and a nil error — callers should
// treat that as "no pin," not as an error.
package pin

import (
	"bufio"
	"errors"
	"os"
	"path/filepath"
	"strings"

	"github.com/mihnen/armup/internal/arm"
)

const (
	envVar         = "ARMUP_VERSION"
	envSource      = "ARMUP_VERSION env var"
	toolVersionsFn = ".tool-versions"
	armupVersionFn = ".armup-version"
	toolKey        = "armup"
)

// Result is the outcome of a pin resolution.
type Result struct {
	// Version is the pinned version (already normalized via arm.Normalize),
	// or "" when nothing is pinned.
	Version string
	// Source describes where Version came from — a file path or
	// `ARMUP_VERSION env var`. Empty when Version is empty.
	Source string
}

// Found reports whether the resolution produced a pinned version.
func (r Result) Found() bool { return r.Version != "" }

// Resolve performs the full resolution from `cwd`. Pass an absolute path.
//
// Errors are returned only for unexpected I/O failures (e.g. a version
// file exists but is unreadable). A pin file that exists but doesn't
// mention armup is treated as "no pin from this file" and the walk
// continues.
func Resolve(cwd string) (Result, error) {
	if v := strings.TrimSpace(os.Getenv(envVar)); v != "" {
		return Result{Version: arm.Normalize(v), Source: envSource}, nil
	}

	dir, err := filepath.Abs(cwd)
	if err != nil {
		return Result{}, err
	}

	for {
		// Try .tool-versions first (asdf/mise compat); fall back to
		// .armup-version in the same dir before walking up. This lets
		// projects use either format without surprises.
		if r, err := readToolVersions(filepath.Join(dir, toolVersionsFn)); err != nil {
			return Result{}, err
		} else if r.Found() {
			return r, nil
		}
		if r, err := readArmupVersion(filepath.Join(dir, armupVersionFn)); err != nil {
			return Result{}, err
		} else if r.Found() {
			return r, nil
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			return Result{}, nil
		}
		dir = parent
	}
}

// readToolVersions parses a file in asdf/mise format and returns the
// `armup <version>` entry, if any.
func readToolVersions(path string) (Result, error) {
	f, err := os.Open(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return Result{}, nil
		}
		return Result{}, err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) >= 2 && fields[0] == toolKey {
			return Result{
				Version: arm.Normalize(fields[1]),
				Source:  path,
			}, nil
		}
	}
	if err := scanner.Err(); err != nil {
		return Result{}, err
	}
	return Result{}, nil
}

// readArmupVersion reads a single-version file. Whitespace and a single
// trailing blank line are ignored.
func readArmupVersion(path string) (Result, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return Result{}, nil
		}
		return Result{}, err
	}
	v := strings.TrimSpace(string(b))
	if v == "" {
		return Result{}, nil
	}
	return Result{
		Version: arm.Normalize(v),
		Source:  path,
	}, nil
}
