package shell

import "errors"

var ErrUnsupported = errors.New("shell integration not supported on this platform yet")

const (
	BeginMarker = "# >>> armup >>>"
	EndMarker   = "# <<< armup <<<"
)
