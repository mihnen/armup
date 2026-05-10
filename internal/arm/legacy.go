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

// Legacy maps a version string (ARM's display form on the gnu-rm page,
// e.g. "10.3-2021.10", "9-2019-q4-major", "5-2016-q3-update") to
// per-platform entries. The platform key is "<GOOS>-<GOARCH>".
//
// Versions ARM published only as 32-bit Windows / x86_64 Linux / x86_64
// macOS naturally have only those entries; arm64 Linux and Apple Silicon
// builds appeared in late releases (10.x onward).
//
// Legacy archives are .zip on Windows and .tar.bz2 on Linux/macOS (vs
// the modern line's .tar.xz). archive.Extract handles all of these.
//
// A few quirks for the older entries:
//   - 5.x and 6_1-2017q1 Windows must use the `-win32-zip.zip` URL
//     variant — the plain `-win32.zip` URL silently redirects to a
//     Nullsoft installer `.exe` which we can't extract.
//   - 5-2016-q1/q2 Linux + macOS use a mangled-filename URL with a
//     `?revision=<uuid>` query string (Akamai needs the revision UUID
//     to dispatch correctly). The query string must be preserved.
//   - 5-2016-q1/q2 ship Windows only as a Nullsoft installer; we omit
//     a Windows entry for those (use `armup install --from <local>`
//     after running the .exe by hand).
//
// To add a new legacy entry: download the archive, verify any
// MD5 ARM or Launchpad publishes, compute SHA-256 locally, and add a
// row.
var Legacy = map[string]map[string]LegacyEntry{
	"10.3-2021.10": {
		"windows-amd64": {URL: "https://developer.arm.com/-/media/files/downloads/gnu-rm/10.3-2021.10/gcc-arm-none-eabi-10.3-2021.10-win32.zip", SHA256: "d287439b3090843f3f4e29c7c41f81d958a5323aecefcf705c203bfd8ae3f2e7"},
		"linux-amd64":   {URL: "https://developer.arm.com/-/media/files/downloads/gnu-rm/10.3-2021.10/gcc-arm-none-eabi-10.3-2021.10-x86_64-linux.tar.bz2", SHA256: "97dbb4f019ad1650b732faffcc881689cedc14e2b7ee863d390e0a41ef16c9a3"},
		"linux-arm64":   {URL: "https://developer.arm.com/-/media/files/downloads/gnu-rm/10.3-2021.10/gcc-arm-none-eabi-10.3-2021.10-aarch64-linux.tar.bz2", SHA256: "f605b5f23ca898e9b8b665be208510a54a6e9fdd0fa5bfc9592002f6e7431208"},
		"darwin-amd64":  {URL: "https://developer.arm.com/-/media/files/downloads/gnu-rm/10.3-2021.10/gcc-arm-none-eabi-10.3-2021.10-mac.tar.bz2", SHA256: "fb613dacb25149f140f73fe9ff6c380bb43328e6bf813473986e9127e2bc283b"},
	},
	"10.3-2021.07": {
		"windows-amd64": {URL: "https://developer.arm.com/-/media/files/downloads/gnu-rm/10.3-2021.07/gcc-arm-none-eabi-10.3-2021.07-win32.zip", SHA256: "2f4d7410e5b69a643f6ab1de20e1c74dbfd35b06f2b92900cf4160b869bef20f"},
		"linux-amd64":   {URL: "https://developer.arm.com/-/media/files/downloads/gnu-rm/10.3-2021.07/gcc-arm-none-eabi-10.3-2021.07-x86_64-linux.tar.bz2", SHA256: "8c5b8de344e23cd035ca2b53bbf2075c58131ad61223cae48510641d3e556cea"},
		"linux-arm64":   {URL: "https://developer.arm.com/-/media/files/downloads/gnu-rm/10.3-2021.07/gcc-arm-none-eabi-10.3-2021.07-aarch64-linux.tar.bz2", SHA256: "3a75e66541d527f4497f9ea6180cd20b05faf003098a4fc80609afe25cf69678"},
		"darwin-amd64":  {URL: "https://developer.arm.com/-/media/files/downloads/gnu-rm/10.3-2021.07/gcc-arm-none-eabi-10.3-2021.07-mac-10.14.6.tar.bz2", SHA256: "0a4554b248a1626496eeba56ad59d2bba4279cb485099f820bb887fe6a8b7ee4"},
	},
	"10-2020-q4-major": {
		"windows-amd64": {URL: "https://developer.arm.com/-/media/files/downloads/gnu-rm/10-2020q4/gcc-arm-none-eabi-10-2020-q4-major-win32.zip", SHA256: "90057b8737b888c53ca5aee332f1f73c401d6d3873124d2c2906df4347ebef9e"},
		"linux-amd64":   {URL: "https://developer.arm.com/-/media/files/downloads/gnu-rm/10-2020q4/gcc-arm-none-eabi-10-2020-q4-major-x86_64-linux.tar.bz2", SHA256: "21134caa478bbf5352e239fbc6e2da3038f8d2207e089efc96c3b55f1edcd618"},
		"linux-arm64":   {URL: "https://developer.arm.com/-/media/files/downloads/gnu-rm/10-2020q4/gcc-arm-none-eabi-10-2020-q4-major-aarch64-linux.tar.bz2", SHA256: "343d8c812934fe5a904c73583a91edd812b1ac20636eb52de04135bb0f5cf36a"},
		"darwin-amd64":  {URL: "https://developer.arm.com/-/media/files/downloads/gnu-rm/10-2020q4/gcc-arm-none-eabi-10-2020-q4-major-mac.tar.bz2", SHA256: "bed12de3565d4eb02e7b58be945376eaca79a8ae3ebb785ec7344e7e2db0bdc0"},
	},
	"10-2020-q2-preview": {
		"windows-amd64": {URL: "https://developer.arm.com/-/media/files/downloads/gnu-rm/10-2020q2/gcc-arm-none-eabi-10-2020-q2-preview-win32.zip", SHA256: "61834cc61e13c96531ea468189739256d1613015938ecbd9c6f09ed8df9e8159"},
		"linux-amd64":   {URL: "https://developer.arm.com/-/media/files/downloads/gnu-rm/10-2020q2/gcc-arm-none-eabi-10-2020-q2-preview-x86_64-linux.tar.bz2", SHA256: "300a8e782fa7a2c93bed8fde9cf42054edc06a98afba4bdf8046ccd6d1304299"},
		"linux-arm64":   {URL: "https://developer.arm.com/-/media/files/downloads/gnu-rm/10-2020q2/gcc-arm-none-eabi-10-2020-q2-preview-aarch64-linux.tar.bz2", SHA256: "526213c23f81bd60a7d4e546a77911cbf0ee28c4080185780d941d81d8fb02ee"},
		"darwin-amd64":  {URL: "https://developer.arm.com/-/media/files/downloads/gnu-rm/10-2020q2/gcc-arm-none-eabi-10-2020-q2-preview-mac.tar.bz2", SHA256: "bdf45397a01f7184d2fec71e8c3efe2bb435fc5399d3cf1d682ca52fd5e33c75"},
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
	"6-2017-q2-update": {
		"windows-amd64": {URL: "https://developer.arm.com/-/media/files/downloads/gnu-rm/6-2017q2/gcc-arm-none-eabi-6-2017-q2-update-win32.zip", SHA256: "615f9c44d3fba555e1a794b244d483575c45177b70da632dcd6e749ba5741826"},
		"linux-amd64":   {URL: "https://developer.arm.com/-/media/files/downloads/gnu-rm/6-2017q2/gcc-arm-none-eabi-6-2017-q2-update-linux.tar.bz2", SHA256: "e68e4b2fe348ecb567c27985355dff75b65319a0f6595d44a18a8c5e05887cc3"},
		"darwin-amd64":  {URL: "https://developer.arm.com/-/media/files/downloads/gnu-rm/6-2017q2/gcc-arm-none-eabi-6-2017-q2-update-mac.tar.bz2", SHA256: "7d3080514a2899d05fc55466cdc477e2448b6a62f536ffca3dd846822ff52900"},
	},
	"6-2017-q1-update": {
		"windows-amd64": {URL: "https://developer.arm.com/-/media/files/downloads/gnu-rm/6_1-2017q1/gcc-arm-none-eabi-6-2017-q1-update-win32-zip.zip", SHA256: "05b857cfce2936edfbdf54356dae28f916b98ac98fa87040bd5d342b9e313a9b"},
		"linux-amd64":   {URL: "https://developer.arm.com/-/media/files/downloads/gnu-rm/6_1-2017q1/gcc-arm-none-eabi-6-2017-q1-update-linux.tar.bz2", SHA256: "e7aad2579f02e3b095c6d7899ca5e6a70cfa9b8a8cbd6abd868da849d416c2eb"},
		"darwin-amd64":  {URL: "https://developer.arm.com/-/media/files/downloads/gnu-rm/6_1-2017q1/gcc-arm-none-eabi-6-2017-q1-update-mac.tar.bz2", SHA256: "de4de95b09740272aa95ca5a43bb234ba29c323eddcad2ee34e901eebda910a2"},
	},
	"6-2016-q4-major": {
		"windows-amd64": {URL: "https://developer.arm.com/-/media/files/downloads/gnu-rm/6-2016q4/gcc-arm-none-eabi-6_2-2016q4-20161216-win32-zip.zip", SHA256: "2984c4a6db7f51bb19be1027aa13f1fdf46367d6b36b77ea727329a29f1b78f4"},
		"linux-amd64":   {URL: "https://developer.arm.com/-/media/files/downloads/gnu-rm/6-2016q4/gcc-arm-none-eabi-6_2-2016q4-20161216-linux.tar.bz2", SHA256: "2cb3515290ab31ec95e035bae6db37f64e422a61dd04ffaaf11c50e65b403353"},
		"darwin-amd64":  {URL: "https://developer.arm.com/-/media/files/downloads/gnu-rm/6-2016q4/gcc-arm-none-eabi-6_2-2016q4-20161216-mac.tar.bz2", SHA256: "cb52433610d0084ee85abcd1ac4879303acba0b6a4ecfe5a5113c09f0ee265f0"},
	},
	"5-2016-q3-update": {
		"windows-amd64": {URL: "https://developer.arm.com/-/media/files/downloads/gnu-rm/5_4-2016q3/gcc-arm-none-eabi-5_4-2016q3-20160926-win32-zip.zip", SHA256: "c6ad9c000460c3ee98c59552736f23c04510966ab29c53f963ba890d1ebd5905"},
		"linux-amd64":   {URL: "https://developer.arm.com/-/media/files/downloads/gnu-rm/5_4-2016q3/gcc-arm-none-eabi-5_4-2016q3-20160926-linux.tar.bz2", SHA256: "a397c49bdd0cf17a38a494cb691baf68b8dcffa4d4c06561ef3d71b2ab4c92a1"},
		"darwin-amd64":  {URL: "https://developer.arm.com/-/media/files/downloads/gnu-rm/5_4-2016q3/gcc-arm-none-eabi-5_4-2016q3-20160926-mac.tar.bz2", SHA256: "5656cdec40f99d5c054a85bbc694276e1c4a1488cdacbbc448bc6acd3bbe070d"},
	},
	"5-2016-q2-update": {
		"linux-amd64":  {URL: "https://developer.arm.com/-/media/files/downloads/gnu-rm/5_4-2016q2/gccarmnoneeabi542016q220160622linuxtar.bz2?revision=8f445a99-c1ae-4ed8-9eb8-f41929a671c4", SHA256: "9910b6b5df12efe564dbb3856bf1599d4c16178a6f28bd8a23c9e5c3edc219e4"},
		"darwin-amd64": {URL: "https://developer.arm.com/-/media/files/downloads/gnu-rm/5_4-2016q2/gccarmnoneeabi542016q220160622mactar.bz2?revision=03ae9f41-1f43-40ed-9db8-a4b6342378ac", SHA256: "e175a0eb7ee312013d9078a5031a14bf14dff82c7e697549f04b22e6084264c8"},
	},
	"5-2016-q1-update": {
		"linux-amd64":  {URL: "https://developer.arm.com/-/media/files/downloads/gnu-rm/5_3-2016q1/gccarmnoneeabi532016q120160330linuxtar.bz2?revision=417e2623-c259-4a12-aacc-c87200b569c7", SHA256: "217850b0f3297014e8e52010aa52da0a83a073ddec4dc49b1a747458c5d6a223"},
		"darwin-amd64": {URL: "https://developer.arm.com/-/media/files/downloads/gnu-rm/5_3-2016q1/gccarmnoneeabi532016q120160330mactar.bz2?revision=f8c9bfa2-ae89-470c-a845-5fa423439b47", SHA256: "480843ca1ce2d3602307f760872580e999acea0e34ec3d6f8b6ad02d3db409cf"},
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
