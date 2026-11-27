package sqlite

import (
	"os"
	"unsafe"

	"golang.org/x/sys/windows"
)

func linkRollbackFile(stage *os.Root, staged string, destination *os.Root, name string) error {
	from, err := stage.Open(".")
	if err != nil {
		return err
	}
	defer from.Close()
	to, err := destination.Open(".")
	if err != nil {
		return err
	}
	defer to.Close()

	source, err := windows.NewNTUnicodeString(staged)
	if err != nil {
		return err
	}
	attributes := windows.OBJECT_ATTRIBUTES{
		RootDirectory: windows.Handle(from.Fd()), ObjectName: source, Attributes: windows.OBJ_CASE_INSENSITIVE,
	}
	attributes.Length = uint32(unsafe.Sizeof(attributes))
	var file windows.Handle
	const share = windows.FILE_SHARE_READ | windows.FILE_SHARE_WRITE | windows.FILE_SHARE_DELETE
	const options = windows.FILE_OPEN_REPARSE_POINT | windows.FILE_NON_DIRECTORY_FILE |
		windows.FILE_OPEN_FOR_BACKUP_INTENT | windows.FILE_SYNCHRONOUS_IO_NONALERT
	if err := windows.NtCreateFile(&file, windows.SYNCHRONIZE|windows.FILE_WRITE_ATTRIBUTES,
		&attributes, &windows.IO_STATUS_BLOCK{}, nil, 0, share, windows.FILE_OPEN, options, 0, 0); err != nil {
		return rollbackLinkError(err)
	}
	defer windows.CloseHandle(file)

	// The Go Windows Root.Link implementation also uses the legacy class 11
	// rather than FileLinkInformationEx. A zero replacement flag keeps collisions.
	const legacyFileLinkInformation = 11
	info := struct {
		ReplaceIfExists byte
		RootDirectory   windows.Handle
		FileNameLength  uint32
		FileName        [260]uint16
	}{RootDirectory: windows.Handle(to.Fd())}
	target, err := windows.UTF16FromString(name)
	if err != nil {
		return err
	}
	if len(target) > len(info.FileName) {
		return windows.ERROR_INVALID_NAME
	}
	copy(info.FileName[:], target)
	info.FileNameLength = uint32((len(target) - 1) * 2)
	return rollbackLinkError(windows.NtSetInformationFile(file, &windows.IO_STATUS_BLOCK{},
		(*byte)(unsafe.Pointer(&info)), uint32(unsafe.Sizeof(info)), legacyFileLinkInformation))
}

func rollbackLinkError(err error) error {
	if status, ok := err.(windows.NTStatus); ok {
		return status.Errno()
	}
	return err
}
