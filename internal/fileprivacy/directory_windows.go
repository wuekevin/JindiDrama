package fileprivacy

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"unsafe"

	"golang.org/x/sys/windows"
)

// WindowsDirectoryDescriptor permits inheritance by the same user and SYSTEM.
func WindowsDirectoryDescriptor() (*windows.SECURITY_DESCRIPTOR, error) {
	return windowsPrivateDescriptor("OICI")
}

// WindowsFileDescriptor restricts a file to the same user and SYSTEM.
func WindowsFileDescriptor() (*windows.SECURITY_DESCRIPTOR, error) {
	return windowsPrivateDescriptor("")
}

func windowsPrivateDescriptor(aceFlags string) (*windows.SECURITY_DESCRIPTOR, error) {
	user, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		return nil, err
	}
	return windows.SecurityDescriptorFromString("D:P(A;" + aceFlags + ";FA;;;SY)(A;" + aceFlags + ";FA;;;" + user.User.Sid.String() + ")")
}

// CreatePrivateDirectory creates and verifies a protected child through the parent handle.
func CreatePrivateDirectory(parent *os.Root, name string) (_ *os.Root, result error) {
	parentDir, err := parent.Open(".")
	if err != nil {
		return nil, err
	}
	defer parentDir.Close()
	descriptor, err := WindowsDirectoryDescriptor()
	if err != nil {
		return nil, err
	}
	const access = windows.DELETE | windows.READ_CONTROL | windows.FILE_READ_ATTRIBUTES
	const shares = windows.FILE_SHARE_READ | windows.FILE_SHARE_WRITE | windows.FILE_SHARE_DELETE
	handle, err := CreateWindowsObject(windows.Handle(parentDir.Fd()), name, access, shares, windows.FILE_DIRECTORY_FILE, descriptor)
	if err != nil {
		return nil, fmt.Errorf("create staging directory with protected DACL: %w", err)
	}
	created := os.NewFile(uintptr(handle), name)
	defer created.Close()
	defer func() {
		if result != nil {
			remove := byte(1)
			err := windows.SetFileInformationByHandle(handle, windows.FileDispositionInfo, &remove, 1)
			result = errors.Join(result, err)
		}
	}()
	if err := VerifyWindowsDACL(handle, descriptor); err != nil {
		return nil, fmt.Errorf("verify staging directory DACL: %w", err)
	}
	expected, err := created.Stat()
	if err != nil {
		return nil, err
	}
	root, err := parent.OpenRoot(name)
	if err != nil {
		return nil, err
	}
	actual, err := root.Stat(".")
	if err != nil || !actual.IsDir() || !os.SameFile(expected, actual) {
		root.Close()
		return nil, errors.Join(errors.New("staging directory changed"), err)
	}
	return root, nil
}

// ErrWindowsDACL means the effective access list differs from the requested list.
var ErrWindowsDACL = errors.New("Windows did not apply the protected DACL")

// VerifyWindowsDACL requires the exact protected access list on the open object.
func VerifyWindowsDACL(handle windows.Handle, expected *windows.SECURITY_DESCRIPTOR) error {
	descriptor, err := windows.GetSecurityInfo(handle, windows.SE_FILE_OBJECT, windows.DACL_SECURITY_INFORMATION)
	if err != nil {
		return err
	}
	control, _, err := descriptor.Control()
	if err != nil {
		return err
	}
	if control&windows.SE_DACL_PROTECTED == 0 {
		return ErrWindowsDACL
	}
	actual, _, err := descriptor.DACL()
	if err != nil {
		return err
	}
	requested, _, err := expected.DACL()
	if err != nil {
		return err
	}
	if actual == nil || requested == nil || actual.AceCount != requested.AceCount {
		return ErrWindowsDACL
	}
	for i := range uint32(requested.AceCount) {
		var found, wanted *windows.ACCESS_ALLOWED_ACE
		if err := windows.GetAce(actual, i, &found); err != nil {
			return err
		}
		if err := windows.GetAce(requested, i, &wanted); err != nil {
			return err
		}
		if found.Header.AceSize != wanted.Header.AceSize ||
			!bytes.Equal(unsafe.Slice((*byte)(unsafe.Pointer(found)), found.Header.AceSize),
				unsafe.Slice((*byte)(unsafe.Pointer(wanted)), wanted.Header.AceSize)) {
			return ErrWindowsDACL
		}
	}
	return nil
}

// CreateWindowsObject creates a single object relative to an already open directory.
func CreateWindowsObject(dir windows.Handle, name string, access, shares, options uint32, descriptor *windows.SECURITY_DESCRIPTOR) (windows.Handle, error) {
	path, err := windows.NewNTUnicodeString(name)
	if err != nil {
		return 0, err
	}
	attrs := windows.OBJECT_ATTRIBUTES{
		RootDirectory: dir, ObjectName: path, Attributes: windows.OBJ_CASE_INSENSITIVE,
		SecurityDescriptor: descriptor,
	}
	attrs.Length = uint32(unsafe.Sizeof(attrs))
	var handle windows.Handle
	err = windows.NtCreateFile(&handle, access|windows.SYNCHRONIZE, &attrs, &windows.IO_STATUS_BLOCK{}, nil,
		windows.FILE_ATTRIBUTE_NORMAL, shares, windows.FILE_CREATE,
		options|windows.FILE_SYNCHRONOUS_IO_NONALERT|windows.FILE_OPEN_REPARSE_POINT, 0, 0)
	if status, ok := err.(windows.NTStatus); ok {
		return 0, status.Errno()
	}
	return handle, err
}

// RemoveEmptyDirectory removes an empty directory without deleting a regular file.
func RemoveEmptyDirectory(path string) error {
	name, err := WindowsPath(path)
	if err != nil {
		return err
	}
	return windows.RemoveDirectory(name)
}
