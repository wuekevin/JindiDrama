package app

import (
	"crypto/rand"
	"errors"
	"io"
	"os"
	"path/filepath"
	"unsafe"

	"golang.org/x/sys/windows"

	"jindi-drama/internal/fileprivacy"
)

const updateFileShares = windows.FILE_SHARE_READ | windows.FILE_SHARE_WRITE | windows.FILE_SHARE_DELETE

func createOutputStage(parent, prefix string, kind outputStageKind) (*stagedOutput, error) {
	var descriptor *windows.SECURITY_DESCRIPTOR
	if kind != mediaOutputStage {
		var err error
		descriptor, err = fileprivacy.WindowsDirectoryDescriptor()
		if err != nil {
			return nil, err
		}
	}
	// For a directory, FILE_WRITE_DATA and FILE_APPEND_DATA grant creation of
	// files and subdirectories respectively.
	const destinationAccess = windows.FILE_WRITE_DATA | windows.FILE_APPEND_DATA | windows.FILE_READ_ATTRIBUTES | windows.FILE_TRAVERSE
	dest, err := openUpdateDirectory(parent, destinationAccess)
	if err != nil {
		return nil, err
	}

	dirName := prefix + rand.Text()
	const stagingAccess = windows.FILE_WRITE_DATA | windows.FILE_TRAVERSE | windows.DELETE
	dir, err := createUpdateObject(dest, dirName, stagingAccess, updateFileShares, windows.FILE_DIRECTORY_FILE, descriptor)
	if err != nil {
		_ = windows.CloseHandle(dest)
		return nil, err
	}
	closeStageDirectory := func() error {
		return errors.Join(deleteUpdateObject(dir), windows.CloseHandle(dir), windows.CloseHandle(dest))
	}

	const name = "package"
	handle, err := createUpdateObject(dir, name, windows.GENERIC_WRITE|windows.FILE_READ_ATTRIBUTES|windows.DELETE, windows.FILE_SHARE_READ, windows.FILE_NON_DIRECTORY_FILE, nil)
	if err != nil {
		closeStageDirectory()
		return nil, err
	}
	file := os.NewFile(uintptr(handle), filepath.Join(parent, dirName, name))
	published := false
	discarded := false
	discard := func() error {
		if discarded {
			return nil
		}
		discarded = true
		var err error
		if !published {
			err = deleteUpdateObject(handle)
		}
		return errors.Join(err, file.Close())
	}
	cleanup := func() error {
		return errors.Join(discard(), closeStageDirectory())
	}
	if kind == updateOutputStage {
		if err := markUpdateZone(dir, handle, name); err != nil {
			cleanup()
			return nil, errUpdateOrigin
		}
	}
	destinationMatches := func() bool {
		selected, err := openUpdateDirectory(parent, windows.FILE_READ_ATTRIBUTES)
		if err != nil {
			return false
		}
		defer windows.CloseHandle(selected)
		return sameUpdateObject(dest, selected)
	}
	return &stagedOutput{
		file: file,
		publish: func(target string) error {
			return publishInUpdateDestination(destinationMatches, func() error {
				if err := renameUpdateHandle(handle, dest, filepath.Base(target)); err != nil {
					return err
				}
				published = true
				return nil
			})
		},
		cleanup: cleanup,
		copyDirect: func(target string, write func(io.Writer) error) error {
			return publishInUpdateDestination(destinationMatches, func() error {
				if err := discard(); err != nil {
					return err
				}
				name := filepath.Base(target)
				if !filepath.IsLocal(name) {
					return windows.ERROR_INVALID_NAME
				}
				handle, err := createUpdateObject(dest, name, windows.GENERIC_WRITE, windows.FILE_SHARE_READ, windows.FILE_NON_DIRECTORY_FILE, nil)
				if err != nil {
					return err
				}
				return completeDirectExport(os.NewFile(uintptr(handle), target), write)
			})
		},
	}, nil
}

func openUpdateDirectory(path string, access uint32) (windows.Handle, error) {
	name, err := fileprivacy.WindowsPath(path)
	if err != nil {
		return 0, err
	}
	return windows.CreateFile(name, access, updateFileShares, nil, windows.OPEN_EXISTING, windows.FILE_FLAG_BACKUP_SEMANTICS, 0)
}

func createUpdateObject(dir windows.Handle, name string, access, shares, options uint32, descriptor *windows.SECURITY_DESCRIPTOR) (windows.Handle, error) {
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
	return handle, updateWindowsError(err)
}

func updateWindowsError(err error) error {
	if status, ok := err.(windows.NTStatus); ok {
		return status.Errno()
	}
	return err
}

func deleteUpdateObject(handle windows.Handle) error {
	deleteFile := byte(1)
	return windows.SetFileInformationByHandle(handle, windows.FileDispositionInfo, &deleteFile, 1)
}

func sameUpdateObject(left, right windows.Handle) bool {
	type fileIdentity struct {
		volume uint64
		id     [16]byte
	}
	getID := func(handle windows.Handle, info *fileIdentity) error {
		return windows.GetFileInformationByHandleEx(handle, windows.FileIdInfo, (*byte)(unsafe.Pointer(info)), uint32(unsafe.Sizeof(*info)))
	}
	var a, b fileIdentity
	if getID(left, &a) == nil && getID(right, &b) == nil {
		return a.id != [16]byte{} && a == b
	}

	// Some filesystems only expose the older 64-bit file index.
	var olderA, olderB windows.ByHandleFileInformation
	return windows.GetFileInformationByHandle(left, &olderA) == nil &&
		windows.GetFileInformationByHandle(right, &olderB) == nil &&
		(olderA.FileIndexHigh != 0 || olderA.FileIndexLow != 0) &&
		olderA.VolumeSerialNumber == olderB.VolumeSerialNumber &&
		olderA.FileIndexHigh == olderB.FileIndexHigh && olderA.FileIndexLow == olderB.FileIndexLow
}

func markUpdateZone(dir, source windows.Handle, name string) error {
	markName := name + ":Zone.Identifier"
	mark, err := createUpdateObject(dir, markName, windows.GENERIC_WRITE|windows.FILE_READ_ATTRIBUTES, updateFileShares, windows.FILE_NON_DIRECTORY_FILE, nil)
	if err != nil {
		return err
	}
	stream := os.NewFile(uintptr(mark), markName)
	defer stream.Close()
	if !sameUpdateObject(source, mark) {
		return errors.New("update zone stream is on a different file")
	}
	if _, err := io.WriteString(stream, "[ZoneTransfer]\r\nZoneId=3\r\n"); err != nil {
		return err
	}
	return stream.Sync()
}

func renameUpdateHandle(handle, dest windows.Handle, name string) error {
	if !filepath.IsLocal(name) {
		return windows.ERROR_INVALID_NAME
	}
	path, err := windows.UTF16FromString(name)
	if err != nil {
		return err
	}
	// FILE_RENAME_INFORMATION resolves the simple name relative to the open
	// destination directory. Zero replacement flags keep any existing file.
	type renameInfo struct {
		ReplaceIfExists uint32
		RootDirectory   windows.Handle
		FileNameLength  uint32
		FileName        [1]uint16
	}
	var layout renameInfo
	buffer := make([]byte, int(unsafe.Sizeof(layout))+2*(len(path)-1))
	info := (*renameInfo)(unsafe.Pointer(&buffer[0]))
	info.RootDirectory = dest
	info.FileNameLength = uint32(2 * (len(path) - 1))
	copy(unsafe.Slice(&info.FileName[0], len(path)), path)
	return updateWindowsError(windows.NtSetInformationFile(handle, &windows.IO_STATUS_BLOCK{}, &buffer[0], uint32(len(buffer)), windows.FileRenameInformation))
}
