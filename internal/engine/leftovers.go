package engine

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
	"unicode/utf16"

	"jigglypuff/internal/platform"
)

// Leftover is one file, folder, shortcut, registry entry, service or task
// that an app left behind.
type Leftover struct {
	ID         string    `json:"id"`
	Kind       string    `json:"kind"` // folder | file | shortcut | regkey | regvalue | service | task
	Path       string    `json:"path"`
	Name       string    `json:"name,omitempty"`
	Size       int64     `json:"size"`
	Files      int       `json:"files"`
	Confidence string    `json:"confidence"` // high | medium
	Reason     string    `json:"reason"`
	LastUsed   time.Time `json:"lastUsed"`
	HasExe     bool      `json:"hasExe,omitempty"`
	Purpose    string    `json:"-"` // guard purpose; empty means PurposeLeftover
}

// LeftoverGroup bundles the leftovers of one (probably uninstalled) app.
type LeftoverGroup struct {
	Key        string      `json:"key"`
	Name       string      `json:"name"`
	Items      []*Leftover `json:"items"`
	Size       int64       `json:"size"`
	Confidence string      `json:"confidence"`
	LastUsed   time.Time   `json:"lastUsed"`
}

func leftoverID(kind, path, name string) string { return appID(kind + "|" + path + "|" + name) }

func newLeftover(kind, path, name, conf, reason string) *Leftover {
	return &Leftover{ID: leftoverID(kind, path, name), Kind: kind, Path: path, Name: name, Confidence: conf, Reason: reason}
}

// world is a snapshot of what is installed and referenced on this PC, used to
// make sure Jigglypuff never calls something a leftover while it is still in use.
type world struct {
	apps       []*App
	tokens     []string // tokens of every installed app and publisher
	referenced []string // files that installed apps, services, tasks... point to
	services   []serviceInfo
	tasks      []taskInfo
}

type serviceInfo struct{ name, display, image, exe string }
type taskInfo struct{ path, file, command string }

func (e *Engine) snapshotWorld(ctx context.Context, includeStore bool) *world {
	w := &world{apps: e.InstalledApps()}
	if includeStore {
		w.apps = append(w.apps, e.StoreApps()...)
	}
	for _, a := range w.apps {
		for _, t := range a.Tokens() {
			addTok(&w.tokens, t, 3)
		}
		for _, t := range PublisherTokens(a.Publisher) {
			addTok(&w.tokens, t, 3)
		}
		w.ref(a.InstallLocation)
		w.ref(a.IconPath)
		for _, x := range a.Exes {
			w.ref(x)
		}
	}
	w.services = e.services()
	for _, s := range w.services {
		w.ref(s.exe)
	}
	w.tasks = e.tasks()
	for _, t := range w.tasks {
		exe, _ := platform.ParseCommandLine(t.command)
		w.ref(platform.ExpandEnv(exe))
	}
	for _, r := range e.runEntries() {
		w.ref(r)
	}
	for _, p := range e.P.Processes() {
		w.ref(p.Path)
	}
	p := e.P.Paths()
	for _, dir := range []string{filepath.Join(p.StartMenu, "Programs"), filepath.Join(p.CommonStartMenu, "Programs"), p.Desktop, p.PublicDesktop} {
		_ = walk(ctx, dir, nil, func(path string, info fs.FileInfo) error {
			if strings.EqualFold(filepath.Ext(path), ".lnk") {
				if s := ReadShortcut(path); s.Known && platform.Exists(s.Target) {
					w.ref(s.Target)
				}
			}
			return nil
		})
	}
	return w
}

func (w *world) ref(p string) {
	if p != "" && filepath.IsAbs(p) {
		w.referenced = append(w.referenced, platform.Clean(p))
	}
}

// inUse reports whether something installed points inside folder.
func (w *world) inUse(folder string) bool {
	for _, r := range w.referenced {
		if platform.Within(r, folder) {
			return true
		}
	}
	return false
}

// Targeted leftovers ---------------------------------------------------------------

// AppLeftovers finds what an app leaves behind. After a normal uninstall it
// looks for remains; with force=true (broken uninstaller) it also returns
// the install folder and the uninstall entry so the app is removed entirely.
func (e *Engine) AppLeftovers(ctx context.Context, app *App, force bool, progress ProgressFunc) []*Leftover {
	progress(Progress{Phase: "leftovers", Message: "Looking at installed apps"})
	w := e.snapshotWorld(ctx, false)
	p := e.P.Paths()
	toks := app.Tokens()
	pubToks := PublisherTokens(app.Publisher)

	// Tokens and folders of every *other* installed app.
	var otherToks []string
	var otherDirs []string
	samePublisher := false
	for _, o := range w.apps {
		if o.ID == app.ID || (o.Kind == app.Kind && o.Name == app.Name && o.RegKey == app.RegKey) {
			continue
		}
		for _, t := range o.Tokens() {
			addTok(&otherToks, t, 3)
		}
		if o.InstallLocation != "" {
			otherDirs = append(otherDirs, o.InstallLocation)
		}
		for _, t := range PublisherTokens(o.Publisher) {
			for _, pt := range pubToks {
				if t == pt {
					samePublisher = true
				}
			}
		}
	}
	// ownedByOther: something another installed app clearly uses (skip it).
	ownedByOther := func(name, path string) bool {
		if MatchName(name, otherToks) == MatchStrong && MatchName(name, toks) != MatchStrong {
			return true
		}
		for _, d := range otherDirs {
			if path != "" && (platform.Within(d, path) || platform.Within(path, d)) {
				return true
			}
		}
		return false
	}
	// shared: the name also matches another installed app, so it may be
	// shared (e.g. %APPDATA%\Python with two Python versions installed).
	shared := func(name string) bool { return MatchName(name, otherToks) == MatchStrong }

	var out []*Leftover
	addFolder := func(path, conf, reason string) {
		if conf == "high" && shared(filepath.Base(path)) {
			conf, reason = "medium", reason+" (the name also matches another installed app)"
		}
		for _, x := range out {
			if (x.Kind == "folder" || x.Kind == "file") && (platform.Within(path, x.Path)) {
				return
			}
		}
		// A new, larger folder replaces candidates inside it.
		kept := out[:0]
		for _, x := range out {
			if x.Kind == "folder" && platform.StrictlyWithin(x.Path, path) {
				continue
			}
			kept = append(kept, x)
		}
		out = kept
		out = append(out, newLeftover("folder", path, "", conf, reason))
	}

	install := app.InstallLocation
	validInstall := install != "" && platform.IsDir(install) && len(install) > 3 &&
		!platform.SamePath(install, p.ProgramFiles) && !platform.SamePath(install, p.ProgramFilesX86) &&
		!platform.SamePath(install, p.LocalAppData) && !platform.SamePath(install, p.AppData) && !platform.Within(p.UserProfile, install)
	for _, d := range otherDirs {
		if platform.Within(d, install) { // another app lives inside it: it's a shared folder
			validInstall = false
		}
	}
	if validInstall {
		reason := "Install folder"
		if !force {
			reason = "Install folder, still on disk"
		}
		g := e.Guard()
		if g.CheckPath(install, PurposeLeftover, "") == nil {
			addFolder(install, "high", reason)
		} else if g.CheckPath(install, PurposeInstall, "") == nil {
			l := newLeftover("folder", install, "", "medium", reason+" — in a custom location, check it first")
			l.Purpose = PurposeInstall
			out = append(out, l)
		}
	}
	if app.Kind == "store" && app.StoreFamily != "" {
		if d := filepath.Join(p.LocalAppData, "Packages", app.StoreFamily); platform.IsDir(d) {
			addFolder(d, "high", "App data of the Store app")
		}
	}

	progress(Progress{Phase: "leftovers", Message: "Searching app data folders"})
	roots := []struct{ dir, label string }{
		{p.ProgramFiles, "Program Files"}, {p.ProgramFilesX86, "Program Files (x86)"},
		{p.CommonFiles, "Common Files"}, {p.CommonFilesX86, "Common Files (x86)"},
		{p.ProgramData, "ProgramData"}, {p.AppData, "AppData\\Roaming"}, {p.LocalAppData, "AppData\\Local"},
		{filepath.Join(p.LocalAppData, "Programs"), "AppData\\Local\\Programs"}, {p.LocalLow, "AppData\\LocalLow"},
		{p.Temp, "Temp"},
	}
	for _, r := range roots {
		for _, name := range listDirs(r.dir) {
			full := filepath.Join(r.dir, name)
			if skipSystemName(name) || ownedByOther(name, full) {
				continue
			}
			switch MatchName(name, toks) {
			case MatchStrong:
				addFolder(full, "high", "Named after the app in "+r.label)
				continue
			case MatchWeak:
				addFolder(full, "medium", "Similar name in "+r.label)
				continue
			}
			if MatchName(name, pubToks) == MatchStrong && len(pubToks) > 0 {
				// Vendor folder: look inside for the app's own folder.
				children := listDirs(full)
				var mine, others int
				for _, c := range children {
					if MatchName(c, toks) >= MatchWeak && !ownedByOther(c, filepath.Join(full, c)) {
						mine++
					} else {
						others++
					}
				}
				if mine > 0 && others == 0 && !samePublisher {
					addFolder(full, "high", app.Publisher+" folder in "+r.label)
				} else {
					for _, c := range children {
						cf := filepath.Join(full, c)
						if MatchName(c, toks) == MatchStrong && !ownedByOther(c, cf) {
							addFolder(cf, "high", "Named after the app in "+r.label)
						} else if MatchName(c, toks) == MatchWeak && !ownedByOther(c, cf) {
							addFolder(cf, "medium", "Similar name in "+r.label)
						}
					}
				}
				if len(children) == 0 && !samePublisher && fileCount(full) == 0 {
					addFolder(full, "medium", "Empty "+name+" folder in "+r.label)
				}
			}
		}
	}
	for _, name := range listDirs(p.UserProfile) {
		bare := strings.TrimPrefix(name, ".")
		if !strings.HasPrefix(name, ".") || sharedDotFolders[strings.ToLower(bare)] || len(Norm(bare)) < 4 {
			continue
		}
		if MatchName(bare, toks) == MatchStrong && !ownedByOther(bare, filepath.Join(p.UserProfile, name)) {
			addFolder(filepath.Join(p.UserProfile, name), "high", "Settings folder in your user folder")
		}
	}

	progress(Progress{Phase: "leftovers", Message: "Checking shortcuts"})
	for _, sm := range []string{filepath.Join(p.StartMenu, "Programs"), filepath.Join(p.CommonStartMenu, "Programs")} {
		for _, name := range listDirs(sm) {
			if MatchName(name, toks) == MatchStrong && !ownedByOther(name, filepath.Join(sm, name)) {
				addFolder(filepath.Join(sm, name), "high", "Start menu folder")
			}
		}
	}
	for _, dir := range []string{filepath.Join(p.StartMenu, "Programs"), filepath.Join(p.CommonStartMenu, "Programs"), p.Desktop, p.PublicDesktop} {
		recursive := strings.HasSuffix(dir, "Programs")
		_ = walk(ctx, dir, func(string) bool { return !recursive }, func(path string, info fs.FileInfo) error {
			if !strings.EqualFold(filepath.Ext(path), ".lnk") {
				return nil
			}
			for _, x := range out {
				if platform.Within(path, x.Path) {
					return nil
				}
			}
			s := ReadShortcut(path)
			base := strings.TrimSuffix(info.Name(), filepath.Ext(info.Name()))
			pointsIn := s.Known && validInstall && platform.Within(s.Target, install)
			named := MatchName(base, toks) == MatchStrong && s.Known && !platform.Exists(s.Target)
			if pointsIn || named {
				l := newLeftover("shortcut", path, "", "high", "Shortcut to the app")
				l.Size = info.Size()
				out = append(out, l)
			}
			return nil
		})
	}

	progress(Progress{Phase: "leftovers", Message: "Checking the registry"})
	reg := e.P.Reg()
	for _, base := range []string{`HKCU\Software`, `HKLM\SOFTWARE`, `HKLM\SOFTWARE\WOW6432Node`} {
		keys, _ := reg.SubKeys(base)
		for _, k := range keys {
			kn := Norm(k)
			if blockedVendors[kn] || ownedByOther(k, "") {
				continue
			}
			full := base + `\` + k
			switch MatchName(k, toks) {
			case MatchStrong:
				out = append(out, newLeftover("regkey", full, "", "high", "Settings stored in the registry"))
				continue
			case MatchWeak:
				out = append(out, newLeftover("regkey", full, "", "medium", "Registry key with a similar name"))
				continue
			}
			if MatchName(k, pubToks) == MatchStrong && len(pubToks) > 0 {
				subs, _ := reg.SubKeys(full)
				var mine, others int
				for _, s := range subs {
					if MatchName(s, toks) == MatchStrong {
						mine++
					} else {
						others++
					}
				}
				if mine > 0 && others == 0 && !samePublisher {
					out = append(out, newLeftover("regkey", full, "", "high", app.Publisher+" settings in the registry"))
				} else {
					for _, s := range subs {
						if MatchName(s, toks) == MatchStrong {
							out = append(out, newLeftover("regkey", full+`\`+s, "", "high", "Settings stored in the registry"))
						}
					}
				}
			}
		}
	}
	if app.Kind == "desktop" && app.RegKey != "" && reg.Exists(app.RegKey) {
		reason := "Entry in Installed apps that no longer works"
		if force {
			reason = "Entry in Installed apps"
		}
		out = append(out, newLeftover("regkey", app.RegKey, "", "high", reason))
	}
	// Startup entries, App Paths, services and scheduled tasks pointing into the app.
	if validInstall {
		for _, rk := range runKeys {
			vals, _ := reg.Values(rk)
			for _, v := range vals {
				exe, _ := platform.ParseCommandLine(v.S)
				if exe != "" && platform.Within(platform.ExpandEnv(exe), install) {
					l := newLeftover("regvalue", rk, v.Name, "high", "Starts with Windows")
					out = append(out, l)
				}
			}
		}
		for _, ap := range []string{`HKLM\SOFTWARE\Microsoft\Windows\CurrentVersion\App Paths`, `HKCU\Software\Microsoft\Windows\CurrentVersion\App Paths`} {
			subs, _ := reg.SubKeys(ap)
			for _, s := range subs {
				m := platform.ValueMap(must(reg.Values(ap + `\` + s)))
				if t := strings.Trim(platform.Str(m, ""), `"`); t != "" && platform.Within(t, install) {
					out = append(out, newLeftover("regkey", ap+`\`+s, "", "high", "Program registration (App Paths)"))
				}
			}
		}
		for _, s := range w.services {
			if s.exe != "" && platform.Within(s.exe, install) {
				l := newLeftover("service", s.name, s.display, "high", "Background service of the app")
				out = append(out, l)
			}
		}
		for _, t := range w.tasks {
			exe, _ := platform.ParseCommandLine(t.command)
			if exe != "" && platform.Within(platform.ExpandEnv(exe), install) {
				out = append(out, newLeftover("task", t.path, filepath.Base(t.path), "high", "Scheduled task of the app"))
			}
		}
	}

	progress(Progress{Phase: "leftovers", Message: "Measuring"})
	for _, l := range out {
		if l.Kind == "folder" {
			st := dirStats(ctx, l.Path, 400000, time.Now().Add(15*time.Second))
			l.Size, l.Files, l.LastUsed, l.HasExe = st.Size, st.Files, st.Newest, st.HasExe
			if st.Files == 0 && !strings.Contains(l.Reason, "Empty") {
				l.Reason += " (empty)"
			}
		}
	}
	// Never offer something the guard would refuse.
	g := e.Guard()
	kept := out[:0]
	for _, l := range out {
		if checkLeftover(g, l) == nil {
			kept = append(kept, l)
		}
	}
	sort.SliceStable(kept, func(i, j int) bool {
		if kept[i].Confidence != kept[j].Confidence {
			return kept[i].Confidence == "high"
		}
		return kept[i].Size > kept[j].Size
	})
	return kept
}

func fileCount(dir string) int {
	entries, _ := os.ReadDir(dir)
	return len(entries)
}

// sharedDotFolders are settings folders many tools share; never leftovers.
var sharedDotFolders = map[string]bool{"git": true, "ssh": true, "gnupg": true, "config": true, "local": true, "cache": true,
	"aws": true, "azure": true, "kube": true, "docker": true, "npm": true, "nvm": true, "m2": true, "gradle": true, "cargo": true,
	"rustup": true, "nuget": true, "dotnet": true, "android": true, "ipython": true, "jupyter": true, "conda": true, "vscode": true,
	"idea": true, "vs": true, "vim": true, "emacs.d": true, "oh-my-zsh": true, "bun": true, "deno": true, "pnpm-store": true,
	"yarn": true, "matplotlib": true, "keras": true, "huggingface": true, "ollama": true, "thumbnails": true, "templateengine": true}

func checkLeftover(g *Guard, l *Leftover) error {
	switch l.Kind {
	case "folder", "file", "shortcut":
		if l.Purpose != "" {
			return g.CheckPath(l.Path, l.Purpose, "")
		}
		return g.CheckPath(l.Path, PurposeLeftover, "")
	case "regkey":
		return g.CheckRegKey(l.Path)
	case "regvalue":
		return g.CheckRegValue(l.Path, l.Name)
	case "task":
		return g.CheckTask(l.Path)
	case "service":
		return nil // validated with its image path at deletion time
	}
	return nil
}

// Orphaned leftovers ----------------------------------------------------------------

// orphanSkip lists folder names that belong to Windows, drivers or tools
// that don't register themselves; they are never reported as leftovers.
var orphanSkip = map[string]bool{}

func init() {
	for _, n := range strings.Split(`packages|temp|tmp|programs|comms|connecteddevicesplatform|d3dscache|crashdumps|peerdistrepub|publishers|
virtualstore|history|temporaryinternetfiles|applicationdata|identities|mediacenterprograms|elevateddiagnostics|diagnostics|
placeholdertilelogofolder|isolatedstorage|assembly|cef|fontconfig|squirreltemp|desktop|documents|favorites|startmenu|templates|
ssh|usoprivate|usoshared|softwaredistribution|windowsholographicdevices|packagecache|devicesync|intel|nvidia|nvidiacorporation|
amd|ati|atitechnologies|realtek|dolby|waves|dts|conexant|synaptics|elan|dell|hp|hewlettpackard|lenovo|asus|acer|msi|gigabyte|
qualcomm|killernetworking|rivetnetworks|broadcom|chocolatey|scoop|npm|npmcache|nodegyp|pip|python|pypa|pypoetry|poetry|uv|pipx|
jupyter|ipython|conda|nuget|dotnet|yarn|pnpm|pnpmcache|bun|deno|cargo|rustup|go|gobuild|gopls|msplaywright|electron|
electronbuilder|typescript|gradle|maven|android|flutter|pubcache|dart|java|oracle|sun|javasoft|commonfiles|internetexplorer|
modifiablewindowsapps|windowsapps|uninstallinformation|referenceassemblies|msbuild|windowskits|iis|iisexpress|powershell|
onlineservices|applicationverifier|bonjour|docker|dockerdesktop|wsl|jigglypuff|google|mozilla|bravesoftware|vivaldi|operasoftware|
chromium|steam|valve|epicgames|gnupg|github|git|vscode|code|jetbrainstoolbox|ollama|huggingface|torch|cache|logs|data|
boostinterprocess|regid|sysinternals|macromedia|adobe|apple|applecomputer|appleinc|ms|gameanalytics|unity|unitytechnologies|
dbg|vulkan|vulkanrt|improviders|syncengines|wsl|lxss|ssh|openssh|pki|crypto|protect|systemcertificates|credentials|vault|
mozillamaintenanceservice|packagemanagement|nodejs|node|jre|jdk|eclipseadoptium|temurin|zulu|amazoncorretto|common|shared`, "|") {
		orphanSkip[strings.TrimSpace(n)] = true
	}
}

func skipSystemName(name string) bool {
	n := Norm(name)
	if n == "" || orphanSkip[n] {
		return true
	}
	for _, pre := range []string{"microsoft", "windows", "msedge", "onedrive", "regid", "nvidia", "intel", "realtek", "amd"} {
		if strings.HasPrefix(n, pre) {
			return true
		}
	}
	return strings.HasPrefix(name, "{") || strings.HasPrefix(name, "$")
}

// OrphanLeftovers finds folders and registry keys of apps that are no longer
// installed. It is deliberately conservative: anything an installed app,
// service, task, shortcut or running process points to is left alone, and
// so is anything changed in the last 30 days.
func (e *Engine) OrphanLeftovers(ctx context.Context, progress ProgressFunc) []*LeftoverGroup {
	progress(Progress{Phase: "orphans", Message: "Looking at installed apps"})
	w := e.snapshotWorld(ctx, true)
	p := e.P.Paths()
	now := time.Now()
	groups := map[string]*LeftoverGroup{}
	add := func(name string, l *Leftover) {
		key := Norm(name)
		g := groups[key]
		if g == nil {
			g = &LeftoverGroup{Key: key, Name: prettyName(name)}
			groups[key] = g
		}
		g.Items = append(g.Items, l)
		g.Size += l.Size
		// Registry timestamps are only a fallback: Windows touches them for its own reasons.
		if l.Kind == "folder" && l.LastUsed.After(g.LastUsed) {
			g.LastUsed = l.LastUsed
		}
	}
	roots := []struct {
		dir, label string
		program    bool
	}{
		{p.ProgramFiles, "Program Files", true}, {p.ProgramFilesX86, "Program Files (x86)", true},
		{p.ProgramData, "ProgramData", false}, {p.AppData, "AppData\\Roaming", false},
		{p.LocalAppData, "AppData\\Local", false}, {filepath.Join(p.LocalAppData, "Programs"), "AppData\\Local\\Programs", true},
		{p.LocalLow, "AppData\\LocalLow", false},
	}
	guard := e.Guard()
	var candidates []struct {
		path, name, label string
		program           bool
	}
	for _, r := range roots {
		for _, name := range listDirs(r.dir) {
			full := filepath.Join(r.dir, name)
			if skipSystemName(name) || strings.HasPrefix(name, ".") {
				continue
			}
			if MatchName(name, w.tokens) >= MatchWeak || w.inUse(full) || guard.CheckPath(full, PurposeLeftover, "") != nil {
				continue
			}
			candidates = append(candidates, struct {
				path, name, label string
				program           bool
			}{full, name, r.label, r.program})
		}
	}
	for i, c := range candidates {
		if ctx.Err() != nil {
			return nil
		}
		progress(Progress{Phase: "orphans", Message: c.path, Done: int64(i), Total: int64(len(candidates))})
		st := dirStats(ctx, c.path, 300000, time.Now().Add(4*time.Second))
		age := now.Sub(st.Newest)
		if st.Partial || age < 30*day {
			continue // still in use (or too big to judge quickly)
		}
		l := newLeftover("folder", c.path, "", "medium", "")
		l.Size, l.Files, l.LastUsed, l.HasExe = st.Size, st.Files, st.Newest, st.HasExe
		switch {
		case st.Files == 0:
			l.Confidence, l.Reason = "high", "Empty folder in "+c.label
		case st.HasExe:
			l.Reason = "Contains programs, but no installed app uses it (" + c.label + ")"
		case age > 180*day:
			l.Confidence, l.Reason = "high", "Not used for a long time and no installed app uses it ("+c.label+")"
		default:
			l.Reason = "No installed app uses it (" + c.label + ")"
		}
		add(c.name, l)
	}

	progress(Progress{Phase: "orphans", Message: "Checking the registry"})
	reg := e.P.Reg()
	for _, base := range []string{`HKCU\Software`, `HKLM\SOFTWARE`, `HKLM\SOFTWARE\WOW6432Node`} {
		keys, _ := reg.SubKeys(base)
		for _, k := range keys {
			kn := Norm(k)
			if blockedVendors[kn] || skipSystemName(k) || MatchName(k, w.tokens) >= MatchWeak {
				continue
			}
			full := base + `\` + k
			if guard.CheckRegKey(full) != nil {
				continue
			}
			lw, _ := reg.LastWrite(full)
			l := newLeftover("regkey", full, "", "medium", "Settings of an app that is no longer installed")
			l.LastUsed = lw
			if g, ok := groups[kn]; ok {
				l.Confidence = g.Items[0].Confidence
				add(k, l)
			} else if strings.HasPrefix(base, "HKCU") && !lw.IsZero() && now.Sub(lw) > 365*day {
				add(k, l)
			}
		}
	}

	var out []*LeftoverGroup
	for _, g := range groups {
		if g.LastUsed.IsZero() {
			for _, l := range g.Items {
				if l.LastUsed.After(g.LastUsed) {
					g.LastUsed = l.LastUsed
				}
			}
		}
		g.Confidence = "high"
		for _, l := range g.Items {
			if l.Confidence != "high" {
				g.Confidence = "medium"
			}
		}
		out = append(out, g)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Size != out[j].Size {
			return out[i].Size > out[j].Size
		}
		return out[i].Name < out[j].Name
	})
	return out
}

// Services, tasks, startup references --------------------------------------------

func (e *Engine) services() []serviceInfo {
	reg := e.P.Reg()
	base := `HKLM\SYSTEM\CurrentControlSet\Services`
	names, _ := reg.SubKeys(base)
	var out []serviceInfo
	for _, n := range names {
		m := platform.ValueMap(must(reg.Values(base + `\` + n)))
		t, _ := platform.Num(m, "Type")
		if t&0x30 == 0 { // only Win32 services, never drivers
			continue
		}
		img := platform.Str(m, "ImagePath")
		exe, _ := platform.ParseCommandLine(img)
		exe = platform.ExpandEnv(exe)
		if strings.HasPrefix(lower(exe), `\systemroot\`) {
			continue
		}
		out = append(out, serviceInfo{name: n, display: platform.Str(m, "DisplayName"), image: img, exe: exe})
	}
	return out
}

var reTaskCommand = regexp.MustCompile(`(?s)<Command>(.*?)</Command>`)

func (e *Engine) tasks() []taskInfo {
	root := filepath.Join(e.P.Paths().WinDir, "System32", "Tasks")
	var out []taskInfo
	_ = walk(context.Background(), root, func(p string) bool {
		return strings.EqualFold(filepath.Base(p), "Microsoft")
	}, func(path string, info fs.FileInfo) error {
		if info.Size() > 512*1024 {
			return nil
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		text := decodeMaybeUTF16(b)
		m := reTaskCommand.FindStringSubmatch(text)
		if m == nil {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return nil
		}
		cmd := strings.TrimSpace(strings.NewReplacer("&quot;", `"`, "&amp;", "&").Replace(m[1]))
		out = append(out, taskInfo{path: `\` + strings.ReplaceAll(rel, "/", `\`), file: path, command: cmd})
		return nil
	})
	return out
}

func decodeMaybeUTF16(b []byte) string {
	if len(b) >= 2 && b[0] == 0xFF && b[1] == 0xFE {
		u := make([]uint16, (len(b)-2)/2)
		for i := range u {
			u[i] = uint16(b[2+i*2]) | uint16(b[3+i*2])<<8
		}
		return string(utf16.Decode(u))
	}
	return string(b)
}

var runKeys = []string{
	`HKCU\Software\Microsoft\Windows\CurrentVersion\Run`,
	`HKLM\SOFTWARE\Microsoft\Windows\CurrentVersion\Run`,
	`HKLM\SOFTWARE\WOW6432Node\Microsoft\Windows\CurrentVersion\Run`,
}

func (e *Engine) runEntries() []string {
	var out []string
	for _, k := range runKeys {
		vals, _ := e.P.Reg().Values(k)
		for _, v := range vals {
			exe, _ := platform.ParseCommandLine(v.S)
			out = append(out, platform.ExpandEnv(exe))
		}
	}
	return out
}
