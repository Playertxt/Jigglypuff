package engine

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"jigglypuff/internal/platform"
)

// App is an installed program (desktop app from the registry, or Store app).
type App struct {
	ID              string `json:"id"`
	Kind            string `json:"kind"` // desktop | store
	Name            string `json:"name"`
	Publisher       string `json:"publisher"`
	Version         string `json:"version"`
	InstallDate     string `json:"installDate,omitempty"` // YYYY-MM-DD
	Size            int64  `json:"size"`
	SizeKnown       bool   `json:"sizeKnown"`
	InstallLocation string `json:"installLocation,omitempty"`
	Scope           string `json:"scope"` // machine | user
	Arch            string `json:"arch,omitempty"`
	Broken          bool   `json:"broken"` // its uninstaller is missing
	CanUninstall    bool   `json:"canUninstall"`
	HasIcon         bool   `json:"hasIcon"`
	Drive           string `json:"drive,omitempty"` // like C:\, "" when unknown

	UninstallCmd string   `json:"-"`
	QuietCmd     string   `json:"-"`
	IconPath     string   `json:"-"`
	IconIndex    int      `json:"-"`
	RegKey       string   `json:"-"` // full uninstall key path
	KeyName      string   `json:"-"`
	MSI          bool     `json:"-"`
	ProductCode  string   `json:"-"`
	StoreFull    string   `json:"-"`
	StoreFamily  string   `json:"-"`
	Exes         []string `json:"-"`
}

// Tokens returns the names this app's leftovers are likely to use.
func (a *App) Tokens() []string {
	t := AppTokens(a.Name, a.Publisher, a.InstallLocation, a.Exes)
	// Uninstall key names are often the app's internal name ("ZoomUMX").
	if k := strings.TrimSuffix(a.KeyName, "_is1"); k != "" && !reGUIDKey.MatchString(k) && !strings.HasPrefix(k, "{") {
		addTok(&t, Norm(k), 4)
	}
	return t
}

var uninstallRoots = []struct{ key, scope, arch string }{
	{`HKLM\SOFTWARE\Microsoft\Windows\CurrentVersion\Uninstall`, "machine", "64-bit"},
	{`HKLM\SOFTWARE\WOW6432Node\Microsoft\Windows\CurrentVersion\Uninstall`, "machine", "32-bit"},
	{`HKCU\Software\Microsoft\Windows\CurrentVersion\Uninstall`, "user", ""},
}

var (
	reKB      = regexp.MustCompile(`(?i)\bKB\d{6,}\b`)
	reGUIDKey = regexp.MustCompile(`^\{[0-9A-Fa-f\-]{36}\}$`)
	reMsiCode = regexp.MustCompile(`(?i)/[IX]\s*(\{[0-9A-F\-]{36}\})`)
)

func appID(s string) string {
	h := sha1.Sum([]byte(strings.ToLower(s)))
	return hex.EncodeToString(h[:8])
}

// InstalledApps reads every uninstallable desktop program from the registry.
func (e *Engine) InstalledApps() []*App {
	reg := e.P.Reg()
	var apps []*App
	seen := map[string]bool{}
	for _, root := range uninstallRoots {
		keys, _ := reg.SubKeys(root.key)
		for _, k := range keys {
			full := root.key + `\` + k
			vals, err := reg.Values(full)
			if err != nil {
				continue
			}
			m := platform.ValueMap(vals)
			name := platform.Str(m, "DisplayName")
			if name == "" {
				continue
			}
			if v, ok := platform.Num(m, "SystemComponent"); ok && v == 1 {
				continue
			}
			if platform.Str(m, "ParentKeyName") != "" || platform.Str(m, "ParentDisplayName") != "" {
				continue
			}
			rt := strings.ToLower(platform.Str(m, "ReleaseType"))
			if strings.Contains(rt, "update") || strings.Contains(rt, "hotfix") || reKB.MatchString(name) {
				continue
			}
			a := &App{
				ID: appID(full), Kind: "desktop", Name: name, KeyName: k, RegKey: full,
				Publisher:       platform.Str(m, "Publisher"),
				Version:         platform.Str(m, "DisplayVersion"),
				InstallLocation: platform.Clean(strings.Trim(platform.Str(m, "InstallLocation"), `"`)),
				UninstallCmd:    platform.Str(m, "UninstallString"),
				QuietCmd:        platform.Str(m, "QuietUninstallString"),
				Scope:           root.scope, Arch: root.arch,
			}
			if v, ok := platform.Num(m, "WindowsInstaller"); ok && v == 1 {
				a.MSI = true
			}
			if reGUIDKey.MatchString(k) {
				a.ProductCode = k
			}
			if mm := reMsiCode.FindStringSubmatch(a.UninstallCmd); mm != nil {
				a.ProductCode = strings.ToUpper(mm[1])
				a.MSI = true
			}
			if a.ProductCode != "" && strings.Contains(strings.ToLower(a.UninstallCmd), "msiexec") {
				a.MSI = true
			}
			if d := platform.Str(m, "InstallDate"); len(d) == 8 {
				a.InstallDate = d[:4] + "-" + d[4:6] + "-" + d[6:]
			}
			if kb, ok := platform.Num(m, "EstimatedSize"); ok && kb > 0 {
				a.Size, a.SizeKnown = int64(kb)*1024, true
			}
			icon := platform.Str(m, "DisplayIcon")
			a.IconPath, a.IconIndex = platform.ParseIconLocation(icon)
			uexe, _ := platform.ParseCommandLine(a.UninstallCmd)
			for _, x := range []string{a.IconPath, uexe} {
				if strings.EqualFold(filepath.Ext(x), ".exe") {
					a.Exes = append(a.Exes, x)
				}
			}
			if a.IconPath == "" || !platform.Exists(a.IconPath) {
				a.IconPath = guessIcon(a.InstallLocation, a.Name)
				a.IconIndex = 0
			}
			a.HasIcon = a.IconPath != ""
			switch {
			case a.MSI && a.ProductCode != "":
				a.CanUninstall = true
			case a.UninstallCmd != "":
				a.CanUninstall = true
				if filepath.IsAbs(uexe) && !platform.Exists(uexe) {
					a.Broken = true
				}
			default:
				a.Broken = true
			}
			if v, ok := platform.Num(m, "NoRemove"); ok && v == 1 && !a.Broken {
				a.CanUninstall = false
			}
			dedupe := lower(a.Name + "|" + a.Version + "|" + a.Scope)
			if seen[dedupe] {
				continue
			}
			seen[dedupe] = true
			a.Drive = e.appDrive(a)
			apps = append(apps, a)
		}
	}
	sort.Slice(apps, func(i, j int) bool { return lower(apps[i].Name) < lower(apps[j].Name) })
	return apps
}

// guessIcon picks the main executable of an install folder.
func guessIcon(dir, name string) string {
	if dir == "" || !platform.IsDir(dir) {
		return ""
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return ""
	}
	toks := AppTokens(name, "", "", nil)
	var first string
	for _, d := range entries {
		n := d.Name()
		if d.IsDir() || !strings.EqualFold(filepath.Ext(n), ".exe") {
			continue
		}
		base := strings.TrimSuffix(n, filepath.Ext(n))
		if isGenericExe(lower(base)) {
			continue
		}
		if MatchName(base, toks) > MatchNone {
			return filepath.Join(dir, n)
		}
		if first == "" {
			first = filepath.Join(dir, n)
		}
	}
	return first
}

// StoreApps lists removable Microsoft Store apps.
func (e *Engine) StoreApps() []*App {
	list, err := e.P.StoreApps()
	if err != nil {
		return nil
	}
	var apps []*App
	for _, s := range list {
		if s.FullName == "" {
			continue
		}
		a := &App{ID: appID("store|" + s.FullName), Kind: "store", Name: s.DisplayName, Version: s.Version,
			Publisher: storePublisher(s.Publisher, s.Name), InstallLocation: s.Location, Scope: "user",
			StoreFull: s.FullName, StoreFamily: s.Family, CanUninstall: true}
		if a.Name == "" {
			a.Name = s.Name
		}
		a.IconPath = storeLogo(s.Location)
		a.HasIcon = a.IconPath != ""
		a.Drive = e.P.DriveOf(s.Location)
		apps = append(apps, a)
	}
	sort.Slice(apps, func(i, j int) bool { return lower(apps[i].Name) < lower(apps[j].Name) })
	return apps
}

var reGUIDish = regexp.MustCompile(`^[0-9A-Fa-f\-]{8,}$`)

// storePublisher turns "CN=Microsoft Corporation, O=..." into a readable name.
func storePublisher(dn, pkg string) string {
	for _, part := range strings.Split(dn, ",") {
		part = strings.TrimSpace(part)
		if strings.HasPrefix(part, "O=") {
			return strings.Trim(part[2:], `"`)
		}
	}
	for _, part := range strings.Split(dn, ",") {
		part = strings.TrimSpace(part)
		if strings.HasPrefix(part, "CN=") {
			v := strings.Trim(part[3:], `"{}`)
			if !reGUIDish.MatchString(v) && len(v) < 60 {
				return v
			}
		}
	}
	// Package names look like "SpotifyAB.SpotifyMusic": use the first part.
	if i := strings.IndexByte(pkg, '.'); i > 0 && !reGUIDish.MatchString(pkg[:i]) {
		return splitCamel(pkg[:i])
	}
	return ""
}

// splitCamel turns "SpotifyAB" into "Spotify AB".
func splitCamel(s string) string {
	var b strings.Builder
	rs := []rune(s)
	for i, r := range rs {
		if i > 0 && r >= 'A' && r <= 'Z' && rs[i-1] >= 'a' && rs[i-1] <= 'z' {
			b.WriteRune(' ')
		}
		b.WriteRune(r)
	}
	return b.String()
}

var reLogo = regexp.MustCompile(`<Logo>([^<]+)</Logo>`)

// storeLogo finds the best logo image of a Store package.
func storeLogo(loc string) string {
	if loc == "" {
		return ""
	}
	b, err := os.ReadFile(filepath.Join(loc, "AppxManifest.xml"))
	if err != nil {
		return ""
	}
	m := reLogo.FindSubmatch(b)
	if m == nil {
		return ""
	}
	rel := filepath.FromSlash(strings.ReplaceAll(string(m[1]), `\`, "/"))
	full := filepath.Join(loc, rel)
	if platform.Exists(full) {
		return full
	}
	dir, base := filepath.Dir(full), strings.TrimSuffix(filepath.Base(full), filepath.Ext(full))
	entries, _ := os.ReadDir(dir)
	best, bestScore := "", -1
	for _, d := range entries {
		n := d.Name()
		if !strings.HasPrefix(strings.ToLower(n), strings.ToLower(base)+".") || !strings.EqualFold(filepath.Ext(n), ".png") {
			continue
		}
		score := 1
		low := lower(n)
		for _, pref := range []string{"scale-200", "targetsize-48", "scale-150", "scale-100", "targetsize-32"} {
			if strings.Contains(low, pref) {
				score = 10
				break
			}
		}
		if strings.Contains(low, "contrast") {
			score = 0
		}
		if score > bestScore {
			best, bestScore = filepath.Join(dir, n), score
		}
	}
	return best
}

// AppSizes measures install folders of apps that don't report a size. It
// stops after the time budget so the app list never waits on it.
func (e *Engine) AppSizes(ctx context.Context, apps []*App, budget time.Duration) map[string]int64 {
	out := map[string]int64{}
	var mu sync.Mutex
	deadline := time.Now().Add(budget)
	sem := make(chan struct{}, 3)
	var wg sync.WaitGroup
	for _, a := range apps {
		if a.SizeKnown || a.InstallLocation == "" || !platform.IsDir(a.InstallLocation) {
			continue
		}
		if len(filepath.Clean(a.InstallLocation)) <= 3 || platform.SamePath(a.InstallLocation, e.P.Paths().ProgramFiles) ||
			platform.SamePath(a.InstallLocation, filepath.Join(e.P.Paths().ProgramFiles, "WindowsApps")) ||
			platform.SamePath(a.InstallLocation, e.P.Paths().ProgramFilesX86) {
			continue
		}
		a := a
		wg.Add(1)
		sem <- struct{}{}
		go func() {
			defer wg.Done()
			defer func() { <-sem }()
			if time.Now().After(deadline) {
				return
			}
			st := dirStats(ctx, a.InstallLocation, 200000, deadline)
			if !st.Partial && st.Size > 0 {
				mu.Lock()
				out[a.ID] = st.Size
				mu.Unlock()
			}
		}()
	}
	wg.Wait()
	return out
}

// UninstallCommand returns the command line Jigglypuff runs for an app. It always
// prefers the interactive uninstaller so you see what the app asks.
func (a *App) UninstallCommand() string {
	if a.MSI && a.ProductCode != "" {
		return "MsiExec.exe /X" + a.ProductCode
	}
	return a.UninstallCmd
}
