package secrets

import (
	"bytes"
	"crypto/rand"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestDPAPIProfileAndProviderBinding(t *testing.T) {
	profile := t.TempDir()
	otherProfile := t.TempDir()
	vault := openTestVault(t, profile)
	other := openTestVault(t, otherProfile)
	credential := make([]byte, 64)
	rand.Read(credential)
	if err := vault.Save("catalog", credential); err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(profile, "secrets")
	path := filepath.Join(dir, "provider-catalog.dpapi")
	ciphertext, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(ciphertext, credential) {
		t.Fatal("on-disk data contains the plaintext credential")
	}
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) != 1 {
		t.Fatalf("unexpected credential directory entries: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "provider-media.dpapi"), ciphertext, 0600); err != nil {
		t.Fatal(err)
	}
	assertUnavailable(t, vault, "media")

	otherDir := filepath.Join(otherProfile, "secrets")
	if err := os.Mkdir(otherDir, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(otherDir, "provider-catalog.dpapi"), ciphertext, 0600); err != nil {
		t.Fatal(err)
	}
	assertUnavailable(t, other, "catalog")

	if err := os.WriteFile(path, make([]byte, maxCiphertextBytes+1), 0600); err != nil {
		t.Fatal(err)
	}
	assertUnavailable(t, vault, "catalog")
}

func assertUnavailable(t *testing.T, vault Vault, provider string) {
	t.Helper()
	if data, err := vault.Load(provider); data != nil || !errors.Is(err, ErrUnavailable) {
		t.Fatalf("want unavailable credential: %v", err)
	}
}
