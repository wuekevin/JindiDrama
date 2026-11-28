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
	var directoryDescriptor, fileDescriptor *windows.SECURITY_DESCRIPTOR
	if kind != mediaOutputStage {
		var err error
		directoryDescriptor, err = fileprivacy.WindowsDirectoryDescriptor()
		if err != nil {
			return nil, err
		}
	}
	if kind == privateBackupOutputStage {
		var err error
		fileDescriptor, err = fileprivacy.WindowsFileDescriptor()
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

	if kind == privateBackupOutputStage {
		var volumeFlags uint32
		// Some shares expose file ACLs without supporting this volume query.
		// The staging directory and file ACLs are always checked below.
		if err := windows.GetVolumeInformationByHandle(dest, nil, 0, nil, nil, &volumeFlags, nil, 0); err == nil &&
			volumeFlags&windows.FILE_PERSISTENT_ACLS == 0 {
			return nil, errors.Join(errPrivateBackupACL, windows.CloseHandle(dest))
		}
	}
	dirName := prefix + rand.Text()
	stagingAccess := uint32(windows.FILE_WRITE_DATA | windows.FILE_TRAVERSE | windows.DELETE)
	if kind == privateBackupOutputStage {
		stagingAccess |= windows.READ_CONTROL
	}
	dir, err := fileprivacy.CreateWindowsObject(dest, dirName, stagingAccess, updateFileShares, windows.FILE_DIRECTORY_FILE, directoryDescriptor)
	if err != nil {
		_ = windows.CloseHandle(dest)
		return nil, err
	}
	closeStageDirectory := func() error {
		return errors.Join(deleteUpdateObject(dir), windows.CloseHandle(dir), windows.CloseHandle(dest))
	}

	if kind == privateBackupOutputStage {
		if err := verifyPrivateBackupACL(dir, directoryDescriptor); err != nil {
			return nil, errors.Join(err, closeStageDirectory())
		}
	}
	const name = "package"
	access := uint32(windows.GENERIC_WRITE | windows.FILE_READ_ATTRIBUTES | windows.DELETE)
	if kind == privateBackupOutputStage {
		access |= windows.GENERIC_READ
	}
	handle, err := fileprivacy.CreateWindowsObject(dir, name, access, windows.FILE_SHARE_READ, windows.FILE_NON_DIRECTORY_FILE, fileDescriptor)
	if err != nil {
		return nil, outputStageSetupError(kind, err, closeStageDirectory())
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
	if kind == privateBackupOutputStage {
		if err := verifyPrivateBackupACL(handle, fileDescriptor); err != nil {
			return nil, errors.Join(err, cleanup())
		}
	}
	if kind == updateOutputStage {
		if err := markUpdateZone(dir, handle, name); err != nil {
			return nil, outputStageSetupError(kind, errUpdateOrigin, cleanup())
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
				handle, err := fileprivacy.CreateWindowsObject(dest, name, windows.GENERIC_WRITE, windows.FILE_SHARE_READ, windows.FILE_NON_DIRECTORY_FILE, nil)
				if err != nil {
					return err
				}
				return completeDirectExport(os.NewFile(uintptr(handle), target), write)
			})
		},
	}, nil
}

var errPrivateBackupACL = errors.New("无法为数据库备份设置私有权限")

func verifyPrivateBackupACL(handle windows.Handle, expected *windows.SECURITY_DESCRIPTOR) error {
	err := fileprivacy.VerifyWindowsDACL(handle, expected)
	if errors.Is(err, fileprivacy.ErrWindowsDACL) {
		return errPrivateBackupACL
	}
	return err
}

func openUpdateDirectory(path string, access uint32) (windows.Handle, error) {
	name, err := fileprivacy.WindowsPath(path)
	if err != nil {
		return 0, err
	}
	return windows.CreateFile(name, access, updateFileShares, nil, windows.OPEN_EXISTING, windows.FILE_FLAG_BACKUP_SEMANTICS, 0)
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
	mark, err := fileprivacy.CreateWindowsObject(dir, markName, windows.GENERIC_WRITE|windows.FILE_READ_ATTRIBUTES, updateFileShares, windows.FILE_NON_DIRECTORY_FILE, nil)
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
