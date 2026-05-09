package arm

import (
	"runtime"
	"sort"
)

// LegacyEntry is one platform-specific archive of a legacy ARM toolchain
// release. The URL points at developer.arm.com's gnu-rm CDN path; the
// SHA-256 was computed locally after verifying ARM's MD5 (legacy archives
// don't ship a SHA-256 sidecar, so we embed our own here for ongoing
// verification).
type LegacyEntry struct {
	URL    string
	SHA256 string
}

// Legacy maps a version string (in ARM's filename form, e.g. "10.3-2021.10"
// or "9-2019-q4-major") to per-platform entries. The platform key is
// "<GOOS>-<GOARCH>".
//
// Versions ARM published only as 32-bit Windows / x86_64 Linux / x86_64
// macOS naturally have only those entries; arm64 Linux and Apple Silicon
// builds appeared in late releases (10.x onward).
//
// Legacy archives are .zip on Windows and .tar.bz2 on Linux/macOS (vs
// the modern line's .tar.xz). archive.Extract handles all of these.
//
// To add a new legacy entry: download the archive, verify the MD5 ARM
// publishes (sometimes), compute SHA-256 locally, and add a row.
var Legacy = map[string]map[string]LegacyEntry{
	"10.3-2021.10": {
		"windows-amd64": {URL: "https://developer.arm.com/-/media/files/downloads/gnu-rm/10.3-2021.10/gcc-arm-none-eabi-10.3-2021.10-win32.zip", SHA256: "d287439b3090843f3f4e29c7c41f81d958a5323aecefcf705c203bfd8ae3f2e7"},
		"linux-amd64":   {URL: "https://developer.arm.com/-/media/files/downloads/gnu-rm/10.3-2021.10/gcc-arm-none-eabi-10.3-2021.10-x86_64-linux.tar.bz2", SHA256: "97dbb4f019ad1650b732faffcc881689cedc14e2b7ee863d390e0a41ef16c9a3"},
		"linux-arm64":   {URL: "https://developer.arm.com/-/media/files/downloads/gnu-rm/10.3-2021.10/gcc-arm-none-eabi-10.3-2021.10-aarch64-linux.tar.bz2", SHA256: "f605b5f23ca898e9b8b665be208510a54a6e9fdd0fa5bfc9592002f6e7431208"},
		"darwin-amd64":  {URL: "https://developer.arm.com/-/media/files/downloads/gnu-rm/10.3-2021.10/gcc-arm-none-eabi-10.3-2021.10-mac.tar.bz2", SHA256: "fb613dacb25149f140f73fe9ff6c380bb43328e6bf813473986e9127e2bc283b"},
	},
	"9-2020-q2-update": {
		"windows-amd64": {URL: "https://developer.arm.com/-/media/files/downloads/gnu-rm/9-2020q2/gcc-arm-none-eabi-9-2020-q2-update-win32.zip", SHA256: "49d6029ecd176deaa437a15b3404f54792079a39f3b23cb46381b0e6fbbe9070"},
		"linux-amd64":   {URL: "https://developer.arm.com/-/media/files/downloads/gnu-rm/9-2020q2/gcc-arm-none-eabi-9-2020-q2-update-x86_64-linux.tar.bz2", SHA256: "5adc2ee03904571c2de79d5cfc0f7fe2a5c5f54f44da5b645c17ee57b217f11f"},
		"linux-arm64":   {URL: "https://developer.arm.com/-/media/files/downloads/gnu-rm/9-2020q2/gcc-arm-none-eabi-9-2020-q2-update-aarch64-linux.tar.bz2", SHA256: "1f4165c25e2cff80e29870f409862487ba470afd436e245ba3c743108e17b8ac"},
		"darwin-amd64":  {URL: "https://developer.arm.com/-/media/files/downloads/gnu-rm/9-2020q2/gcc-arm-none-eabi-9-2020-q2-update-mac.tar.bz2", SHA256: "bbb9b87e442b426eca3148fa74705c66b49a63f148902a0ea46f676ec24f9ac6"},
	},
	"9-2019-q4-major": {
		"windows-amd64": {URL: "https://developer.arm.com/-/media/files/downloads/gnu-rm/9-2019q4/gcc-arm-none-eabi-9-2019-q4-major-win32.zip", SHA256: "e4c964add8d0fdcc6b14f323e277a0946456082a84a1cc560da265b357762b62"},
		"linux-amd64":   {URL: "https://developer.arm.com/-/media/files/downloads/gnu-rm/9-2019q4/gcc-arm-none-eabi-9-2019-q4-major-x86_64-linux.tar.bz2", SHA256: "bcd840f839d5bf49279638e9f67890b2ef3a7c9c7a9b25271e83ec4ff41d177a"},
		"linux-arm64":   {URL: "https://developer.arm.com/-/media/files/downloads/gnu-rm/9-2019q4/gcc-arm-none-eabi-9-2019-q4-major-aarch64-linux.tar.bz2", SHA256: "1f5b9309006737950b2218250e6bb392e2d68d4f1a764fe66be96e2a78888d83"},
		"darwin-amd64":  {URL: "https://developer.arm.com/-/media/files/downloads/gnu-rm/9-2019q4/gcc-arm-none-eabi-9-2019-q4-major-mac.tar.bz2", SHA256: "1249f860d4155d9c3ba8f30c19e7a88c5047923cea17e0d08e633f12408f01f0"},
	},
	"8-2019-q3-update": {
		"windows-amd64": {URL: "https://developer.arm.com/-/media/files/downloads/gnu-rm/8-2019q3/rc1.1/gcc-arm-none-eabi-8-2019-q3-update-win32.zip", SHA256: "94913fe2414b424d81cd53fbf906f1865444c634260b7544b6098eb2a74dae1c"},
		"linux-amd64":   {URL: "https://developer.arm.com/-/media/files/downloads/gnu-rm/8-2019q3/rc1.1/gcc-arm-none-eabi-8-2019-q3-update-linux.tar.bz2", SHA256: "b50b02b0a16e5aad8620e9d7c31110ef285c1dde28980b1a9448b764d77d8f92"},
		"darwin-amd64":  {URL: "https://developer.arm.com/-/media/files/downloads/gnu-rm/8-2019q3/rc1.1/gcc-arm-none-eabi-8-2019-q3-update-mac.tar.bz2", SHA256: "fc235ce853bf3bceba46eff4b95764c5935ca07fc4998762ef5e5b7d05f37085"},
	},
	"8-2018-q4-major": {
		"windows-amd64": {URL: "https://developer.arm.com/-/media/files/downloads/gnu-rm/8-2018q4/gcc-arm-none-eabi-8-2018-q4-major-win32.zip", SHA256: "be5e2f68549efaecb79bdc34ff03c06f27deb2fcec3badddb5729cfb5ce43d6b"},
		"linux-amd64":   {URL: "https://developer.arm.com/-/media/files/downloads/gnu-rm/8-2018q4/gcc-arm-none-eabi-8-2018-q4-major-linux.tar.bz2", SHA256: "fb31fbdfe08406ece43eef5df623c0b2deb8b53e405e2c878300f7a1f303ee52"},
		"darwin-amd64":  {URL: "https://developer.arm.com/-/media/files/downloads/gnu-rm/8-2018q4/gcc-arm-none-eabi-8-2018-q4-major-mac.tar.bz2", SHA256: "0b528ed24db9f0fa39e5efdae9bcfc56bf9e07555cb267c70ff3fee84ec98460"},
	},
	"7-2018-q2-update": {
		"windows-amd64": {URL: "https://developer.arm.com/-/media/files/downloads/gnu-rm/7-2018q2/gcc-arm-none-eabi-7-2018-q2-update-win32.zip", SHA256: "8a1957063f7ee6b5c4f7b025bd4ebca2a4405a2f30d88d711353c72647df9e21"},
		"linux-amd64":   {URL: "https://developer.arm.com/-/media/files/downloads/gnu-rm/7-2018q2/gcc-arm-none-eabi-7-2018-q2-update-linux.tar.bz2", SHA256: "bb17109f0ee697254a5d4ae6e5e01440e3ea8f0277f2e8169bf95d07c7d5fe69"},
		"darwin-amd64":  {URL: "https://developer.arm.com/-/media/files/downloads/gnu-rm/7-2018q2/gcc-arm-none-eabi-7-2018-q2-update-mac.tar.bz2", SHA256: "c1c4af5226d52bd1b688cf1bd78f60eeea53b19fb337ef1df4380d752ba88759"},
	},
	"7-2017-q4-major": {
		"windows-amd64": {URL: "https://developer.arm.com/-/media/files/downloads/gnu-rm/7-2017q4/gcc-arm-none-eabi-7-2017-q4-major-win32.zip", SHA256: "ffafad128c9a8e2fe8023ff41ebc9a6ca9503dbe01d871dd07c68e2ba40b9b3f"},
		"linux-amd64":   {URL: "https://developer.arm.com/-/media/files/downloads/gnu-rm/7-2017q4/gcc-arm-none-eabi-7-2017-q4-major-linux.tar.bz2", SHA256: "96a029e2ae130a1210eaa69e309ea40463028eab18ba19c1086e4c2dafe69a6a"},
		"darwin-amd64":  {URL: "https://developer.arm.com/-/media/files/downloads/gnu-rm/7-2017q4/gcc-arm-none-eabi-7-2017-q4-major-mac.tar.bz2", SHA256: "89b776c7cf0591c810b5b60067e4dc113b5b71bc50084a536e71b894a97fdccb"},
	},
}

// LegacyLookup returns the legacy entry for `version` on the running
// host's GOOS/GOARCH. Returns (LegacyEntry{}, false) when the version
// isn't in the legacy table or wasn't shipped for this platform.
func LegacyLookup(version string) (LegacyEntry, bool) {
	perPlatform, ok := Legacy[version]
	if !ok {
		return LegacyEntry{}, false
	}
	e, ok := perPlatform[runtime.GOOS+"-"+runtime.GOARCH]
	return e, ok
}

// LegacyVersions returns the legacy version names available for the
// running host's GOOS/GOARCH, sorted newest first by the same heuristic
// the modern curated list uses.
func LegacyVersions() []string {
	key := runtime.GOOS + "-" + runtime.GOARCH
	var out []string
	for v, perPlatform := range Legacy {
		if _, ok := perPlatform[key]; ok {
			out = append(out, v)
		}
	}
	sortVersionsDesc(out)
	return out
}

// LegacyAllVersions returns every legacy version name regardless of
// platform availability. Used by tests and by `armup available --legacy`
// when the user wants to see the full set.
func LegacyAllVersions() []string {
	out := make([]string, 0, len(Legacy))
	for v := range Legacy {
		out = append(out, v)
	}
	sort.Slice(out, func(i, j int) bool { return cmpVersions(out[i], out[j]) > 0 })
	return out
}
