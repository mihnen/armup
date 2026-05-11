package arm

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

// FetchChecksum downloads the SHA-256 hash file for a given source
// and returns the expected hex hash. src may be an https:// URL, a
// file:// URI, or a bare local path (the local forms support mirrors
// pointed at a directory).
func FetchChecksum(ctx context.Context, src string) (string, error) {
	body, err := readChecksumBody(ctx, src)
	if err != nil {
		return "", err
	}
	// .sha256 files contain "<hex>  <filename>"; we just want the hex.
	fields := strings.Fields(string(body))
	if len(fields) == 0 {
		return "", fmt.Errorf("checksum file at %s is empty", src)
	}
	return strings.ToLower(fields[0]), nil
}

func readChecksumBody(ctx context.Context, src string) ([]byte, error) {
	if isLocal(src) {
		return os.ReadFile(localPath(src))
	}
	client := &http.Client{Timeout: 30 * time.Second}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, src, nil)
	if err != nil {
		return nil, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("GET %s: %w", src, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GET %s: %s", src, resp.Status)
	}
	return io.ReadAll(resp.Body)
}

func isLocal(src string) bool {
	if strings.HasPrefix(src, "file://") {
		return true
	}
	return !strings.HasPrefix(src, "http://") && !strings.HasPrefix(src, "https://")
}

func localPath(src string) string {
	return strings.TrimPrefix(src, "file://")
}

// VerifyFile hashes the file at path and compares to expected (hex).
func VerifyFile(path, expected string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return err
	}
	got := hex.EncodeToString(h.Sum(nil))
	if !strings.EqualFold(got, expected) {
		return fmt.Errorf("checksum mismatch for %s: expected %s, got %s",
			path, expected, got)
	}
	return nil
}
