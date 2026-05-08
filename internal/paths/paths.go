package paths

import "path/filepath"

const appName = "arm-toolchains"

func VersionsDir() string   { return filepath.Join(DataDir(), "versions") }
func CacheDir() string      { return filepath.Join(DataDir(), "cache") }
func CurrentLink() string   { return filepath.Join(DataDir(), "current") }
func AvailableFile() string { return filepath.Join(DataDir(), "available.txt") }
func ActiveBinDir() string  { return filepath.Join(CurrentLink(), "bin") }
func VersionDir(version string) string {
	return filepath.Join(VersionsDir(), version)
}
func VersionBinDir(version string) string {
	return filepath.Join(VersionDir(version), "bin")
}
