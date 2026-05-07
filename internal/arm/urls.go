package arm

import "fmt"

const baseURL = "https://developer.arm.com/-/media/Files/downloads/gnu"

// ArchiveFilename returns the on-disk filename of the toolchain archive for
// (version, host).
func (h Host) ArchiveFilename(version string) string {
	return fmt.Sprintf("arm-gnu-toolchain-%s-%s%s", version, h.Triple, h.Ext)
}

// ArchiveURL returns the download URL for the toolchain archive.
func (h Host) ArchiveURL(version string) string {
	return fmt.Sprintf("%s/%s/binrel/%s", baseURL, version, h.ArchiveFilename(version))
}

// ChecksumURL returns the URL for the SHA-256 sibling file.
// ARM publishes both .sha256 (which actually contains an MD5 sum, despite
// the name) and .sha256asc (which contains the real SHA-256). We use the
// latter.
func (h Host) ChecksumURL(version string) string {
	return h.ArchiveURL(version) + ".sha256asc"
}
