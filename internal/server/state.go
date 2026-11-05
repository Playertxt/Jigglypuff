package server

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"

	"jigglypuff/internal/engine"
)

// HistoryEntry records one confirmed clean-up.
type HistoryEntry struct {
	ID        string    `json:"id"`
	Time      time.Time `json:"time"`
	Title     string    `json:"title"`
	Source    string    `json:"source"`
	Freed     int64     `json:"freed"`
	Items     int       `json:"items"`
	Recycled  int       `json:"recycled"`
	Failed    int       `json:"failed"`
	InUse     int       `json:"inUse"`
	Skipped   int       `json:"skipped"`
	BackupDir string    `json:"backupDir,omitempty"`
	Details   []string  `json:"details,omitempty"`
}

// store keeps settings and history on disk, and scan results in memory.
type store struct {
	dir string
	mu  sync.Mutex

	history []HistoryEntry

	junkID    string
	junk      []*engine.JunkResult
	orphansID string
	orphans   []*engine.LeftoverGroup
	largeID   string
	large     []*engine.FileItem
	oldID     string
	old       []*engine.FileItem
	dupsID    string
	dups      []*engine.DupGroup
	appLeft   map[string][]*engine.Leftover // scan id → leftovers
	appLeftOf map[string]string             // scan id → app id

	apps     []*engine.App
	appsAt   time.Time
	store    []*engine.App
	storeAt  time.Time
	appSizes map[string]int64

	plans map[string]*Plan
}

func newStore(dir string) *store {
	s := &store{dir: dir, appLeft: map[string][]*engine.Leftover{}, appLeftOf: map[string]string{},
		plans: map[string]*Plan{}, appSizes: map[string]int64{}}
	os.MkdirAll(dir, 0o755)
	if b, err := os.ReadFile(filepath.Join(dir, "history.json")); err == nil {
		json.Unmarshal(b, &s.history)
	}
	return s
}

func (s *store) loadSettings() engine.Settings {
	set := engine.DefaultSettings()
	if b, err := os.ReadFile(filepath.Join(s.dir, "settings.json")); err == nil {
		json.Unmarshal(b, &set)
	}
	if set.TempMinAgeHours < 0 {
		set.TempMinAgeHours = 24
	}
	if set.LargeMinMB <= 0 {
		set.LargeMinMB = 200
	}
	if set.OldDownloadDays <= 0 {
		set.OldDownloadDays = 60
	}
	if set.DupMinKB <= 0 {
		set.DupMinKB = 1024
	}
	return set
}

func (s *store) saveSettings(set engine.Settings) error {
	b, _ := json.MarshalIndent(set, "", "  ")
	return writeAtomic(filepath.Join(s.dir, "settings.json"), b)
}

func (s *store) addHistory(h HistoryEntry) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.history = append([]HistoryEntry{h}, s.history...)
	if len(s.history) > 300 {
		s.history = s.history[:300]
	}
	b, _ := json.Marshal(s.history)
	writeAtomic(filepath.Join(s.dir, "history.json"), b)
}

func (s *store) clearHistory() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.history = nil
	os.Remove(filepath.Join(s.dir, "history.json"))
}

func (s *store) getHistory() []HistoryEntry {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]HistoryEntry(nil), s.history...)
}

func writeAtomic(path string, b []byte) error {
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
