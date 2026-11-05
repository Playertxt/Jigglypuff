package engine

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"jigglypuff/internal/platform"
)

// JunkRule selects files below Root.
type JunkRule struct {
	Root      string
	Pattern   string // lower-case file name pattern, "" = every file
	Recursive bool
	MinAge    time.Duration
	Label     string
}

// JunkCategory is one row in the junk cleaner.
type JunkCategory struct {
	ID, Name, Group, Desc, Note string
	Safety                      string // safe | caution | review
	Admin                       bool   // needs administrator rights
	Default                     bool   // ticked by default (a recommendation)
	Recycle                     bool   // goes to the Recycle Bin instead of being deleted
	Procs                       []string
	Special                     string // recyclebin | shortcuts
	Rules                       []JunkRule
}

// JunkFile is one file found by a junk scan.
type JunkFile struct {
	Path   string
	Size   int64
	Root   string
	MinAge time.Duration
}

// JunkLocation summarises where a category's files are.
type JunkLocation struct {
	Path  string `json:"path"`
	Label string `json:"label,omitempty"`
	Size  int64  `json:"size"`
	Files int    `json:"files"`
}

// JunkResult is the scan result of one category.
type JunkResult struct {
	ID        string         `json:"id"`
	Name      string         `json:"name"`
	Group     string         `json:"group"`
	Desc      string         `json:"desc"`
	Note      string         `json:"note,omitempty"`
	Safety    string         `json:"safety"`
	Admin     bool           `json:"admin"`
	Default   bool           `json:"default"`
	Recycle   bool           `json:"recycle"`
	Available bool           `json:"available"`
	Size      int64          `json:"size"`
	Files     int            `json:"files"`
	Skipped   int            `json:"skipped"`
	Running   []string       `json:"running,omitempty"`
	Locations []JunkLocation `json:"locations"`
	Special   string         `json:"special,omitempty"`
	Drive     string         `json:"drive,omitempty"` // the drive scanned, "" = every drive

	Items []JunkFile `json:"-"`
}

const day = 24 * time.Hour

// JunkCategories lists every junk category that applies to this PC.
func (e *Engine) JunkCategories() []*JunkCategory {
	p := e.P.Paths()
	s := e.Settings()
	tempAge := time.Duration(s.TempMinAgeHours) * time.Hour
	j := filepath.Join
	var cats []*JunkCategory
	add := func(c *JunkCategory) {
		var rules []JunkRule
		for _, r := range c.Rules {
			if r.Root != "" && platform.IsDir(r.Root) {
				rules = append(rules, r)
			}
		}
		c.Rules = rules
		if len(rules) > 0 || c.Special != "" {
			cats = append(cats, c)
		}
	}
	all := func(root string, age time.Duration) JunkRule {
		return JunkRule{Root: root, Recursive: true, MinAge: age}
	}

	// System ---------------------------------------------------------------
	add(&JunkCategory{ID: "user-temp", Name: "Temporary files", Group: "system", Safety: "safe", Default: true,
		Desc:  "Files apps and installers leave in your Temp folder.",
		Note:  "Files changed recently are kept in case an app still needs them.",
		Rules: []JunkRule{all(p.Temp, tempAge)}})
	add(&JunkCategory{ID: "win-temp", Name: "Windows temporary files", Group: "system", Safety: "safe", Default: true, Admin: true,
		Desc:  "Temporary files created by Windows and system services.",
		Rules: []JunkRule{all(j(p.WinDir, "Temp"), tempAge)}})
	add(&JunkCategory{ID: "wu-cache", Name: "Windows Update cache", Group: "system", Safety: "safe", Default: true, Admin: true,
		Desc:  "Update packages Windows already installed. Windows downloads them again if it ever needs to.",
		Rules: []JunkRule{all(j(p.WinDir, "SoftwareDistribution", "Download"), tempAge)}})
	add(&JunkCategory{ID: "delivery-opt", Name: "Delivery Optimization files", Group: "system", Safety: "safe", Default: true, Admin: true,
		Desc:  "Update pieces Windows keeps to share with other PCs.",
		Rules: []JunkRule{all(j(p.WinDir, "ServiceProfiles", "NetworkService", "AppData", "Local", "Microsoft", "Windows", "DeliveryOptimization", "Cache"), 0)}})
	add(&JunkCategory{ID: "error-reports", Name: "Error reports", Group: "system", Safety: "safe", Default: true,
		Desc: "Crash and problem reports collected by Windows Error Reporting.",
		Rules: []JunkRule{
			all(j(p.LocalAppData, "Microsoft", "Windows", "WER"), 0),
			all(j(p.ProgramData, "Microsoft", "Windows", "WER", "ReportArchive"), 0),
			all(j(p.ProgramData, "Microsoft", "Windows", "WER", "ReportQueue"), 0),
			all(j(p.ProgramData, "Microsoft", "Windows", "WER", "Temp"), 0),
		}})
	add(&JunkCategory{ID: "crash-dumps", Name: "Crash dumps", Group: "system", Safety: "safe", Default: true,
		Desc: "Memory snapshots saved when apps or Windows crashed. Only useful for debugging.",
		Rules: []JunkRule{
			all(j(p.LocalAppData, "CrashDumps"), 0),
			all(j(p.WinDir, "Minidump"), 0),
			{Root: j(p.WinDir, "LiveKernelReports"), Pattern: "*.dmp", Recursive: true},
			{Root: p.WinDir, Pattern: "memory.dmp"},
		}})
	add(&JunkCategory{ID: "win-logs", Name: "Windows log files", Group: "system", Safety: "safe", Default: true, Admin: true,
		Desc: "Setup and servicing logs older than a week.",
		Rules: []JunkRule{
			{Root: j(p.WinDir, "Logs"), Pattern: "*.log", Recursive: true, MinAge: 7 * day},
			{Root: j(p.WinDir, "Logs"), Pattern: "*.etl", Recursive: true, MinAge: 7 * day},
			{Root: j(p.WinDir, "Logs"), Pattern: "*.cab", Recursive: true, MinAge: 7 * day},
		}})
	add(&JunkCategory{ID: "thumbnails", Name: "Thumbnail cache", Group: "system", Safety: "safe", Default: true,
		Desc:  "Picture previews File Explorer recreates when you open a folder.",
		Note:  "File Explorer keeps some of these open; those are skipped.",
		Rules: []JunkRule{{Root: j(p.LocalAppData, "Microsoft", "Windows", "Explorer"), Pattern: "thumbcache_*.db"}}})
	add(&JunkCategory{ID: "inet-cache", Name: "Windows internet cache", Group: "system", Safety: "safe", Default: true,
		Desc:  "Web cache used by older Windows components and Office.",
		Rules: []JunkRule{all(j(p.LocalAppData, "Microsoft", "Windows", "INetCache"), 0)}})
	add(&JunkCategory{ID: "shader-cache", Name: "Graphics shader cache", Group: "system", Safety: "safe", Default: true,
		Desc: "Compiled shaders from DirectX and your graphics driver.",
		Note: "Games rebuild these the next time they start, which can take a moment.",
		Rules: []JunkRule{
			all(j(p.LocalAppData, "D3DSCache"), 0),
			all(j(p.LocalAppData, "NVIDIA", "DXCache"), 0),
			all(j(p.LocalAppData, "NVIDIA", "GLCache"), 0),
			all(j(p.LocalAppData, "AMD", "DxCache"), 0),
			all(j(p.LocalAppData, "AMD", "DxcCache"), 0),
			all(j(p.LocalAppData, "AMD", "GLCache"), 0),
			all(j(p.LocalAppData, "AMD", "VkCache"), 0),
			all(j(p.LocalLow, "Intel", "ShaderCache"), 0),
		}})
	add(&JunkCategory{ID: "broken-shortcuts", Name: "Broken shortcuts", Group: "system", Safety: "safe", Default: true, Recycle: true, Special: "shortcuts",
		Desc: "Desktop and Start menu shortcuts that point to programs that no longer exist."})
	add(&JunkCategory{ID: "recycle-bin", Name: "Recycle Bin", Group: "system", Safety: "review", Special: "recyclebin",
		Desc: "Files you deleted earlier. Emptying it can't be undone, so check it first."})

	// Browsers ---------------------------------------------------------------
	type chromium struct{ id, name, data, exe string }
	for _, b := range []chromium{
		{"chrome", "Google Chrome", j(p.LocalAppData, "Google", "Chrome", "User Data"), "chrome.exe"},
		{"edge", "Microsoft Edge", j(p.LocalAppData, "Microsoft", "Edge", "User Data"), ""},
		{"brave", "Brave", j(p.LocalAppData, "BraveSoftware", "Brave-Browser", "User Data"), "brave.exe"},
		{"vivaldi", "Vivaldi", j(p.LocalAppData, "Vivaldi", "User Data"), "vivaldi.exe"},
		{"chromium", "Chromium", j(p.LocalAppData, "Chromium", "User Data"), "chrome.exe"},
	} {
		if !platform.IsDir(b.data) {
			continue
		}
		c := &JunkCategory{ID: "browser-" + b.id, Name: b.name + " cache", Group: "browsers", Safety: "safe", Default: true,
			Desc: "Cached web pages, images and scripts. Your history, passwords and logins are not touched."}
		if b.exe != "" {
			c.Procs = []string{b.exe}
		}
		c.Rules = chromiumRules(b.data)
		add(c)
	}
	for _, op := range []struct{ id, name, dir string }{{"opera", "Opera", "Opera Stable"}, {"operagx", "Opera GX", "Opera GX Stable"}} {
		c := &JunkCategory{ID: "browser-" + op.id, Name: op.name + " cache", Group: "browsers", Safety: "safe", Default: true, Procs: []string{"opera.exe"},
			Desc: "Cached web pages, images and scripts. Your history, passwords and logins are not touched."}
		c.Rules = append(chromiumProfileRules(j(p.AppData, "Opera Software", op.dir), op.name), chromiumProfileRules(j(p.LocalAppData, "Opera Software", op.dir), op.name)...)
		add(c)
	}
	if prof := j(p.LocalAppData, "Mozilla", "Firefox", "Profiles"); platform.IsDir(prof) {
		c := &JunkCategory{ID: "browser-firefox", Name: "Firefox cache", Group: "browsers", Safety: "safe", Default: true, Procs: []string{"firefox.exe"},
			Desc: "Cached web pages and images. Bookmarks, history and logins are not touched."}
		for _, d := range listDirs(prof) {
			for _, sub := range []string{"cache2", "startupCache", "thumbnails", "jumpListCache", "OfflineCache"} {
				c.Rules = append(c.Rules, JunkRule{Root: j(prof, d, sub), Recursive: true, Label: d})
			}
		}
		add(c)
	}

	// Apps -------------------------------------------------------------------
	add(e.electronCaches())
	if c := e.storeAppCaches(); c != nil {
		add(c)
	}
	if steam := e.steamPath(); steam != "" {
		add(&JunkCategory{ID: "steam", Name: "Steam cache & logs", Group: "apps", Safety: "safe", Default: true, Procs: []string{"steam.exe"},
			Desc:  "Steam's web cache, logs and crash dumps. Your games are not touched.",
			Rules: []JunkRule{all(j(steam, "appcache", "httpcache"), 0), all(j(steam, "logs"), 0), all(j(steam, "dumps"), 0)}})
	}

	// Developer ----------------------------------------------------------------
	dev := func(id, name, desc, safety string, def bool, procs []string, roots ...string) {
		c := &JunkCategory{ID: "dev-" + id, Name: name, Group: "developer", Safety: safety, Default: def, Desc: desc, Procs: procs}
		for _, r := range roots {
			c.Rules = append(c.Rules, all(r, 0))
		}
		add(c)
	}
	up := p.UserProfile
	dev("npm", "npm cache", "Packages npm downloaded before. npm fetches them again when needed.", "safe", true, []string{"node.exe"},
		j(p.LocalAppData, "npm-cache"), j(p.AppData, "npm-cache"))
	dev("yarn", "Yarn cache", "Yarn's package download cache.", "safe", true, []string{"node.exe"},
		j(p.LocalAppData, "Yarn", "Cache"), j(p.LocalAppData, "Yarn", "Berry", "cache"))
	dev("pnpm", "pnpm store", "Packages shared between your pnpm projects. Projects re-download what they need.", "caution", false, []string{"node.exe"},
		j(p.LocalAppData, "pnpm", "store"), j(p.LocalAppData, "pnpm-cache"))
	dev("bun", "Bun cache", "Bun's package install cache.", "safe", true, []string{"bun.exe"}, j(up, ".bun", "install", "cache"))
	dev("pip", "pip cache", "Python packages pip downloaded before.", "safe", true, []string{"python.exe", "pip.exe"},
		j(p.LocalAppData, "pip", "cache"), j(p.LocalAppData, "pip", "Cache"))
	dev("uv", "uv cache", "Python packages cached by uv.", "safe", true, []string{"uv.exe"}, j(p.LocalAppData, "uv", "cache"))
	dev("poetry", "Poetry cache", "Packages cached by Poetry.", "safe", true, nil, j(p.LocalAppData, "pypoetry", "Cache"))
	dev("nuget-http", "NuGet HTTP cache", "Package downloads cached by NuGet and dotnet.", "safe", true, []string{"dotnet.exe", "devenv.exe"},
		j(p.LocalAppData, "NuGet", "v3-cache"), j(p.LocalAppData, "NuGet", "plugins-cache"), j(p.LocalAppData, "NuGet", "http-cache"))
	dev("nuget-packages", "NuGet packages folder", "All NuGet packages your projects use. Your next build downloads them again.", "caution", false, []string{"dotnet.exe", "devenv.exe"},
		j(up, ".nuget", "packages"))
	dev("gradle", "Gradle caches", "Gradle dependency and build caches. The next build is slower.", "caution", false, []string{"java.exe", "studio64.exe"},
		j(up, ".gradle", "caches"))
	dev("maven", "Maven repository", "Libraries Maven downloaded. Builds download them again.", "caution", false, []string{"java.exe"},
		j(up, ".m2", "repository"))
	dev("go-build", "Go build cache", "Compiled Go packages. Go rebuilds them as needed.", "safe", true, []string{"go.exe"},
		j(p.LocalAppData, "go-build"))
	dev("go-mod", "Go module cache", "Downloaded Go modules. Go downloads them again when needed.", "caution", false, []string{"go.exe"},
		j(up, "go", "pkg", "mod", "cache", "download"))
	dev("cargo", "Cargo registry cache", "Crate archives Cargo downloaded.", "safe", true, []string{"cargo.exe"},
		j(up, ".cargo", "registry", "cache"))
	dev("electron", "Electron & node-gyp caches", "Electron, electron-builder and node-gyp downloads.", "safe", true, nil,
		j(p.LocalAppData, "electron", "Cache"), j(p.LocalAppData, "electron-builder", "Cache"), j(p.LocalAppData, "node-gyp", "Cache"))
	dev("playwright", "Playwright browsers", "Browsers downloaded by Playwright for testing.", "caution", false, nil,
		j(p.LocalAppData, "ms-playwright"))
	dev("vscode", "VS Code leftovers", "Old VS Code update data, cached extension downloads and logs.", "safe", true, []string{"code.exe"},
		j(p.AppData, "Code", "CachedData"), j(p.AppData, "Code", "CachedExtensionVSIXs"), j(p.AppData, "Code", "logs"),
		j(p.AppData, "Code", "Crashpad", "completed"))
	dev("typescript", "TypeScript cache", "Type definitions cached by the TypeScript language service.", "safe", true, nil,
		j(p.LocalAppData, "Microsoft", "TypeScript"))
	return cats
}

func chromiumRules(userData string) []JunkRule {
	var rules []JunkRule
	for _, d := range listDirs(userData) {
		if d == "Default" || strings.HasPrefix(d, "Profile ") || d == "Guest Profile" || d == "System Profile" {
			rules = append(rules, chromiumProfileRules(filepath.Join(userData, d), d)...)
		}
	}
	for _, sub := range []string{"GrShaderCache", "ShaderCache", "GraphiteDawnCache", filepath.Join("Crashpad", "reports")} {
		rules = append(rules, JunkRule{Root: filepath.Join(userData, sub), Recursive: true, Label: "Shared"})
	}
	return rules
}

func chromiumProfileRules(profile, label string) []JunkRule {
	var rules []JunkRule
	for _, sub := range []string{"Cache", "Code Cache", "GPUCache", "DawnCache", "DawnGraphiteCache", "DawnWebGPUCache", "Media Cache",
		filepath.Join("Service Worker", "CacheStorage"), filepath.Join("Service Worker", "ScriptCache")} {
		rules = append(rules, JunkRule{Root: filepath.Join(profile, sub), Recursive: true, Label: label})
	}
	return rules
}

// electronCaches finds desktop apps built on Chromium/Electron (Discord,
// Slack, VS Code, Figma, Postman...) by the cache folders they create.
func (e *Engine) electronCaches() *JunkCategory {
	p := e.P.Paths()
	c := &JunkCategory{ID: "app-caches", Name: "App caches", Group: "apps", Safety: "safe", Default: true,
		Desc: "Web caches of desktop apps such as Discord, Slack, Teams and VS Code. Your chats and settings are not touched."}
	skip := map[string]bool{"google": true, "microsoft": true, "bravesoftware": true, "vivaldi": true, "operasoftware": true,
		"mozilla": true, "chromium": true, "packages": true, "temp": true, "jigglypuff": true, "programs": true}
	seen := map[string]bool{}
	var check func(dir, label string, depth int)
	check = func(dir, label string, depth int) {
		if platform.IsDir(filepath.Join(dir, "Cache", "Cache_Data")) || platform.IsDir(filepath.Join(dir, "Code Cache")) || platform.IsDir(filepath.Join(dir, "GPUCache")) {
			if seen[lower(dir)] {
				return
			}
			seen[lower(dir)] = true
			for _, sub := range []string{"Cache", "Code Cache", "GPUCache", "DawnCache", "DawnGraphiteCache", "DawnWebGPUCache",
				filepath.Join("Service Worker", "CacheStorage"), filepath.Join("Service Worker", "ScriptCache")} {
				c.Rules = append(c.Rules, JunkRule{Root: filepath.Join(dir, sub), Recursive: true, Label: label})
			}
			c.Procs = append(c.Procs, lower(filepath.Base(dir))+".exe")
			return
		}
		if depth == 0 {
			for _, d := range listDirs(dir) {
				check(filepath.Join(dir, d), label+" "+d, 1)
			}
		}
	}
	for _, root := range []string{p.AppData, p.LocalAppData} {
		for _, d := range listDirs(root) {
			if skip[Norm(d)] {
				continue
			}
			check(filepath.Join(root, d), prettyName(d), 0)
		}
	}
	return c
}

func (e *Engine) storeAppCaches() *JunkCategory {
	pk := filepath.Join(e.P.Paths().LocalAppData, "Packages")
	c := &JunkCategory{ID: "store-caches", Name: "Store app caches", Group: "apps", Safety: "safe", Default: true,
		Desc: "Temporary internet files of Microsoft Store apps."}
	for _, d := range listDirs(pk) {
		label := d
		if i := strings.IndexByte(d, '_'); i > 0 {
			label = d[:i]
		}
		for _, sub := range []string{filepath.Join("AC", "INetCache"), filepath.Join("AC", "Temp"), "TempState"} {
			c.Rules = append(c.Rules, JunkRule{Root: filepath.Join(pk, d, sub), Recursive: true, Label: label})
		}
	}
	return c
}

func (e *Engine) steamPath() string {
	m := platform.ValueMap(must(e.P.Reg().Values(`HKCU\Software\Valve\Steam`)))
	if sp := platform.Str(m, "SteamPath"); sp != "" {
		sp = filepath.FromSlash(sp)
		if platform.IsDir(sp) {
			return sp
		}
	}
	return ""
}

func must(v []platform.RegValue, _ error) []platform.RegValue { return v }

// prettyName turns a folder name like "discord" into "Discord".
func prettyName(s string) string {
	if s == "" {
		return s
	}
	if strings.ToLower(s) == s {
		return strings.ToUpper(s[:1]) + s[1:]
	}
	return s
}

// ScanJunk measures every junk category. It never deletes anything. When
// drive is set (like D:\) only junk on that drive is counted.
func (e *Engine) ScanJunk(ctx context.Context, drive string, progress ProgressFunc) ([]*JunkResult, error) {
	cats := e.JunkCategories()
	running := e.processNames()
	admin := e.P.IsAdmin()
	guard := e.Guard()
	seen := map[string]bool{}
	now := time.Now()
	var results []*JunkResult
	var total int64
	for i, c := range cats {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		progress(Progress{Phase: "junk", Message: c.Name, Done: int64(i), Total: int64(len(cats)), Bytes: total})
		r := &JunkResult{ID: c.ID, Name: c.Name, Group: c.Group, Desc: c.Desc, Note: c.Note, Safety: c.Safety,
			Admin: c.Admin, Default: c.Default, Recycle: c.Recycle, Available: admin || !c.Admin, Special: c.Special, Drive: drive}
		for _, pn := range c.Procs {
			if running[lower(pn)] {
				r.Running = append(r.Running, pn)
			}
		}
		if !r.Available {
			results = append(results, r)
			continue
		}
		switch c.Special {
		case "recyclebin":
			if size, n, err := e.P.RecycleBinInfo(drive); err == nil {
				r.Size, r.Files = size, int(n)
			}
		case "shortcuts":
			for _, s := range e.BrokenShortcuts(ctx) {
				if !e.OnDrive(s.Path, drive) {
					continue
				}
				r.Items = append(r.Items, s)
				r.Size += s.Size
				r.Files++
				r.Locations = append(r.Locations, JunkLocation{Path: s.Path, Size: s.Size, Files: 1})
			}
		}
		locs := map[string]*JunkLocation{}
		for _, rule := range c.Rules {
			rule := rule
			if !e.OnDrive(rule.Root, drive) {
				continue
			}
			skipDir := func(string) bool { return !rule.Recursive }
			_ = walk(ctx, rule.Root, skipDir, func(path string, info fs.FileInfo) error {
				if rule.Pattern != "" {
					if ok, _ := filepath.Match(rule.Pattern, lower(info.Name())); !ok {
						return nil
					}
				}
				if rule.MinAge > 0 && now.Sub(info.ModTime()) < rule.MinAge {
					r.Skipped++
					return nil
				}
				lp := lower(path)
				if seen[lp] || guard.Excluded(path) {
					return nil
				}
				seen[lp] = true
				r.Items = append(r.Items, JunkFile{Path: path, Size: info.Size(), Root: rule.Root, MinAge: rule.MinAge})
				r.Size += info.Size()
				r.Files++
				key := rule.Root
				if rel, err := filepath.Rel(rule.Root, path); err == nil {
					if first := strings.SplitN(rel, string(filepath.Separator), 2); len(first) == 2 {
						key = filepath.Join(rule.Root, first[0])
					}
				}
				l := locs[key]
				if l == nil {
					l = &JunkLocation{Path: key, Label: rule.Label}
					locs[key] = l
				}
				l.Size += info.Size()
				l.Files++
				return nil
			})
		}
		for _, l := range locs {
			r.Locations = append(r.Locations, *l)
		}
		sort.Slice(r.Locations, func(a, b int) bool { return r.Locations[a].Size > r.Locations[b].Size })
		if len(r.Locations) > 30 {
			r.Locations = r.Locations[:30]
		}
		total += r.Size
		if r.Size > 0 || c.Special != "" || (c.Group == "system" && (drive == "" || e.OnDrive(e.P.Paths().SystemDrive, drive))) {
			results = append(results, r)
		}
	}
	progress(Progress{Phase: "junk", Message: "Done", Done: int64(len(cats)), Total: int64(len(cats)), Bytes: total})
	return results, nil
}

// BrokenShortcuts finds .lnk files on the desktops and in the Start menus
// whose target is a local file that no longer exists.
func (e *Engine) BrokenShortcuts(ctx context.Context) []JunkFile {
	p := e.P.Paths()
	var out []JunkFile
	type place struct {
		dir       string
		recursive bool
	}
	for _, pl := range []place{{p.Desktop, false}, {p.PublicDesktop, false},
		{filepath.Join(p.StartMenu, "Programs"), true}, {filepath.Join(p.CommonStartMenu, "Programs"), true}} {
		if pl.dir == "" {
			continue
		}
		root := pl.dir
		_ = walk(ctx, root, func(string) bool { return !pl.recursive }, func(path string, info fs.FileInfo) error {
			if !strings.EqualFold(filepath.Ext(path), ".lnk") {
				return nil
			}
			if isBrokenShortcut(path) {
				out = append(out, JunkFile{Path: path, Size: info.Size(), Root: root})
			}
			return nil
		})
	}
	return out
}

func isBrokenShortcut(path string) bool {
	s := ReadShortcut(path)
	if !s.Known || s.Network || s.Target == "" {
		return false
	}
	t := s.Target
	if strings.HasPrefix(strings.ToLower(t), "shell:") || strings.Contains(t, "://") {
		return false
	}
	if vol := filepath.VolumeName(t); vol != "" {
		if _, err := os.Stat(vol + string(filepath.Separator)); err != nil {
			return false // the drive is not connected right now; don't judge
		}
	}
	_, err := os.Stat(t)
	return os.IsNotExist(err)
}
