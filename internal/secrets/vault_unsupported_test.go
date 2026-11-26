//go:build (!darwin && !windows) || (darwin && !cgo)

package secrets

import (
	"errors"
	"testing"
)

func TestNativeVaultUnavailable(t *testing.T) {
	if vault, err := Open(t.TempDir()); vault != nil || !errors.Is(err, ErrUnavailable) {
		t.Fatalf("unsupported platform opened a credential vault: %v", err)
	}
}
