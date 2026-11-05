package engine

import (
	"context"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"jigglypuff/internal/platform"
)

// OnDrive reports whether path is on drive (like D:\). An empty drive means
// every drive.
func (e *Engine) OnDrive(path, drive string) bool {
	if drive == "" {
		return true
	}
	return path != "" && strings.EqualFold(e.P.DriveOf(path), drive)
}

// appDrive works out which drive an app is installed on: its install folder,
// or else the folder of its program or icon. Windows' own tools (msiexec) don't count.
func (e *Engine) appDrive(a *App) string {
	win := e.P.Paths().WinDir
	for _, p := range append([]string{a.InstallLocation, a.IconPath}, a.Exes...) {
		if p == "" || !filepath.IsAbs(p) || platform.Within(p, win) {
			continue
		}
		if d := e.P.DriveOf(p); d != "" {
			return d
		}
	}
	return ""
}

// Program is software found in a folder on a drive that isn't in Windows'
// list of installed apps: portable programs, games, tools unzipped by hand.
type Program struct {
	ID       string    `json:"id"`
	Name     string    `json:"name"`
	Kind     string    `json:"kind"` // program | game
	Path     string    `json:"path"`
	Drive    string    `json:"drive"`
	Exe      string    `json:"exe,omitempty"`
	Size     int64     `json:"size"`
	Partial  bool      `json:"partial,omitempty"` // size is a lower bound
	Modified time.Time `json:"modified"`
}

// Top-level folders that hold Windows itself or user data, never programs.
var notProgramRoots = map[string]bool{
	"windows": true, "users": true, "programdata": true, "program files": true, "program files (x86)": true,
	"system volume information": true, "recovery": true, "perflogs": true, "boot": true, "efi": true,
	"onedrivetemp": true, "windows.old": true, "msocache": true, "intel": true, "amd": true, "nvidia": true,
	"drivers": true, "config.msi": true, "documents and settings": true, "inetpub": true,
}

// Folders inside Program Files that are part of Windows.
var windowsProgramDirs = map[string]bool{
	"common files": true, "windowsapps": true, "modifiablewindowsapps": true, "internet explorer": true,
	"reference assemblies": true, "uninstall information": true, "msbuild": true, "windowspowershell": true,
	"microsoft.net": true, "windows nt": true, "windows mail": true, "windows media player": true,
	"windows photo viewer": true, "windows portable devices": true, "windows security": true,
	"windows defender": true, "windows defender advanced threat protection": true, "windows sidebar": true,
	"windows multimedia platform": true, "windows journal": true, "package cache": true,
}

// ProgramsOnDrive finds software on drive that Windows' installed-apps list
// doesn't know about. Folders of the apps in known are left out, so the two
// lists together show everything installed on the drive. It never changes anything.
func (e *Engine) ProgramsOnDrive(ctx context.Context, drive string, known []*App, progress ProgressFunc) []*Program {
	root := e.P.DriveRoot(drive)
	if root == "" || !platform.IsDir(root) {
		return nil
	}
	var knownDirs []string
	win := e.P.Paths().WinDir
	for _, a := range known {
		if a.InstallLocation != "" && len(platform.Clean(a.InstallLocation)) > 3 {
			knownDirs = append(knownDirs, a.InstallLocation)
		}
		for _, x := range append([]string{a.IconPath}, a.Exes...) {
			if x != "" && filepath.IsAbs(x) && !platform.Within(x, win) {
				knownDirs = append(knownDirs, filepath.Dir(x))
			}
		}
	}
	isKnown := func(dir string) bool {
		for _, k := range knownDirs {
			if platform.Within(dir, k) || platform.Within(k, dir) {
				return true
			}
		}
		return false
	}

	type cand struct{ dir, kind string }
	var cands []cand
	seen := map[string]bool{}
	add := func(dir, kind string) {
		key := strings.ToLower(platform.Clean(dir))
		if seen[key] {
			return
		}
		if kind == "game" {
			// Games live inside their launcher's folder; only skip a game
			// that has its own entry in the installed-apps list.
			for _, k := range knownDirs {
				if platform.Within(k, dir) {
					return
				}
			}
		} else if isKnown(dir) {
			return
		}
		seen[key] = true
		cands = append(cands, cand{dir, kind})
	}
	// Games in Steam and Xbox libraries: every folder in there is one game.
	games := func(dir string) bool {
		lib := filepath.Join(dir, "steamapps", "common")
		if !platform.IsDir(lib) {
			if !strings.EqualFold(filepath.Base(dir), "XboxGames") {
				return false
			}
			lib = dir
		}
		for _, g := range listDirs(lib) {
			add(filepath.Join(lib, g), "game")
		}
		return true
	}

	progress(Progress{Phase: "software", Message: "Looking through " + drive})
	for _, pf := range []string{"Program Files", "Program Files (x86)"} {
		base := filepath.Join(root, pf)
		for _, name := range listDirs(base) {
			dir := filepath.Join(base, name)
			if windowsProgramDirs[strings.ToLower(name)] || strings.HasPrefix(strings.ToLower(name), "windows ") {
				continue
			}
			if games(dir) {
				continue
			}
			if hasExe(dir, 3) {
				add(dir, "program")
			}
		}
	}
	for _, name := range listDirs(root) {
		ln := strings.ToLower(name)
		if notProgramRoots[ln] || strings.HasPrefix(ln, "$") || strings.HasPrefix(ln, ".") {
			continue
		}
		top := filepath.Join(root, name)
		if games(top) {
			continue
		}
		if exeIn(top) || exeIn(filepath.Join(top, "bin")) {
			add(top, "program") // a program unzipped straight into the drive
			continue
		}
		// A folder of programs, like D:\Tools or D:\PortableApps.
		for _, sub := range listDirs(top) {
			dir := filepath.Join(top, sub)
			if games(dir) {
				continue
			}
			if hasExe(dir, 3) {
				add(dir, "program")
			}
		}
	}

	out := make([]*Program, len(cands))
	deadline := time.Now().Add(45 * time.Second)
	sem := make(chan struct{}, 4)
	var wg sync.WaitGroup
	var mu sync.Mutex
	done := 0
	for i, c := range cands {
		i, c := i, c
		wg.Add(1)
		sem <- struct{}{}
		go func() {
			defer wg.Done()
			defer func() { <-sem }()
			name := prettyFolder(filepath.Base(c.dir))
			p := &Program{ID: appID("program|" + c.dir), Name: name, Kind: c.kind, Path: c.dir, Drive: drive, Exe: mainExe(c.dir, name)}
			st := dirStats(ctx, c.dir, 500000, deadline)
			p.Size, p.Partial, p.Modified = st.Size, st.Partial, st.Newest
			out[i] = p
			mu.Lock()
			done++
			progress(Progress{Phase: "software", Message: name, Done: int64(done), Total: int64(len(cands))})
			mu.Unlock()
		}()
	}
	wg.Wait()
	if ctx.Err() != nil {
		return nil
	}
	sort.Slice(out, func(a, b int) bool { return out[a].Size > out[b].Size })
	return out
}

// exeIn reports whether dir directly holds a program (not just an installer).
func exeIn(dir string) bool {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return false
	}
	for _, d := range entries {
		n := d.Name()
		if !d.IsDir() && strings.EqualFold(filepath.Ext(n), ".exe") && !isGenericExe(lower(strings.TrimSuffix(n, filepath.Ext(n)))) {
			return true
		}
	}
	return false
}

// hasExe looks for a program up to depth folders down.
func hasExe(dir string, depth int) bool {
	if exeIn(dir) {
		return true
	}
	if depth <= 1 {
		return false
	}
	for _, sub := range listDirs(dir) {
		if hasExe(filepath.Join(dir, sub), depth-1) {
			return true
		}
	}
	return false
}

// mainExe picks the program a folder is most likely about: one named like
// the folder, else the first program found (breadth first, 3 levels).
func mainExe(dir, name string) string {
	toks := AppTokens(name, "", "", nil)
	level := []string{dir}
	var first string
	for depth := 0; depth < 3 && len(level) > 0; depth++ {
		var next []string
		for _, d := range level {
			entries, err := os.ReadDir(d)
			if err != nil {
				continue
			}
			for _, en := range entries {
				n := en.Name()
				if en.IsDir() {
					if !platform.IsLink(en.Type()) {
						next = append(next, filepath.Join(d, n))
					}
					continue
				}
				if !strings.EqualFold(filepath.Ext(n), ".exe") {
					continue
				}
				base := strings.TrimSuffix(n, filepath.Ext(n))
				if isGenericExe(lower(base)) {
					continue
				}
				if MatchName(base, toks) > MatchNone {
					return filepath.Join(d, n)
				}
				if first == "" {
					first = filepath.Join(d, n)
				}
			}
		}
		if first != "" {
			return first
		}
		level = next
	}
	return first
}

// prettyFolder turns "GIMPPortable" or "ffmpeg-7.1-full_build" into a display name.
func prettyFolder(s string) string {
	s = strings.NewReplacer("_", " ").Replace(s)
	if strings.ToLower(s) == s && !strings.Contains(s, " ") && len(s) > 1 {
		return strings.ToUpper(s[:1]) + s[1:]
	}
	return s
}

// LeftoversOnDrive keeps the leftover groups that have files on drive. Their
// registry entries and other non-file items stay with them.
func (e *Engine) LeftoversOnDrive(groups []*LeftoverGroup, drive string) []*LeftoverGroup {
	if drive == "" {
		return groups
	}
	var out []*LeftoverGroup
	for _, g := range groups {
		var items []*Leftover
		var size int64
		onDrive := false
		for _, it := range g.Items {
			switch it.Kind {
			case "folder", "file", "shortcut":
				if !e.OnDrive(it.Path, drive) {
					continue
				}
				onDrive = true
			}
			items = append(items, it)
			size += it.Size
		}
		if onDrive {
			ng := *g
			ng.Items, ng.Size = items, size
			out = append(out, &ng)
		}
	}
	return out
}
