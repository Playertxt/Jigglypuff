package server

import (
	"context"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"

	"jigglypuff/internal/engine"
	"jigglypuff/internal/platform"
)

func (s *Server) routes(mux *http.ServeMux) {
	static := http.FileServer(http.FS(s.opts.Web))
	mux.Handle("GET /", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, ".js") || strings.HasSuffix(r.URL.Path, ".css") || r.URL.Path == "/" {
			w.Header().Set("Cache-Control", "no-cache")
		}
		static.ServeHTTP(w, r)
	}))
	mux.HandleFunc("GET /api/ping", func(w http.ResponseWriter, r *http.Request) {
		s.hub.mu.Lock()
		n, ever := len(s.hub.clients), s.hub.everConnected
		s.hub.mu.Unlock()
		writeJSON(w, map[string]any{"ok": true, "clients": n, "ever": ever})
	})
	mux.HandleFunc("GET /api/events", s.handleEvents)
	mux.HandleFunc("GET /api/info", s.handleInfo)
	mux.HandleFunc("GET /api/drives", func(w http.ResponseWriter, r *http.Request) { writeJSON(w, s.P.Drives()) })
	mux.HandleFunc("POST /api/settings", s.handleSettings)

	mux.HandleFunc("POST /api/scan/{kind}", s.handleScan)
	mux.HandleFunc("GET /api/jobs/{id}", func(w http.ResponseWriter, r *http.Request) {
		b, ok := s.jobs.snapshot(r.PathValue("id"))
		if !ok {
			writeErr(w, 404, errors.New("job not found"))
			return
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.Write(b)
	})
	mux.HandleFunc("POST /api/jobs/{id}/cancel", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, map[string]bool{"ok": s.jobs.cancel(r.PathValue("id"))})
	})

	mux.HandleFunc("GET /api/apps", s.handleApps)
	mux.HandleFunc("GET /api/apps/store", s.handleStoreApps)
	mux.HandleFunc("POST /api/apps/{id}/uninstall", s.handleUninstall)
	mux.HandleFunc("POST /api/apps/{id}/leftovers", s.handleAppLeftovers)
	mux.HandleFunc("GET /api/icon", s.handleIcon)

	mux.HandleFunc("GET /api/startup", func(w http.ResponseWriter, r *http.Request) { writeJSON(w, s.eng.StartupItems()) })
	mux.HandleFunc("POST /api/startup/{id}", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Enabled bool `json:"enabled"`
		}
		if err := readJSON(r, &body); err != nil {
			writeErr(w, 400, err)
			return
		}
		if err := s.eng.SetStartupEnabled(r.PathValue("id"), body.Enabled); err != nil {
			writeErr(w, 500, err)
			return
		}
		writeJSON(w, s.eng.StartupItems())
	})

	mux.HandleFunc("POST /api/plans", func(w http.ResponseWriter, r *http.Request) {
		var req PlanRequest
		if err := readJSON(r, &req); err != nil {
			writeErr(w, 400, err)
			return
		}
		p, err := s.buildPlan(req)
		if err != nil {
			writeErr(w, 409, err)
			return
		}
		writeJSON(w, p)
	})
	mux.HandleFunc("POST /api/plans/{id}/execute", s.handleExecute)

	mux.HandleFunc("GET /api/history", func(w http.ResponseWriter, r *http.Request) { writeJSON(w, s.state.getHistory()) })
	mux.HandleFunc("POST /api/history/clear", func(w http.ResponseWriter, r *http.Request) {
		s.state.clearHistory()
		writeJSON(w, map[string]bool{"ok": true})
	})
	mux.HandleFunc("POST /api/open", s.handleOpen)
	mux.HandleFunc("POST /api/launch", s.handleLaunch)
	mux.HandleFunc("GET /api/fs", s.handleFS)
	mux.HandleFunc("POST /api/shortcut", s.handleShortcut)
	mux.HandleFunc("POST /api/restart-admin", func(w http.ResponseWriter, r *http.Request) {
		if s.opts.RestartAdmin == nil {
			writeErr(w, 400, errors.New("not available"))
			return
		}
		url, err := s.opts.RestartAdmin()
		if err != nil {
			writeErr(w, 500, err)
			return
		}
		writeJSON(w, map[string]string{"url": url})
		go func() {
			time.Sleep(15 * time.Second)
			if !s.jobs.busy() {
				s.Shutdown()
			}
		}()
	})
	mux.HandleFunc("POST /api/quit", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, map[string]bool{"ok": true})
		go func() {
			time.Sleep(300 * time.Millisecond)
			if !s.jobs.busy() {
				s.Shutdown()
			}
		}()
	})
}

func (s *Server) handleEvents(w http.ResponseWriter, r *http.Request) {
	fl, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", 500)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Connection", "keep-alive")
	ch := s.hub.add()
	defer s.hub.remove(ch)
	w.Write([]byte("event: hello\ndata: {}\n\n"))
	fl.Flush()
	ping := time.NewTicker(15 * time.Second)
	defer ping.Stop()
	for {
		select {
		case <-r.Context().Done():
			return
		case <-s.quit:
			return
		case msg := <-ch:
			w.Write(msg)
			fl.Flush()
		case <-ping.C:
			w.Write([]byte(": ping\n\n"))
			fl.Flush()
		}
	}
}

func (s *Server) handleInfo(w http.ResponseWriter, r *http.Request) {
	p := s.P.Paths()
	writeJSON(w, map[string]any{
		"version":    s.opts.Version,
		"user":       s.P.UserName(),
		"admin":      s.P.IsAdmin(),
		"kind":       s.P.Kind(),
		"canElevate": s.opts.RestartAdmin != nil && !s.P.IsAdmin(),
		"settings":   s.eng.Settings(),
		"drives":     s.P.Drives(),
		"dataDir":    p.JigglypuffData,
		"paths": map[string]string{
			"userProfile": p.UserProfile, "desktop": p.Desktop, "documents": p.Documents, "downloads": p.Downloads,
			"pictures": p.Pictures, "music": p.Music, "videos": p.Videos,
		},
		"os": runtime.GOOS,
	})
}

func (s *Server) handleSettings(w http.ResponseWriter, r *http.Request) {
	var set engine.Settings
	if err := readJSON(r, &set); err != nil {
		writeErr(w, 400, err)
		return
	}
	switch set.Theme {
	case "light", "dark", "system":
	default:
		set.Theme = "system"
	}
	clamp := func(v *int, lo, hi, def int) {
		if *v < lo || *v > hi {
			*v = def
		}
	}
	clamp(&set.TempMinAgeHours, 0, 24*30, 24)
	clamp(&set.LargeMinMB, 10, 1<<20, 200)
	clamp(&set.OldDownloadDays, 1, 3650, 60)
	clamp(&set.DupMinKB, 1, 1<<22, 1024)
	var ex []string
	for _, e := range set.Exclusions {
		if e = strings.TrimSpace(e); e != "" && filepath.IsAbs(e) {
			ex = append(ex, platform.Clean(e))
		}
	}
	set.Exclusions = ex
	s.eng.SetSettings(set)
	if err := s.state.saveSettings(set); err != nil {
		writeErr(w, 500, err)
		return
	}
	writeJSON(w, set)
}

// Scans --------------------------------------------------------------------------

func (s *Server) handleScan(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Folder  string   `json:"folder"`
		MinMB   int      `json:"minMB"`
		AppData bool     `json:"appData"`
		Days    int      `json:"days"`
		Folders []string `json:"folders"`
		MinKB   int      `json:"minKB"`
	}
	if err := readJSON(r, &body); err != nil {
		writeErr(w, 400, err)
		return
	}
	set := s.eng.Settings()
	st := s.state
	var j *Job
	switch r.PathValue("kind") {
	case "junk":
		j = s.jobs.start("junk", false, func(ctx context.Context, pr engine.ProgressFunc) (any, error) {
			res, err := s.eng.ScanJunk(ctx, pr)
			if err != nil {
				return nil, err
			}
			id := newID()
			st.mu.Lock()
			st.junkID, st.junk = id, res
			st.mu.Unlock()
			return map[string]any{"scanId": id, "categories": res}, nil
		})
	case "orphans":
		j = s.jobs.start("orphans", false, func(ctx context.Context, pr engine.ProgressFunc) (any, error) {
			groups := s.eng.OrphanLeftovers(ctx, pr)
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}
			id := newID()
			st.mu.Lock()
			st.orphansID, st.orphans = id, groups
			st.mu.Unlock()
			return map[string]any{"scanId": id, "groups": groups}, nil
		})
	case "large":
		folder := body.Folder
		if folder == "" {
			folder = s.P.Paths().UserProfile
		}
		minMB := body.MinMB
		if minMB <= 0 {
			minMB = set.LargeMinMB
		}
		j = s.jobs.start("large", false, func(ctx context.Context, pr engine.ProgressFunc) (any, error) {
			files, err := s.eng.LargeFiles(ctx, folder, int64(minMB)<<20, body.AppData, pr)
			if err != nil && ctx.Err() != nil {
				return nil, ctx.Err()
			}
			id := newID()
			st.mu.Lock()
			st.largeID, st.large = id, files
			st.mu.Unlock()
			return map[string]any{"scanId": id, "files": files, "folder": folder}, nil
		})
	case "old":
		days := body.Days
		if days <= 0 {
			days = set.OldDownloadDays
		}
		j = s.jobs.start("old", false, func(ctx context.Context, pr engine.ProgressFunc) (any, error) {
			files := s.eng.OldDownloads(ctx, days)
			id := newID()
			st.mu.Lock()
			st.oldID, st.old = id, files
			st.mu.Unlock()
			return map[string]any{"scanId": id, "files": files, "days": days}, nil
		})
	case "dups":
		folders := body.Folders
		if len(folders) == 0 {
			p := s.P.Paths()
			folders = []string{p.Desktop, p.Documents, p.Downloads, p.Pictures, p.Music, p.Videos}
		}
		minKB := body.MinKB
		if minKB <= 0 {
			minKB = set.DupMinKB
		}
		j = s.jobs.start("dups", false, func(ctx context.Context, pr engine.ProgressFunc) (any, error) {
			groups, err := s.eng.Duplicates(ctx, folders, int64(minKB)<<10, pr)
			if err != nil {
				return nil, err
			}
			id := newID()
			st.mu.Lock()
			st.dupsID, st.dups = id, groups
			st.mu.Unlock()
			return map[string]any{"scanId": id, "groups": groups, "folders": folders}, nil
		})
	case "smart":
		j = s.jobs.start("smart", false, s.smartScan)
	default:
		writeErr(w, 404, errors.New("unknown scan"))
		return
	}
	writeJSON(w, map[string]string{"jobId": j.ID})
}

// Apps -----------------------------------------------------------------------------

func (s *Server) handleApps(w http.ResponseWriter, r *http.Request) {
	st := s.state
	st.mu.Lock()
	fresh := st.apps != nil && time.Since(st.appsAt) < 2*time.Minute && r.URL.Query().Get("refresh") == ""
	st.mu.Unlock()
	if !fresh {
		apps := s.eng.InstalledApps()
		st.mu.Lock()
		for _, a := range apps {
			if sz, ok := st.appSizes[a.ID]; ok && !a.SizeKnown {
				a.Size, a.SizeKnown = sz, true
			}
		}
		st.apps, st.appsAt = apps, time.Now()
		st.mu.Unlock()
		go func() {
			sizes := s.eng.AppSizes(context.Background(), apps, 20*time.Second)
			if len(sizes) == 0 {
				return
			}
			st.mu.Lock()
			for id, sz := range sizes {
				st.appSizes[id] = sz
			}
			for _, a := range st.apps {
				if sz, ok := sizes[a.ID]; ok {
					a.Size, a.SizeKnown = sz, true
				}
			}
			st.mu.Unlock()
			s.hub.broadcast("appsizes", sizes)
		}()
	}
	st.mu.Lock()
	apps := st.apps
	st.mu.Unlock()
	writeJSON(w, apps)
}

func (s *Server) handleStoreApps(w http.ResponseWriter, r *http.Request) {
	st := s.state
	st.mu.Lock()
	fresh := st.store != nil && time.Since(st.storeAt) < 5*time.Minute && r.URL.Query().Get("refresh") == ""
	st.mu.Unlock()
	if !fresh {
		list := s.eng.StoreApps()
		if list == nil {
			list = []*engine.App{}
		}
		for id, sz := range s.eng.AppSizes(r.Context(), list, 6*time.Second) {
			for _, a := range list {
				if a.ID == id {
					a.Size, a.SizeKnown = sz, true
				}
			}
		}
		st.mu.Lock()
		st.store, st.storeAt = list, time.Now()
		st.mu.Unlock()
	}
	st.mu.Lock()
	list := st.store
	st.mu.Unlock()
	writeJSON(w, list)
}

func (s *Server) findApp(id string) *engine.App {
	for _, a := range s.state.apps {
		if a.ID == id {
			return a
		}
	}
	for _, a := range s.state.store {
		if a.ID == id {
			return a
		}
	}
	return nil
}

func (s *Server) lookupApp(id string) *engine.App {
	s.state.mu.Lock()
	defer s.state.mu.Unlock()
	return s.findApp(id)
}

func (s *Server) handleUninstall(w http.ResponseWriter, r *http.Request) {
	app := s.lookupApp(r.PathValue("id"))
	if app == nil {
		writeErr(w, 404, errors.New("app not found, refresh the list"))
		return
	}
	var body struct {
		RestorePoint bool `json:"restorePoint"`
	}
	readJSON(r, &body)
	j := s.jobs.start("uninstall", true, func(ctx context.Context, pr engine.ProgressFunc) (any, error) {
		res := s.eng.Uninstall(ctx, app, body.RestorePoint, pr)
		s.state.mu.Lock()
		s.state.appsAt = time.Time{}
		s.state.storeAt = time.Time{}
		s.state.mu.Unlock()
		if res.Status == "removed" {
			s.state.addHistory(HistoryEntry{ID: newID(), Time: time.Now(), Title: "Uninstalled " + app.Name, Source: "uninstall",
				Details: []string{"Ran the app's own uninstaller", "Restore point: " + res.RestorePoint}})
		}
		return res, nil
	})
	writeJSON(w, map[string]string{"jobId": j.ID})
}

func (s *Server) handleAppLeftovers(w http.ResponseWriter, r *http.Request) {
	app := s.lookupApp(r.PathValue("id"))
	if app == nil {
		writeErr(w, 404, errors.New("app not found, refresh the list"))
		return
	}
	var body struct {
		Force bool `json:"force"`
	}
	readJSON(r, &body)
	j := s.jobs.start("app-leftovers", false, func(ctx context.Context, pr engine.ProgressFunc) (any, error) {
		items := s.eng.AppLeftovers(ctx, app, body.Force, pr)
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		id := newID()
		s.state.mu.Lock()
		s.state.appLeft[id] = items
		s.state.appLeftOf[id] = app.ID
		s.state.mu.Unlock()
		if items == nil {
			items = []*engine.Leftover{}
		}
		return map[string]any{"scanId": id, "items": items, "app": app}, nil
	})
	writeJSON(w, map[string]string{"jobId": j.ID})
}

func (s *Server) handleIcon(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	path, index := q.Get("path"), 0
	if id := q.Get("app"); id != "" {
		app := s.lookupApp(id)
		if app == nil || app.IconPath == "" {
			http.NotFound(w, r)
			return
		}
		path, index = app.IconPath, app.IconIndex
	} else if i, err := strconv.Atoi(q.Get("index")); err == nil {
		index = i
	}
	if path == "" || !filepath.IsAbs(path) {
		http.NotFound(w, r)
		return
	}
	b, err := s.P.Icon(path, index, 64)
	if err != nil || len(b) == 0 {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("Cache-Control", "private, max-age=3600")
	w.Write(b)
}

// Cleaning ----------------------------------------------------------------------------

func (s *Server) handleExecute(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	s.state.mu.Lock()
	p, ok := s.state.plans[id]
	if ok {
		delete(s.state.plans, id) // a plan runs once
	}
	s.state.mu.Unlock()
	if !ok {
		writeErr(w, 404, errors.New("this confirmation expired, please review again"))
		return
	}
	j := s.jobs.start("clean", true, func(ctx context.Context, pr engine.ProgressFunc) (any, error) {
		res := s.eng.Execute(ctx, p.Targets, s.backupDir(), pr)
		title := p.Title
		if title == "" {
			title = "Cleaned up"
		}
		details := res.Done
		if len(details) > 300 {
			details = details[:300]
		}
		s.state.addHistory(HistoryEntry{ID: newID(), Time: time.Now(), Title: title, Source: p.Source, Freed: res.Freed,
			Items: res.Removed + res.Recycled, Recycled: res.Recycled, Failed: res.Failed, InUse: res.InUse, Skipped: res.Skipped,
			BackupDir: res.BackupDir, Details: details})
		s.forget(p.Source, res.Done)
		done := res.Done
		if len(done) > 20000 {
			done = done[:20000]
		}
		return map[string]any{"result": res, "done": done}, nil
	})
	writeJSON(w, map[string]string{"jobId": j.ID})
}

// forget drops deleted items from cached scan results.
func (s *Server) forget(source string, done []string) {
	gone := map[string]bool{}
	for _, d := range done {
		gone[strings.ToLower(d)] = true
	}
	st := s.state
	st.mu.Lock()
	defer st.mu.Unlock()
	filter := func(list []*engine.FileItem) []*engine.FileItem {
		var out []*engine.FileItem
		for _, f := range list {
			if !gone[strings.ToLower(f.Path)] {
				out = append(out, f)
			}
		}
		return out
	}
	switch source {
	case "large":
		st.large = filter(st.large)
	case "old-downloads":
		st.old = filter(st.old)
	case "dups":
		var groups []*engine.DupGroup
		for _, g := range st.dups {
			g.Files = filter(g.Files)
			if len(g.Files) > 1 {
				groups = append(groups, g)
			}
		}
		st.dups = groups
	case "junk":
		st.junkID = "" // sizes changed: scan again
	}
	st.appsAt = time.Time{}
}

// Misc -------------------------------------------------------------------------------

func (s *Server) handleOpen(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Path   string `json:"path"`
		Reveal bool   `json:"reveal"`
	}
	if err := readJSON(r, &body); err != nil || !filepath.IsAbs(body.Path) {
		writeErr(w, 400, errors.New("bad path"))
		return
	}
	var err error
	if body.Reveal {
		err = s.P.Reveal(body.Path)
	} else {
		err = s.P.OpenFolder(body.Path)
	}
	if err != nil {
		writeErr(w, 500, err)
		return
	}
	writeJSON(w, map[string]bool{"ok": true})
}

func (s *Server) handleLaunch(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Target string `json:"target"`
	}
	readJSON(r, &body)
	targets := map[string]string{
		"storage":  "ms-settings:storagesense",
		"cleanmgr": filepath.Join(s.P.Paths().WinDir, "System32", "cleanmgr.exe"),
		"backups":  filepath.Join(s.P.Paths().JigglypuffData, "Backups"),
		"data":     s.P.Paths().JigglypuffData,
	}
	t, ok := targets[body.Target]
	if !ok {
		writeErr(w, 400, errors.New("unknown target"))
		return
	}
	var err error
	if body.Target == "backups" || body.Target == "data" {
		os.MkdirAll(t, 0o755)
		err = s.P.OpenFolder(t)
	} else {
		err = s.P.Launch(t)
		if err != nil && body.Target == "storage" {
			err = s.P.Launch(targets["cleanmgr"])
		}
	}
	if err != nil {
		writeErr(w, 500, err)
		return
	}
	writeJSON(w, map[string]bool{"ok": true})
}

type fsEntry struct {
	Name string `json:"name"`
	Path string `json:"path"`
	Kind string `json:"kind,omitempty"`
}

// handleFS powers the in-app folder picker.
func (s *Server) handleFS(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Query().Get("path")
	p := s.P.Paths()
	if path == "" {
		places := []fsEntry{{"Your user folder", p.UserProfile, "home"}, {"Desktop", p.Desktop, "desktop"},
			{"Downloads", p.Downloads, "downloads"}, {"Documents", p.Documents, "documents"},
			{"Pictures", p.Pictures, "pictures"}, {"Videos", p.Videos, "videos"}, {"Music", p.Music, "music"}}
		var drives []fsEntry
		for _, d := range s.P.Drives() {
			root := d.Root
			if s.P.Kind() == "demo" {
				root = p.SystemDrive
			}
			name := strings.TrimRight(d.Root, `\`)
			if d.Label != "" {
				name = d.Label + " (" + name + ")"
			}
			drives = append(drives, fsEntry{name, root, "drive"})
			if s.P.Kind() == "demo" {
				break
			}
		}
		writeJSON(w, map[string]any{"places": places, "drives": drives})
		return
	}
	if !filepath.IsAbs(path) || !platform.IsDir(path) {
		fi, err := os.Stat(path)
		if err != nil || !fi.IsDir() {
			writeErr(w, 404, errors.New("folder not found"))
			return
		}
	}
	var dirs []fsEntry
	entries, _ := os.ReadDir(path)
	for _, e := range entries {
		if e.IsDir() && !platform.IsLink(e.Type()) && !strings.HasPrefix(e.Name(), "$") {
			dirs = append(dirs, fsEntry{Name: e.Name(), Path: filepath.Join(path, e.Name())})
		}
	}
	sort.Slice(dirs, func(i, j int) bool { return strings.ToLower(dirs[i].Name) < strings.ToLower(dirs[j].Name) })
	parent := filepath.Dir(path)
	if parent == path {
		parent = ""
	}
	writeJSON(w, map[string]any{"path": path, "parent": parent, "dirs": dirs})
}

func (s *Server) handleShortcut(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Where string `json:"where"`
	}
	readJSON(r, &body)
	exe, err := os.Executable()
	if err != nil {
		writeErr(w, 500, err)
		return
	}
	p := s.P.Paths()
	var lnk string
	switch body.Where {
	case "desktop":
		lnk = filepath.Join(p.Desktop, "Jigglypuff.lnk")
	case "startmenu":
		lnk = filepath.Join(p.StartMenu, "Programs", "Jigglypuff.lnk")
	default:
		writeErr(w, 400, errors.New("unknown place"))
		return
	}
	if err := s.P.CreateShortcut(lnk, exe, "", filepath.Dir(exe), "Jigglypuff — keep your PC clean"); err != nil {
		writeErr(w, 500, err)
		return
	}
	writeJSON(w, map[string]string{"path": lnk})
}
