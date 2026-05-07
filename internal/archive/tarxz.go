package archive

import (
	"archive/tar"
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/ulikunitz/xz"
)

// extractTarXZ picks the fastest available extraction path:
//   - both `xz` and `tar` on PATH: pipe `xz -T 0 -dc` into `tar -x` (multi-threaded)
//   - only `tar` on PATH:           `tar -xJf -` (single-threaded native)
//   - neither:                      pure-Go ulikunitz/xz fallback (single-threaded)
//
// Setting ARMTOOLCHAIN_PURE_GO=1 forces the pure-Go path regardless of what's
// on PATH (useful for testing the fallback).
func extractTarXZ(ctx context.Context, src, dst string) error {
	if os.Getenv("ARMTOOLCHAIN_PURE_GO") == "1" {
		return extractTarXZPureGo(ctx, src, dst)
	}
	xzPath, _ := exec.LookPath("xz")
	tarPath, _ := exec.LookPath("tar")

	switch {
	case xzPath != "" && tarPath != "":
		return extractTarXZPipe(ctx, xzPath, tarPath, src, dst)
	case tarPath != "":
		return extractTarXZTarOnly(ctx, tarPath, src, dst)
	default:
		return extractTarXZPureGo(ctx, src, dst)
	}
}

// extractTarXZPipe runs `xz -T 0 -dc` reading from a progress-counted file
// reader, piped into `tar -xf - -C dst`. Both processes inherit ctx so a
// ctx cancel kills them.
func extractTarXZPipe(ctx context.Context, xzBin, tarBin, src, dst string) error {
	f, err := os.Open(src)
	if err != nil {
		return err
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil {
		return err
	}
	pr := newProgressReader(ctx, f, st.Size(), "extracting")

	threads := runtime.NumCPU()
	xzCmd := exec.CommandContext(ctx, xzBin, "-dc", "-T", fmt.Sprintf("%d", threads))
	xzCmd.Stdin = pr
	xzCmd.Stderr = os.Stderr
	xzOut, err := xzCmd.StdoutPipe()
	if err != nil {
		return err
	}

	tarCmd := exec.CommandContext(ctx, tarBin, "-xf", "-", "-C", dst)
	tarCmd.Stdin = xzOut
	tarCmd.Stdout = os.Stderr
	tarCmd.Stderr = os.Stderr

	if err := xzCmd.Start(); err != nil {
		return fmt.Errorf("start xz: %w", err)
	}
	if err := tarCmd.Start(); err != nil {
		_ = xzCmd.Process.Kill()
		_ = xzCmd.Wait()
		return fmt.Errorf("start tar: %w", err)
	}

	xzErr := xzCmd.Wait()
	tarErr := tarCmd.Wait()
	pr.done()

	if ctx.Err() != nil {
		return ctx.Err()
	}
	if xzErr != nil {
		return fmt.Errorf("xz: %w", xzErr)
	}
	if tarErr != nil {
		return fmt.Errorf("tar: %w", tarErr)
	}
	return nil
}

// extractTarXZTarOnly uses `tar -xJf -` reading from stdin (so we can show
// progress on the input file). GNU tar and bsdtar both accept -J for xz.
func extractTarXZTarOnly(ctx context.Context, tarBin, src, dst string) error {
	f, err := os.Open(src)
	if err != nil {
		return err
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil {
		return err
	}
	pr := newProgressReader(ctx, f, st.Size(), "extracting")

	cmd := exec.CommandContext(ctx, tarBin, "-xJf", "-", "-C", dst)
	cmd.Stdin = pr
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr

	err = cmd.Run()
	pr.done()
	if ctx.Err() != nil {
		return ctx.Err()
	}
	return err
}

func extractTarXZPureGo(ctx context.Context, src, dst string) error {
	f, err := os.Open(src)
	if err != nil {
		return err
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil {
		return err
	}

	// bufio between the file and xz is a major speed-up for ulikunitz/xz —
	// the lib makes many tiny reads, and without buffering each one is a
	// syscall. Default 4 KiB is enough; larger sizes don't help (the
	// bottleneck past that is the LZMA algorithm, not I/O).
	br := bufio.NewReader(f)
	pr := newProgressReader(ctx, br, st.Size(), "extracting")
	xr, err := xz.NewReader(pr)
	if err != nil {
		return fmt.Errorf("xz reader: %w", err)
	}
	tr := tar.NewReader(xr)

	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		hdr, err := tr.Next()
		if err == io.EOF {
			pr.done()
			return nil
		}
		if err != nil {
			return fmt.Errorf("tar next: %w", err)
		}
		if err := writeEntry(dst, hdr, tr); err != nil {
			return err
		}
	}
}

func writeEntry(dst string, hdr *tar.Header, r io.Reader) error {
	target, err := safeJoin(dst, hdr.Name)
	if err != nil {
		return err
	}
	switch hdr.Typeflag {
	case tar.TypeDir:
		return os.MkdirAll(target, os.FileMode(hdr.Mode)&0o777|0o700)
	case tar.TypeReg:
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		out, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC,
			os.FileMode(hdr.Mode)&0o777)
		if err != nil {
			return err
		}
		if _, err := io.Copy(out, r); err != nil {
			out.Close()
			return err
		}
		return out.Close()
	case tar.TypeSymlink:
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		os.Remove(target)
		return os.Symlink(hdr.Linkname, target)
	case tar.TypeLink:
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		linkSrc, err := safeJoin(dst, hdr.Linkname)
		if err != nil {
			return err
		}
		os.Remove(target)
		return os.Link(linkSrc, target)
	case tar.TypeXGlobalHeader, tar.TypeXHeader:
		return nil
	default:
		return nil
	}
}

func safeJoin(base, name string) (string, error) {
	clean := filepath.Clean("/" + name)[1:]
	full := filepath.Join(base, clean)
	rel, err := filepath.Rel(base, full)
	if err != nil || strings.HasPrefix(rel, "..") {
		return "", fmt.Errorf("archive: unsafe path %q", name)
	}
	return full, nil
}
