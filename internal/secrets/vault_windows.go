package secrets

import (
	"crypto/rand"
	"crypto/sha256"
	"errors"
	"io"
	"io/fs"
	"os"
	"runtime"
	"unsafe"

	"golang.org/x/sys/windows"
)

const maxCiphertextBytes = 128 * 1024

type dpapiVault struct {
	profile string
	scope   string
}

func openNative(profile string) (Vault, error) {
	return dpapiVault{profile: profile, scope: profileScope(profile)}, nil
}

func (v dpapiVault) Load(provider string) ([]byte, error) {
	if !providerID.MatchString(provider) {
		return nil, ErrInvalid
	}
	dir, err := v.openDirectory(false)
	if err != nil {
		return nil, err
	}
	defer dir.Close()

	name := "provider-" + provider + ".dpapi"
	info, err := dir.Lstat(name)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, ErrMissing
	}
	if err != nil || !info.Mode().IsRegular() {
		return nil, ErrUnavailable
	}
	file, err := dir.Open(name)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, ErrMissing
	}
	if err != nil {
		return nil, ErrUnavailable
	}
	defer file.Close()
	actual, err := file.Stat()
	if err != nil || !actual.Mode().IsRegular() {
		return nil, ErrUnavailable
	}
	ciphertext, err := io.ReadAll(io.LimitReader(file, maxCiphertextBytes+1))
	if err != nil || len(ciphertext) == 0 || len(ciphertext) > maxCiphertextBytes {
		return nil, ErrUnavailable
	}
	return v.transform(provider, ciphertext, true)
}

func (v dpapiVault) Save(provider string, credential []byte) error {
	if !providerID.MatchString(provider) || !validCredential(credential) {
		return ErrInvalid
	}
	ciphertext, err := v.transform(provider, credential, false)
	if err != nil {
		return err
	}
	dir, err := v.openDirectory(true)
	if err != nil {
		return err
	}
	defer dir.Close()

	tempName := "." + rand.Text() + ".tmp"
	file, err := dir.OpenFile(tempName, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return ErrUnavailable
	}
	defer dir.Remove(tempName)
	_, writeErr := file.Write(ciphertext)
	syncErr := file.Sync()
	closeErr := file.Close()
	if writeErr != nil || syncErr != nil || closeErr != nil {
		return ErrUnavailable
	}
	if err := dir.Rename(tempName, "provider-"+provider+".dpapi"); err != nil {
		return ErrUnavailable
	}
	return nil
}

func (v dpapiVault) Delete(provider string) error {
	if !providerID.MatchString(provider) {
		return ErrInvalid
	}
	dir, err := v.openDirectory(false)
	if errors.Is(err, ErrMissing) {
		return nil
	}
	if err != nil {
		return err
	}
	defer dir.Close()
	if err := dir.Remove("provider-" + provider + ".dpapi"); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return ErrUnavailable
	}
	return nil
}

func (v dpapiVault) openDirectory(create bool) (*os.Root, error) {
	profile, err := os.OpenRoot(v.profile)
	if err != nil {
		return nil, ErrUnavailable
	}
	defer profile.Close()
	const name = "secrets"
	if create {
		if err := profile.Mkdir(name, 0700); err != nil && !errors.Is(err, fs.ErrExist) {
			return nil, ErrUnavailable
		}
	}
	info, err := profile.Lstat(name)
	if errors.Is(err, fs.ErrNotExist) && !create {
		return nil, ErrMissing
	}
	if err != nil || !info.IsDir() {
		return nil, ErrUnavailable
	}
	dir, err := profile.OpenRoot(name)
	if err != nil {
		return nil, ErrUnavailable
	}
	actual, err := dir.Stat(".")
	if err != nil || !os.SameFile(info, actual) {
		dir.Close()
		return nil, ErrUnavailable
	}
	return dir, nil
}

func (v dpapiVault) transform(provider string, data []byte, decrypt bool) ([]byte, error) {
	entropy := sha256.Sum256([]byte(v.scope + "\x00" + provider))
	input := windows.DataBlob{Size: uint32(len(data)), Data: unsafe.SliceData(data)}
	salt := windows.DataBlob{Size: uint32(len(entropy)), Data: &entropy[0]}
	var output windows.DataBlob
	var err error
	limit := uint32(maxCiphertextBytes)
	if decrypt {
		limit = maxCredentialBytes
		err = windows.CryptUnprotectData(&input, nil, &salt, 0, nil, windows.CRYPTPROTECT_UI_FORBIDDEN, &output)
	} else {
		err = windows.CryptProtectData(&input, nil, &salt, 0, nil, windows.CRYPTPROTECT_UI_FORBIDDEN, &output)
	}
	runtime.KeepAlive(data)
	runtime.KeepAlive(&entropy)
	if output.Data != nil {
		defer windows.LocalFree(windows.Handle(uintptr(unsafe.Pointer(output.Data))))
	}
	if err != nil || output.Data == nil || output.Size == 0 || output.Size > limit {
		return nil, ErrUnavailable
	}
	bytes := unsafe.Slice(output.Data, int(output.Size))
	if decrypt {
		defer clear(bytes)
	}
	return append([]byte(nil), bytes...), nil
}
