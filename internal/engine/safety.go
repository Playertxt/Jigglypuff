package engine

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"jigglypuff/internal/platform"
)

// Purposes a deletion target can have. Each purpose is checked against its
// own, narrow set of rules.
const (
	PurposeJunk     = "junk"     // temp / cache / log files below a known junk folder
	PurposeLeftover = "leftover" // files, folders and shortcuts left by an app
	PurposeUserFile = "user"     // large files, duplicates, old downloads
	PurposeInstall  = "install"  // an app's own install folder outside the usual program folders
)

// Guard decides whether something may be deleted. It is consulted twice: when
// a deletion plan is built and again right before each item is removed.
type Guard struct {
	p          *platform.Paths
	protected  []string // these folders, and every folder containing them, are untouchable
	exclusions []string // user exclusions: never touch anything inside these
	noUserFile []string // user-file deletion is never allowed inside these
	leftRoots  []string // leftovers may only live inside these
	leftDeny   []string // ...but never inside these
	junkWin    []string // the only places inside Windows where junk may be removed
	self       string
}

// NewGuard creates a guard for the given folders.
func NewGuard(p *platform.Paths, exclusions []string, self string) *Guard {
	g := &Guard{p: p, self: self}
	add := func(list *[]string, paths ...string) {
		for _, x := range paths {
			if x != "" {
				*list = append(*list, platform.Clean(x))
			}
		}
	}
	j := filepath.Join
	usersRoot := filepath.Dir(p.UserProfile)
	add(&g.protected,
		p.WinDir, j(p.WinDir, "System32"), j(p.WinDir, "SysWOW64"), j(p.WinDir, "WinSxS"),
		p.ProgramFiles, p.ProgramFilesX86, p.CommonFiles, p.CommonFilesX86,
		p.ProgramData, j(p.ProgramData, "Microsoft"),
		usersRoot, p.UserProfile, p.Public, p.AppData, p.LocalAppData, p.LocalLow, p.Temp,
		j(p.AppData, "Microsoft"), j(p.LocalAppData, "Microsoft"), j(p.LocalAppData, "Packages"), j(p.LocalAppData, "Programs"),
		p.Desktop, p.PublicDesktop, p.Documents, p.Downloads, p.Pictures, p.Music, p.Videos,
		p.StartMenu, j(p.StartMenu, "Programs"), p.CommonStartMenu, j(p.CommonStartMenu, "Programs"),
		p.Startup, p.CommonStartup, p.JigglypuffData,
		j(p.ProgramFiles, "WindowsApps"),
		os.Getenv("OneDrive"), os.Getenv("OneDriveConsumer"), os.Getenv("OneDriveCommercial"),
	)
	if p.SystemDrive != "" {
		sd := p.SystemDrive + string(filepath.Separator)
		add(&g.protected, sd, j(sd, "Recovery"), j(sd, "System Volume Information"), j(sd, "$Recycle.Bin"), j(sd, "Boot"))
	}
	for _, x := range exclusions {
		add(&g.exclusions, x)
	}
	add(&g.exclusions, p.JigglypuffData)
	if self != "" {
		add(&g.exclusions, self)
	}
	add(&g.noUserFile, p.WinDir, p.ProgramFiles, p.ProgramFilesX86, p.ProgramData)
	add(&g.leftRoots,
		p.ProgramFiles, p.ProgramFilesX86, p.CommonFiles, p.CommonFilesX86, p.ProgramData,
		p.AppData, p.LocalAppData, j(p.LocalAppData, "Programs"), p.LocalLow, p.Temp, p.UserProfile,
		j(p.StartMenu, "Programs"), j(p.CommonStartMenu, "Programs"), p.Desktop, p.PublicDesktop,
		j(p.LocalAppData, "Packages"),
	)
	add(&g.leftDeny,
		p.WinDir, j(p.ProgramFiles, "WindowsApps"), j(p.ProgramFiles, "Windows Defender"), j(p.ProgramFiles, "Windows NT"),
		j(p.ProgramFiles, "Common Files", "Microsoft Shared"), j(p.ProgramData, "Microsoft", "Windows Defender"),
		j(p.ProgramData, "Packages"), j(p.ProgramData, "Package Cache"),
		p.Documents, p.Pictures, p.Music, p.Videos, p.Downloads, j(p.UserProfile, "OneDrive"),
	)
	// Microsoft's own data folders are off limits, except the Start Menu, which
	// lives inside them and is a legitimate place for leftover shortcuts.
	add(&g.leftDeny, j(p.AppData, "Microsoft"), j(p.LocalAppData, "Microsoft"), j(p.ProgramData, "Microsoft"))
	add(&g.junkWin,
		j(p.WinDir, "Temp"), j(p.WinDir, "SoftwareDistribution", "Download"), j(p.WinDir, "Logs"),
		j(p.WinDir, "Minidump"), j(p.WinDir, "LiveKernelReports"), j(p.WinDir, "MEMORY.DMP"),
		j(p.WinDir, "ServiceProfiles", "NetworkService", "AppData", "Local", "Microsoft", "Windows", "DeliveryOptimization", "Cache"),
	)
	return g
}

func anyWithin(path string, roots []string) bool {
	for _, r := range roots {
		if platform.Within(path, r) {
			return true
		}
	}
	return false
}

// Excluded reports whether the user excluded path (or Jigglypuff itself lives there).
func (g *Guard) Excluded(path string) bool { return anyWithin(path, g.exclusions) }

// CheckPath validates a file system target. root is the folder a junk file
// must stay inside (empty for other purposes).
func (g *Guard) CheckPath(path, purpose, root string) error {
	if path == "" || !filepath.IsAbs(path) {
		return errors.New("not an absolute path")
	}
	clean := platform.Clean(path)
	if strings.Contains(path, "..") && clean != path {
		return errors.New("path contains '..'")
	}
	if filepath.Dir(clean) == clean || len(clean) <= 3 {
		return errors.New("refusing to delete a drive")
	}
	if g.Excluded(clean) {
		return errors.New("excluded in Settings")
	}
	for _, pr := range g.protected {
		if platform.Within(pr, clean) { // clean is a protected folder or contains one
			return fmt.Errorf("%s is a protected Windows folder", clean)
		}
	}
	if anyWithin(clean, []string{filepath.Join(g.p.WinDir, "System32"), filepath.Join(g.p.WinDir, "SysWOW64"), filepath.Join(g.p.WinDir, "WinSxS")}) {
		return errors.New("inside a Windows system folder")
	}
	switch purpose {
	case PurposeJunk:
		if root == "" || !platform.StrictlyWithin(clean, root) {
			return errors.New("outside the cleaned folder")
		}
		if platform.Within(clean, g.p.WinDir) && !anyWithin(clean, g.junkWin) {
			return errors.New("inside Windows")
		}
	case PurposeLeftover:
		if !g.leftoverAllowed(clean) {
			return errors.New("not a place where app leftovers are removed")
		}
	case PurposeUserFile:
		if anyWithin(clean, g.noUserFile) {
			return errors.New("inside a system or program folder")
		}
	case PurposeInstall:
		// Custom install locations (D:\Games\App, C:\Tools\App) may live
		// anywhere, but never in Windows, in your personal folders, or at the
		// top of a drive.
		if platform.Within(clean, g.p.WinDir) || anyWithin(clean, g.leftDeny) {
			return errors.New("not a place where programs are removed from")
		}
		if filepath.Dir(filepath.Dir(clean)) == filepath.Dir(clean) && len(filepath.Base(clean)) < 3 {
			return errors.New("too close to the top of the drive")
		}
	default:
		return errors.New("unknown purpose")
	}
	return nil
}

func (g *Guard) leftoverAllowed(p string) bool {
	inRoot := false
	for _, r := range g.leftRoots {
		if platform.StrictlyWithin(p, r) {
			inRoot = true
			break
		}
	}
	if !inRoot {
		return false
	}
	startMenus := []string{filepath.Join(g.p.StartMenu, "Programs"), filepath.Join(g.p.CommonStartMenu, "Programs")}
	if anyWithin(p, startMenus) {
		return true
	}
	if anyWithin(p, g.leftDeny) {
		return false
	}
	// Directly inside the user profile only hidden ".app" folders count.
	if platform.SamePath(filepath.Dir(p), g.p.UserProfile) && !strings.HasPrefix(filepath.Base(p), ".") {
		return false
	}
	// Inside Packages only whole package folders may go (Store app leftovers).
	pk := filepath.Join(g.p.LocalAppData, "Packages")
	if platform.StrictlyWithin(p, pk) && !platform.SamePath(filepath.Dir(p), pk) {
		return false
	}
	return true
}

// Registry ------------------------------------------------------------------------

var blockedVendors = map[string]bool{
	"microsoft": true, "classes": true, "policies": true, "wow6432node": true, "clients": true,
	"registeredapplications": true, "khronos": true, "intel": true, "nvidia": true, "nvidiacorporation": true,
	"amd": true, "ati": true, "atitechnologies": true, "realtek": true, "odbc": true, "windows": true,
	"defaultuserenvironment": true, "description": true, "oem": true, "partner": true, "setup": true,
	"appdatalow": true, "jigglypuff": true,
}

// CheckRegKey validates a registry key deletion.
func (g *Guard) CheckRegKey(key string) error {
	hive, path := platform.SplitKey(key)
	if hive != "HKCU" && hive != "HKLM" {
		return errors.New("only HKCU and HKLM keys can be removed")
	}
	parts := strings.Split(path, `\`)
	for _, p := range parts {
		if p == "" {
			return errors.New("malformed key")
		}
	}
	if len(parts) < 2 || !strings.EqualFold(parts[0], "Software") {
		return errors.New("only keys below Software can be removed")
	}
	rest := parts[1:]
	if strings.EqualFold(rest[0], "WOW6432Node") {
		rest = rest[1:]
		if len(rest) == 0 {
			return errors.New("refusing to remove WOW6432Node")
		}
	}
	vendor := Norm(rest[0])
	if vendor == "microsoft" {
		// Allowed: one uninstall entry or one App Paths entry.
		low := strings.ToLower(strings.Join(rest, `\`))
		for _, pre := range []string{`microsoft\windows\currentversion\uninstall\`, `microsoft\windows\currentversion\app paths\`} {
			if strings.HasPrefix(low, pre) && len(rest) == 5 {
				return nil
			}
		}
		return errors.New("Microsoft's registry keys are protected")
	}
	if blockedVendors[vendor] {
		return fmt.Errorf("the %s key is protected", rest[0])
	}
	return nil
}

// CheckRegValue validates deleting a single value (startup entries only).
func (g *Guard) CheckRegValue(key, name string) error {
	hive, path := platform.SplitKey(key)
	if hive != "HKCU" && hive != "HKLM" {
		return errors.New("only HKCU and HKLM values can be removed")
	}
	low := strings.ToLower(path)
	for _, ok := range []string{
		`software\microsoft\windows\currentversion\run`,
		`software\wow6432node\microsoft\windows\currentversion\run`,
	} {
		if low == ok && name != "" {
			return nil
		}
	}
	return errors.New("only startup entries can be removed as single values")
}

// CheckService validates deleting a Windows service.
func (g *Guard) CheckService(name, imagePath string) error {
	if name == "" {
		return errors.New("no service name")
	}
	exe, _ := platform.ParseCommandLine(imagePath)
	exe = platform.ExpandEnv(exe)
	if exe == "" || !filepath.IsAbs(exe) {
		return errors.New("the service has no clear program path")
	}
	if platform.Within(exe, g.p.WinDir) {
		return errors.New("Windows services are protected")
	}
	return nil
}

// CheckTask validates deleting a scheduled task.
func (g *Guard) CheckTask(taskPath string) error {
	low := strings.ToLower(taskPath)
	if !strings.HasPrefix(low, `\`) || strings.HasPrefix(low, `\microsoft\`) || strings.Count(low, `\`) < 1 || len(low) < 3 {
		return errors.New("Windows tasks are protected")
	}
	return nil
}
