package fileprivacy

/*
#include <errno.h>
#include <stdint.h>
#include <sys/acl.h>
#include <sys/attr.h>
#include <unistd.h>

static int clear_acl(int fd) {
	acl_t empty = acl_init(0);
	if (empty == NULL) return errno;
	int result = acl_set_fd_np(fd, empty, ACL_TYPE_EXTENDED);
	int failure = result == 0 ? 0 : errno;
	acl_free(empty);
	return failure == ENOTSUP ? 0 : failure;
}

static int file_write_generation(int fd, uint32_t *generation) {
	struct attrlist attrs = {
		.bitmapcount = ATTR_BIT_MAP_COUNT,
		.commonattr = ATTR_CMN_GEN_COUNT,
	};
	struct {
		uint32_t length;
		uint32_t generation;
	} value = {0};
	if (fgetattrlist(fd, &attrs, &value, sizeof(value), FSOPT_ATTR_CMN_EXTENDED) != 0) {
		return errno;
	}
	if (value.length != sizeof(value)) {
		return EIO;
	}
	*generation = value.generation;
	return 0;
}
*/
import "C"

import "syscall"

func ClearDirectoryACL(fd int) error {
	if code := C.clear_acl(C.int(fd)); code != 0 {
		return syscall.Errno(code)
	}
	return nil
}

// WriteGeneration returns the filesystem write generation for an open file.
// A zero generation indicates the bytes may be changing through a writable mapping.
func WriteGeneration(fd int) (uint32, error) {
	var generation C.uint32_t
	if code := C.file_write_generation(C.int(fd), &generation); code != 0 {
		return 0, syscall.Errno(code)
	}
	return uint32(generation), nil
}
