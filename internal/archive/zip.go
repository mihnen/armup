package archive

import (
	"archive/zip"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"
)

func extractZip(ctx context.Context, src, dst string) error {
	r, err := zip.OpenReader(src)
	if err != nil {
		return err
	}
	defer r.Close()

	// Sum uncompressed bytes for the progress denominator. Cheap — header
	// data is already in memory after OpenReader.
	var total int64
	for _, f := range r.File {
		if !f.FileInfo().IsDir() {
			total += int64(f.UncompressedSize64)
		}
	}

	var done int64
	var lastTick time.Time
	printProgress := func(force bool) {
		if !force && time.Since(lastTick) < 200*time.Millisecond {
			return
		}
		lastTick = time.Now()
		if total > 0 {
			fmt.Fprintf(os.Stderr, "\r  extracting %.1f%%  %s / %s",
				float64(done)/float64(total)*100,
				humanBytes(done), humanBytes(total))
		} else {
			fmt.Fprintf(os.Stderr, "\r  extracting %s", humanBytes(done))
		}
	}
	defer func() {
		printProgress(true)
		fmt.Fprintln(os.Stderr)
	}()

	for _, f := range r.File {
		if err := ctx.Err(); err != nil {
			return err
		}
		target, err := safeJoin(dst, f.Name)
		if err != nil {
			return err
		}
		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(target, f.Mode()|0o700); err != nil {
				return err
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		mode := f.Mode()
		if mode&os.ModeSymlink != 0 {
			rc, err := f.Open()
			if err != nil {
				return err
			}
			b, err := io.ReadAll(rc)
			rc.Close()
			if err != nil {
				return err
			}
			os.Remove(target)
			if err := os.Symlink(string(b), target); err != nil {
				return err
			}
			done += int64(len(b))
			printProgress(false)
			continue
		}
		rc, err := f.Open()
		if err != nil {
			return err
		}
		out, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
		if err != nil {
			rc.Close()
			return err
		}
		// Copy in 64 KiB chunks so a large file doesn't stall progress.
		buf := make([]byte, 64*1024)
		for {
			n, rerr := rc.Read(buf)
			if n > 0 {
				if _, werr := out.Write(buf[:n]); werr != nil {
					rc.Close()
					out.Close()
					return werr
				}
				done += int64(n)
				printProgress(false)
			}
			if rerr == io.EOF {
				break
			}
			if rerr != nil {
				rc.Close()
				out.Close()
				return rerr
			}
		}
		rc.Close()
		if err := out.Close(); err != nil {
			return err
		}
	}
	return nil
}
