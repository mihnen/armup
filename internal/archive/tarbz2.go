package archive

import (
	"archive/tar"
	"bufio"
	"compress/bzip2"
	"context"
	"fmt"
	"io"
	"os"
)

// extractTarBZ2 unpacks a .tar.bz2 with the stdlib bzip2 reader. Used for
// legacy ARM toolchain releases (gnu-rm path) that ship as bz2. Pure-Go,
// single-threaded — fine for the small legacy archives (~80–150 MiB).
func extractTarBZ2(ctx context.Context, src, dst string) error {
	f, err := os.Open(src)
	if err != nil {
		return err
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil {
		return err
	}

	// bzip2 makes lots of small reads; same syscall-amplification fix
	// we use for ulikunitz/xz.
	br := bufio.NewReader(f)
	pr := newProgressReader(ctx, br, st.Size(), "extracting")
	zr := bzip2.NewReader(pr) // bzip2 reader has no Close method
	tr := tar.NewReader(zr)

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
