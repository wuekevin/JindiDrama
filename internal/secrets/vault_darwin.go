//go:build darwin && cgo

package secrets

/*
#cgo LDFLAGS: -framework CoreFoundation -framework Security
#include <Security/Security.h>
#include <fcntl.h>
#include <limits.h>
#include <stdlib.h>

// The file-based Keychain still uses the process interaction setting.
#pragma clang diagnostic push
#pragma clang diagnostic ignored "-Wdeprecated-declarations"
static OSStatus getCredentialInteraction(Boolean *allowed) {
	return SecKeychainGetUserInteractionAllowed(allowed);
}

static OSStatus setCredentialInteraction(Boolean allowed) {
	return SecKeychainSetUserInteractionAllowed(allowed);
}
#pragma clang diagnostic pop

static int getProfilePath(int descriptor, char *path) {
	return fcntl(descriptor, F_GETPATH, path);
}

static CFMutableDictionaryRef credentialQuery(const char *service, const char *provider) {
	CFMutableDictionaryRef query = CFDictionaryCreateMutable(
		NULL, 0, &kCFTypeDictionaryKeyCallBacks, &kCFTypeDictionaryValueCallBacks);
	CFStringRef scope = CFStringCreateWithCString(NULL, service, kCFStringEncodingUTF8);
	CFStringRef account = CFStringCreateWithCString(NULL, provider, kCFStringEncodingUTF8);
	if (query && scope && account) {
		CFDictionarySetValue(query, kSecClass, kSecClassGenericPassword);
		CFDictionarySetValue(query, kSecAttrService, scope);
		CFDictionarySetValue(query, kSecAttrAccount, account);
		CFDictionarySetValue(query, kSecAttrSynchronizable, kCFBooleanFalse);
		CFDictionarySetValue(query, kSecUseAuthenticationUI, kSecUseAuthenticationUIFail);
	} else if (query) {
		CFRelease(query);
		query = NULL;
	}
	if (scope) CFRelease(scope);
	if (account) CFRelease(account);
	return query;
}

static OSStatus readCredential(CFMutableDictionaryRef query, CFTypeRef *result) {
	CFDictionarySetValue(query, kSecMatchLimit, kSecMatchLimitOne);
	CFDictionarySetValue(query, kSecReturnData, kCFBooleanTrue);
	return SecItemCopyMatching(query, result);
}

static OSStatus writeCredential(CFMutableDictionaryRef query, const UInt8 *bytes, CFIndex length) {
	CFDataRef data = CFDataCreate(NULL, bytes, length);
	if (!data) return errSecAllocate;
	const void *key = kSecValueData;
	const void *value = data;
	CFDictionaryRef update = CFDictionaryCreate(
		NULL, &key, &value, 1, &kCFTypeDictionaryKeyCallBacks, &kCFTypeDictionaryValueCallBacks);
	if (!update) {
		CFRelease(data);
		return errSecAllocate;
	}
	OSStatus status = SecItemUpdate(query, update);
	if (status == errSecItemNotFound) {
		CFDictionarySetValue(query, kSecValueData, data);
		status = SecItemAdd(query, NULL);
		CFDictionaryRemoveValue(query, kSecValueData);
		if (status == errSecDuplicateItem) status = SecItemUpdate(query, update);
	}
	CFRelease(update);
	CFRelease(data);
	return status;
}
*/
import "C"

import (
	"os"
	"sync"
	"unsafe"
)

var keychainInteractionMu sync.Mutex

func keychainInteractionAllowed() (bool, error) {
	var allowed C.Boolean
	if C.getCredentialInteraction(&allowed) != C.errSecSuccess {
		return false, ErrUnavailable
	}
	return allowed != 0, nil
}

func setKeychainInteraction(allowed bool) error {
	var flag C.Boolean
	if allowed {
		flag = 1
	}
	if C.setCredentialInteraction(flag) != C.errSecSuccess {
		return ErrUnavailable
	}
	return nil
}

func withoutKeychainInteraction(run func()) (err error) {
	keychainInteractionMu.Lock()
	defer keychainInteractionMu.Unlock()
	allowed, err := keychainInteractionAllowed()
	if err != nil {
		return err
	}
	if allowed {
		if err := setKeychainInteraction(false); err != nil {
			return err
		}
		defer func() { err = setKeychainInteraction(true) }()
	}
	run()
	return nil
}

type keychainVault struct {
	service string
}

func openNative(profile string) (Vault, error) {
	dir, err := os.Open(profile)
	if err != nil {
		return nil, ErrUnavailable
	}
	defer dir.Close()
	// Darwin EvalSymlinks preserves caller casing in regular path components.
	// Ask the filesystem for the opened directory's path.
	var path [C.PATH_MAX]C.char
	if C.getProfilePath(C.int(dir.Fd()), &path[0]) != 0 || path[0] == 0 {
		return nil, ErrUnavailable
	}
	return keychainVault{service: profileScope(C.GoString(&path[0]))}, nil
}

func (v keychainVault) Load(provider string) ([]byte, error) {
	query, err := v.query(provider)
	if err != nil {
		return nil, err
	}
	defer C.CFRelease(C.CFTypeRef(query))

	var result C.CFTypeRef
	var status C.OSStatus
	err = withoutKeychainInteraction(func() { status = C.readCredential(query, &result) })
	if result != 0 {
		defer C.CFRelease(result)
	}
	if err != nil {
		return nil, err
	}
	if status == C.errSecItemNotFound {
		return nil, ErrMissing
	}
	if status != C.errSecSuccess || result == 0 || C.CFGetTypeID(result) != C.CFDataGetTypeID() {
		return nil, ErrUnavailable
	}
	data := C.CFDataRef(result)
	size := C.CFDataGetLength(data)
	if size <= 0 || size > maxCredentialBytes {
		return nil, ErrUnavailable
	}
	return C.GoBytes(unsafe.Pointer(C.CFDataGetBytePtr(data)), C.int(size)), nil
}

func (v keychainVault) Save(provider string, credential []byte) error {
	if !validCredential(credential) {
		return ErrInvalid
	}
	query, err := v.query(provider)
	if err != nil {
		return err
	}
	defer C.CFRelease(C.CFTypeRef(query))
	var status C.OSStatus
	if err := withoutKeychainInteraction(func() {
		status = C.writeCredential(query, (*C.UInt8)(unsafe.Pointer(unsafe.SliceData(credential))), C.CFIndex(len(credential)))
	}); err != nil {
		return err
	}
	if status != C.errSecSuccess {
		return ErrUnavailable
	}
	return nil
}

func (v keychainVault) Delete(provider string) error {
	query, err := v.query(provider)
	if err != nil {
		return err
	}
	defer C.CFRelease(C.CFTypeRef(query))
	var status C.OSStatus
	if err := withoutKeychainInteraction(func() { status = C.SecItemDelete(C.CFDictionaryRef(query)) }); err != nil {
		return err
	}
	if status != C.errSecSuccess && status != C.errSecItemNotFound {
		return ErrUnavailable
	}
	return nil
}

func (v keychainVault) query(provider string) (C.CFMutableDictionaryRef, error) {
	if !providerID.MatchString(provider) {
		return 0, ErrInvalid
	}
	service := C.CString(v.service)
	defer C.free(unsafe.Pointer(service))
	account := C.CString(provider)
	defer C.free(unsafe.Pointer(account))
	query := C.credentialQuery(service, account)
	if query == 0 {
		return 0, ErrUnavailable
	}
	return query, nil
}
