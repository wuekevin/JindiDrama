//go:build !darwin

package autostart

func darwinAutostartEnabled(*Manager) (bool, error) {
	return false, ErrUnsupported
}

func setDarwinAutostart(*Manager, bool) error {
	return ErrUnsupported
}
