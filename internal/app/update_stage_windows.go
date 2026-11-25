package app

import (
	"crypto/rand"
	"errors"
	"io"
	"os"
	"path/filepath"
	"unsafe"

	"golang.org/x/sys/windows"
)

func createUpdateStage(parent string) (*stagedUpdate, error) {
	user, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		return nil, err
	}
	descriptor, err := windows.SecurityDescriptorFromString("D:P(A;OICI;FA;;;SY)(A;OICI;FA;;;" + user.User.Sid.String() + ")")
	if err != nil {
		return nil, err
	}
	security := windows.SecurityAttributes{SecurityDescriptor: descriptor}
	security.Length = uint32(unsafe.Sizeof(security))
	dir := filepath.Join(parent, ".jindidrama-update-"+rand.Text())
	directoryPath, err := exportWindowsPath(dir)
	if err != nil {
		return nil, err
	}
	if err := windows.CreateDirectory(directoryPath, &security); err != nil {
		return nil, err
	}
	name := filepath.Join(dir, "package")
	path, err := exportWindowsPath(name)
	if err != nil {
		_ = os.Remove(dir)
		return nil, err
	}
	// Exclude other writers and pathname deletion while this handle is open.
	handle, err := windows.CreateFile(path, windows.GENERIC_WRITE|windows.DELETE, windows.FILE_SHARE_READ, &security,
		windows.CREATE_NEW, windows.FILE_ATTRIBUTE_NORMAL, 0)
	if err != nil {
		_ = os.Remove(dir)
		return nil, err
	}
	file := os.NewFile(uintptr(handle), name)
	published := false
	cleanup := func() {
		if !published {
			deleteFile := byte(1)
			_ = windows.SetFileInformationByHandle(handle, windows.FileDispositionInfo, &deleteFile, 1)
		}
		_ = file.Close()
		_ = os.Remove(dir)
	}
	if err := markUpdateZone(handle, name); err != nil {
		cleanup()
		return nil, errUpdateOrigin
	}
	return &stagedUpdate{
		file: file,
		publish: func(target string) error {
			if err := renameUpdateHandle(handle, target); err != nil {
				return err
			}
			published = true
			return nil
		},
		cleanup: cleanup,
	}, nil
}

func markUpdateZone(source windows.Handle, name string) error {
	markName := name + ":Zone.Identifier"
	path, err := exportWindowsPath(markName)
	if err != nil {
		return err
	}
	const shares = windows.FILE_SHARE_READ | windows.FILE_SHARE_WRITE | windows.FILE_SHARE_DELETE
	mark, err := windows.CreateFile(path, windows.GENERIC_WRITE, shares, nil, windows.CREATE_NEW, windows.FILE_ATTRIBUTE_NORMAL, 0)
	if err != nil {
		return err
	}
	stream := os.NewFile(uintptr(mark), markName)
	defer stream.Close()
	var file, zone windows.ByHandleFileInformation
	if err := windows.GetFileInformationByHandle(source, &file); err != nil {
		return err
	}
	if err := windows.GetFileInformationByHandle(mark, &zone); err != nil {
		return err
	}
	if file.VolumeSerialNumber != zone.VolumeSerialNumber || file.FileIndexHigh != zone.FileIndexHigh || file.FileIndexLow != zone.FileIndexLow {
		return errors.New("update zone stream is on a different file")
	}
	if _, err := io.WriteString(stream, "[ZoneTransfer]\r\nZoneId=3\r\n"); err != nil {
		return err
	}
	return stream.Sync()
}

func renameUpdateHandle(handle windows.Handle, target string) error {
	fullPath, err := exportWindowsPath(target)
	if err != nil {
		return err
	}
	path, err := windows.UTF16FromString(windows.UTF16PtrToString(fullPath))
	if err != nil {
		return err
	}
	// FILE_RENAME_INFO contains a variable-length UTF-16 filename. The zero
	// flags select no replacement, and the null directory uses the full path.
	type renameInfo struct {
		Flags          uint32
		RootDirectory  windows.Handle
		FileNameLength uint32
		FileName       [1]uint16
	}
	var layout renameInfo
	offset := int(unsafe.Offsetof(layout.FileName))
	buffer := make([]byte, offset+2*len(path))
	info := (*renameInfo)(unsafe.Pointer(&buffer[0]))
	info.FileNameLength = uint32(2 * (len(path) - 1))
	copy(unsafe.Slice(&info.FileName[0], len(path)), path)
	return windows.SetFileInformationByHandle(handle, windows.FileRenameInfo, &buffer[0], uint32(len(buffer)))
}
