package engine

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"jigglypuff/internal/platform"
)

func demoEngine(t *testing.T) *Engine {
	t.Helper()
	p, err := platform.NewDemo(filepath.Join(t.TempDir(), "pc"))
	if err != nil {
		t.Fatal(err)
	}
	return New(p, DefaultSettings())
}

func noProgress(Progress) {}

func TestJunkScanAndClean(t *testing.T) {
	e := demoEngine(t)
	res, err := e.ScanJunk(context.Background(), noProgress)
	if err != nil {
		t.Fatal(err)
	}
	byID := map[string]*JunkResult{}
	var total int64
	for _, r := range res {
		byID[r.ID] = r
		total += r.Size
		t.Logf("%-18s %-28s %8.1f MB  %5d files  skipped=%d running=%v", r.Group, r.Name, float64(r.Size)/1e6, r.Files, r.Skipped, r.Running)
	}
	t.Logf("total %.2f GB", float64(total)/1e9)
	for _, id := range []string{"user-temp", "win-temp", "wu-cache", "browser-chrome", "browser-edge", "browser-firefox", "app-caches", "dev-npm", "broken-shortcuts", "recycle-bin", "dev-gradle"} {
		if byID[id] == nil {
			t.Errorf("category %s missing", id)
		}
	}
	if r := byID["user-temp"]; r == nil || r.Skipped < 30 {
		t.Errorf("recent temp files should be skipped, got %d", r.Skipped)
	}
	if r := byID["broken-shortcuts"]; r == nil || r.Files != 3 {
		t.Errorf("expected 3 broken shortcuts, got %d", r.Files)
	}
	if r := byID["browser-chrome"]; r == nil || len(r.Running) == 0 {
		t.Errorf("chrome should be reported running")
	}
	// Clean temp + chrome cache.
	var targets []Target
	for _, id := range []string{"user-temp", "browser-chrome"} {
		for _, f := range byID[id].Items {
			targets = append(targets, Target{Kind: "file", Path: f.Path, Size: f.Size, Purpose: PurposeJunk, Root: f.Root, MinAge: f.MinAge})
		}
	}
	out := e.Execute(context.Background(), targets, filepath.Join(t.TempDir(), "bk"), noProgress)
	t.Logf("clean: %+v", out)
	if out.Removed != len(targets) || out.Failed != 0 {
		t.Errorf("expected all %d removed, got %+v", len(targets), out)
	}
	p := e.P.Paths()
	if !platform.IsDir(p.Temp) {
		t.Errorf("Temp folder itself must stay")
	}
	if _, err := os.Stat(filepath.Join(p.LocalAppData, "Google", "Chrome", "User Data", "Default", "Bookmarks")); err != nil {
		t.Errorf("bookmarks must not be touched: %v", err)
	}
	recent := 0
	filepath.Walk(filepath.Join(p.Temp, "recent"), func(_ string, fi os.FileInfo, err error) error {
		if err == nil && !fi.IsDir() {
			recent++
		}
		return nil
	})
	if recent != 30 {
		t.Errorf("recent temp files must stay, found %d", recent)
	}
}

func TestAppsAndUninstallLeftovers(t *testing.T) {
	e := demoEngine(t)
	apps := e.InstalledApps()
	var zoom, toolbar *App
	for _, a := range apps {
		t.Logf("%-40s %-32s %-10s size=%dMB broken=%v msi=%v cmd=%s", a.Name, a.Publisher, a.Version, a.Size>>20, a.Broken, a.MSI, a.UninstallCommand())
		switch a.Name {
		case "Zoom Workplace":
			zoom = a
		case "Toolbar Helper":
			toolbar = a
		}
		if strings.Contains(a.Name, "Redistributable") {
			t.Errorf("system components must be hidden")
		}
	}
	if zoom == nil || toolbar == nil || !toolbar.Broken {
		t.Fatalf("expected Zoom and a broken Toolbar Helper")
	}
	sizes := e.AppSizes(context.Background(), apps, 5e9)
	t.Logf("measured sizes: %d", len(sizes))
	r := e.Uninstall(context.Background(), zoom, true, noProgress)
	t.Logf("uninstall: %+v", r)
	if r.Status != "removed" {
		t.Fatalf("uninstall failed: %+v", r)
	}
	left := e.AppLeftovers(context.Background(), zoom, false, noProgress)
	kinds := map[string]int{}
	for _, l := range left {
		kinds[l.Kind]++
		t.Logf("  %-8s %-6s %8dKB %s %s  (%s)", l.Kind, l.Confidence, l.Size>>10, l.Path, l.Name, l.Reason)
	}
	for _, k := range []string{"folder", "regkey", "shortcut", "service", "task"} {
		if kinds[k] == 0 {
			t.Errorf("expected a %s leftover for Zoom", k)
		}
	}
	for _, l := range left {
		if strings.Contains(l.Path, "Spotify") || strings.Contains(l.Path, "Steam") {
			t.Errorf("leftover belongs to another app: %s", l.Path)
		}
	}
	// Delete them all and make sure only those went.
	var targets []Target
	for _, l := range left {
		targets = append(targets, Target{Kind: l.Kind, Path: l.Path, Name: l.Name, Size: l.Size, Purpose: PurposeLeftover, Recycle: l.Kind != "regkey"})
	}
	res := e.Execute(context.Background(), targets, filepath.Join(t.TempDir(), "bk"), noProgress)
	t.Logf("clean: %+v", res)
	if res.Failed != 0 {
		t.Errorf("failures: %v", res.Errors)
	}
	if !e.P.Reg().Exists(`HKCU\Software\Spotify`) {
		t.Errorf("Spotify settings must stay")
	}
	if e.P.Reg().Exists(`HKCU\Software\Zoom`) {
		t.Errorf("Zoom key should be gone")
	}
	// Force remove the broken app.
	fl := e.AppLeftovers(context.Background(), toolbar, true, noProgress)
	for _, l := range fl {
		t.Logf("  force: %-8s %-6s %s (%s)", l.Kind, l.Confidence, l.Path, l.Reason)
	}
	if len(fl) < 2 {
		t.Errorf("force remove should find install folder + uninstall entry")
	}
}

func TestOrphans(t *testing.T) {
	e := demoEngine(t)
	groups := e.OrphanLeftovers(context.Background(), noProgress)
	names := map[string]bool{}
	for _, g := range groups {
		names[g.Key] = true
		t.Logf("%-16s %-6s %8.1f MB last=%s", g.Name, g.Confidence, float64(g.Size)/1e6, g.LastUsed.Format("2006-01-02"))
		for _, l := range g.Items {
			t.Logf("     %-7s %-6s %s (%s)", l.Kind, l.Confidence, l.Path, l.Reason)
		}
	}
	for _, want := range []string{"skype", "bluestacksnxt", "avastsoftware", "jetbrains", "winzip"} {
		if !names[want] {
			t.Errorf("expected orphan group %s", want)
		}
	}
	for _, never := range []string{"npm", "sometool", "zoom", "spotify", "discord", "code", "steam", "google"} {
		if names[never] {
			t.Errorf("%s must not be reported as a leftover", never)
		}
	}
}

func TestFilesAndStartup(t *testing.T) {
	e := demoEngine(t)
	large, err := e.LargeFiles(context.Background(), "", 200<<20, false, noProgress)
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range large {
		t.Logf("large %8d MB %s", f.Size>>20, f.Path)
	}
	if len(large) < 8 {
		t.Errorf("expected large files, got %d", len(large))
	}
	old := e.OldDownloads(context.Background(), 60)
	for _, f := range old {
		t.Logf("old   %8d MB %s", f.Size>>20, f.Name)
	}
	for _, f := range old {
		if f.Name == "node-v22.11.0-x64.msi" {
			t.Errorf("recent download must not be listed")
		}
	}
	p := e.P.Paths()
	dups, err := e.Duplicates(context.Background(), []string{p.Desktop, p.Documents, p.Downloads, p.Pictures, p.Music, p.Videos}, 1<<20, noProgress)
	if err != nil {
		t.Fatal(err)
	}
	for _, g := range dups {
		var names []string
		for _, f := range g.Files {
			names = append(names, f.Path[len(p.UserProfile):])
		}
		t.Logf("dup %6d KB x%d  %v", g.Size>>10, len(g.Files), names)
	}
	if len(dups) != 7 {
		t.Errorf("expected 7 duplicate groups, got %d", len(dups))
	}
	items := e.StartupItems()
	for _, it := range items {
		t.Logf("startup %-40s enabled=%-5v missing=%-5v %s", it.Name, it.Enabled, it.Missing, it.Command)
	}
	var spotify *StartupItem
	for _, it := range items {
		if it.Entry == "Spotify" {
			spotify = it
		}
	}
	if spotify == nil || spotify.Enabled {
		t.Fatalf("Spotify should be listed as disabled")
	}
	if err := e.SetStartupEnabled(spotify.ID, true); err != nil {
		t.Fatal(err)
	}
	for _, it := range e.StartupItems() {
		if it.Entry == "Spotify" && !it.Enabled {
			t.Errorf("Spotify should be enabled now")
		}
	}
}

func TestGuard(t *testing.T) {
	p := &platform.Paths{SystemDrive: `/c`, WinDir: "/c/Windows", ProgramFiles: "/c/Program Files", ProgramFilesX86: "/c/Program Files (x86)",
		ProgramData: "/c/ProgramData", CommonFiles: "/c/Program Files/Common Files", UserProfile: "/c/Users/k",
		AppData: "/c/Users/k/AppData/Roaming", LocalAppData: "/c/Users/k/AppData/Local", LocalLow: "/c/Users/k/AppData/LocalLow",
		Temp: "/c/Users/k/AppData/Local/Temp", Desktop: "/c/Users/k/Desktop", Documents: "/c/Users/k/Documents",
		Downloads: "/c/Users/k/Downloads", StartMenu: "/c/Users/k/AppData/Roaming/Microsoft/Windows/Start Menu",
		CommonStartMenu: "/c/ProgramData/Microsoft/Windows/Start Menu", JigglypuffData: "/c/Users/k/AppData/Local/Jigglypuff"}
	g := NewGuard(p, []string{"/c/Users/k/AppData/Roaming/Keep"}, "")
	type tc struct {
		path, purpose, root string
		ok                  bool
	}
	for _, c := range []tc{
		{"/c/Users/k/AppData/Local/Temp/x.tmp", PurposeJunk, p.Temp, true},
		{"/c/Users/k/AppData/Local/Temp", PurposeJunk, p.Temp, false},
		{"/c/Users/k/AppData/Local", PurposeJunk, p.Temp, false},
		{"/c/Windows/Temp/a.tmp", PurposeJunk, "/c/Windows/Temp", true},
		{"/c/Windows/notepad.exe", PurposeJunk, "/c/Windows", false},
		{"/c/Windows/System32/x.log", PurposeJunk, "/c/Windows/System32", false},
		{"/c/Users/k/AppData/Roaming/Zoom", PurposeLeftover, "", true},
		{"/c/Users/k/AppData/Roaming/Keep/sub", PurposeLeftover, "", false},
		{"/c/Users/k/AppData/Roaming/Microsoft/Word", PurposeLeftover, "", false},
		{"/c/Users/k/AppData/Roaming/Microsoft/Windows/Start Menu/Programs/Zoom", PurposeLeftover, "", true},
		{"/c/Users/k/Documents/Zoom", PurposeLeftover, "", false},
		{"/c/Program Files/Zoom", PurposeLeftover, "", true},
		{"/c/Program Files", PurposeLeftover, "", false},
		{"/c/Program Files/WindowsApps/x", PurposeLeftover, "", false},
		{"/c/Users/k/.zoom", PurposeLeftover, "", true},
		{"/c/Users/k/Projects", PurposeLeftover, "", false},
		{"/c/Users/k", PurposeLeftover, "", false},
		{"/c/Users/k/Downloads/big.iso", PurposeUserFile, "", true},
		{"/c/Program Files/App/big.dat", PurposeUserFile, "", false},
		{"/c/Users/k/AppData/Local/Jigglypuff/settings.json", PurposeUserFile, "", false},
	} {
		err := g.CheckPath(c.path, c.purpose, c.root)
		if (err == nil) != c.ok {
			t.Errorf("CheckPath(%s, %s) = %v, want ok=%v", c.path, c.purpose, err, c.ok)
		}
	}
	for key, ok := range map[string]bool{
		`HKCU\Software\Zoom`: true, `HKCU\Software`: false, `HKCU\Software\Microsoft`: false,
		`HKCU\Software\Microsoft\Office`: false, `HKLM\SOFTWARE\Khronos`: false, `HKLM\SOFTWARE\WOW6432Node`: false,
		`HKLM\SOFTWARE\WOW6432Node\Zoom`: true, `HKLM\SYSTEM\CurrentControlSet`: false,
		`HKLM\SOFTWARE\Microsoft\Windows\CurrentVersion\Uninstall\Zoom`: true,
		`HKLM\SOFTWARE\Microsoft\Windows\CurrentVersion\Uninstall`:      false,
		`HKLM\SOFTWARE\Microsoft\Windows\CurrentVersion\Run`:            false,
	} {
		if err := g.CheckRegKey(key); (err == nil) != ok {
			t.Errorf("CheckRegKey(%s) = %v, want ok=%v", key, err, ok)
		}
	}
	if g.CheckTask(`\Microsoft\Windows\Defrag\ScheduledDefrag`) == nil || g.CheckTask(`\ZoomUpdateTask`) != nil {
		t.Errorf("task guard wrong")
	}
	if g.CheckService("Spooler", `%SystemRoot%\System32\spoolsv.exe`) == nil {
		t.Errorf("windows services must be protected")
	}
}

func TestNamesAndShortcuts(t *testing.T) {
	cases := map[string][]string{
		"Notepad++ (64-bit x64)|Notepad++ Team":                     {"notepad"},
		"Zoom Workplace|Zoom Video Communications, Inc.":            {"zoomworkplace", "workplace"},
		"Mozilla Firefox (x64 en-US)|Mozilla":                       {"mozillafirefox", "firefox"},
		"Python 3.12.7 (64-bit)|Python Software Foundation":         {"python"},
		"Microsoft Visual Studio Code (User)|Microsoft Corporation": {"visualstudiocode"},
	}
	for in, want := range cases {
		parts := strings.SplitN(in, "|", 2)
		got := AppTokens(parts[0], parts[1], "", nil)
		for _, w := range want {
			found := false
			for _, g := range got {
				if g == w {
					found = true
				}
			}
			if !found {
				t.Errorf("AppTokens(%q) = %v, missing %q", parts[0], got, w)
			}
		}
	}
	for _, tok := range AppTokens("Google Chrome", "Google LLC", filepath.Join("pf", "Google", "Chrome", "Application"), []string{filepath.Join("pf", "Google", "Chrome", "Application", "chrome.exe")}) {
		if tok == "google" {
			t.Errorf("the vendor name must never be an app token (it would match the shared Google folder)")
		}
	}
	zt := AppTokens("Zoom Workplace", "Zoom Video Communications, Inc.", filepath.Join("users", "k", "Roaming", "Zoom"), []string{filepath.Join("users", "k", "Roaming", "Zoom", "bin", "Zoom.exe")})
	if MatchName("Zoom", zt) != MatchStrong {
		t.Errorf("Zoom's folder should match via its install folder and exe: %v", zt)
	}
	if MatchName("PyCharm2023.1", []string{"pycharm"}) != MatchStrong {
		t.Errorf("versioned folder should match strongly")
	}
	if MatchName("Code", []string{"zoom"}) != MatchNone {
		t.Errorf("unrelated names must not match")
	}
	exe, args := platform.ParseCommandLine(`C:\Program Files\App Name\uninst.exe /S /x`)
	if exe != `C:\Program Files\App Name\uninst.exe` || args != "/S /x" {
		t.Errorf("ParseCommandLine: %q %q", exe, args)
	}
	exe, args = platform.ParseCommandLine(`"C:\Program Files\App\unins000.exe" /SILENT`)
	if exe != `C:\Program Files\App\unins000.exe` || args != "/SILENT" {
		t.Errorf("ParseCommandLine quoted: %q %q", exe, args)
	}
	ip, idx := platform.ParseIconLocation(`"C:\Program Files\App\app.exe",-101`)
	if ip != `C:\Program Files\App\app.exe` || idx != -101 {
		t.Errorf("ParseIconLocation: %q %d", ip, idx)
	}
	s := parseLnk(platform.BuildShortcut(`C:\Games\Old\game.exe`, "-fullscreen"))
	if !s.Known || s.Target != `C:\Games\Old\game.exe` || s.Args != "-fullscreen" {
		t.Errorf("shortcut round trip: %+v", s)
	}
}
