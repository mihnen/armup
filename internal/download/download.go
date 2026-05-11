package download

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

// ToFile streams src to dst, writing a percentage progress line to stderr
// when content-length is known.
func ToFile(ctx context.Context, src, dst string) error {
	return toFile(ctx, src, dst, true, nil)
}

// ToFileQuiet behaves like ToFile but suppresses the per-byte progress
// bar. Used by code paths that run many downloads concurrently (e.g.
// `armup mirror create`), where interleaved progress lines would be
// unreadable.
func ToFileQuiet(ctx context.Context, src, dst string) error {
	return toFile(ctx, src, dst, false, nil)
}

// ProgressHook is called per chunk with the byte count of just that
// chunk (not cumulative). The caller can sum these atomically to
// track total progress across many concurrent downloads.
type ProgressHook func(delta int64)

// ToFileWithProgress is like ToFileQuiet but invokes hook on every
// chunk, allowing the caller to aggregate progress across many
// goroutines. hook is called from the goroutine doing the I/O, so it
// must be safe for concurrent use if shared.
func ToFileWithProgress(ctx context.Context, src, dst string, hook ProgressHook) error {
	return toFile(ctx, src, dst, false, hook)
}

// toFile is the common body. src may be:
//   - https://... or http://...   — fetched via the default HTTP client
//   - file:///...                  — read from the local filesystem
//   - /absolute/path or relative   — read from the local filesystem
//
// The file:// and bare-path forms make ARMUP_MIRROR pointing at a local
// directory work without a separate code path.
func toFile(ctx context.Context, src, dst string, showProgress bool, hook ProgressHook) error {
	if IsLocal(src) {
		return copyLocal(LocalPath(src), dst, showProgress, hook)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, src, nil)
	if err != nil {
		return err
	}
	client := &http.Client{Timeout: 0}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("GET %s: %w", src, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("GET %s: %s", src, resp.Status)
	}

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	var writers []io.Writer
	writers = append(writers, out)
	var pw *progressWriter
	if showProgress {
		pw = &progressWriter{
			total: resp.ContentLength,
			label: dst,
			start: time.Now(),
		}
		writers = append(writers, pw)
	}
	if hook != nil {
		writers = append(writers, &hookWriter{hook: hook})
	}
	w := io.MultiWriter(writers...)
	if _, err := io.Copy(w, resp.Body); err != nil {
		os.Remove(dst)
		return err
	}
	if pw != nil {
		pw.done()
	}
	return nil
}

// hookWriter discards bytes but forwards each chunk's size to a hook.
type hookWriter struct {
	hook ProgressHook
}

func (h *hookWriter) Write(b []byte) (int, error) {
	n := len(b)
	h.hook(int64(n))
	return n, nil
}

// IsLocal reports whether src refers to the local filesystem (a
// file:// URI or a non-scheme bare path).
func IsLocal(src string) bool {
	if strings.HasPrefix(src, "file://") {
		return true
	}
	return !strings.HasPrefix(src, "http://") && !strings.HasPrefix(src, "https://")
}

// LocalPath strips the file:// prefix if present and returns the
// underlying filesystem path. Caller has typically already gated
// the value through IsLocal.
func LocalPath(src string) string {
	return strings.TrimPrefix(src, "file://")
}

func copyLocal(src, dst string, showProgress bool, hook ProgressHook) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	stat, err := in.Stat()
	if err != nil {
		return err
	}
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	var writers []io.Writer
	writers = append(writers, out)
	var pw *progressWriter
	if showProgress {
		pw = &progressWriter{
			total: stat.Size(),
			label: dst,
			start: time.Now(),
		}
		writers = append(writers, pw)
	}
	if hook != nil {
		writers = append(writers, &hookWriter{hook: hook})
	}
	w := io.MultiWriter(writers...)
	if _, err := io.Copy(w, in); err != nil {
		os.Remove(dst)
		return err
	}
	if pw != nil {
		pw.done()
	}
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
