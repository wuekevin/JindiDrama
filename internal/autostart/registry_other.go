//go:build !windows

package autostart

func windowsAutostartEnabled(string) (bool, error) {
	return false, ErrUnsupported
}

func setWindowsAutostart(string, bool) error {
	return ErrUnsupported
}
