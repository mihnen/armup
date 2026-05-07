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

// FetchChecksum downloads the .sha256 sidecar for a given URL and returns the
// expected hex hash.
func FetchChecksum(ctx context.Context, url string) (string, error) {
	client := &http.Client{Timeout: 30 * time.Second}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("GET %s: %w", url, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("GET %s: %s", url, resp.Status)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	// .sha256 files contain "<hex>  <filename>"; we just want the hex.
	fields := strings.Fields(string(body))
	if len(fields) == 0 {
		return "", fmt.Errorf("checksum file at %s is empty", url)
	}
	return strings.ToLower(fields[0]), nil
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
