package fileprivacy

import (
	"errors"
	"os"

	"golang.org/x/sys/windows"
)

// WindowsDirectoryDescriptor permits inheritance by the same user and SYSTEM.
func WindowsDirectoryDescriptor() (*windows.SECURITY_DESCRIPTOR, error) {
	user, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		return nil, err
	}
	return windows.SecurityDescriptorFromString("D:P(A;OICI;FA;;;SY)(A;OICI;FA;;;" + user.User.Sid.String() + ")")
}

// ProtectDirectory applies the protected DACL to the same directory as root.
func ProtectDirectory(root *os.Root) error {
	expected, err := root.Stat(".")
	if err != nil {
		return err
	}
	path, err := WindowsPath(root.Name())
	if err != nil {
		return err
	}
	const shares = windows.FILE_SHARE_READ | windows.FILE_SHARE_WRITE | windows.FILE_SHARE_DELETE
	const flags = windows.FILE_FLAG_BACKUP_SEMANTICS | windows.FILE_FLAG_OPEN_REPARSE_POINT
	handle, err := windows.CreateFile(path, windows.WRITE_DAC|windows.FILE_READ_ATTRIBUTES, shares, nil, windows.OPEN_EXISTING, flags, 0)
	if err != nil {
		return err
	}
	dir := os.NewFile(uintptr(handle), root.Name())
	defer dir.Close()
	actual, err := dir.Stat()
	if err != nil {
		return err
	}
	if !actual.IsDir() || !os.SameFile(expected, actual) {
		return errors.New("staging directory changed")
	}
	descriptor, err := WindowsDirectoryDescriptor()
	if err != nil {
		return err
	}
	dacl, _, err := descriptor.DACL()
	if err != nil {
		return err
	}
	return windows.SetSecurityInfo(handle, windows.SE_FILE_OBJECT,
		windows.DACL_SECURITY_INFORMATION|windows.PROTECTED_DACL_SECURITY_INFORMATION, nil, nil, dacl, nil)
}

// RemoveEmptyDirectory removes an empty directory without deleting a regular file.
func RemoveEmptyDirectory(path string) error {
	name, err := WindowsPath(path)
	if err != nil {
		return err
	}
	return windows.RemoveDirectory(name)
}
