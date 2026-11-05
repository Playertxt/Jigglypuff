package platform

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// demoPlatform simulates a Windows PC inside a folder, so Jigglypuff can be run,
// tested and shown anywhere without touching real files. Large files are
// sparse, so the demo uses almost no disk space.
type demoPlatform struct {
	root  string
	paths *Paths
	reg   *fakeRegistry

	mu           sync.Mutex
	recycleBytes int64
	recycleItems int64
	store        []StoreApp
	LastAction   string
}

const (
	kb = int64(1024)
	mb = 1024 * kb
	gb = 1024 * mb
)

// NewDemo builds (or rebuilds) the demo PC below root.
func NewDemo(root string) (Platform, error) {
	if root == "" {
		root = filepath.Join(os.TempDir(), "jigglypuff-demo")
	}
	root, _ = filepath.Abs(root)
	marker := filepath.Join(root, ".jigglypuff-demo")
	if entries, err := os.ReadDir(root); err == nil && len(entries) > 0 && !Exists(marker) {
		return nil, fmt.Errorf("%s is not empty and is not a Jigglypuff demo folder; refusing to use it", root)
	}
	if err := os.RemoveAll(root); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(root, 0o755); err != nil {
		return nil, err
	}
	os.WriteFile(marker, []byte("Jigglypuff demo PC. Safe to delete.\n"), 0o644)
	c := filepath.Join(root, "C")
	user := filepath.Join(c, "Users", "Kabir")
	j := filepath.Join
	p := &Paths{
		SystemDrive: c, WinDir: j(c, "Windows"), ProgramFiles: j(c, "Program Files"),
		ProgramFilesX86: j(c, "Program Files (x86)"), ProgramData: j(c, "ProgramData"),
		CommonFiles: j(c, "Program Files", "Common Files"), CommonFilesX86: j(c, "Program Files (x86)", "Common Files"),
		UserProfile: user, AppData: j(user, "AppData", "Roaming"), LocalAppData: j(user, "AppData", "Local"),
		LocalLow: j(user, "AppData", "LocalLow"), Temp: j(user, "AppData", "Local", "Temp"), Public: j(c, "Users", "Public"),
		Desktop: j(user, "Desktop"), PublicDesktop: j(c, "Users", "Public", "Desktop"), Documents: j(user, "Documents"),
		Downloads: j(user, "Downloads"), Pictures: j(user, "Pictures"), Music: j(user, "Music"), Videos: j(user, "Videos"),
		UserName: "Kabir",
	}
	p.StartMenu = j(p.AppData, "Microsoft", "Windows", "Start Menu")
	p.CommonStartMenu = j(p.ProgramData, "Microsoft", "Windows", "Start Menu")
	p.Startup = j(p.StartMenu, "Programs", "Startup")
	p.CommonStartup = j(p.CommonStartMenu, "Programs", "StartUp")
	p.JigglypuffData = j(p.LocalAppData, "Jigglypuff")
	d := &demoPlatform{root: root, paths: p, reg: newFakeRegistry(), recycleBytes: 1840 * mb, recycleItems: 214}
	if err := d.build(); err != nil {
		return nil, err
	}
	return d, nil
}

func (d *demoPlatform) Kind() string     { return "demo" }
func (d *demoPlatform) Paths() *Paths    { return d.paths }
func (d *demoPlatform) Reg() Registry    { return d.reg }
func (d *demoPlatform) IsAdmin() bool    { return os.Getenv("JIGGLYPUFF_DEMO_LIMITED") == "" }
func (d *demoPlatform) UserName() string { return d.paths.UserName }

func (d *demoPlatform) Drives() []Drive {
	return []Drive{
		{Root: `C:\`, Label: "Windows", Total: 476 * gb, Free: 61 * gb, System: true},
		{Root: `D:\`, Label: "Data", Total: 931 * gb, Free: 412 * gb},
	}
}

func (d *demoPlatform) Processes() []Process {
	return []Process{{1, "explorer.exe", ""}, {2, "chrome.exe", ""}, {3, "Discord.exe", ""}, {4, "msedge.exe", ""}}
}

// The demo keeps each drive in a folder named after its letter: <root>\C, <root>\D.
func (d *demoPlatform) DriveOf(path string) string {
	rel, err := filepath.Rel(d.root, path)
	if err != nil || rel == "." || strings.HasPrefix(rel, "..") {
		return ""
	}
	letter := strings.SplitN(rel, string(filepath.Separator), 2)[0]
	if len(letter) != 1 {
		return ""
	}
	return strings.ToUpper(letter) + `:\`
}

func (d *demoPlatform) DriveRoot(root string) string {
	if len(root) < 2 || root[1] != ':' {
		return root
	}
	return filepath.Join(d.root, strings.ToUpper(root[:1]))
}

// Only the demo's C: drive has anything in its Recycle Bin.
func demoRecycleDrive(root string) bool { return root == "" || strings.EqualFold(root, `C:\`) }

func (d *demoPlatform) RecycleBinInfo(root string) (int64, int64, error) {
	if !demoRecycleDrive(root) {
		return 0, 0, nil
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.recycleBytes, d.recycleItems, nil
}

func (d *demoPlatform) EmptyRecycleBin(root string) error {
	if !demoRecycleDrive(root) {
		return nil
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	time.Sleep(300 * time.Millisecond)
	d.recycleBytes, d.recycleItems = 0, 0
	return nil
}

func (d *demoPlatform) Recycle(paths []string) error {
	for _, p := range paths {
		if !strings.HasPrefix(p, d.root) {
			return errors.New("outside the demo PC")
		}
		size := int64(0)
		filepath.Walk(p, func(_ string, fi os.FileInfo, err error) error {
			if err == nil && !fi.IsDir() {
				size += fi.Size()
			}
			return nil
		})
		if err := os.RemoveAll(p); err != nil {
			return err
		}
		d.mu.Lock()
		d.recycleBytes += size
		d.recycleItems++
		d.mu.Unlock()
	}
	return nil
}

func (d *demoPlatform) RecycleLimit(string) (int64, bool) { return 24 * gb, false }

func (d *demoPlatform) action(s string) error {
	d.mu.Lock()
	d.LastAction = s
	d.mu.Unlock()
	return nil
}

func (d *demoPlatform) Reveal(path string) error     { return d.action("reveal " + path) }
func (d *demoPlatform) OpenFolder(path string) error { return d.action("open " + path) }
func (d *demoPlatform) Launch(target string) error   { return d.action("launch " + target) }

func (d *demoPlatform) Icon(path string, index, size int) ([]byte, error) {
	if strings.HasSuffix(strings.ToLower(path), ".png") {
		return os.ReadFile(path)
	}
	return nil, errors.New("no icon in the demo")
}

var demoFileInfo = map[string][2]string{
	"update.exe":                {"Discord Inc.", "Discord"},
	"spotify.exe":               {"Spotify AB", "Spotify"},
	"steam.exe":                 {"Valve Corporation", "Steam Client Bootstrapper"},
	"securityhealthsystray.exe": {"Microsoft Corporation", "Windows Security notification icon"},
	"rtkauduservice64.exe":      {"Realtek Semiconductor", "Realtek HD Audio Universal Service"},
	"onedrive.exe":              {"Microsoft Corporation", "Microsoft OneDrive"},
	"obs64.exe":                 {"OBS Project", "OBS Studio"},
	"zoom.exe":                  {"Zoom Video Communications, Inc.", "Zoom Workplace"},
	"figma.exe":                 {"Figma, Inc.", "Figma"},
}

func (d *demoPlatform) FileInfo(path string) (string, string) {
	if v, ok := demoFileInfo[strings.ToLower(filepath.Base(path))]; ok {
		return v[0], v[1]
	}
	return "", ""
}

// RunUninstaller simulates an uninstaller: it removes the program files and
// the uninstall entry but, like many real uninstallers, leaves settings,
// caches, shortcuts and registry keys behind.
func (d *demoPlatform) RunUninstaller(ctx context.Context, cmdline string) (int, error) {
	select {
	case <-ctx.Done():
		return -1, ctx.Err()
	case <-time.After(2500 * time.Millisecond):
	}
	for _, root := range []string{`HKLM\SOFTWARE\Microsoft\Windows\CurrentVersion\Uninstall`,
		`HKLM\SOFTWARE\WOW6432Node\Microsoft\Windows\CurrentVersion\Uninstall`, `HKCU\Software\Microsoft\Windows\CurrentVersion\Uninstall`} {
		keys, _ := d.reg.SubKeys(root)
		for _, k := range keys {
			m := ValueMap(must(d.reg.Values(root + `\` + k)))
			if Str(m, "UninstallString") != cmdline && !strings.Contains(cmdline, k) {
				continue
			}
			if loc := Str(m, "InstallLocation"); loc != "" && strings.HasPrefix(loc, d.root) {
				filepath.Walk(loc, func(p string, fi os.FileInfo, err error) error {
					if err == nil && !fi.IsDir() {
						switch strings.ToLower(filepath.Ext(p)) {
						case ".exe", ".dll", ".pak", ".bin", ".dat":
							os.Remove(p)
						}
					}
					return nil
				})
			}
			d.reg.DeleteTree(root + `\` + k)
			return 0, nil
		}
	}
	return 1603, nil
}

func (d *demoPlatform) CreateRestorePoint(string) (string, error) {
	time.Sleep(1200 * time.Millisecond)
	return "created", nil
}

func (d *demoPlatform) StoreApps() ([]StoreApp, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	return append([]StoreApp(nil), d.store...), nil
}

func (d *demoPlatform) RemoveStoreApp(full string) error {
	time.Sleep(1500 * time.Millisecond)
	d.mu.Lock()
	defer d.mu.Unlock()
	for i, s := range d.store {
		if s.FullName == full {
			d.store = append(d.store[:i], d.store[i+1:]...)
			return nil
		}
	}
	return errors.New("package not found")
}

func (d *demoPlatform) StopAndDeleteService(name string) error {
	return d.reg.DeleteTree(`HKLM\SYSTEM\CurrentControlSet\Services\` + name)
}

func (d *demoPlatform) DeleteScheduledTask(taskPath string) error {
	return os.Remove(filepath.Join(d.paths.WinDir, "System32", "Tasks", filepath.FromSlash(strings.ReplaceAll(strings.TrimPrefix(taskPath, `\`), `\`, "/"))))
}

func (d *demoPlatform) CreateShortcut(lnk, target, args, workDir, desc string) error {
	os.MkdirAll(filepath.Dir(lnk), 0o755)
	return os.WriteFile(lnk, BuildShortcut(target, args), 0o644)
}

// Building the demo PC ------------------------------------------------------

type demoBuilder struct {
	d        *demoPlatform
	rnd      *rand.Rand
	now      time.Time
	err      error
	dirTimes map[string]time.Time
}

func ago(days float64) time.Duration { return time.Duration(days * 24 * float64(time.Hour)) }

// file creates a sparse file of the given size whose first bytes depend on
// seed: files with the same size and seed are identical (duplicates).
func (b *demoBuilder) file(path string, size int64, age time.Duration, seed int64) {
	if b.err != nil {
		return
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		b.err = err
		return
	}
	f, err := os.Create(path)
	if err != nil {
		b.err = err
		return
	}
	if seed == 0 {
		seed = b.rnd.Int63()
	}
	head := make([]byte, 4096)
	rand.New(rand.NewSource(seed)).Read(head)
	if size < int64(len(head)) {
		head = head[:size]
	}
	if size > int64(len(head)) {
		setSparse(f) // so a 6 GB demo file takes no real disk space on NTFS
	}
	f.Write(head)
	if size > int64(len(head)) {
		f.Truncate(size)
	}
	f.Close()
	t := b.now.Add(-age)
	os.Chtimes(path, t, t)
}

// files spreads total bytes over n files inside dir.
func (b *demoBuilder) files(dir string, n int, total int64, maxAge time.Duration, ext string) {
	for i := 0; i < n; i++ {
		size := total / int64(n)
		size = size/2 + b.rnd.Int63n(size+1)
		age := maxAge/2 + time.Duration(b.rnd.Int63n(int64(maxAge/2)+1))
		sub := dir
		if n > 20 && i%3 == 0 {
			sub = filepath.Join(dir, fmt.Sprintf("%02x", b.rnd.Intn(16)))
		}
		b.file(filepath.Join(sub, fmt.Sprintf("f_%04d%s", i, ext)), size, age, 0)
	}
}

func (b *demoBuilder) oldDir(path string, age time.Duration) {
	os.MkdirAll(path, 0o755)
	b.dirTimes[path] = b.now.Add(-age)
}

func (b *demoBuilder) lnk(path, target string) {
	os.MkdirAll(filepath.Dir(path), 0o755)
	if err := os.WriteFile(path, BuildShortcut(target, ""), 0o644); err != nil && b.err == nil {
		b.err = err
	}
}

type demoApp struct {
	key, name, publisher, version, date, scope, arch string
	install, exe                                     string
	sizeMB                                           int64
	msi, broken                                      bool
}

func (d *demoPlatform) build() error {
	b := &demoBuilder{d: d, rnd: rand.New(rand.NewSource(42)), now: time.Now(), dirTimes: map[string]time.Time{}}
	p := d.paths
	j := filepath.Join
	r := d.reg
	pf, pf86, local, roam := p.ProgramFiles, p.ProgramFilesX86, p.LocalAppData, p.AppData

	// Installed desktop apps -------------------------------------------------
	apps := []demoApp{
		{"Google Chrome", "Google Chrome", "Google LLC", "131.0.6778.86", "20241118", "machine", "64", j(pf, "Google", "Chrome", "Application"), "chrome.exe", 612, false, false},
		{"{771FD6B0-FA20-440A-A002-3B3BAC16DC50}_is1", "Microsoft Visual Studio Code (User)", "Microsoft Corporation", "1.95.3", "20241002", "user", "", j(local, "Programs", "Microsoft VS Code"), "Code.exe", 384, false, false},
		{"Discord", "Discord", "Discord Inc.", "1.0.9171", "20240611", "user", "", j(local, "Discord"), "Update.exe", 421, false, false},
		{"Spotify", "Spotify", "Spotify AB", "1.2.50.335", "20240905", "user", "", j(roam, "Spotify"), "Spotify.exe", 318, false, false},
		{"Steam", "Steam", "Valve Corporation", "2.10.91.91", "20230214", "machine", "32", j(pf86, "Steam"), "steam.exe", 1240, false, false},
		{"7-Zip", "7-Zip 24.08 (x64)", "Igor Pavlov", "24.08", "20240812", "machine", "64", j(pf, "7-Zip"), "7zFM.exe", 6, false, false},
		{"Notepad++", "Notepad++ (64-bit x64)", "Notepad++ Team", "8.7.1", "20241020", "machine", "64", j(pf, "Notepad++"), "notepad++.exe", 15, false, false},
		{"{C2F1D5E8-4B3A-4A8B-9D11-0C1E8E3B6A21}", "Node.js", "Node.js Foundation", "22.11.0", "20241030", "machine", "64", j(pf, "nodejs"), "node.exe", 98, true, false},
		{"{9F0B6C2D-4E7A-4E0B-A3B4-5D27C2B91C11}", "Python 3.12.7 (64-bit)", "Python Software Foundation", "3.12.7150.0", "20241003", "user", "", j(local, "Programs", "Python", "Python312"), "python.exe", 112, false, false},
		{"ZoomUMX", "Zoom Workplace", "Zoom Video Communications, Inc.", "6.2.11", "20240719", "user", "", j(roam, "Zoom"), "Zoom.exe", 287, false, false},
		{"VLC media player", "VLC media player", "VideoLAN", "3.0.21", "20240616", "machine", "64", j(pf, "VideoLAN", "VLC"), "vlc.exe", 171, false, false},
		{"OBS Studio", "OBS Studio", "OBS Project", "30.2.3", "20240822", "machine", "64", j(pf, "obs-studio"), "obs64.exe", 426, false, false},
		{"WinRAR archiver", "WinRAR 7.01 (64-bit)", "win.rar GmbH", "7.01.0", "20240524", "machine", "64", j(pf, "WinRAR"), "WinRAR.exe", 9, false, false},
		{"Git_is1", "Git", "The Git Development Community", "2.47.0", "20241008", "machine", "64", j(pf, "Git"), "git-bash.exe", 342, false, false},
		{"{AC76BA86-1033-FFFF-7760-BC15014EA700}", "Adobe Acrobat Reader (64-bit)", "Adobe", "24.004.20220", "20241015", "machine", "64", j(pf, "Adobe", "Acrobat DC"), "Acrobat.exe", 640, true, false},
		{"{0E4C4A3B-7A7D-4C2C-9A4C-6B0B1A6A9E20}", "Epic Games Launcher", "Epic Games, Inc.", "1.3.128.0", "20230930", "machine", "32", j(pf86, "Epic Games"), "EpicGamesLauncher.exe", 301, true, false},
		{"Figma", "Figma", "Figma, Inc.", "124.5.6", "20241101", "user", "", j(local, "Figma"), "Figma.exe", 0, false, false},
		{"Postman-x64", "Postman x64 11.18.0", "Postman", "11.18.0", "20241025", "user", "", j(local, "Postman"), "Postman.exe", 0, false, false},
		{"OldToolbarHelper", "Toolbar Helper", "Search Tools Inc.", "2.1.0", "20210307", "machine", "32", j(pf86, "ToolbarHelper"), "helper.exe", 4, false, true},
	}
	for _, a := range apps {
		base := `HKLM\SOFTWARE\Microsoft\Windows\CurrentVersion\Uninstall`
		if a.scope == "user" {
			base = `HKCU\Software\Microsoft\Windows\CurrentVersion\Uninstall`
		} else if a.arch == "32" {
			base = `HKLM\SOFTWARE\WOW6432Node\Microsoft\Windows\CurrentVersion\Uninstall`
		}
		key := base + `\` + a.key
		r.str(key, "DisplayName", a.name)
		r.str(key, "Publisher", a.publisher)
		r.str(key, "DisplayVersion", a.version)
		r.str(key, "InstallDate", a.date)
		r.str(key, "InstallLocation", a.install)
		r.str(key, "DisplayIcon", j(a.install, a.exe)+",0")
		if a.sizeMB > 0 {
			r.dword(key, "EstimatedSize", uint64(a.sizeMB*1024))
		}
		if a.msi {
			r.dword(key, "WindowsInstaller", 1)
			r.str(key, "UninstallString", "MsiExec.exe /X"+a.key)
		} else {
			r.str(key, "UninstallString", `"`+j(a.install, "Uninstall "+strings.Split(a.name, " ")[0]+".exe")+`"`)
		}
		if !a.broken {
			b.file(j(a.install, a.exe), 3*mb, ago(30), 0)
			b.file(j(a.install, "Uninstall "+strings.Split(a.name, " ")[0]+".exe"), 600*kb, ago(30), 0)
			b.file(j(a.install, "resources.pak"), 40*mb, ago(30), 0)
			b.file(j(a.install, "settings.ini"), 2*kb, ago(5), 0)
		}
	}
	// The D: drive: one installed app, a Steam library and some portable programs.
	dd := j(d.root, "D")
	blender := j(dd, "Program Files", "Blender Foundation", "Blender 4.2")
	bkey := `HKLM\SOFTWARE\Microsoft\Windows\CurrentVersion\Uninstall\{F5C3B2A1-6D4E-4F0A-9B8C-7E6D5C4B3A21}`
	r.str(bkey, "DisplayName", "Blender 4.2")
	r.str(bkey, "Publisher", "Blender Foundation")
	r.str(bkey, "DisplayVersion", "4.2.3")
	r.str(bkey, "InstallDate", "20241012")
	r.str(bkey, "InstallLocation", blender)
	r.str(bkey, "DisplayIcon", j(blender, "blender.exe")+",0")
	r.dword(bkey, "EstimatedSize", 1100*1024)
	r.dword(bkey, "WindowsInstaller", 1)
	r.str(bkey, "UninstallString", "MsiExec.exe /X{F5C3B2A1-6D4E-4F0A-9B8C-7E6D5C4B3A21}")
	b.file(j(blender, "blender.exe"), 180*mb, ago(24), 0)
	b.file(j(blender, "4.2", "datafiles", "studiolights.dat"), 300*mb, ago(24), 0)
	steamapps := j(dd, "SteamLibrary", "steamapps", "common")
	b.file(j(steamapps, "Hades", "x64", "Hades.exe"), 30*mb, ago(70), 0)
	b.file(j(steamapps, "Hades", "Content", "Packages.pkg"), 14*gb, ago(70), 0)
	b.file(j(steamapps, "Celeste", "Celeste.exe"), 9*mb, ago(200), 0)
	b.file(j(steamapps, "Celeste", "Content", "Atlases.data"), 1200*mb, ago(200), 0)
	b.file(j(dd, "SteamLibrary", "libraryfolder.vdf"), 1*kb, ago(5), 0)
	b.file(j(dd, "PortableApps", "GIMPPortable", "GIMPPortable.exe"), 4*mb, ago(40), 0)
	b.file(j(dd, "PortableApps", "GIMPPortable", "App", "gimp", "bin", "gimp-2.10.exe"), 25*mb, ago(40), 0)
	b.file(j(dd, "PortableApps", "GIMPPortable", "App", "gimp", "share", "data.pak"), 380*mb, ago(40), 0)
	b.file(j(dd, "Tools", "ffmpeg", "bin", "ffmpeg.exe"), 130*mb, ago(15), 0)
	b.file(j(dd, "Tools", "ffmpeg", "bin", "ffprobe.exe"), 130*mb, ago(15), 0)
	b.file(j(dd, "Tools", "Rufus", "rufus-4.6.exe"), 2*mb, ago(120), 0)
	b.files(j(dd, "Photos", "2024"), 40, 6*gb, ago(90), ".jpg")
	b.file(j(dd, "Backups", "laptop-backup-2023.zip"), 9*gb, ago(400), 0)

	// Figma and Postman have no size in the registry: Jigglypuff measures them.
	b.file(j(local, "Figma", "app-124.5.6", "Figma.exe"), 160*mb, ago(10), 0)
	b.file(j(local, "Postman", "app-11.18.0", "Postman.exe"), 210*mb, ago(10), 0)
	// A system component (hidden) and an update (hidden).
	r.str(`HKLM\SOFTWARE\Microsoft\Windows\CurrentVersion\Uninstall\{VCRUNTIME}`, "DisplayName", "Microsoft Visual C++ 2015-2022 Redistributable (x64) - 14.40.33810")
	r.dword(`HKLM\SOFTWARE\Microsoft\Windows\CurrentVersion\Uninstall\{VCRUNTIME}`, "SystemComponent", 1)

	// App data, settings and shortcuts of installed apps (leftovers after uninstall).
	b.files(j(roam, "Zoom", "data"), 12, 40*mb, ago(20), ".db")
	b.files(j(local, "Zoom"), 6, 25*mb, ago(20), ".log")
	r.str(`HKCU\Software\Zoom\Settings`, "Lang", "en")
	r.str(`HKCU\Software\Zoom UMX`, "Version", "6.2.11")
	b.lnk(j(p.StartMenu, "Programs", "Zoom", "Zoom Workplace.lnk"), j(roam, "Zoom", "Zoom.exe"))
	b.lnk(j(p.Desktop, "Zoom Workplace.lnk"), j(roam, "Zoom", "Zoom.exe"))
	b.file(j(roam, "Zoom", "bin", "CptService.exe"), 2*mb, ago(20), 0)
	r.str(`HKLM\SYSTEM\CurrentControlSet\Services\ZoomCptService`, "ImagePath", `"`+j(roam, "Zoom", "bin", "CptService.exe")+`"`)
	r.dword(`HKLM\SYSTEM\CurrentControlSet\Services\ZoomCptService`, "Type", 0x10)
	r.str(`HKLM\SYSTEM\CurrentControlSet\Services\ZoomCptService`, "DisplayName", "Zoom Sharing Service")
	r.str(`HKLM\SYSTEM\CurrentControlSet\Services\Spooler`, "ImagePath", `%SystemRoot%\System32\spoolsv.exe`)
	r.dword(`HKLM\SYSTEM\CurrentControlSet\Services\Spooler`, "Type", 0x10)
	task := "\ufeff<?xml version=\"1.0\" encoding=\"UTF-16\"?><Task><Actions><Exec><Command>" + j(roam, "Zoom", "bin", "Zoom.exe") + "</Command><Arguments>--update</Arguments></Exec></Actions></Task>"
	writeUTF16(j(p.WinDir, "System32", "Tasks", "ZoomUpdateTask"), task)
	writeUTF16(j(p.WinDir, "System32", "Tasks", "Microsoft", "Windows", "Defrag", "ScheduledDefrag"), "\ufeff<Task><Command>%windir%\\system32\\defrag.exe</Command></Task>")
	b.files(j(roam, "discord"), 8, 30*mb, ago(1), ".json")
	b.files(j(roam, "Spotify", "Users"), 4, 8*mb, ago(2), ".dat")
	b.files(j(local, "Spotify", "Data"), 30, 900*mb, ago(3), ".file")
	b.files(j(local, "Steam", "htmlcache"), 10, 60*mb, ago(4), ".bin")
	r.str(`HKCU\Software\Valve\Steam`, "SteamPath", filepath.ToSlash(j(pf86, "Steam")))
	r.str(`HKCU\Software\Spotify`, "Version", "1.2.50")
	r.str(`HKCU\Software\7-Zip\FM`, "Lang", "-")
	r.str(`HKCU\Software\Notepad++`, "", "")
	r.str(`HKCU\Software\VideoLAN\VLC`, "Lang", "auto")
	b.lnk(j(p.CommonStartMenu, "Programs", "Steam", "Steam.lnk"), j(pf86, "Steam", "steam.exe"))
	b.lnk(j(p.CommonStartMenu, "Programs", "7-Zip", "7-Zip File Manager.lnk"), j(pf, "7-Zip", "7zFM.exe"))
	b.lnk(j(p.CommonStartMenu, "Programs", "VLC media player.lnk"), j(pf, "VideoLAN", "VLC", "vlc.exe"))
	b.lnk(j(p.PublicDesktop, "VLC media player.lnk"), j(pf, "VideoLAN", "VLC", "vlc.exe"))
	b.lnk(j(p.Desktop, "Discord.lnk"), j(local, "Discord", "Update.exe"))

	// Store apps.
	d.store = []StoreApp{
		{Name: "5319275A.WhatsAppDesktop", FullName: "5319275A.WhatsAppDesktop_2.2444.5.0_x64__cv1g1gvanyjgm", Family: "5319275A.WhatsAppDesktop_cv1g1gvanyjgm", Publisher: "CN=24803D75-212C-471A-BC57-9EF86AB91435", Version: "2.2444.5.0", DisplayName: "WhatsApp"},
		{Name: "Clipchamp.Clipchamp", FullName: "Clipchamp.Clipchamp_3.1.10920.0_neutral__yxz26nhyzhsrt", Family: "Clipchamp.Clipchamp_yxz26nhyzhsrt", Publisher: "CN=33F0F141-36F3-4EC2-A77D-51B53D0BA0E4, O=Microsoft Corporation", Version: "3.1.10920.0", DisplayName: "Microsoft Clipchamp"},
		{Name: "Microsoft.MicrosoftSolitaireCollection", FullName: "Microsoft.MicrosoftSolitaireCollection_4.21.9100.0_x64__8wekyb3d8bbwe", Family: "Microsoft.MicrosoftSolitaireCollection_8wekyb3d8bbwe", Publisher: "CN=Microsoft Corporation, O=Microsoft Corporation", Version: "4.21.9100.0", DisplayName: "Solitaire & Casual Games"},
		{Name: "Microsoft.GamingApp", FullName: "Microsoft.GamingApp_2410.1001.12.0_x64__8wekyb3d8bbwe", Family: "Microsoft.GamingApp_8wekyb3d8bbwe", Publisher: "CN=Microsoft Corporation, O=Microsoft Corporation", Version: "2410.1001.12.0", DisplayName: "Xbox"},
		{Name: "Microsoft.Todos", FullName: "Microsoft.Todos_2.140.73481.0_x64__8wekyb3d8bbwe", Family: "Microsoft.Todos_8wekyb3d8bbwe", Publisher: "CN=Microsoft Corporation, O=Microsoft Corporation", Version: "2.140.73481.0", DisplayName: "Microsoft To Do"},
		{Name: "SpotifyAB.SpotifyMusic", FullName: "SpotifyAB.SpotifyMusic_1.250.0.0_x64__zpdnekdrzrea0", Family: "SpotifyAB.SpotifyMusic_zpdnekdrzrea0", Publisher: "CN=453637B3-4E12-4CDF-B0D3-2A3C863BF6EF", Version: "1.250.0.0", DisplayName: "Spotify Music"},
	}

	// Startup entries.
	run := `HKCU\Software\Microsoft\Windows\CurrentVersion\Run`
	r.str(run, "Discord", `"`+j(local, "Discord", "Update.exe")+`" --processStart Discord.exe`)
	r.str(run, "Spotify", `"`+j(roam, "Spotify", "Spotify.exe")+`" /minimized`)
	r.str(run, "Steam", `"`+j(pf86, "Steam", "steam.exe")+`" -silent`)
	r.str(run, "OneDrive", `"`+j(local, "Microsoft", "OneDrive", "OneDrive.exe")+`" /background`)
	r.str(run, "OldUpdater", `"`+j(pf86, "OldApp", "updater.exe")+`" /check`)
	r.set(`HKCU\Software\Microsoft\Windows\CurrentVersion\Explorer\StartupApproved\Run`, RegValue{Name: "Spotify", Type: RegBinary, B: []byte{3, 0, 0, 0, 1, 2, 3, 4, 5, 6, 7, 8}})
	hrun := `HKLM\SOFTWARE\Microsoft\Windows\CurrentVersion\Run`
	r.str(hrun, "SecurityHealth", j(p.WinDir, "System32", "SecurityHealthSystray.exe"))
	r.str(hrun, "RtkAudUService", `"`+j(p.WinDir, "System32", "DriverStore", "RtkAudUService64.exe")+`" -background`)
	b.file(j(local, "Microsoft", "OneDrive", "OneDrive.exe"), 2*mb, ago(9), 0)
	b.file(j(p.WinDir, "System32", "SecurityHealthSystray.exe"), 100*kb, ago(90), 0)
	b.file(j(p.WinDir, "System32", "DriverStore", "RtkAudUService64.exe"), 1*mb, ago(90), 0)
	b.file(j(pf, "obs-studio", "bin", "64bit", "obs64.exe"), 4*mb, ago(30), 0)
	b.lnk(j(p.Startup, "OBS Studio.lnk"), j(pf, "obs-studio", "bin", "64bit", "obs64.exe"))
	b.file(j(roam, "Spotify", "Spotify.exe"), 30*mb, ago(10), 0)

	// Junk ---------------------------------------------------------------------
	b.files(p.Temp, 240, 2140*mb, ago(120), ".tmp")
	b.files(j(p.Temp, "recent"), 30, 200*mb, ago(0.4), ".tmp") // newer than 24h: kept
	b.files(j(p.WinDir, "Temp"), 60, 380*mb, ago(90), ".tmp")
	b.files(j(p.WinDir, "SoftwareDistribution", "Download"), 14, 1620*mb, ago(40), ".cab")
	b.files(j(p.WinDir, "ServiceProfiles", "NetworkService", "AppData", "Local", "Microsoft", "Windows", "DeliveryOptimization", "Cache"), 20, 910*mb, ago(20), "")
	b.files(j(local, "Microsoft", "Windows", "WER", "ReportArchive"), 12, 85*mb, ago(60), ".wer")
	b.files(j(local, "CrashDumps"), 5, 420*mb, ago(45), ".dmp")
	b.files(j(p.WinDir, "Logs", "CBS"), 40, 160*mb, ago(200), ".log")
	b.files(j(p.WinDir, "Logs", "DISM"), 4, 6*mb, ago(2), ".log") // recent logs: kept
	for _, t := range []string{"thumbcache_256.db", "thumbcache_1280.db", "thumbcache_idx.db"} {
		b.file(j(local, "Microsoft", "Windows", "Explorer", t), 70*mb, ago(1), 0)
	}
	b.files(j(local, "Microsoft", "Windows", "INetCache", "IE"), 50, 95*mb, ago(100), ".dat")
	b.files(j(local, "D3DSCache"), 30, 410*mb, ago(50), "")
	b.files(j(local, "NVIDIA", "DXCache"), 30, 350*mb, ago(50), ".bin")
	chrome := j(local, "Google", "Chrome", "User Data")
	b.files(j(chrome, "Default", "Cache", "Cache_Data"), 120, 1210*mb, ago(30), "")
	b.files(j(chrome, "Default", "Code Cache", "js"), 60, 310*mb, ago(30), "")
	b.files(j(chrome, "Default", "Service Worker", "CacheStorage"), 30, 520*mb, ago(30), "")
	b.files(j(chrome, "Profile 1", "Cache", "Cache_Data"), 50, 380*mb, ago(30), "")
	b.files(j(chrome, "GrShaderCache"), 6, 20*mb, ago(30), "")
	b.file(j(chrome, "Default", "Bookmarks"), 200*kb, ago(1), 0)
	edge := j(local, "Microsoft", "Edge", "User Data")
	b.files(j(edge, "Default", "Cache", "Cache_Data"), 60, 610*mb, ago(30), "")
	b.files(j(local, "Mozilla", "Firefox", "Profiles", "x8f2k1.default-release", "cache2", "entries"), 60, 405*mb, ago(30), "")
	b.files(j(roam, "discord", "Cache", "Cache_Data"), 60, 620*mb, ago(10), "")
	b.files(j(roam, "discord", "Code Cache", "js"), 20, 80*mb, ago(10), "")
	b.files(j(roam, "Code", "Cache", "Cache_Data"), 30, 140*mb, ago(10), "")
	b.files(j(roam, "Code", "CachedExtensionVSIXs"), 12, 150*mb, ago(40), ".vsix")
	b.files(j(roam, "Code", "logs"), 40, 42*mb, ago(40), ".log")
	b.files(j(roam, "Slack", "Cache", "Cache_Data"), 30, 240*mb, ago(10), "")
	b.files(j(local, "Packages", "Microsoft.WindowsStore_8wekyb3d8bbwe", "AC", "INetCache"), 20, 64*mb, ago(10), "")
	b.files(j(pf86, "Steam", "appcache", "httpcache"), 30, 230*mb, ago(10), "")
	b.files(j(pf86, "Steam", "logs"), 20, 35*mb, ago(10), ".txt")
	// Developer caches.
	b.files(j(local, "npm-cache", "_cacache", "content-v2"), 150, 1930*mb, ago(200), "")
	b.files(j(local, "pip", "cache", "http-v2"), 60, 640*mb, ago(200), "")
	b.files(j(local, "NuGet", "v3-cache"), 30, 300*mb, ago(100), ".dat")
	b.files(j(p.UserProfile, ".nuget", "packages"), 80, 2400*mb, ago(100), ".nupkg")
	b.files(j(p.UserProfile, ".gradle", "caches"), 80, 3100*mb, ago(100), ".jar")
	b.files(j(local, "go-build"), 100, 910*mb, ago(50), "")
	b.files(j(p.UserProfile, ".cargo", "registry", "cache"), 40, 450*mb, ago(50), ".crate")
	b.files(j(local, "Yarn", "Cache"), 50, 700*mb, ago(150), ".zip")
	b.files(j(local, "ms-playwright", "chromium-1140"), 10, 1100*mb, ago(150), "")
	// Broken shortcuts.
	b.lnk(j(p.Desktop, "Old Game.lnk"), j(p.SystemDrive, "Games", "OldGame", "game.exe"))
	b.lnk(j(p.StartMenu, "Programs", "Toolbar Helper", "Toolbar Helper.lnk"), j(pf86, "ToolbarHelper", "helper.exe"))
	b.lnk(j(p.PublicDesktop, "Trial Antivirus.lnk"), j(pf, "TrialAV", "av.exe"))

	// Leftovers of apps that were uninstalled long ago -------------------------------
	b.files(j(roam, "Skype"), 20, 184*mb, ago(700), ".db")
	b.oldDir(j(roam, "Skype"), ago(700))
	r.str(`HKCU\Software\Skype\Phone`, "Lang", "en")
	r.touch(`HKCU\Software\Skype`, b.now.Add(-ago(700)))
	b.files(j(p.ProgramData, "BlueStacks_nxt"), 30, 2140*mb, ago(400), ".vdi")
	b.oldDir(j(p.ProgramData, "BlueStacks_nxt"), ago(400))
	b.files(j(local, "BlueStacks X"), 10, 160*mb, ago(400), ".dat")
	b.oldDir(j(local, "BlueStacks X"), ago(400))
	r.str(`HKLM\SOFTWARE\BlueStacks_nxt`, "InstallDir", "")
	r.touch(`HKLM\SOFTWARE\BlueStacks_nxt`, b.now.Add(-ago(400)))
	b.files(j(p.ProgramData, "AVAST Software"), 40, 340*mb, ago(520), ".def")
	b.oldDir(j(p.ProgramData, "AVAST Software"), ago(520))
	b.files(j(pf, "Avast Software", "Avast"), 10, 90*mb, ago(520), ".dll")
	b.oldDir(j(pf, "Avast Software"), ago(520))
	b.files(j(local, "JetBrains", "PyCharm2023.1"), 30, 610*mb, ago(380), ".log")
	b.oldDir(j(local, "JetBrains"), ago(380))
	b.files(j(roam, "JetBrains", "PyCharm2023.1"), 10, 12*mb, ago(380), ".xml")
	b.oldDir(j(roam, "JetBrains"), ago(380))
	b.oldDir(j(roam, "WinZip"), ago(900))
	b.files(j(local, "CCleaner"), 3, 2*mb, ago(95), ".log")
	b.oldDir(j(local, "CCleaner"), ago(95))
	b.files(j(roam, "npm", "node_modules"), 20, 60*mb, ago(300), ".js") // global npm packages: never a leftover
	b.files(j(roam, "SomeTool"), 3, 1*mb, ago(2), ".cfg")               // used two days ago: kept

	// Large & old files ---------------------------------------------------------------
	b.file(j(p.Downloads, "Win11_24H2_English_x64.iso"), 5800*mb, ago(140), 0)
	b.file(j(p.Downloads, "ubuntu-24.04.1-desktop-amd64.iso"), 6030*mb, ago(95), 0)
	b.file(j(p.Downloads, "project-backup-2023.zip"), 2310*mb, ago(410), 0)
	b.file(j(p.Downloads, "OBS-Studio-30.2-Full-Installer-x64.exe"), 142*mb, ago(75), 0)
	b.file(j(p.Downloads, "BlueStacksInstaller_5.21.exe"), 2*mb, ago(400), 0)
	b.file(j(p.Downloads, "vlc-3.0.21-win64.exe"), 44*mb, ago(140), 0)
	b.file(j(p.Downloads, "node-v22.11.0-x64.msi"), 30*mb, ago(6), 0)
	b.file(j(p.Downloads, "assets_v2.rar"), 820*mb, ago(70), 0)
	b.file(j(p.Videos, "Captures", "Gameplay 2024-03-11.mp4"), 7900*mb, ago(240), 0)
	b.file(j(p.Videos, "Screen Recording 2024-06-02.mp4"), 3400*mb, ago(150), 0)
	b.file(j(p.UserProfile, "VirtualBox VMs", "Ubuntu", "Ubuntu.vdi"), 18200*mb, ago(60), 0)
	b.file(j(p.Desktop, "renders.zip"), 1130*mb, ago(30), 0)
	b.file(j(p.Documents, "Thesis", "dataset_full.csv"), 640*mb, ago(200), 0)
	b.file(j(p.Music, "Recordings", "jam-session.wav"), 410*mb, ago(300), 0)

	// Duplicates (same size and seed = same content).
	dup := func(size int64, seed int64, paths ...string) {
		for _, x := range paths {
			b.file(x, size, ago(float64(30+b.rnd.Intn(300))), seed)
		}
	}
	dup(4200*kb, 11, j(p.Pictures, "Camera Roll", "IMG_2041.jpg"), j(p.Downloads, "IMG_2041.jpg"), j(p.Desktop, "Photos backup", "IMG_2041.jpg"))
	dup(3100*kb, 12, j(p.Pictures, "Wallpapers", "mountains.png"), j(p.Downloads, "mountains.png"), j(p.Downloads, "mountains (1).png"), j(p.Desktop, "mountains.png"))
	dup(7600*kb, 13, j(p.Music, "track01.mp3"), j(p.Downloads, "track01.mp3"))
	dup(1300*kb, 14, j(p.Documents, "Resume_Kabir.pdf"), j(p.Downloads, "Resume_Kabir (1).pdf"))
	dup(210*mb, 15, j(p.Videos, "holiday.mp4"), j(p.Desktop, "Photos backup", "holiday.mp4"))
	dup(6200*kb, 16, j(p.Downloads, "setup.exe"), j(p.Downloads, "setup (1).exe"))
	dup(2600*kb, 17, j(p.Pictures, "Screenshots", "Screenshot 2024-10-02.png"), j(p.Documents, "Screenshot 2024-10-02.png"))
	dup(4200*kb, 18, j(p.Pictures, "Camera Roll", "IMG_2042.jpg")) // same size, different content: not a duplicate

	for _, dir := range []string{p.Desktop, p.Documents, p.Downloads, p.Pictures, p.Music, p.Videos, p.Startup, p.CommonStartup, p.JigglypuffData} {
		os.MkdirAll(dir, 0o755)
	}
	for dir, t := range b.dirTimes {
		filepath.Walk(dir, func(path string, fi os.FileInfo, err error) error {
			if err == nil && fi.IsDir() {
				os.Chtimes(path, t, t)
			}
			return nil
		})
	}
	return b.err
}

func writeUTF16(path, s string) {
	os.MkdirAll(filepath.Dir(path), 0o755)
	var b []byte
	for _, r := range s {
		if r > 0xFFFF {
			r = '?'
		}
		b = append(b, byte(r), byte(r>>8))
	}
	if strings.HasPrefix(s, "\ufeff") {
		b[0], b[1] = 0xFF, 0xFE
	}
	os.WriteFile(path, b, 0o644)
}

func must(v []RegValue, _ error) []RegValue { return v }
