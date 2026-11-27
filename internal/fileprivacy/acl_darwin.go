package fileprivacy

/*
#include <errno.h>
#include <sys/acl.h>

static int clear_acl(int fd) {
	acl_t empty = acl_init(0);
	if (empty == NULL) return errno;
	int result = acl_set_fd_np(fd, empty, ACL_TYPE_EXTENDED);
	int failure = result == 0 ? 0 : errno;
	acl_free(empty);
	return failure == ENOTSUP ? 0 : failure;
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
