//go:build (!darwin && !windows) || (darwin && !cgo)

package secrets

func openNative(_ string) (Vault, error) {
	return nil, ErrUnavailable
}
