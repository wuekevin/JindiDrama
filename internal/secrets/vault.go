// Package secrets stores opaque provider credentials in the operating system's
// per-user credential storage, independently of the application database.
package secrets

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"regexp"
)

const maxCredentialBytes = 64 * 1024

var (
	ErrMissing     = errors.New("credential not found")
	ErrInvalid     = errors.New("invalid credential or profile")
	ErrUnavailable = errors.New("system credential storage unavailable")
	providerID     = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,62}$`)
)

type Vault interface {
	Load(provider string) ([]byte, error)
	Save(provider string, credential []byte) error
	// Delete succeeds if the credential is already absent.
	Delete(provider string) error
}

// Open scopes the vault to an existing absolute application profile directory.
func Open(profileDir string) (Vault, error) {
	if !filepath.IsAbs(profileDir) {
		return nil, ErrInvalid
	}
	resolved, err := filepath.EvalSymlinks(profileDir)
	if err != nil {
		return nil, ErrUnavailable
	}
	info, err := os.Stat(resolved)
	if err != nil || !info.IsDir() {
		return nil, ErrUnavailable
	}
	return openNative(resolved)
}

func profileScope(path string) string {
	digest := sha256.Sum256([]byte(path))
	return "com.jindidrama.account.v1." + hex.EncodeToString(digest[:])
}

func validCredential(data []byte) bool {
	return len(data) > 0 && len(data) <= maxCredentialBytes
}
