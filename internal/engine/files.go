package engine

import (
	"container/heap"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"jigglypuff/internal/platform"
)

// FileItem is a single file shown in the large files, old downloads or
// duplicates lists.
type FileItem struct {
	ID       string    `json:"id"`
	Path     string    `json:"path"`
	Name     string    `json:"name"`
	Dir      string    `json:"dir"`
	Size     int64     `json:"size"`
	Modified time.Time `json:"modified"`
	Type     string    `json:"type"`
}

func newFileItem(path string, info fs.FileInfo) *FileItem {
	return &FileItem{ID: appID("file|" + path), Path: path, Name: info.Name(), Dir: filepath.Dir(path),
		Size: info.Size(), Modified: info.ModTime(), Type: fileType(info.Name())}
}

var typeByExt = map[string]string{}

func init() {
	set := func(kind string, exts ...string) {
		for _, e := range exts {
			typeByExt[e] = kind
		}
	}
	set("video", ".mp4", ".mkv", ".mov", ".avi", ".wmv", ".webm", ".m4v", ".flv", ".mpg", ".mpeg", ".ts")
	set("archive", ".zip", ".rar", ".7z", ".tar", ".gz", ".tgz", ".bz2", ".xz", ".zst", ".cab")
	set("disk", ".iso", ".img", ".vhd", ".vhdx", ".vmdk", ".vdi", ".qcow2", ".wim", ".esd", ".dmg")
	set("installer", ".exe", ".msi", ".msix", ".msixbundle", ".appx", ".appxbundle", ".apk")
	set("image", ".jpg", ".jpeg", ".png", ".gif", ".bmp", ".tif", ".tiff", ".webp", ".heic", ".raw", ".cr2", ".nef", ".psd")
	set("audio", ".mp3", ".wav", ".flac", ".aac", ".m4a", ".ogg", ".wma")
	set("document", ".pdf", ".doc", ".docx", ".xls", ".xlsx", ".ppt", ".pptx", ".txt", ".csv")
}

func fileType(name string) string {
	if t, ok := typeByExt[strings.ToLower(filepath.Ext(name))]; ok {
		return t
	}
	return "other"
}

// skipUserScan decides which folders the large-file and duplicate scans skip.
func (e *Engine) skipUserScan(includeAppData bool) func(string) bool {
	p := e.P.Paths()
	skip := []string{p.WinDir, p.ProgramFiles, p.ProgramFilesX86, p.ProgramData, p.JigglypuffData}
	if !includeAppData {
		skip = append(skip, p.AppData, p.LocalAppData, p.LocalLow, filepath.Join(p.UserProfile, "AppData"))
	}
	names := map[string]bool{"$recycle.bin": true, "system volume information": true, "recovery": true,
		"$windows.~bt": true, "$windows.~ws": true, "$winreagent": true, "windows.old": true, "config.msi": true}
	return func(dir string) bool {
		if names[lower(filepath.Base(dir))] {
			return true
		}
		for _, s := range skip {
			if platform.SamePath(dir, s) {
				return true
			}
		}
		return false
	}
}

type sizeHeap []*FileItem

func (h sizeHeap) Len() int           { return len(h) }
func (h sizeHeap) Less(i, j int) bool { return h[i].Size < h[j].Size }
func (h sizeHeap) Swap(i, j int)      { h[i], h[j] = h[j], h[i] }
func (h *sizeHeap) Push(x any)        { *h = append(*h, x.(*FileItem)) }
func (h *sizeHeap) Pop() any {
	old := *h
	x := old[len(old)-1]
	*h = old[:len(old)-1]
	return x
}

var systemRootFiles = map[string]bool{"pagefile.sys": true, "hiberfil.sys": true, "swapfile.sys": true, "dumpstack.log.tmp": true, "dumpstack.log": true}

// LargeFiles returns the biggest files below root (at most 500).
func (e *Engine) LargeFiles(ctx context.Context, root string, minBytes int64, includeAppData bool, progress ProgressFunc) ([]*FileItem, error) {
	if root == "" {
		root = e.P.Paths().UserProfile
	}
	guard := e.Guard()
	skip := e.skipUserScan(includeAppData)
	h := &sizeHeap{}
	var scanned int64
	last := time.Now()
	err := walk(ctx, root, skip, func(path string, info fs.FileInfo) error {
		scanned++
		if time.Since(last) > 150*time.Millisecond {
			last = time.Now()
			progress(Progress{Phase: "large", Message: filepath.Dir(path), Done: scanned})
		}
		if info.Size() < minBytes || systemRootFiles[lower(info.Name())] || guard.Excluded(path) {
			return nil
		}
		if guard.CheckPath(path, PurposeUserFile, "") != nil {
			return nil
		}
		if h.Len() < 500 {
			heap.Push(h, newFileItem(path, info))
		} else if (*h)[0].Size < info.Size() {
			(*h)[0] = newFileItem(path, info)
			heap.Fix(h, 0)
		}
		return nil
	})
	out := []*FileItem(*h)
	sort.Slice(out, func(i, j int) bool { return out[i].Size > out[j].Size })
	if ctx.Err() != nil {
		return out, ctx.Err()
	}
	return out, err
}

var oldDownloadTypes = map[string]bool{"installer": true, "archive": true, "disk": true}

// OldDownloads finds installers, archives and disk images in Downloads that
// haven't been touched for the given number of days.
func (e *Engine) OldDownloads(ctx context.Context, days int) []*FileItem {
	if days <= 0 {
		days = 60
	}
	dl := e.P.Paths().Downloads
	guard := e.Guard()
	cutoff := time.Now().Add(-time.Duration(days) * day)
	var out []*FileItem
	_ = walk(ctx, dl, nil, func(path string, info fs.FileInfo) error {
		if !oldDownloadTypes[fileType(info.Name())] || info.ModTime().After(cutoff) || guard.Excluded(path) {
			return nil
		}
		if guard.CheckPath(path, PurposeUserFile, "") == nil {
			out = append(out, newFileItem(path, info))
		}
		return nil
	})
	sort.Slice(out, func(i, j int) bool { return out[i].Size > out[j].Size })
	return out
}

// DupGroup is a set of identical files.
type DupGroup struct {
	ID     string      `json:"id"`
	Size   int64       `json:"size"`
	Wasted int64       `json:"wasted"`
	Files  []*FileItem `json:"files"`
}

// Duplicates finds files with identical content below the given folders.
func (e *Engine) Duplicates(ctx context.Context, roots []string, minBytes int64, progress ProgressFunc) ([]*DupGroup, error) {
	if minBytes < 1 {
		minBytes = 1
	}
	guard := e.Guard()
	skip := e.skipUserScan(false)
	// Drop roots that sit inside other roots.
	var clean []string
	for _, r := range roots {
		r = platform.Clean(r)
		if r == "" || !platform.IsDir(r) {
			continue
		}
		dup := false
		for _, o := range roots {
			if !platform.SamePath(o, r) && platform.StrictlyWithin(r, o) && platform.IsDir(o) {
				dup = true
			}
		}
		for _, o := range clean {
			if platform.SamePath(o, r) {
				dup = true
			}
		}
		if !dup {
			clean = append(clean, r)
		}
	}
	bySize := map[int64][]*FileItem{}
	var listed int64
	last := time.Now()
	for _, root := range clean {
		err := walk(ctx, root, skip, func(path string, info fs.FileInfo) error {
			listed++
			if time.Since(last) > 150*time.Millisecond {
				last = time.Now()
				progress(Progress{Phase: "listing", Message: filepath.Dir(path), Done: listed})
			}
			n := lower(info.Name())
			if info.Size() < minBytes || n == "desktop.ini" || n == "thumbs.db" || guard.Excluded(path) {
				return nil
			}
			if guard.CheckPath(path, PurposeUserFile, "") != nil {
				return nil
			}
			bySize[info.Size()] = append(bySize[info.Size()], newFileItem(path, info))
			return nil
		})
		if err != nil && ctx.Err() != nil {
			return nil, ctx.Err()
		}
	}
	// Candidates: same size.
	var candidates [][]*FileItem
	var totalBytes int64
	for _, list := range bySize {
		if len(list) > 1 {
			candidates = append(candidates, list)
			for _, f := range list {
				totalBytes += f.Size
			}
		}
	}
	var hashed int64
	var mu sync.Mutex
	report := func(n int64, msg string) {
		mu.Lock()
		hashed += n
		h := hashed
		send := time.Since(last) > 150*time.Millisecond
		if send {
			last = time.Now()
		}
		mu.Unlock()
		if send {
			progress(Progress{Phase: "comparing", Message: msg, Done: h, Total: totalBytes})
		}
	}
	hashAll := func(list []*FileItem, full bool) map[string][]*FileItem {
		out := map[string][]*FileItem{}
		type res struct {
			f *FileItem
			h string
		}
		ch := make(chan res, len(list))
		sem := make(chan struct{}, 3)
		var wg sync.WaitGroup
		for _, f := range list {
			f := f
			wg.Add(1)
			sem <- struct{}{}
			go func() {
				defer wg.Done()
				defer func() { <-sem }()
				if ctx.Err() != nil {
					return
				}
				h, n := hashFile(f.Path, f.Size, full)
				if full {
					report(n, f.Path)
				}
				if h != "" {
					ch <- res{f, h}
				}
			}()
		}
		wg.Wait()
		close(ch)
		for r := range ch {
			out[r.h] = append(out[r.h], r.f)
		}
		return out
	}
	var groups []*DupGroup
	for _, list := range candidates {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		quick := hashAll(list, false)
		for _, ql := range quick {
			if len(ql) < 2 {
				continue
			}
			var fullGroups map[string][]*FileItem
			if ql[0].Size <= 2*quickChunk {
				fullGroups = map[string][]*FileItem{"q": ql} // quick hash already covered the whole file
			} else {
				fullGroups = hashAll(ql, true)
			}
			for hsh, fl := range fullGroups {
				if len(fl) < 2 {
					continue
				}
				sort.Slice(fl, func(i, j int) bool { return fl[i].Modified.Before(fl[j].Modified) })
				groups = append(groups, &DupGroup{ID: appID("dup|" + hsh + fl[0].Path), Size: fl[0].Size,
					Wasted: fl[0].Size * int64(len(fl)-1), Files: fl})
			}
		}
	}
	sort.Slice(groups, func(i, j int) bool { return groups[i].Wasted > groups[j].Wasted })
	if len(groups) > 2000 {
		groups = groups[:2000]
	}
	return groups, nil
}

const quickChunk = 64 * 1024

// hashFile hashes the first and last 64 KB (quick) or the whole file.
func hashFile(path string, size int64, full bool) (string, int64) {
	f, err := os.Open(path)
	if err != nil {
		return "", 0
	}
	defer f.Close()
	h := sha256.New()
	var n int64
	if full {
		n, err = io.Copy(h, f)
		if err != nil {
			return "", n
		}
	} else {
		buf := make([]byte, quickChunk)
		k, _ := io.ReadFull(f, buf)
		h.Write(buf[:k])
		if size > 2*quickChunk {
			if _, err := f.Seek(-quickChunk, io.SeekEnd); err == nil {
				k, _ = io.ReadFull(f, buf)
				h.Write(buf[:k])
			}
		} else if size > quickChunk {
			rest, _ := io.ReadAll(f)
			h.Write(rest)
		}
	}
	return hex.EncodeToString(h.Sum(nil)), n
}
