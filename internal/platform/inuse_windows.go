//go:build windows

package platform

import (
	"errors"
	"syscall"
)

// IsInUse reports whether err means another program has the file open.
func IsInUse(err error) bool {
	var errno syscall.Errno
	if errors.As(err, &errno) {
		return errno == 32 || errno == 33 // ERROR_SHARING_VIOLATION, ERROR_LOCK_VIOLATION
	}
	return false
}
