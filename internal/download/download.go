package download

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

// ToFile streams url to dst, writing a percentage progress line to stderr
// when content-length is known. Honors HTTP_PROXY/HTTPS_PROXY via the default
// transport.
func ToFile(ctx context.Context, url, dst string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	client := &http.Client{Timeout: 0}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("GET %s: %w", url, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("GET %s: %s", url, resp.Status)
	}

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	pw := &progressWriter{
		total: resp.ContentLength,
		label: dst,
		start: time.Now(),
	}
	if _, err := io.Copy(io.MultiWriter(out, pw), resp.Body); err != nil {
		os.Remove(dst)
		return err
	}
	pw.done()
	return nil
}

type progressWriter struct {
	total    int64
	read     int64
	label    string
	start    time.Time
	lastTick time.Time
}

func (p *progressWriter) Write(b []byte) (int, error) {
	n := len(b)
	p.read += int64(n)
	now := time.Now()
	if now.Sub(p.lastTick) < 200*time.Millisecond {
		return n, nil
	}
	p.lastTick = now
	p.print()
	return n, nil
}

func (p *progressWriter) print() {
	if p.total > 0 {
		pct := float64(p.read) / float64(p.total) * 100
		fmt.Fprintf(os.Stderr, "\r  %.1f%%  %s / %s",
			pct, humanBytes(p.read), humanBytes(p.total))
	} else {
		fmt.Fprintf(os.Stderr, "\r  %s", humanBytes(p.read))
	}
}

func (p *progressWriter) done() {
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
