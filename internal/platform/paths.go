package platform

import (
	"os"
	"path/filepath"
	"strings"
)

// Paths holds the well-known folders of the signed-in user and the system.
// The launcher (which runs un-elevated as the real user) fills this in and
// hands it to the elevated server, so cleaning always targets the right user.
type Paths struct {
	SystemDrive     string `json:"systemDrive"`
	WinDir          string `json:"winDir"`
	ProgramFiles    string `json:"programFiles"`
	ProgramFilesX86 string `json:"programFilesX86"`
	ProgramData     string `json:"programData"`
	CommonFiles     string `json:"commonFiles"`
	CommonFilesX86  string `json:"commonFilesX86"`
	UserProfile     string `json:"userProfile"`
	AppData         string `json:"appData"`
	LocalAppData    string `json:"localAppData"`
	LocalLow        string `json:"localLow"`
	Temp            string `json:"temp"`
	Public          string `json:"public"`
	Desktop         string `json:"desktop"`
	PublicDesktop   string `json:"publicDesktop"`
	Documents       string `json:"documents"`
	Downloads       string `json:"downloads"`
	Pictures        string `json:"pictures"`
	Music           string `json:"music"`
	Videos          string `json:"videos"`
	StartMenu       string `json:"startMenu"`
	CommonStartMenu string `json:"commonStartMenu"`
	Startup         string `json:"startup"`
	CommonStartup   string `json:"commonStartup"`
	JigglypuffData       string `json:"jigglypuffData"` // Jigglypuff's own settings, history and backups
	UserSID         string `json:"userSid"`
	UserName        string `json:"userName"`
}

// UserKey returns the registry root for the signed-in user.
func (p *Paths) UserKey() string { return "HKCU" }

// Clean normalises a path.
func Clean(p string) string {
	if p == "" {
		return ""
	}
	return filepath.Clean(p)
}

// SamePath compares two paths case-insensitively.
func SamePath(a, b string) bool {
	if a == "" || b == "" {
		return false
	}
	return strings.EqualFold(Clean(a), Clean(b))
}

// Within reports whether path is root itself or anything inside it.
func Within(path, root string) bool {
	if path == "" || root == "" {
		return false
	}
	p := strings.ToLower(Clean(path))
	r := strings.ToLower(Clean(root))
	if p == r {
		return true
	}
	if !strings.HasSuffix(r, string(filepath.Separator)) {
		r += string(filepath.Separator)
	}
	return strings.HasPrefix(p, r)
}

// StrictlyWithin reports whether path is inside root (but not root itself).
func StrictlyWithin(path, root string) bool {
	return Within(path, root) && !SamePath(path, root)
}

// Exists reports whether a file or folder exists (without following links).
func Exists(path string) bool {
	if path == "" {
		return false
	}
	_, err := os.Lstat(path)
	return err == nil
}

// IsDir reports whether path is a real directory (not a link).
func IsDir(path string) bool {
	fi, err := os.Lstat(path)
	return err == nil && fi.IsDir() && fi.Mode()&(os.ModeSymlink|os.ModeIrregular) == 0
}

// IsLink reports whether a directory entry mode is a link / reparse point
// (symlink, junction, OneDrive placeholder ...). Jigglypuff never descends into or
// deletes through such entries.
func IsLink(m os.FileMode) bool {
	return m&(os.ModeSymlink|os.ModeIrregular|os.ModeDevice|os.ModeNamedPipe|os.ModeSocket) != 0
}
