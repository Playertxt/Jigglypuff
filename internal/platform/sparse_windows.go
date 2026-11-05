//go:build windows

package platform

import (
	"os"
	"syscall"
)

const fsctlSetSparse = 0x000900C4

// setSparse marks a file sparse, so extending it doesn't allocate disk space.
func setSparse(f *os.File) {
	var n uint32
	syscall.DeviceIoControl(syscall.Handle(f.Fd()), fsctlSetSparse, nil, 0, nil, 0, &n, nil)
}
