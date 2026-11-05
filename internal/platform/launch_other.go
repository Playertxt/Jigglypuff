//go:build !windows

package platform

import (
	"errors"
	"fmt"
	"os"
)

// ErrCancelled means the user said "No" to a UAC prompt.
var ErrCancelled = errors.New("cancelled by the user")

// CurrentPaths is only meaningful on Windows; the demo builds its own paths.
func CurrentPaths() *Paths { return nil }

// IsElevated reports whether the process runs as root.
func IsElevated() bool { return os.Geteuid() == 0 }

// ElevateSelf is not available outside Windows.
func ElevateSelf(params string) (func() bool, error) { return nil, ErrUnsupported }

// FindBrowser is not used outside Windows.
func FindBrowser() string { return "" }

// OpenAppWindow just prints the address outside Windows.
func OpenAppWindow(url, profileDir string) error {
	fmt.Println("Jigglypuff is running at", url)
	return nil
}

// MessageBox prints to stderr outside Windows.
func MessageBox(title, text string) { fmt.Fprintln(os.Stderr, title+": "+text) }

// NewWindows is unavailable outside Windows.
func NewWindows(p *Paths) Platform { panic("NewWindows is only available on Windows") }

// OpenDefaultBrowser prints the address outside Windows.
func OpenDefaultBrowser(url string) { fmt.Println("Open", url) }
