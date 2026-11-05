package engine

import (
	"encoding/binary"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"jigglypuff/internal/platform"
)

// StartupItem is an app that starts when the user signs in.
type StartupItem struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Entry       string `json:"entry"`
	Command     string `json:"command"`
	Path        string `json:"path"`
	Source      string `json:"source"` // registry | folder
	Scope       string `json:"scope"`  // user | machine
	Enabled     bool   `json:"enabled"`
	Publisher   string `json:"publisher"`
	Description string `json:"description"`
	Missing     bool   `json:"missing"`

	ApprovedKey string `json:"-"`
	Location    string `json:"-"` // Run key, or the shortcut file
}

const approvedBase = `\Microsoft\Windows\CurrentVersion\Explorer\StartupApproved\`

// StartupItems lists startup entries from the Run keys and Startup folders,
// with their on/off state as Task Manager shows it.
func (e *Engine) StartupItems() []*StartupItem {
	reg := e.P.Reg()
	p := e.P.Paths()
	var out []*StartupItem
	type runSrc struct{ key, approved, scope string }
	for _, s := range []runSrc{
		{`HKCU\Software\Microsoft\Windows\CurrentVersion\Run`, `HKCU\Software` + approvedBase + `Run`, "user"},
		{`HKLM\SOFTWARE\Microsoft\Windows\CurrentVersion\Run`, `HKLM\SOFTWARE` + approvedBase + `Run`, "machine"},
		{`HKLM\SOFTWARE\WOW6432Node\Microsoft\Windows\CurrentVersion\Run`, `HKLM\SOFTWARE` + approvedBase + `Run32`, "machine"},
	} {
		vals, _ := reg.Values(s.key)
		approved := platform.ValueMap(must(reg.Values(s.approved)))
		for _, v := range vals {
			if v.Name == "" || strings.TrimSpace(v.S) == "" {
				continue
			}
			exe, _ := platform.ParseCommandLine(v.S)
			exe = platform.ExpandEnv(exe)
			it := &StartupItem{ID: appID("run|" + s.key + "|" + v.Name), Entry: v.Name, Name: v.Name, Command: v.S,
				Path: exe, Source: "registry", Scope: s.scope, ApprovedKey: s.approved, Location: s.key}
			it.Enabled = approvedEnabled(approved, v.Name)
			out = append(out, it)
		}
	}
	type dirSrc struct{ dir, approved, scope string }
	for _, s := range []dirSrc{
		{p.Startup, `HKCU\Software` + approvedBase + `StartupFolder`, "user"},
		{p.CommonStartup, `HKLM\SOFTWARE` + approvedBase + `StartupFolder`, "machine"},
	} {
		entries, err := os.ReadDir(s.dir)
		if err != nil {
			continue
		}
		approved := platform.ValueMap(must(reg.Values(s.approved)))
		for _, d := range entries {
			n := d.Name()
			if d.IsDir() || strings.EqualFold(n, "desktop.ini") {
				continue
			}
			full := filepath.Join(s.dir, n)
			target := full
			cmd := full
			if strings.EqualFold(filepath.Ext(n), ".lnk") {
				if sc := ReadShortcut(full); sc.Known {
					target = sc.Target
					cmd = strings.TrimSpace(`"` + sc.Target + `" ` + sc.Args)
				} else {
					target = ""
				}
			}
			it := &StartupItem{ID: appID("dir|" + full), Entry: n, Name: strings.TrimSuffix(n, filepath.Ext(n)), Command: cmd,
				Path: target, Source: "folder", Scope: s.scope, ApprovedKey: s.approved, Location: full}
			it.Enabled = approvedEnabled(approved, n)
			out = append(out, it)
		}
	}
	for _, it := range out {
		if it.Path != "" && filepath.IsAbs(it.Path) {
			if !platform.Exists(it.Path) {
				it.Missing = true
			} else {
				it.Publisher, it.Description = e.P.FileInfo(it.Path)
				if it.Description != "" && len(it.Description) < 60 {
					it.Name = it.Description
				}
			}
		}
	}
	sort.Slice(out, func(i, j int) bool { return lower(out[i].Name) < lower(out[j].Name) })
	return out
}

// approvedEnabled reads Task Manager's StartupApproved flag: even first byte
// (02, 06) = on, odd (03) = off, missing = on.
func approvedEnabled(m map[string]platform.RegValue, name string) bool {
	v, ok := m[strings.ToLower(name)]
	if !ok || len(v.B) == 0 {
		return true
	}
	return v.B[0]&1 == 0
}

// SetStartupEnabled switches an entry on or off the same way Task Manager
// does, so it is fully reversible.
func (e *Engine) SetStartupEnabled(id string, enabled bool) error {
	for _, it := range e.StartupItems() {
		if it.ID != id {
			continue
		}
		data := make([]byte, 12)
		if enabled {
			data[0] = 2
		} else {
			data[0] = 3
			ft := time.Now().UnixNano()/100 + 116444736000000000
			binary.LittleEndian.PutUint64(data[4:], uint64(ft))
		}
		return e.P.Reg().SetBinary(it.ApprovedKey, it.Entry, data)
	}
	return errors.New("startup entry not found")
}
