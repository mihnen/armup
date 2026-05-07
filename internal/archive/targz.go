package archive

import (
	"archive/tar"
	"bufio"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"os"
)

// extractTarGZ unpacks a .tar.gz with the stdlib gzip reader. Used by
// self-update for armup's own release archives. Pure-Go is fine here —
// gzip in stdlib is fast and the archives are small (a few MB).
func extractTarGZ(ctx context.Context, src, dst string) error {
	f, err := os.Open(src)
	if err != nil {
		return err
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil {
		return err
	}

	br := bufio.NewReader(f)
	pr := newProgressReader(ctx, br, st.Size(), "extracting")
	gr, err := gzip.NewReader(pr)
	if err != nil {
		return fmt.Errorf("gzip reader: %w", err)
	}
	defer gr.Close()
	tr := tar.NewReader(gr)

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
