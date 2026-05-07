//go:build windows

package shell

func EnsureOnPath(dir string) ([]string, error) {
	return nil, ErrUnsupported
}
