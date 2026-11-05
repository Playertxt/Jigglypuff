package engine

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"jigglypuff/internal/platform"
)

// Target is one thing a confirmed plan removes.
type Target struct {
	Kind    string        `json:"kind"` // file | folder | shortcut | regkey | regvalue | service | task | recyclebin
	Path    string        `json:"path"`
	Name    string        `json:"name,omitempty"`
	Size    int64         `json:"size"`
	Files   int           `json:"files,omitempty"`
	Purpose string        `json:"purpose"`
	Recycle bool          `json:"recycle"`
	Group   string        `json:"group"`
	Root    string        `json:"-"`
	MinAge  time.Duration `json:"-"`
}

// CleanResult reports what happened.
type CleanResult struct {
	Freed     int64    `json:"freed"`
	Removed   int      `json:"removed"`
	Recycled  int      `json:"recycled"`
	Failed    int      `json:"failed"`
	InUse     int      `json:"inUse"`
	Skipped   int      `json:"skipped"`
	Errors    []string `json:"errors"`
	BackupDir string   `json:"backupDir,omitempty"`
	Done      []string `json:"-"`
}

func (r *CleanResult) fail(what string, err error) {
	r.Failed++
	if len(r.Errors) < 40 {
		r.Errors = append(r.Errors, fmt.Sprintf("%s: %v", what, err))
	}
}

// Validate runs the guard over a target. Plans are only created from
// targets that pass, and Execute checks again right before deleting.
func (e *Engine) Validate(g *Guard, t Target) error {
	switch t.Kind {
	case "file", "folder", "shortcut":
		return g.CheckPath(t.Path, t.Purpose, t.Root)
	case "regkey":
		return g.CheckRegKey(t.Path)
	case "regvalue":
		return g.CheckRegValue(t.Path, t.Name)
	case "service":
		m := platform.ValueMap(must(e.P.Reg().Values(`HKLM\SYSTEM\CurrentControlSet\Services\` + t.Path)))
		return g.CheckService(t.Path, platform.Str(m, "ImagePath"))
	case "task":
		return g.CheckTask(t.Path)
	case "recyclebin":
		return nil
	}
	return errors.New("unknown item type")
}

var kindOrder = map[string]int{"service": 0, "task": 1, "regvalue": 2, "shortcut": 3, "folder": 4, "file": 5, "regkey": 6, "recyclebin": 7}

// Execute removes the targets. Registry keys, services and tasks are backed
// up into backupDir first; if a backup fails, that item is not touched.
func (e *Engine) Execute(ctx context.Context, targets []Target, backupDir string, progress ProgressFunc) CleanResult {
	var res CleanResult
	g := e.Guard()
	sorted := append([]Target(nil), targets...)
	sort.SliceStable(sorted, func(i, j int) bool { return kindOrder[sorted[i].Kind] < kindOrder[sorted[j].Kind] })
	total := int64(len(sorted))
	var done int64
	tick := time.Now()
	report := func(msg string, force bool) {
		if force || time.Since(tick) > 120*time.Millisecond {
			tick = time.Now()
			progress(Progress{Phase: "cleaning", Message: msg, Done: done, Total: total, Bytes: res.Freed})
		}
	}
	backupN := 0
	backup := func(kind string) (string, error) {
		if err := os.MkdirAll(backupDir, 0o755); err != nil {
			return "", err
		}
		res.BackupDir = backupDir
		backupN++
		return filepath.Join(backupDir, fmt.Sprintf("%03d-%s", backupN, kind)), nil
	}
	var recycleBatch []Target
	flushRecycle := func() {
		if len(recycleBatch) == 0 {
			return
		}
		var paths []string
		for _, t := range recycleBatch {
			paths = append(paths, t.Path)
		}
		err := e.P.Recycle(paths)
		for _, t := range recycleBatch {
			if !platform.Exists(t.Path) {
				res.Recycled++
				res.Freed += t.Size
				res.Done = append(res.Done, t.Path)
			} else if err != nil {
				res.fail(t.Path, err)
			} else {
				res.fail(t.Path, errors.New("still there after moving to the Recycle Bin"))
			}
		}
		recycleBatch = recycleBatch[:0]
	}
	roots := map[string]bool{}

	for _, t := range sorted {
		if ctx.Err() != nil {
			res.Skipped += int(total - done)
			break
		}
		done++
		if t.Kind != "file" || done%200 == 0 {
			report(t.Path, false)
		}
		if err := e.Validate(g, t); err != nil {
			res.Skipped++
			if len(res.Errors) < 40 {
				res.Errors = append(res.Errors, fmt.Sprintf("%s: skipped, %v", t.Path, err))
			}
			continue
		}
		switch t.Kind {
		case "service":
			f, err := backup("service.reg")
			if err == nil {
				err = e.P.Reg().Export(`HKLM\SYSTEM\CurrentControlSet\Services\`+t.Path, f)
			}
			if err != nil {
				res.fail("service "+t.Path, fmt.Errorf("not removed because the backup failed: %v", err))
				continue
			}
			if err := e.P.StopAndDeleteService(t.Path); err != nil {
				res.fail("service "+t.Path, err)
				continue
			}
			res.Removed++
			res.Done = append(res.Done, "Service "+t.Path)
		case "task":
			src := filepath.Join(e.P.Paths().WinDir, "System32", "Tasks", strings.TrimPrefix(t.Path, `\`))
			f, err := backup("task.xml")
			if err == nil {
				err = copyFile(src, f)
			}
			if err != nil {
				res.fail("task "+t.Path, fmt.Errorf("not removed because the backup failed: %v", err))
				continue
			}
			if err := e.P.DeleteScheduledTask(t.Path); err != nil {
				res.fail("task "+t.Path, err)
				continue
			}
			res.Removed++
			res.Done = append(res.Done, "Task "+t.Path)
		case "regvalue":
			f, err := backup("startup.reg")
			if err == nil {
				err = e.P.Reg().Export(t.Path, f)
			}
			if err != nil {
				res.fail(t.Path+`\`+t.Name, fmt.Errorf("not removed because the backup failed: %v", err))
				continue
			}
			if err := e.P.Reg().DeleteValue(t.Path, t.Name); err != nil {
				res.fail(t.Path+`\`+t.Name, err)
				continue
			}
			res.Removed++
			res.Done = append(res.Done, t.Path+`\`+t.Name)
		case "regkey":
			if !e.P.Reg().Exists(t.Path) {
				res.Removed++
				continue
			}
			f, err := backup("key.reg")
			if err == nil {
				err = e.P.Reg().Export(t.Path, f)
			}
			if err != nil {
				res.fail(t.Path, fmt.Errorf("not removed because the backup failed: %v", err))
				continue
			}
			if err := e.P.Reg().DeleteTree(t.Path); err != nil {
				res.fail(t.Path, err)
				continue
			}
			res.Removed++
			res.Done = append(res.Done, t.Path)
		case "recyclebin":
			if err := e.P.EmptyRecycleBin(); err != nil {
				res.fail("Recycle Bin", err)
				continue
			}
			res.Removed++
			res.Freed += t.Size
			res.Done = append(res.Done, "Recycle Bin")
		case "file", "shortcut", "folder":
			fi, err := os.Lstat(t.Path)
			if err != nil {
				if os.IsNotExist(err) {
					res.Skipped++ // already gone
				} else {
					res.fail(t.Path, err)
				}
				continue
			}
			if platform.IsLink(fi.Mode()) {
				res.Skipped++
				continue
			}
			if t.Kind == "folder" != fi.IsDir() {
				res.Skipped++
				continue
			}
			if t.MinAge > 0 && time.Since(fi.ModTime()) < t.MinAge {
				res.Skipped++ // changed since the scan: an app is using it again
				continue
			}
			if t.Recycle {
				recycleBatch = append(recycleBatch, t)
				if len(recycleBatch) >= 40 {
					flushRecycle()
				}
				continue
			}
			if t.Kind == "folder" {
				freed, n, err := removeTree(ctx, t.Path)
				res.Freed += freed
				if err != nil {
					res.fail(t.Path, err)
				} else {
					res.Removed++
					res.Done = append(res.Done, t.Path)
				}
				_ = n
				continue
			}
			if err := removeFile(t.Path); err != nil {
				if platform.IsInUse(err) {
					res.InUse++
				} else {
					res.fail(t.Path, err)
				}
				continue
			}
			res.Removed++
			res.Freed += fi.Size()
			if t.Purpose == PurposeJunk {
				roots[t.Root] = true
			} else {
				res.Done = append(res.Done, t.Path)
			}
		}
	}
	flushRecycle()
	for root := range roots {
		pruneEmptyDirs(root, root)
	}
	report("Done", true)
	return res
}

func removeFile(path string) error {
	err := os.Remove(path)
	if err != nil && errors.Is(err, fs.ErrPermission) {
		// Read-only files: clear the flag and try once more.
		if os.Chmod(path, 0o666) == nil {
			err = os.Remove(path)
		}
	}
	return err
}

// removeTree deletes a folder and everything in it. Links inside are never
// followed; if one is found the folder is left in place.
func removeTree(ctx context.Context, dir string) (int64, int, error) {
	var freed int64
	var n int
	var firstErr error
	var rec func(d string) bool
	rec = func(d string) bool {
		if ctx.Err() != nil {
			return false
		}
		entries, err := os.ReadDir(d)
		if err != nil {
			if firstErr == nil {
				firstErr = err
			}
			return false
		}
		empty := true
		for _, en := range entries {
			p := filepath.Join(d, en.Name())
			if platform.IsLink(en.Type()) {
				empty = false
				if firstErr == nil {
					firstErr = fmt.Errorf("left %s in place because it is a link", p)
				}
				continue
			}
			if en.IsDir() {
				if !rec(p) {
					empty = false
				}
				continue
			}
			info, _ := en.Info()
			if err := removeFile(p); err != nil {
				empty = false
				if firstErr == nil {
					firstErr = err
				}
				continue
			}
			n++
			if info != nil {
				freed += info.Size()
			}
		}
		if !empty {
			return false
		}
		if err := os.Remove(d); err != nil {
			if os.Chmod(d, 0o777) == nil && os.Remove(d) == nil {
				return true
			}
			if firstErr == nil {
				firstErr = err
			}
			return false
		}
		return true
	}
	rec(dir)
	return freed, n, firstErr
}

// pruneEmptyDirs removes empty folders below root (never root itself).
func pruneEmptyDirs(root, dir string) bool {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return false
	}
	empty := true
	for _, en := range entries {
		if en.IsDir() && !platform.IsLink(en.Type()) {
			if !pruneEmptyDirs(root, filepath.Join(dir, en.Name())) {
				empty = false
			}
		} else {
			empty = false
		}
	}
	if empty && platform.StrictlyWithin(dir, root) {
		return os.Remove(dir) == nil
	}
	return false
}

func copyFile(src, dst string) error {
	b, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, b, 0o644)
}
