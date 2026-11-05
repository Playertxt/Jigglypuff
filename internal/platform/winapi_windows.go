//go:build windows

package platform

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"
	"unicode/utf16"
	"unsafe"
)

var (
	modadvapi32 = syscall.NewLazyDLL("advapi32.dll")
	modshell32  = syscall.NewLazyDLL("shell32.dll")
	modkernel32 = syscall.NewLazyDLL("kernel32.dll")
	moduser32   = syscall.NewLazyDLL("user32.dll")
	modgdi32    = syscall.NewLazyDLL("gdi32.dll")
	modole32    = syscall.NewLazyDLL("ole32.dll")
	modversion  = syscall.NewLazyDLL("version.dll")

	procRegOpenKeyExW    = modadvapi32.NewProc("RegOpenKeyExW")
	procRegCloseKey      = modadvapi32.NewProc("RegCloseKey")
	procRegEnumKeyExW    = modadvapi32.NewProc("RegEnumKeyExW")
	procRegEnumValueW    = modadvapi32.NewProc("RegEnumValueW")
	procRegQueryInfoKeyW = modadvapi32.NewProc("RegQueryInfoKeyW")
	procRegDeleteTreeW   = modadvapi32.NewProc("RegDeleteTreeW")
	procRegDeleteKeyExW  = modadvapi32.NewProc("RegDeleteKeyExW")
	procRegDeleteValueW  = modadvapi32.NewProc("RegDeleteValueW")
	procRegCreateKeyExW  = modadvapi32.NewProc("RegCreateKeyExW")
	procRegSetValueExW   = modadvapi32.NewProc("RegSetValueExW")

	procSHFileOperationW     = modshell32.NewProc("SHFileOperationW")
	procSHQueryRecycleBinW   = modshell32.NewProc("SHQueryRecycleBinW")
	procSHEmptyRecycleBinW   = modshell32.NewProc("SHEmptyRecycleBinW")
	procShellExecuteExW      = modshell32.NewProc("ShellExecuteExW")
	procSHGetKnownFolderPath = modshell32.NewProc("SHGetKnownFolderPath")
	procSHDefExtractIconW    = modshell32.NewProc("SHDefExtractIconW")

	procCoInitializeEx = modole32.NewProc("CoInitializeEx")
	procCoUninitialize = modole32.NewProc("CoUninitialize")
	procCoTaskMemFree  = modole32.NewProc("CoTaskMemFree")

	procGetLogicalDriveStringsW           = modkernel32.NewProc("GetLogicalDriveStringsW")
	procGetDriveTypeW                     = modkernel32.NewProc("GetDriveTypeW")
	procGetDiskFreeSpaceExW               = modkernel32.NewProc("GetDiskFreeSpaceExW")
	procGetVolumeInformationW             = modkernel32.NewProc("GetVolumeInformationW")
	procGetVolumeNameForVolumeMountPointW = modkernel32.NewProc("GetVolumeNameForVolumeMountPointW")
	procGetVolumePathNameW                = modkernel32.NewProc("GetVolumePathNameW")
	procQueryFullProcessImageNameW        = modkernel32.NewProc("QueryFullProcessImageNameW")
	procCreateJobObjectW                  = modkernel32.NewProc("CreateJobObjectW")
	procAssignProcessToJobObject          = modkernel32.NewProc("AssignProcessToJobObject")
	procQueryInformationJobObject         = modkernel32.NewProc("QueryInformationJobObject")
	procResumeThread                      = modkernel32.NewProc("ResumeThread")

	procDestroyIcon           = moduser32.NewProc("DestroyIcon")
	procGetIconInfo           = moduser32.NewProc("GetIconInfo")
	procSystemParametersInfoW = moduser32.NewProc("SystemParametersInfoW")
	procMessageBoxW           = moduser32.NewProc("MessageBoxW")

	procGetObjectW         = modgdi32.NewProc("GetObjectW")
	procGetDIBits          = modgdi32.NewProc("GetDIBits")
	procDeleteObject       = modgdi32.NewProc("DeleteObject")
	procCreateCompatibleDC = modgdi32.NewProc("CreateCompatibleDC")
	procDeleteDC           = modgdi32.NewProc("DeleteDC")

	procGetFileVersionInfoSizeW = modversion.NewProc("GetFileVersionInfoSizeW")
	procGetFileVersionInfoW     = modversion.NewProc("GetFileVersionInfoW")
	procVerQueryValueW          = modversion.NewProc("VerQueryValueW")
)

const (
	createNoWindow         = 0x08000000
	createSuspended        = 0x00000004
	errorMoreData          = 234
	errorNoMoreItems       = 259
	errorFileNotFound      = 2
	errorCancelled         = 1223
	errorElevationRequired = 740
)

// u16 converts a Go string to a NUL terminated UTF-16 pointer. Strings that
// contain a NUL byte become empty, which every caller treats as "not found".
func u16(s string) *uint16 {
	p, err := syscall.UTF16PtrFromString(s)
	if err != nil {
		p, _ = syscall.UTF16PtrFromString("")
	}
	return p
}

// u16ptrToString reads a NUL terminated UTF-16 string from memory owned by
// Windows.
func u16ptrToString(p *uint16) string {
	if p == nil {
		return ""
	}
	var s []uint16
	for ptr := unsafe.Pointer(p); ; ptr = unsafe.Add(ptr, 2) {
		c := *(*uint16)(ptr)
		if c == 0 {
			break
		}
		s = append(s, c)
		if len(s) > 32768 {
			break
		}
	}
	return string(utf16.Decode(s))
}

// withCOM runs fn on a locked OS thread with a single-threaded COM apartment,
// which several shell functions require.
func withCOM(fn func()) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	hr, _, _ := procCoInitializeEx.Call(0, 0x2|0x4) // APARTMENTTHREADED | DISABLE_OLE1DDE
	if hr == 0 || hr == 1 {
		defer procCoUninitialize.Call()
	}
	fn()
}

// hiddenCmd prepares a command that never flashes a console window.
func hiddenCmd(ctx context.Context, name string, args ...string) *exec.Cmd {
	var cmd *exec.Cmd
	if ctx != nil {
		cmd = exec.CommandContext(ctx, name, args...)
	} else {
		cmd = exec.Command(name, args...)
	}
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: createNoWindow}
	return cmd
}

func system32(exe string) string {
	win := os.Getenv("SystemRoot")
	if win == "" {
		win = `C:\Windows`
	}
	return filepath.Join(win, "System32", exe)
}

// powershell runs a short script with Windows PowerShell and returns stdout.
// The script is passed as plain text (never encoded) so security software
// can read exactly what Jigglypuff runs. Scripts use single quotes only.
func powershell(ctx context.Context, timeout time.Duration, script string) (string, error) {
	if strings.Contains(script, `"`) {
		return "", errors.New("internal error: PowerShell scripts must not contain double quotes")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	full := "$ProgressPreference='SilentlyContinue'; [Console]::OutputEncoding=[Text.Encoding]::UTF8; " + script
	ps := system32(`WindowsPowerShell\v1.0\powershell.exe`)
	cmd := hiddenCmd(ctx, ps, "-NoLogo", "-NoProfile", "-NonInteractive", "-Command", full)
	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if ctx.Err() == context.DeadlineExceeded {
		return stdout.String(), errors.New("PowerShell took too long to answer")
	}
	if err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = strings.TrimSpace(stdout.String())
		}
		if msg == "" {
			msg = err.Error()
		}
		return stdout.String(), errors.New(firstLine(msg))
	}
	return stdout.String(), nil
}

// psQuote quotes a string for a single-quoted PowerShell literal.
func psQuote(s string) string {
	s = strings.ReplaceAll(s, `"`, "") // never valid in paths; keeps scripts free of double quotes
	return "'" + strings.ReplaceAll(s, "'", "''") + "'"
}

func firstLine(s string) string {
	s = strings.TrimSpace(s)
	for _, line := range strings.Split(s, "\n") {
		line = strings.TrimSpace(line)
		if line != "" && !strings.HasPrefix(line, "At line:") && !strings.HasPrefix(line, "+") {
			return strings.TrimPrefix(line, "Checkpoint-Computer : ")
		}
	}
	return s
}

// MessageBox shows a simple native message box (used for fatal start-up errors).
func MessageBox(title, text string) {
	procMessageBoxW.Call(0, uintptr(unsafe.Pointer(u16(text))), uintptr(unsafe.Pointer(u16(title))), 0x40|0x10000) // MB_ICONINFORMATION | MB_SETFOREGROUND
}
