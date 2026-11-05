//go:build !windows

package platform

import (
	"errors"
	"syscall"
)

// IsInUse reports whether err means another program has the file open.
func IsInUse(err error) bool {
	return errors.Is(err, syscall.EBUSY) || errors.Is(err, syscall.ETXTBSY)
}
