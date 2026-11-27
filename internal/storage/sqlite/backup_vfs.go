package sqlite

import (
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"os"
	"slices"
	"sync"
	"time"
	"unsafe"

	"modernc.org/libc"
	sqlite3 "modernc.org/sqlite/lib"
)

const (
	backupMainFile    = "jindi-snapshot"
	backupJournalFile = backupMainFile + "-journal"
)

// The main database is disk-backed. Only its rollback journal is in memory.
type backupVFS struct {
	file          *os.File
	journal       []byte
	journalExists bool
}

type backupFile struct {
	base    sqlite3.Tsqlite3_file
	vfs     uintptr
	journal bool
}

var registeredBackupVFS sync.Map
var backupOutputMethods = sqlite3.Tsqlite3_io_methods{FiVersion: 1}

func init() {
	bind(&backupOutputMethods.FxClose, func(*libc.TLS, uintptr) int32 { return sqlite3.SQLITE_OK })
	bind(&backupOutputMethods.FxRead, outputRead)
	bind(&backupOutputMethods.FxWrite, outputWrite)
	bind(&backupOutputMethods.FxTruncate, outputTruncate)
	bind(&backupOutputMethods.FxSync, outputSync)
	bind(&backupOutputMethods.FxFileSize, outputSize)
	// The unique registered name gives this pager exclusive access to its file.
	bind(&backupOutputMethods.FxLock, func(*libc.TLS, uintptr, int32) int32 { return sqlite3.SQLITE_OK })
	bind(&backupOutputMethods.FxUnlock, func(*libc.TLS, uintptr, int32) int32 { return sqlite3.SQLITE_OK })
	bind(&backupOutputMethods.FxCheckReservedLock, func(_ *libc.TLS, _, result uintptr) int32 {
		storeBackupValue(result, int32(0))
		return sqlite3.SQLITE_OK
	})
	bind(&backupOutputMethods.FxFileControl, func(*libc.TLS, uintptr, int32, uintptr) int32 { return sqlite3.SQLITE_NOTFOUND })
	bind(&backupOutputMethods.FxSectorSize, func(*libc.TLS, uintptr) int32 { return 4096 })
	bind(&backupOutputMethods.FxDeviceCharacteristics, func(*libc.TLS, uintptr) int32 { return 0 })
}

// Generated modernc callbacks store a Go function value in each uintptr field.
func bind[T any](ptr *uintptr, cb T) { *(*T)(unsafe.Pointer(ptr)) = cb }

func loadBackupValue[T any](from uintptr) (value T) {
	copy(unsafe.Slice((*byte)(unsafe.Pointer(&value)), unsafe.Sizeof(value)), libc.GoBytes(from, int(unsafe.Sizeof(value))))
	return value
}

func storeBackupValue[T any](to uintptr, value T) {
	copy(libc.GoBytes(to, int(unsafe.Sizeof(value))), unsafe.Slice((*byte)(unsafe.Pointer(&value)), unsafe.Sizeof(value)))
}

// VACUUM INTO gets only a synthetic name. This VFS never opens a host path.
func withBackupVFS(output *os.File, vacuum func(string) error) error {
	name := "jindi-backup-" + rand.Text()

	tls := libc.NewTLS()
	defer tls.Close()
	cname, err := libc.CString(name)
	if err != nil {
		return err
	}
	defer libc.Xfree(tls, cname)

	vfs := sqlite3.Tsqlite3_vfs{FiVersion: 1, FmxPathname: 64, FzName: cname}
	bind(&vfs.FxRandomness, func(_ *libc.TLS, _ uintptr, n int32, out uintptr) int32 {
		read, _ := rand.Read(libc.GoBytes(out, int(n)))
		return int32(read)
	})
	bind(&vfs.FxSleep, func(_ *libc.TLS, _ uintptr, us int32) int32 {
		time.Sleep(time.Duration(us) * time.Microsecond)
		return us
	})
	bind(&vfs.FxCurrentTime, func(_ *libc.TLS, _ uintptr, out uintptr) int32 {
		storeBackupValue(out, float64(time.Now().UnixMilli())/86400000+2440587.5)
		return sqlite3.SQLITE_OK
	})
	vfs.FszOsFile = int32(unsafe.Sizeof(backupFile{}))
	bind(&vfs.FxOpen, outputOpen)
	bind(&vfs.FxFullPathname, outputFullPath)
	bind(&vfs.FxDelete, outputDelete)
	bind(&vfs.FxAccess, outputAccess)
	addr := libc.Xmalloc(tls, libc.Tsize_t(unsafe.Sizeof(vfs)))
	if addr == 0 {
		return errors.New("cannot allocate backup VFS")
	}
	defer libc.Xfree(tls, addr)
	storeBackupValue(addr, vfs)
	registeredBackupVFS.Store(addr, &backupVFS{file: output})
	defer registeredBackupVFS.Delete(addr)
	if rc := sqlite3.Xsqlite3_vfs_register(tls, addr, 0); rc != sqlite3.SQLITE_OK {
		return fmt.Errorf("register backup VFS: %d", rc)
	}
	defer sqlite3.Xsqlite3_vfs_unregister(tls, addr)
	return vacuum("file:" + backupMainFile + "?vfs=" + name + "&cache=private")
}

func getVFS(vfs uintptr) *backupVFS {
	s, _ := registeredBackupVFS.Load(vfs)
	return s.(*backupVFS)
}

func getOutput(token uintptr) (*backupVFS, bool) {
	f := loadBackupValue[backupFile](token)
	return getVFS(f.vfs), f.journal
}

func outputOpen(_ *libc.TLS, vfs, name, file uintptr, flags int32, out uintptr) int32 {
	storeBackupValue(file, backupFile{})
	path := libc.GoString(name)
	isJournal := path == backupJournalFile && flags&sqlite3.SQLITE_OPEN_MAIN_JOURNAL != 0
	if !isJournal && (path != backupMainFile || flags&sqlite3.SQLITE_OPEN_MAIN_DB == 0) {
		return sqlite3.SQLITE_CANTOPEN
	}
	if isJournal {
		getVFS(vfs).journalExists = true
	}
	storeBackupValue(file, backupFile{base: sqlite3.Tsqlite3_file{FpMethods: uintptr(unsafe.Pointer(&backupOutputMethods))}, vfs: vfs, journal: isJournal})
	if out != 0 {
		storeBackupValue(out, flags)
	}
	return sqlite3.SQLITE_OK
}

func outputFullPath(_ *libc.TLS, _, name uintptr, cap int32, out uintptr) int32 {
	if libc.GoString(name) != backupMainFile || cap <= int32(len(backupMainFile)) {
		return sqlite3.SQLITE_CANTOPEN
	}
	output := libc.GoBytes(out, len(backupMainFile)+1)
	copy(output, backupMainFile)
	output[len(backupMainFile)] = 0
	return sqlite3.SQLITE_OK
}

func outputDelete(_ *libc.TLS, vfs, name uintptr, _ int32) int32 {
	if libc.GoString(name) != backupJournalFile {
		return sqlite3.SQLITE_IOERR_DELETE
	}
	s := getVFS(vfs)
	s.journal = s.journal[:0]
	s.journalExists = false
	return sqlite3.SQLITE_OK
}

func outputAccess(_ *libc.TLS, vfs, name uintptr, _ int32, out uintptr) int32 {
	var exists int32
	if path := libc.GoString(name); path == backupMainFile || path == backupJournalFile && getVFS(vfs).journalExists {
		exists = 1
	}
	storeBackupValue(out, exists)
	return sqlite3.SQLITE_OK
}

func (s *backupVFS) resizeJournal(size int) {
	if size > len(s.journal) {
		previous := len(s.journal)
		s.journal = slices.Grow(s.journal, size-previous)[:size]
		clear(s.journal[previous:])
		return
	}
	s.journal = s.journal[:size]
}

func outputRead(_ *libc.TLS, file, dst uintptr, n int32, offset int64) int32 {
	if n <= 0 || offset < 0 {
		return sqlite3.SQLITE_IOERR_READ
	}
	s, journal := getOutput(file)
	b := libc.GoBytes(dst, int(n))
	var read int
	var err error
	if journal {
		if offset < int64(len(s.journal)) {
			read = copy(b, s.journal[offset:])
		}
	} else {
		read, err = s.file.ReadAt(b, offset)
		if err != nil && !errors.Is(err, io.EOF) {
			return sqlite3.SQLITE_IOERR_READ
		}
	}
	clear(b[read:])
	if read < int(n) {
		return sqlite3.SQLITE_IOERR_SHORT_READ
	}
	return sqlite3.SQLITE_OK
}

func outputWrite(_ *libc.TLS, file, src uintptr, n int32, offset int64) int32 {
	if n <= 0 || offset < 0 {
		return sqlite3.SQLITE_IOERR_WRITE
	}
	s, journal := getOutput(file)
	b := libc.GoBytes(src, int(n))
	if journal {
		end := offset + int64(n)
		if end < offset || int64(int(end)) != end {
			return sqlite3.SQLITE_IOERR_WRITE
		}
		if end > int64(len(s.journal)) {
			s.resizeJournal(int(end))
		}
		copy(s.journal[offset:end], b)
		return sqlite3.SQLITE_OK
	}
	written, err := s.file.WriteAt(b, offset)
	if err != nil || written != len(b) {
		return sqlite3.SQLITE_IOERR_WRITE
	}
	return sqlite3.SQLITE_OK
}

func outputTruncate(_ *libc.TLS, file uintptr, size int64) int32 {
	if size < 0 {
		return sqlite3.SQLITE_IOERR_TRUNCATE
	}
	s, journal := getOutput(file)
	if journal {
		if int64(int(size)) != size {
			return sqlite3.SQLITE_IOERR_TRUNCATE
		}
		s.resizeJournal(int(size))
		return sqlite3.SQLITE_OK
	}
	if err := s.file.Truncate(size); err != nil {
		return sqlite3.SQLITE_IOERR_TRUNCATE
	}
	return sqlite3.SQLITE_OK
}

func outputSync(_ *libc.TLS, file uintptr, _ int32) int32 {
	s, journal := getOutput(file)
	if !journal {
		if err := s.file.Sync(); err != nil {
			return sqlite3.SQLITE_IOERR_FSYNC
		}
	}
	return sqlite3.SQLITE_OK
}

func outputSize(_ *libc.TLS, file, size uintptr) int32 {
	s, journal := getOutput(file)
	if journal {
		storeBackupValue(size, int64(len(s.journal)))
		return sqlite3.SQLITE_OK
	}
	fi, err := s.file.Stat()
	if err != nil {
		return sqlite3.SQLITE_IOERR_FSTAT
	}
	storeBackupValue(size, fi.Size())
	return sqlite3.SQLITE_OK
}
