// Package platform hides every operating-system specific operation behind a
// small interface. The real implementation lives in the *_windows.go files;
// demo.go provides a fully simulated PC so the app can be developed and tested
// on any OS without touching real files.
package platform

import (
	"context"
	"errors"
	"strings"
	"time"
)

// ErrUnsupported is returned by operations that make no sense on this platform.
var ErrUnsupported = errors.New("not supported on this system")

// Drive is a fixed disk volume.
type Drive struct {
	Root   string `json:"root"` // "C:\"
	Label  string `json:"label"`
	Total  int64  `json:"total"`
	Free   int64  `json:"free"`
	System bool   `json:"system"`
}

// Process is a running process.
type Process struct {
	PID  int
	Name string // "chrome.exe"
	Path string // full image path when it could be read
}

// StoreApp is a Microsoft Store (MSIX/AppX) package.
type StoreApp struct {
	Name        string `json:"name"`
	FullName    string `json:"fullName"`
	Family      string `json:"family"`
	Publisher   string `json:"publisher"`
	Version     string `json:"version"`
	Location    string `json:"location"`
	DisplayName string `json:"displayName"`
}

// Registry value kinds (same numbers as Windows).
const (
	RegNone     = 0
	RegSZ       = 1
	RegExpandSZ = 2
	RegBinary   = 3
	RegDWORD    = 4
	RegMultiSZ  = 7
	RegQWORD    = 11
)

// RegValue is one registry value. S always holds a readable string form.
type RegValue struct {
	Name string
	Type uint32
	S    string
	N    uint64
	B    []byte
}

// Registry gives access to the Windows registry. Keys are full paths that
// start with a hive name: HKLM\..., HKCU\... or HKU\...
type Registry interface {
	SubKeys(key string) ([]string, error)
	Values(key string) ([]RegValue, error)
	LastWrite(key string) (time.Time, error)
	Exists(key string) bool
	DeleteTree(key string) error
	DeleteValue(key, name string) error
	SetBinary(key, name string, data []byte) error
	// Export writes a .reg backup of key (and its subkeys) to file.
	Export(key, file string) error
}

// Platform is everything Jigglypuff needs from the operating system.
type Platform interface {
	Kind() string // "windows" or "demo"
	Paths() *Paths
	Reg() Registry
	IsAdmin() bool
	UserName() string

	Drives() []Drive
	// DriveOf returns the root of the drive that holds path (like D:\), or ""
	// when path is not on a lettered drive.
	DriveOf(path string) string
	// DriveRoot returns the folder to scan for a drive root like D:\.
	DriveRoot(root string) string
	Processes() []Process

	// RecycleBinInfo and EmptyRecycleBin work on one drive's Recycle Bin, or
	// on every drive when root is "".
	RecycleBinInfo(root string) (size int64, items int64, err error)
	EmptyRecycleBin(root string) error
	// Recycle moves files or folders to the Recycle Bin.
	Recycle(paths []string) error
	// RecycleLimit reports the largest item (bytes) the Recycle Bin of the
	// volume holding path accepts, and whether that volume deletes instead.
	RecycleLimit(path string) (maxBytes int64, nukes bool)

	Reveal(path string) error     // open Explorer with the item selected
	OpenFolder(path string) error // open a folder in Explorer
	Launch(target string) error   // open a URL, ms-settings: link or program

	// Icon returns a PNG of the icon stored in a file (exe, dll, ico, png).
	Icon(path string, index, size int) ([]byte, error)
	// FileInfo reads the company and description from an executable.
	FileInfo(path string) (company, description string)

	// RunUninstaller starts an uninstaller command line and waits until it and
	// every process it started have exited, or ctx is cancelled.
	RunUninstaller(ctx context.Context, cmdline string) (exitCode int, err error)
	CreateRestorePoint(description string) (status string, err error)

	StoreApps() ([]StoreApp, error)
	RemoveStoreApp(fullName string) error

	StopAndDeleteService(name string) error
	DeleteScheduledTask(taskPath string) error
	CreateShortcut(lnkPath, target, args, workDir, description string) error
}

// Shared helpers -----------------------------------------------------------

// SplitKey separates "HKLM\SOFTWARE\X" into ("HKLM", "SOFTWARE\X").
func SplitKey(key string) (hive, path string) {
	key = strings.Trim(key, `\`)
	if i := strings.IndexByte(key, '\\'); i >= 0 {
		return strings.ToUpper(key[:i]), key[i+1:]
	}
	return strings.ToUpper(key), ""
}

// JoinKey joins registry path parts with backslashes.
func JoinKey(parts ...string) string {
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.Trim(p, `\`)
		if p != "" {
			out = append(out, p)
		}
	}
	return strings.Join(out, `\`)
}

// ValueMap turns a value list into a case-insensitive lookup.
func ValueMap(vals []RegValue) map[string]RegValue {
	m := make(map[string]RegValue, len(vals))
	for _, v := range vals {
		m[strings.ToLower(v.Name)] = v
	}
	return m
}

// Str returns a string value from a ValueMap ("" when missing).
func Str(m map[string]RegValue, name string) string {
	if v, ok := m[strings.ToLower(name)]; ok {
		return strings.TrimSpace(v.S)
	}
	return ""
}

// Num returns a numeric value from a ValueMap.
func Num(m map[string]RegValue, name string) (uint64, bool) {
	v, ok := m[strings.ToLower(name)]
	if !ok {
		return 0, false
	}
	if v.Type == RegDWORD || v.Type == RegQWORD {
		return v.N, true
	}
	return 0, false
}
