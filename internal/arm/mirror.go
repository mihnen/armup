package arm

import "strings"

// upstreamPrefix is the developer.arm.com prefix that mirror-aware
// install replaces with the user's configured mirror.
const upstreamPrefix = "https://developer.arm.com"

// ApplyMirror returns `url` rewritten to come from `mirror` instead of
// developer.arm.com. The mirror is expected to expose ARM's URL path
// structure under its own base (a plain `wget --mirror`, an rsync of
// /-/media/files/..., a directory served by `python -m http.server`,
// etc.).
//
// Empty mirror returns url unchanged. URLs that don't start with the
// developer.arm.com prefix are passed through untouched — the caller
// already handed us something we don't own. Query strings (used by
// ARM's pre-2017 mangled URLs to feed Akamai a routing rev=<uuid>)
// are stripped: the mirror serves the file by name alone.
func ApplyMirror(url, mirror string) string {
	if mirror == "" {
		return url
	}
	if !strings.HasPrefix(url, upstreamPrefix) {
		return url
	}
	rest := strings.TrimPrefix(url, upstreamPrefix)
	if i := strings.IndexByte(rest, '?'); i >= 0 {
		rest = rest[:i]
	}
	return strings.TrimRight(mirror, "/") + rest
}
