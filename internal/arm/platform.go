package arm

import (
	"context"
	"fmt"
	"net/http"
	"runtime"
	"time"
)

type Host struct {
	Triple string // e.g. "x86_64-arm-none-eabi"
	Ext    string // ".tar.xz" or ".zip"
}

// CurrentHost returns the ARM-toolchain host descriptor for the running
// process. Errors out for unsupported GOOS/GOARCH combos.
//
// On Windows the returned triple is the *preferred* one (x86_64 mingw, only
// shipped from 14.2.rel1 onward); call ResolveForVersion before using the
// host's URLs to fall back to the i686 mingw variant for older releases.
func CurrentHost() (Host, error) {
	switch runtime.GOOS {
	case "linux":
		switch runtime.GOARCH {
		case "amd64":
			return Host{"x86_64-arm-none-eabi", ".tar.xz"}, nil
		case "arm64":
			return Host{"aarch64-arm-none-eabi", ".tar.xz"}, nil
		}
	case "darwin":
		switch runtime.GOARCH {
		case "arm64":
			return Host{"darwin-arm64-arm-none-eabi", ".tar.xz"}, nil
		case "amd64":
			return Host{"darwin-x86_64-arm-none-eabi", ".tar.xz"}, nil
		}
	case "windows":
		if runtime.GOARCH == "amd64" {
			return Host{"mingw-w64-x86_64-arm-none-eabi", ".zip"}, nil
		}
	}
	return Host{}, fmt.Errorf("arm: unsupported platform %s/%s", runtime.GOOS, runtime.GOARCH)
}

// ResolveForVersion picks the actual triple to use for `version`.
//
// Unix triples are stable across releases — this is a no-op there.
//
// Windows is messier: ARM ships an i686 (32-bit) mingw build for every
// release back to 11.x, but only added a native x86_64 mingw variant at
// 14.2.rel1. We prefer x86_64 when available and fall back to i686
// otherwise. Both produce the same arm-none-eabi compiler; the difference
// is just whether the compiler itself runs as a 32- or 64-bit Windows
// process.
func (h Host) ResolveForVersion(ctx context.Context, version string) (Host, error) {
	if runtime.GOOS != "windows" {
		return h, nil
	}
	for _, triple := range []string{
		"mingw-w64-x86_64-arm-none-eabi",
		"mingw-w64-i686-arm-none-eabi",
	} {
		cand := Host{Triple: triple, Ext: h.Ext}
		ok, err := probeArchive(ctx, cand.ArchiveURL(version))
		if err != nil {
			return Host{}, err
		}
		if ok {
			return cand, nil
		}
	}
	return Host{}, fmt.Errorf("ARM does not publish a Windows build for %s", version)
}

// InnerDirName returns the top-level directory name inside the archive that
// ARM ships, which we rename to the bare version after extraction.
func (h Host) InnerDirName(version string) string {
	return fmt.Sprintf("arm-gnu-toolchain-%s-%s", version, h.Triple)
}

func probeArchive(ctx context.Context, url string) (bool, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodHead, url, nil)
	if err != nil {
		return false, err
	}
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return false, err
	}
	resp.Body.Close()
	return resp.StatusCode >= 200 && resp.StatusCode < 400, nil
}
