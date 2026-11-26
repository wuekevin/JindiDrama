package secrets

import (
	"bytes"
	"crypto/rand"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestOpenRequiresExistingAbsoluteProfile(t *testing.T) {
	if _, err := Open("relative"); !errors.Is(err, ErrInvalid) {
		t.Fatalf("relative profile: %v", err)
	}
	profile := t.TempDir()
	if _, err := Open(filepath.Join(profile, "missing")); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("missing profile: %v", err)
	}
	file := filepath.Join(profile, "file")
	if err := os.WriteFile(file, nil, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := Open(file); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("file as profile: %v", err)
	}
}

func TestNativeVault(t *testing.T) {
	if runtime.GOOS == "darwin" && os.Getenv("JINDI_TEST_NATIVE_VAULT") != "1" {
		t.Skip("set JINDI_TEST_NATIVE_VAULT=1 for a temporary local Keychain entry")
	}
	if runtime.GOOS != "darwin" && runtime.GOOS != "windows" {
		t.Skip("no supported system credential store")
	}
	parent := t.TempDir()
	profile := filepath.Join(parent, "Profile")
	if err := os.Mkdir(profile, 0700); err != nil {
		t.Fatal(err)
	}
	vault := openTestVault(t, profile)
	otherProfile := openTestVault(t, t.TempDir())
	credential := make([]byte, 64)
	rand.Read(credential)
	const provider = "aux"
	t.Cleanup(func() {
		if err := vault.Delete(provider); err != nil {
			t.Errorf("remove temporary credential: %v", err)
		}
	})

	for _, invalid := range []string{"", "../provider", "Provider", "a/b", "provider\x00hidden"} {
		_, loadErr := vault.Load(invalid)
		if !errors.Is(loadErr, ErrInvalid) || !errors.Is(vault.Save(invalid, credential), ErrInvalid) ||
			!errors.Is(vault.Delete(invalid), ErrInvalid) {
			t.Fatal("invalid provider accepted")
		}
	}
	if !errors.Is(vault.Save(provider, nil), ErrInvalid) ||
		!errors.Is(vault.Save(provider, make([]byte, maxCredentialBytes+1)), ErrInvalid) {
		t.Fatal("invalid credential size accepted")
	}
	assertMissing(t, vault, provider)
	if err := vault.Delete(provider); err != nil {
		t.Fatalf("delete absent credential: %v", err)
	}
	if err := vault.Save(provider, credential); err != nil {
		t.Fatalf("save credential: %v", err)
	}
	reopened := openTestVault(t, profile)
	assertCredential(t, reopened, provider, credential)
	testProfilePathAliases(t, parent, provider, credential)
	assertMissing(t, reopened, "other-provider")
	assertMissing(t, otherProfile, provider)

	replacement := make([]byte, maxCredentialBytes)
	rand.Read(replacement)
	if err := reopened.Save(provider, replacement); err != nil {
		t.Fatalf("replace credential: %v", err)
	}
	assertCredential(t, vault, provider, replacement)
	if err := reopened.Delete(provider); err != nil {
		t.Fatalf("delete credential: %v", err)
	}
	assertMissing(t, vault, provider)
}

func openTestVault(t *testing.T, path string) Vault {
	t.Helper()
	vault, err := Open(path)
	if err != nil {
		t.Fatalf("open vault: %v", err)
	}
	return vault
}

func assertCredential(t *testing.T, vault Vault, provider string, want []byte) {
	t.Helper()
	got, err := vault.Load(provider)
	if err != nil || !bytes.Equal(got, want) {
		t.Fatalf("read credential of expected length and contents: %v", err)
	}
}

func assertMissing(t *testing.T, vault Vault, provider string) {
	t.Helper()
	data, err := vault.Load(provider)
	if data != nil || !errors.Is(err, ErrMissing) {
		t.Fatalf("want missing credential: %v", err)
	}
}

func testProfilePathAliases(t *testing.T, parent, provider string, credential []byte) {
	t.Helper()
	if runtime.GOOS == "darwin" {
		alias := filepath.Join(parent, "alias")
		if err := os.Symlink(filepath.Join(parent, "Profile"), alias); err != nil {
			t.Fatal(err)
		}
		assertCredential(t, openTestVault(t, alias), provider, credential)
	}

	alternateCase := filepath.Join(parent, "profile")
	_, err := os.Stat(alternateCase)
	if err == nil {
		assertCredential(t, openTestVault(t, alternateCase), provider, credential)
		return
	}
	if !errors.Is(err, fs.ErrNotExist) {
		t.Fatal(err)
	}
	if err := os.Mkdir(alternateCase, 0700); err != nil {
		t.Fatal(err)
	}
	assertMissing(t, openTestVault(t, alternateCase), provider)
}
