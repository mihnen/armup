package archive

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"time"
)

// Extract unpacks src into dst (which must already exist). Format is chosen
// by extension: .tar.xz, .tar.gz, or .zip. Honors ctx for cancellation.
func Extract(ctx context.Context, src, dst string) error {
	switch {
	case strings.HasSuffix(src, ".tar.xz"):
		return extractTarXZ(ctx, src, dst)
	case strings.HasSuffix(src, ".tar.gz"):
		return extractTarGZ(ctx, src, dst)
	case strings.HasSuffix(src, ".zip"):
		return extractZip(ctx, src, dst)
	default:
		return fmt.Errorf("archive: unsupported extension on %s", src)
	}
}

// progressReader wraps an io.Reader and prints a "X / Y" line to stderr based
// on bytes read. Honors ctx: returns ctx.Err() if cancelled.
type progressReader struct {
	ctx      context.Context
	r        io.Reader
	total    int64
	read     int64
	lastTick time.Time
	label    string
}

func newProgressReader(ctx context.Context, r io.Reader, total int64, label string) *progressReader {
	return &progressReader{ctx: ctx, r: r, total: total, label: label}
}

func (p *progressReader) Read(b []byte) (int, error) {
	if p.ctx != nil {
		if err := p.ctx.Err(); err != nil {
			return 0, err
		}
	}
	n, err := p.r.Read(b)
	p.read += int64(n)
	now := time.Now()
	if now.Sub(p.lastTick) >= 200*time.Millisecond {
		p.lastTick = now
		p.print()
	}
	return n, err
}

func (p *progressReader) print() {
	if p.total > 0 {
		pct := float64(p.read) / float64(p.total) * 100
		fmt.Fprintf(os.Stderr, "\r  %s %.1f%%  %s / %s",
			p.label, pct, humanBytes(p.read), humanBytes(p.total))
	} else {
		fmt.Fprintf(os.Stderr, "\r  %s %s", p.label, humanBytes(p.read))
	}
}

func (p *progressReader) done() {
	p.print()
	fmt.Fprintln(os.Stderr)
}

func humanBytes(n int64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	div, exp := int64(unit), 0
	for x := n / unit; x >= unit; x /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB", float64(n)/float64(div), "KMGTPE"[exp])
}
