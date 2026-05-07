//go:build windows

package shell

import (
	"fmt"
	"path/filepath"
	"strings"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

// EnsureOnPath appends `dir` to the user's PATH (HKCU\Environment\Path) if
// not already present, then broadcasts WM_SETTINGCHANGE so already-running
// shells can pick up the change. Returns the registry key path as the
// "updated" label so the caller's status output remains meaningful.
//
// No admin required — this only touches HKCU. Existing entries are matched
// case-insensitively with filepath.Clean on both sides so trailing
// slashes and case mismatches don't cause duplicates on re-init.
func EnsureOnPath(dir string) ([]string, error) {
	const keyPath = `Environment`
	k, err := registry.OpenKey(registry.CURRENT_USER, keyPath,
		registry.QUERY_VALUE|registry.SET_VALUE)
	if err != nil {
		return nil, fmt.Errorf("open HKCU\\%s: %w", keyPath, err)
	}
	defer k.Close()

	cur, _, err := k.GetStringValue("Path")
	if err != nil && err != registry.ErrNotExist {
		return nil, fmt.Errorf("read HKCU\\%s\\Path: %w", keyPath, err)
	}
	if alreadyOnPath(cur, dir) {
		return nil, nil
	}

	next := strings.TrimRight(cur, ";")
	if next != "" {
		next += ";"
	}
	next += dir

	// Use REG_EXPAND_SZ so values like %USERPROFILE% in the existing PATH
	// keep working when reread. SetExpandStringValue does that.
	if err := k.SetExpandStringValue("Path", next); err != nil {
		return nil, fmt.Errorf("write HKCU\\%s\\Path: %w", keyPath, err)
	}

	broadcastSettingChange()
	return []string{`HKCU\Environment\Path`}, nil
}

func alreadyOnPath(pathVar, dir string) bool {
	want := strings.ToLower(filepath.Clean(dir))
	for _, entry := range strings.Split(pathVar, ";") {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}
		if strings.ToLower(filepath.Clean(entry)) == want {
			return true
		}
	}
	return false
}

// RemoveFromPath removes `dir` from HKCU\Environment\Path if present.
// Returns the registry path as the "modified" label when a change was made,
// nil if dir wasn't on the path.
func RemoveFromPath(dir string) ([]string, error) {
	const keyPath = `Environment`
	k, err := registry.OpenKey(registry.CURRENT_USER, keyPath,
		registry.QUERY_VALUE|registry.SET_VALUE)
	if err != nil {
		return nil, fmt.Errorf("open HKCU\\%s: %w", keyPath, err)
	}
	defer k.Close()

	cur, _, err := k.GetStringValue("Path")
	if err != nil && err != registry.ErrNotExist {
		return nil, fmt.Errorf("read HKCU\\%s\\Path: %w", keyPath, err)
	}
	if !alreadyOnPath(cur, dir) {
		return nil, nil
	}

	want := strings.ToLower(filepath.Clean(dir))
	var kept []string
	for _, entry := range strings.Split(cur, ";") {
		trimmed := strings.TrimSpace(entry)
		if trimmed == "" {
			continue
		}
		if strings.ToLower(filepath.Clean(trimmed)) == want {
			continue
		}
		kept = append(kept, trimmed)
	}
	next := strings.Join(kept, ";")

	if err := k.SetExpandStringValue("Path", next); err != nil {
		return nil, fmt.Errorf("write HKCU\\%s\\Path: %w", keyPath, err)
	}
	broadcastSettingChange()
	return []string{`HKCU\Environment\Path`}, nil
}

// broadcastSettingChange tells every top-level window the user environment
// changed, so their copy of the env block is refreshed. Without this, only
// processes started after the registry write see the new PATH.
//
// Best-effort: ignore errors. The registry write already succeeded.
func broadcastSettingChange() {
	user32 := windows.NewLazyDLL("user32.dll")
	sendMessageTimeout := user32.NewProc("SendMessageTimeoutW")

	const (
		HWND_BROADCAST   = 0xFFFF
		WM_SETTINGCHANGE = 0x001A
		SMTO_ABORTIFHUNG = 0x0002
	)
	envPtr, err := syscall.UTF16PtrFromString("Environment")
	if err != nil {
		return
	}
	var result uintptr
	_, _, _ = sendMessageTimeout.Call(
		uintptr(HWND_BROADCAST),
		uintptr(WM_SETTINGCHANGE),
		0,
		uintptr(unsafe.Pointer(envPtr)),
		uintptr(SMTO_ABORTIFHUNG),
		5000,
		uintptr(unsafe.Pointer(&result)),
	)
}
