//go:build windows

package platform

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"unsafe"
)

type guid struct {
	Data1 uint32
	Data2 uint16
	Data3 uint16
	Data4 [8]byte
}

var (
	folderDownloads       = guid{0x374DE290, 0x123F, 0x4565, [8]byte{0x91, 0x64, 0x39, 0xC4, 0x92, 0x5E, 0x46, 0x7B}}
	folderDesktop         = guid{0xB4BFCC3A, 0xDB2C, 0x424C, [8]byte{0xB0, 0x29, 0x7F, 0xE9, 0x9A, 0x87, 0xC6, 0x41}}
	folderDocuments       = guid{0xFDD39AD0, 0x238F, 0x46AF, [8]byte{0xAD, 0xB4, 0x6C, 0x85, 0x48, 0x03, 0x69, 0xC7}}
	folderPictures        = guid{0x33E28130, 0x4E1E, 0x4676, [8]byte{0x83, 0x5A, 0x98, 0x39, 0x5C, 0x3B, 0xC3, 0xBB}}
	folderMusic           = guid{0x4BD8D571, 0x6D19, 0x48D3, [8]byte{0xBE, 0x97, 0x42, 0x22, 0x20, 0x08, 0x0E, 0x43}}
	folderVideos          = guid{0x18989B1D, 0x99B5, 0x455B, [8]byte{0x84, 0x1C, 0xAB, 0x7C, 0x74, 0xE4, 0xDD, 0xFC}}
	folderLocalLow        = guid{0xA520A1A4, 0x1780, 0x4FF6, [8]byte{0xBD, 0x18, 0x16, 0x73, 0x43, 0xC5, 0xAF, 0x16}}
	folderStartMenu       = guid{0x625B53C3, 0xAB48, 0x4EC1, [8]byte{0xBA, 0x1F, 0xA1, 0xEF, 0x41, 0x46, 0xFC, 0x19}}
	folderCommonStartMenu = guid{0xA4115719, 0xD62E, 0x491D, [8]byte{0xAA, 0x7C, 0xE7, 0x4B, 0x8B, 0xE3, 0xB0, 0x67}}
	folderStartup         = guid{0xB97D20BB, 0xF46A, 0x4C97, [8]byte{0xBA, 0x10, 0x5E, 0x36, 0x08, 0x43, 0x08, 0x54}}
	folderCommonStartup   = guid{0x82A5EA35, 0xD9CD, 0x47C5, [8]byte{0x96, 0x29, 0xE1, 0x5D, 0x2F, 0x71, 0x4E, 0x6E}}
	folderPublicDesktop   = guid{0xC4AA340D, 0xF20F, 0x4863, [8]byte{0xAF, 0xEF, 0xF8, 0x7E, 0xF2, 0xE6, 0xBA, 0x25}}
)

func knownFolder(id guid, fallback string) string {
	var p *uint16
	hr, _, _ := procSHGetKnownFolderPath.Call(uintptr(unsafe.Pointer(&id)), 0, 0, uintptr(unsafe.Pointer(&p)))
	if hr != 0 || p == nil {
		return fallback
	}
	defer procCoTaskMemFree.Call(uintptr(unsafe.Pointer(p)))
	if s := u16ptrToString(p); s != "" {
		return s
	}
	return fallback
}

// longPath expands 8.3 short names such as C:\Users\KABIRS~1.
func longPath(p string) string {
	if p == "" {
		return p
	}
	in := u16(p)
	buf := make([]uint16, 1024)
	n, err := syscall.GetLongPathName(in, &buf[0], uint32(len(buf)))
	if err != nil || n == 0 || int(n) > len(buf) {
		return filepath.Clean(p)
	}
	return filepath.Clean(syscall.UTF16ToString(buf[:n]))
}

func env(name, fallback string) string {
	if v := os.Getenv(name); v != "" {
		return longPath(v)
	}
	return fallback
}

// CurrentPaths discovers the folders of the user running this process.
func CurrentPaths() *Paths {
	sysDrive := os.Getenv("SystemDrive")
	if sysDrive == "" {
		sysDrive = "C:"
	}
	win := env("SystemRoot", sysDrive+`\Windows`)
	profile := env("USERPROFILE", filepath.Join(sysDrive+`\`, "Users", os.Getenv("USERNAME")))
	pf := env("ProgramW6432", env("ProgramFiles", sysDrive+`\Program Files`))
	p := &Paths{
		SystemDrive:     sysDrive,
		WinDir:          win,
		ProgramFiles:    pf,
		ProgramFilesX86: env("ProgramFiles(x86)", sysDrive+`\Program Files (x86)`),
		ProgramData:     env("ProgramData", sysDrive+`\ProgramData`),
		CommonFiles:     env("CommonProgramW6432", filepath.Join(pf, "Common Files")),
		CommonFilesX86:  env("CommonProgramFiles(x86)", sysDrive+`\Program Files (x86)\Common Files`),
		UserProfile:     profile,
		AppData:         env("APPDATA", filepath.Join(profile, `AppData\Roaming`)),
		LocalAppData:    env("LOCALAPPDATA", filepath.Join(profile, `AppData\Local`)),
		Public:          env("PUBLIC", sysDrive+`\Users\Public`),
		UserName:        os.Getenv("USERNAME"),
	}
	p.Temp = env("TEMP", filepath.Join(p.LocalAppData, "Temp"))
	p.LocalLow = longPath(knownFolder(folderLocalLow, filepath.Join(profile, `AppData\LocalLow`)))
	p.Desktop = longPath(knownFolder(folderDesktop, filepath.Join(profile, "Desktop")))
	p.PublicDesktop = longPath(knownFolder(folderPublicDesktop, filepath.Join(p.Public, "Desktop")))
	p.Documents = longPath(knownFolder(folderDocuments, filepath.Join(profile, "Documents")))
	p.Downloads = longPath(knownFolder(folderDownloads, filepath.Join(profile, "Downloads")))
	p.Pictures = longPath(knownFolder(folderPictures, filepath.Join(profile, "Pictures")))
	p.Music = longPath(knownFolder(folderMusic, filepath.Join(profile, "Music")))
	p.Videos = longPath(knownFolder(folderVideos, filepath.Join(profile, "Videos")))
	p.StartMenu = longPath(knownFolder(folderStartMenu, filepath.Join(p.AppData, `Microsoft\Windows\Start Menu`)))
	p.CommonStartMenu = longPath(knownFolder(folderCommonStartMenu, filepath.Join(p.ProgramData, `Microsoft\Windows\Start Menu`)))
	p.Startup = longPath(knownFolder(folderStartup, filepath.Join(p.StartMenu, `Programs\Startup`)))
	p.CommonStartup = longPath(knownFolder(folderCommonStartup, filepath.Join(p.CommonStartMenu, `Programs\StartUp`)))
	p.JigglypuffData = filepath.Join(p.LocalAppData, "Jigglypuff")
	p.UserSID = currentSID()
	return p
}

func currentSID() string {
	proc, err := syscall.GetCurrentProcess()
	if err != nil {
		return ""
	}
	var token syscall.Token
	if err := syscall.OpenProcessToken(proc, syscall.TOKEN_QUERY, &token); err != nil {
		return ""
	}
	defer token.Close()
	tu, err := token.GetTokenUser()
	if err != nil {
		return ""
	}
	s, err := tu.User.Sid.String()
	if err != nil {
		return ""
	}
	return s
}

// IsElevated reports whether this process runs with administrator rights.
func IsElevated() bool {
	proc, err := syscall.GetCurrentProcess()
	if err != nil {
		return false
	}
	var token syscall.Token
	if err := syscall.OpenProcessToken(proc, syscall.TOKEN_QUERY, &token); err != nil {
		return false
	}
	defer token.Close()
	var elevation, n uint32
	err = syscall.GetTokenInformation(token, 20 /* TokenElevation */, (*byte)(unsafe.Pointer(&elevation)), 4, &n)
	return err == nil && elevation != 0
}

// ElevateSelf starts this executable again with administrator rights (a UAC
// prompt). It returns ErrCancelled when the user declines, and otherwise a
// function that reports whether the new copy is still running.
func ElevateSelf(params string) (func() bool, error) {
	exe, err := os.Executable()
	if err != nil {
		return nil, err
	}
	sei := shellExecuteInfo{fMask: 0x40 | 0x100, nShow: 1} // NOCLOSEPROCESS | NOASYNC
	sei.cbSize = uint32(unsafe.Sizeof(sei))
	sei.lpVerb = u16("runas")
	sei.lpFile = u16(exe)
	sei.lpParameters = u16(params)
	var ok uintptr
	var callErr error
	withCOM(func() { ok, _, callErr = procShellExecuteExW.Call(uintptr(unsafe.Pointer(&sei))) })
	if ok == 0 {
		if errno, isErrno := callErr.(syscall.Errno); isErrno && errno == errorCancelled {
			return nil, ErrCancelled
		}
		return nil, fmt.Errorf("could not start Jigglypuff as administrator: %v", callErr)
	}
	h := syscall.Handle(sei.hProcess)
	return func() bool {
		if h == 0 {
			return true
		}
		ev, _ := syscall.WaitForSingleObject(h, 0)
		return ev == 0x102 // WAIT_TIMEOUT: still running
	}, nil
}

// FindBrowser looks for a Chromium based browser that supports app windows.
func FindBrowser() string {
	r := newWinRegistry("")
	for _, exe := range []string{"msedge.exe", "chrome.exe", "brave.exe"} {
		for _, hive := range []string{"HKCU", "HKLM"} {
			m := ValueMap(must(r.Values(hive + `\SOFTWARE\Microsoft\Windows\CurrentVersion\App Paths\` + exe)))
			if p := Str(m, ""); p != "" && Exists(p) {
				return p
			}
		}
	}
	pf := env("ProgramFiles", `C:\Program Files`)
	pf86 := env("ProgramFiles(x86)", `C:\Program Files (x86)`)
	local := env("LOCALAPPDATA", "")
	for _, p := range []string{
		filepath.Join(pf86, `Microsoft\Edge\Application\msedge.exe`),
		filepath.Join(pf, `Microsoft\Edge\Application\msedge.exe`),
		filepath.Join(pf, `Google\Chrome\Application\chrome.exe`),
		filepath.Join(pf86, `Google\Chrome\Application\chrome.exe`),
		filepath.Join(local, `Google\Chrome\Application\chrome.exe`),
		filepath.Join(pf, `BraveSoftware\Brave-Browser\Application\brave.exe`),
	} {
		if Exists(p) {
			return p
		}
	}
	return ""
}

type rect struct{ left, top, right, bottom int32 }

// OpenAppWindow shows Jigglypuff's interface in a chrome-less browser app window
// (Microsoft Edge on every Windows 10/11 PC), or the default browser.
func OpenAppWindow(url, profileDir string) error {
	exe := FindBrowser()
	if exe == "" {
		_, err := shellExecute(context.Background(), "open", url, "", false)
		return err
	}
	var wa rect
	procSystemParametersInfoW.Call(0x0030 /* SPI_GETWORKAREA */, 0, uintptr(unsafe.Pointer(&wa)), 0)
	ww, wh := int(wa.right-wa.left), int(wa.bottom-wa.top)
	w, h := 1240, 820
	if ww > 200 && w > ww-60 {
		w = ww - 60
	}
	if wh > 200 && h > wh-40 {
		h = wh - 40
	}
	x, y := int(wa.left)+(ww-w)/2, int(wa.top)+(wh-h)/2
	if ww <= 200 {
		x, y = 80, 60
	}
	args := []string{
		"--app=" + url,
		"--user-data-dir=" + profileDir,
		"--no-first-run",
		"--no-default-browser-check",
		"--disable-background-mode",
		"--disable-extensions",
		"--disable-sync",
		"--hide-crash-restore-bubble",
		"--overscroll-history-navigation=0",
		"--disable-features=Translate,msEdgeSidebarV2,msUndersideButton,msImplicitSignin,AutofillServerCommunication,msEdgeTranslate",
		fmt.Sprintf("--window-size=%d,%d", w, h),
		fmt.Sprintf("--window-position=%d,%d", x, y),
	}
	cmd := exec.Command(exe, args...)
	return cmd.Start()
}

// OpenDefaultBrowser opens url in the user's default browser.
func OpenDefaultBrowser(url string) {
	shellExecute(context.Background(), "open", url, "", false)
}
