package arm

import (
	"fmt"
	"runtime"
)

type Host struct {
	Triple string // e.g. "x86_64-arm-none-eabi"
	Ext    string // ".tar.xz" or ".zip"
}

// CurrentHost returns the ARM-toolchain host descriptor for the running
// process. Errors out for unsupported GOOS/GOARCH combos.
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

// InnerDirName returns the top-level directory name inside the archive that
// ARM ships, which we rename to the bare version after extraction.
func (h Host) InnerDirName(version string) string {
	return fmt.Sprintf("arm-gnu-toolchain-%s-%s", version, h.Triple)
}
