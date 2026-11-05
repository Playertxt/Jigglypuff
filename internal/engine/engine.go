// Package engine contains Jigglypuff's scanners and the deletion engine. Scans
// never change anything; only Execute (clean.go) deletes, and only targets
// that pass the safety guard again at the moment of deletion.
package engine

import (
	"context"
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"jigglypuff/internal/platform"
)

// Settings that influence scanning and cleaning.
type Settings struct {
	Theme            string   `json:"theme"`            // system | light | dark
	RecycleUserFiles bool     `json:"recycleUserFiles"` // leftovers, large files, duplicates → Recycle Bin
	TempMinAgeHours  int      `json:"tempMinAgeHours"`  // skip temp files changed more recently
	RestorePoint     bool     `json:"restorePoint"`     // create a restore point before uninstalling
	Exclusions       []string `json:"exclusions"`       // folders Jigglypuff must never touch
	LargeMinMB       int      `json:"largeMinMB"`
	OldDownloadDays  int      `json:"oldDownloadDays"`
	DupMinKB         int      `json:"dupMinKB"`
	DupFolders       []string `json:"dupFolders"`
	LargeFolder      string   `json:"largeFolder"`
	LargeAppData     bool     `json:"largeAppData"` // include AppData in the large file scan
	// Look is the interface theme (colors, shapes, effects). Only the interface reads it; the server just keeps it.
	Look json.RawMessage `json:"look,omitempty"`
}

// DefaultSettings returns the out-of-the-box settings.
func DefaultSettings() Settings {
	return Settings{
		Theme:            "system",
		RecycleUserFiles: true,
		TempMinAgeHours:  24,
		RestorePoint:     true,
		LargeMinMB:       200,
		OldDownloadDays:  60,
		DupMinKB:         1024,
	}
}

// Progress describes what a running scan or clean is doing.
type Progress struct {
	Phase   string `json:"phase"`
	Message string `json:"message"`
	Done    int64  `json:"done"`
	Total   int64  `json:"total"`
	Bytes   int64  `json:"bytes"`
}

// ProgressFunc receives progress updates. It must be cheap and thread safe.
type ProgressFunc func(Progress)

// Engine bundles the platform and the current settings.
type Engine struct {
	P        platform.Platform
	SelfPath string // Jigglypuff's own executable, never offered for deletion

	mu       sync.RWMutex
	settings Settings
}

// New creates an engine.
func New(p platform.Platform, s Settings) *Engine {
	self, _ := os.Executable()
	return &Engine{P: p, settings: s, SelfPath: self}
}

// Settings returns a copy of the current settings.
func (e *Engine) Settings() Settings {
	e.mu.RLock()
	defer e.mu.RUnlock()
	s := e.settings
	s.Exclusions = append([]string(nil), s.Exclusions...)
	s.DupFolders = append([]string(nil), s.DupFolders...)
	return s
}

// SetSettings replaces the settings.
func (e *Engine) SetSettings(s Settings) {
	e.mu.Lock()
	e.settings = s
	e.mu.Unlock()
}

// Guard returns a safety guard for the current settings.
func (e *Engine) Guard() *Guard {
	return NewGuard(e.P.Paths(), e.Settings().Exclusions, e.SelfPath)
}

// Walking -----------------------------------------------------------------------

// FileVisit is called for every regular file found by walk.
type FileVisit func(path string, info fs.FileInfo) error

// errStop lets a visitor end a walk early without reporting an error.
var errStop = fs.SkipAll

// walk visits every regular file below root. It never follows symlinks,
// junctions or other reparse points (OneDrive placeholders included), skips
// folders it cannot read, and stops when ctx is cancelled. skipDir may return
// true to leave out a folder entirely.
func walk(ctx context.Context, root string, skipDir func(path string) bool, visit FileVisit) error {
	fi, err := os.Stat(root) // the root itself may legitimately be a junction
	if err != nil || !fi.IsDir() {
		return err
	}
	return walkDir(ctx, root, skipDir, visit, 0)
}

func walkDir(ctx context.Context, dir string, skipDir func(string) bool, visit FileVisit, depth int) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if depth > 64 {
		return nil
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil // unreadable folders are simply skipped
	}
	for _, d := range entries {
		if platform.IsLink(d.Type()) {
			continue
		}
		p := filepath.Join(dir, d.Name())
		if d.IsDir() {
			if skipDir != nil && skipDir(p) {
				continue
			}
			if err := walkDir(ctx, p, skipDir, visit, depth+1); err != nil {
				return err
			}
			continue
		}
		info, err := d.Info()
		if err != nil || platform.IsLink(info.Mode()) || !info.Mode().IsRegular() {
			continue
		}
		if err := visit(p, info); err != nil {
			return err
		}
	}
	return nil
}

// DirStats summarises a folder.
type DirStats struct {
	Size    int64
	Files   int
	Newest  time.Time // most recent modification inside
	HasExe  bool
	Partial bool // stopped early (time or file limit)
}

// dirStats measures a folder, giving up after limit files or the deadline.
func dirStats(ctx context.Context, root string, limit int, deadline time.Time) DirStats {
	var st DirStats
	if fi, err := os.Stat(root); err == nil {
		st.Newest = fi.ModTime()
	}
	_ = walk(ctx, root, nil, func(p string, info fs.FileInfo) error {
		st.Size += info.Size()
		st.Files++
		if mt := info.ModTime(); mt.After(st.Newest) {
			st.Newest = mt
		}
		if !st.HasExe {
			switch strings.ToLower(filepath.Ext(p)) {
			case ".exe", ".dll", ".sys":
				st.HasExe = true
			}
		}
		if (limit > 0 && st.Files >= limit) || (!deadline.IsZero() && st.Files%256 == 0 && time.Now().After(deadline)) {
			st.Partial = true
			return errStop
		}
		return nil
	})
	return st
}

// listDirs returns the names of real (non-link) sub folders.
func listDirs(dir string) []string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var out []string
	for _, d := range entries {
		if d.IsDir() && !platform.IsLink(d.Type()) {
			out = append(out, d.Name())
		}
	}
	return out
}

func lower(s string) string { return strings.ToLower(s) }

// processNames returns the lower-case names of running processes.
func (e *Engine) processNames() map[string]bool {
	m := map[string]bool{}
	for _, p := range e.P.Processes() {
		m[lower(p.Name)] = true
	}
	return m
}
