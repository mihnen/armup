//go:build windows

package paths

import (
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"

	"golang.org/x/sys/windows"
)

const reparseTagMountPoint = 0xA0000003

// MakeDirLink creates `link` as an NTFS junction pointing at `target`.
// Junctions are like directory symlinks but don't require admin/Developer
// Mode. They only work for directories on local volumes (target and link
// can be on different volumes; Windows handles the indirection).
//
// Steps:
//  1. mkdir the link path (junctions need an existing empty dir to attach to)
//  2. open it with reparse-point + backup semantics
//  3. issue FSCTL_SET_REPARSE_POINT with a MountPointReparseBuffer
func MakeDirLink(target, link string) error {
	abs, err := filepath.Abs(target)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(link, 0o755); err != nil {
		return err
	}

	pLink, err := windows.UTF16PtrFromString(link)
	if err != nil {
		return err
	}
	h, err := windows.CreateFile(
		pLink,
		windows.GENERIC_WRITE,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE,
		nil,
		windows.OPEN_EXISTING,
		windows.FILE_FLAG_OPEN_REPARSE_POINT|windows.FILE_FLAG_BACKUP_SEMANTICS,
		0,
	)
	if err != nil {
		os.Remove(link)
		return fmt.Errorf("open junction target: %w", err)
	}
	defer windows.CloseHandle(h)

	buf, err := buildJunctionReparseBuffer(abs)
	if err != nil {
		os.Remove(link)
		return err
	}

	var bytesReturned uint32
	if err := windows.DeviceIoControl(
		h,
		windows.FSCTL_SET_REPARSE_POINT,
		&buf[0], uint32(len(buf)),
		nil, 0,
		&bytesReturned, nil,
	); err != nil {
		os.Remove(link)
		return fmt.Errorf("set junction reparse point: %w", err)
	}
	return nil
}

// SetCurrent points `link` at `target` on Windows. Atomic-replace of a
// junction by rename isn't supported on Windows, so we delete-then-create.
// `link` is expected to be either nonexistent or an existing junction we
// created earlier — `os.Remove` on a real non-empty directory fails, so a
// hand-placed real directory is protected.
func SetCurrent(target, link string) error {
	if _, err := os.Lstat(link); err == nil {
		if err := os.Remove(link); err != nil {
			return fmt.Errorf("remove existing %s: %w", link, err)
		}
	}
	return MakeDirLink(target, link)
}

// buildJunctionReparseBuffer constructs a REPARSE_DATA_BUFFER with a
// MountPointReparseBuffer body for the given absolute target.
//
// Layout (little-endian):
//
//	uint32 ReparseTag                   (= IO_REPARSE_TAG_MOUNT_POINT)
//	uint16 ReparseDataLength            (size of body, excluding this header)
//	uint16 Reserved                     (0)
//	uint16 SubstituteNameOffset         (0)
//	uint16 SubstituteNameLength         (bytes, no NUL)
//	uint16 PrintNameOffset              (SubstituteNameLength + sizeof NUL)
//	uint16 PrintNameLength              (bytes, no NUL)
//	WCHAR  PathBuffer[]:  Substitute (NUL) Print (NUL)
//
// SubstituteName must use the NT object namespace prefix `\??\`, e.g.
// `\??\C:\absolute\path`. PrintName is the user-facing form without prefix.
func buildJunctionReparseBuffer(absTarget string) ([]byte, error) {
	sub := `\??\` + absTarget
	subUTF16, err := windows.UTF16FromString(sub)
	if err != nil {
		return nil, err
	}
	printUTF16, err := windows.UTF16FromString(absTarget)
	if err != nil {
		return nil, err
	}
	subBytes := (len(subUTF16) - 1) * 2 // exclude trailing NUL from length
	printBytes := (len(printUTF16) - 1) * 2

	pathBuf := make([]byte, 0, len(subUTF16)*2+len(printUTF16)*2)
	pathBuf = appendUTF16(pathBuf, subUTF16)
	pathBuf = appendUTF16(pathBuf, printUTF16)

	// MountPointReparseBuffer header (4 uint16) + path buffer
	mountPointHdrLen := 8
	dataLen := mountPointHdrLen + len(pathBuf)

	// REPARSE_DATA_BUFFER header (uint32 + uint16 + uint16) = 8
	out := make([]byte, 8+dataLen)
	binary.LittleEndian.PutUint32(out[0:4], reparseTagMountPoint)
	binary.LittleEndian.PutUint16(out[4:6], uint16(dataLen))
	// reserved [6:8] = 0
	binary.LittleEndian.PutUint16(out[8:10], 0)                   // SubstituteNameOffset
	binary.LittleEndian.PutUint16(out[10:12], uint16(subBytes))   // SubstituteNameLength
	binary.LittleEndian.PutUint16(out[12:14], uint16(subBytes+2)) // PrintNameOffset (after Substitute + NUL)
	binary.LittleEndian.PutUint16(out[14:16], uint16(printBytes)) // PrintNameLength
	copy(out[16:], pathBuf)
	return out, nil
}

func appendUTF16(dst []byte, src []uint16) []byte {
	for _, c := range src {
		var b [2]byte
		binary.LittleEndian.PutUint16(b[:], c)
		dst = append(dst, b[:]...)
	}
	return dst
}
