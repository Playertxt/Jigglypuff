package server

import (
	"errors"
	"fmt"
	"sort"
	"time"

	"jigglypuff/internal/engine"
)

// Plan is an exact list of what will be removed. The window shows its
// summary in the confirmation dialog, and only this list is ever executed.
type Plan struct {
	ID      string          `json:"id"`
	Title   string          `json:"title"`
	Source  string          `json:"source"`
	Summary PlanSummary     `json:"summary"`
	Created time.Time       `json:"created"`
	AppID   string          `json:"appId,omitempty"`
	Targets []engine.Target `json:"-"`
}

// PlanSummary is what the confirmation dialog shows.
type PlanSummary struct {
	Items     int           `json:"items"`
	Bytes     int64         `json:"bytes"`
	Permanent int           `json:"permanent"`
	Recycle   int           `json:"recycle"`
	Registry  int           `json:"registry"`
	System    int           `json:"system"`
	Groups    []PlanGroup   `json:"groups"`
	Notes     []string      `json:"notes"`
	Preview   []PlanPreview `json:"preview"`
}

// PlanGroup sums a plan by category / app.
type PlanGroup struct {
	Label string `json:"label"`
	Items int    `json:"items"`
	Bytes int64  `json:"bytes"`
}

// PlanPreview is one line in the dialog's detail list.
type PlanPreview struct {
	Kind    string `json:"kind"`
	Path    string `json:"path"`
	Name    string `json:"name,omitempty"`
	Size    int64  `json:"size"`
	Recycle bool   `json:"recycle"`
}

// PlanRequest is what the window sends after you pick items.
type PlanRequest struct {
	Source string   `json:"source"`
	ScanID string   `json:"scanId"`
	IDs    []string `json:"ids"`
	AppID  string   `json:"appId"`
	Title  string   `json:"title"`
}

var errStale = errors.New("these results are out of date. Please scan again")

func (s *Server) buildPlan(req PlanRequest) (*Plan, error) {
	st := s.state
	set := s.eng.Settings()
	sel := map[string]bool{}
	for _, id := range req.IDs {
		sel[id] = true
	}
	if len(sel) == 0 {
		return nil, errors.New("nothing is selected")
	}
	var targets []engine.Target
	st.mu.Lock()
	switch req.Source {
	case "junk":
		if req.ScanID != st.junkID || st.junk == nil {
			st.mu.Unlock()
			return nil, errStale
		}
		for _, r := range st.junk {
			if !sel[r.ID] || !r.Available {
				continue
			}
			if r.Special == "recyclebin" {
				if r.Size > 0 || r.Files > 0 {
					targets = append(targets, engine.Target{Kind: "recyclebin", Path: "Recycle Bin", Size: r.Size, Files: r.Files, Group: r.Name, Purpose: engine.PurposeJunk})
				}
				continue
			}
			for _, f := range r.Items {
				kind := "file"
				if r.Special == "shortcuts" {
					kind = "shortcut"
				}
				targets = append(targets, engine.Target{Kind: kind, Path: f.Path, Size: f.Size, Purpose: engine.PurposeJunk,
					Root: f.Root, MinAge: f.MinAge, Recycle: r.Recycle, Group: r.Name})
			}
		}
	case "leftovers", "app-leftovers":
		var items []*engine.Leftover
		group := map[string]string{}
		if req.Source == "leftovers" {
			if req.ScanID != st.orphansID {
				st.mu.Unlock()
				return nil, errStale
			}
			for _, g := range st.orphans {
				for _, l := range g.Items {
					items = append(items, l)
					group[l.ID] = g.Name
				}
			}
		} else {
			list, ok := st.appLeft[req.ScanID]
			if !ok {
				st.mu.Unlock()
				return nil, errStale
			}
			items = list
			req.AppID = st.appLeftOf[req.ScanID]
		}
		for _, l := range items {
			if !sel[l.ID] {
				continue
			}
			g := group[l.ID]
			if g == "" {
				g = req.Title
			}
			purpose := engine.PurposeLeftover
			if l.Purpose != "" {
				purpose = l.Purpose
			}
			t := engine.Target{Kind: l.Kind, Path: l.Path, Name: l.Name, Size: l.Size, Files: l.Files, Purpose: purpose, Group: g}
			if l.Kind == "folder" || l.Kind == "file" || l.Kind == "shortcut" {
				t.Recycle = set.RecycleUserFiles
			}
			targets = append(targets, t)
		}
	case "large", "old-downloads", "dups":
		var files []*engine.FileItem
		label := "Large files"
		switch req.Source {
		case "large":
			if req.ScanID != st.largeID {
				st.mu.Unlock()
				return nil, errStale
			}
			files = st.large
		case "old-downloads":
			if req.ScanID != st.oldID {
				st.mu.Unlock()
				return nil, errStale
			}
			files, label = st.old, "Old downloads"
		case "dups":
			if req.ScanID != st.dupsID {
				st.mu.Unlock()
				return nil, errStale
			}
			label = "Duplicate copies"
			for _, g := range st.dups {
				left := 0
				for _, f := range g.Files {
					if !sel[f.ID] {
						left++
					}
				}
				if left == 0 {
					st.mu.Unlock()
					return nil, fmt.Errorf("keep at least one copy of %s", g.Files[0].Name)
				}
				files = append(files, g.Files...)
			}
		}
		for _, f := range files {
			if sel[f.ID] {
				targets = append(targets, engine.Target{Kind: "file", Path: f.Path, Size: f.Size, Purpose: engine.PurposeUserFile,
					Recycle: set.RecycleUserFiles, Group: label})
			}
		}
	case "broken-entry":
		app := s.findApp(req.AppID)
		if app == nil || app.Kind != "desktop" || app.RegKey == "" {
			st.mu.Unlock()
			return nil, errors.New("app not found")
		}
		targets = append(targets, engine.Target{Kind: "regkey", Path: app.RegKey, Purpose: engine.PurposeLeftover, Group: app.Name})
	case "startup":
		st.mu.Unlock()
		for _, it := range s.eng.StartupItems() {
			if !sel[it.ID] || !it.Missing {
				continue
			}
			if it.Source == "registry" {
				targets = append(targets, engine.Target{Kind: "regvalue", Path: it.Location, Name: it.Entry, Purpose: engine.PurposeLeftover, Group: "Startup entries"})
			} else {
				targets = append(targets, engine.Target{Kind: "shortcut", Path: it.Location, Purpose: engine.PurposeLeftover, Recycle: true, Group: "Startup entries"})
			}
		}
		st.mu.Lock()
	default:
		st.mu.Unlock()
		return nil, errors.New("unknown source")
	}
	st.mu.Unlock()

	// Validate everything now; protected items never reach the dialog.
	g := s.eng.Guard()
	var ok []engine.Target
	refused := 0
	tooBig := 0
	limits := map[string][2]int64{}
	for _, t := range targets {
		if err := s.eng.Validate(g, t); err != nil {
			refused++
			continue
		}
		if t.Recycle {
			vol := volumeOf(t.Path)
			l, seen := limits[vol]
			if !seen {
				max, nukes := s.P.RecycleLimit(t.Path)
				n := int64(0)
				if nukes {
					n = 1
				}
				l = [2]int64{max, n}
				limits[vol] = l
			}
			if l[1] == 1 || (l[0] > 0 && t.Size > l[0]) {
				t.Recycle = false
				tooBig++
			}
		}
		ok = append(ok, t)
	}
	if len(ok) == 0 {
		if refused > 0 {
			return nil, errors.New("the selected items are protected and can't be removed")
		}
		return nil, errors.New("nothing to remove")
	}
	p := &Plan{ID: newID(), Title: req.Title, Source: req.Source, Created: time.Now(), AppID: req.AppID, Targets: ok}
	sum := &p.Summary
	groups := map[string]*PlanGroup{}
	for _, t := range ok {
		sum.Items++
		sum.Bytes += t.Size
		switch t.Kind {
		case "regkey", "regvalue":
			sum.Registry++
		case "service", "task":
			sum.System++
		case "recyclebin":
			sum.Permanent++
		default:
			if t.Recycle {
				sum.Recycle++
			} else {
				sum.Permanent++
			}
		}
		gr := groups[t.Group]
		if gr == nil {
			gr = &PlanGroup{Label: t.Group}
			groups[t.Group] = gr
		}
		gr.Items++
		gr.Bytes += t.Size
	}
	for _, gr := range groups {
		sum.Groups = append(sum.Groups, *gr)
	}
	sort.Slice(sum.Groups, func(i, j int) bool { return sum.Groups[i].Bytes > sum.Groups[j].Bytes })
	prev := append([]engine.Target(nil), ok...)
	sort.SliceStable(prev, func(i, j int) bool { return prev[i].Size > prev[j].Size })
	for i, t := range prev {
		if i >= 300 {
			break
		}
		sum.Preview = append(sum.Preview, PlanPreview{Kind: t.Kind, Path: t.Path, Name: t.Name, Size: t.Size, Recycle: t.Recycle})
	}
	if refused > 0 {
		sum.Notes = append(sum.Notes, fmt.Sprintf("%d protected item(s) were left out.", refused))
	}
	if tooBig > 0 {
		sum.Notes = append(sum.Notes, fmt.Sprintf("%d item(s) are too big for the Recycle Bin (or on a drive without one) and will be deleted permanently.", tooBig))
	}
	st.mu.Lock()
	for id, old := range st.plans {
		if time.Since(old.Created) > 30*time.Minute {
			delete(st.plans, id)
		}
	}
	st.plans[p.ID] = p
	st.mu.Unlock()
	return p, nil
}

func volumeOf(path string) string {
	if len(path) >= 2 && path[1] == ':' {
		return path[:2]
	}
	return "/"
}
