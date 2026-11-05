package server

import (
	"context"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"jigglypuff/internal/engine"
	"jigglypuff/internal/platform"
)

// Rec is one recommendation on the overview page.
type Rec struct {
	ID         string   `json:"id"`
	Kind       string   `json:"kind"` // clean (can be ticked) | review (opens a page) | info
	Icon       string   `json:"icon"`
	Title      string   `json:"title"`
	Desc       string   `json:"desc"`
	Size       int64    `json:"size"`
	Count      int      `json:"count"`
	Selected   bool     `json:"selected"`
	Safety     string   `json:"safety"`
	Page       string   `json:"page,omitempty"`
	Categories []string `json:"categories,omitempty"`
	Action     string   `json:"action,omitempty"`
}

var groupTitles = map[string][2]string{
	"system":    {"System junk", "system"},
	"browsers":  {"Browser caches", "browser"},
	"apps":      {"App caches", "apps"},
	"developer": {"Developer caches", "code"},
}

// smartScan runs the quick scans and turns them into recommendations. When
// drive is set (like D:\) it only looks at that drive.
func (s *Server) smartScan(ctx context.Context, drive string, pr engine.ProgressFunc) (any, error) {
	step := func(n int64, msg string) {
		pr(engine.Progress{Phase: "smart", Message: msg, Done: n, Total: 4})
	}
	st := s.state
	step(0, "Looking for junk files")
	junk, err := s.eng.ScanJunk(ctx, drive, func(p engine.Progress) {
		pr(engine.Progress{Phase: "smart", Message: p.Message, Done: 0, Total: 4, Bytes: p.Bytes})
	})
	if err != nil {
		return nil, err
	}
	junkID := newID()
	st.mu.Lock()
	st.junkID, st.junk = junkID, junk
	st.mu.Unlock()

	step(1, "Looking for leftovers of removed apps")
	orphans := s.eng.LeftoversOnDrive(s.eng.OrphanLeftovers(ctx, func(p engine.Progress) {
		pr(engine.Progress{Phase: "smart", Message: "Leftovers: " + filepath.Base(p.Message), Done: 1, Total: 4})
	}), drive)
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	orphID := newID()
	st.mu.Lock()
	st.orphansID, st.orphans = orphID, orphans
	st.mu.Unlock()

	step(2, "Checking your Downloads folder")
	set := s.eng.Settings()
	old := s.oldOnDrive(s.eng.OldDownloads(ctx, set.OldDownloadDays), drive)
	oldID := newID()
	st.mu.Lock()
	st.oldID, st.old = oldID, old
	st.mu.Unlock()

	// Startup apps and Windows.old belong to the Windows drive.
	system := s.eng.OnDrive(s.P.Paths().SystemDrive+string(filepath.Separator), drive)
	var startup []*engine.StartupItem
	if system {
		step(3, "Checking startup apps")
		startup = s.eng.StartupItems()
	}

	var recs []Rec
	// Junk, one recommendation per group.
	type agg struct {
		size  int64
		files int
		cats  []string
		names []string
	}
	safe := map[string]*agg{}
	var caution agg
	var needAdmin int
	for _, r := range junk {
		if !r.Available {
			needAdmin++
			continue
		}
		if r.Special == "recyclebin" {
			if r.Size > 0 {
				recs = append(recs, Rec{ID: "recycle-bin", Kind: "clean", Icon: "trash", Title: "Empty the Recycle Bin",
					Desc: fmt.Sprintf("%d items you deleted earlier. Check it first — emptying can't be undone.", r.Files),
					Size: r.Size, Count: r.Files, Safety: "review", Categories: []string{r.ID}})
			}
			continue
		}
		if r.Size == 0 && r.Files == 0 {
			continue
		}
		if r.Default && r.Safety == "safe" {
			a := safe[r.Group]
			if a == nil {
				a = &agg{}
				safe[r.Group] = a
			}
			a.size += r.Size
			a.files += r.Files
			a.cats = append(a.cats, r.ID)
			a.names = append(a.names, r.Name)
		} else {
			caution.size += r.Size
			caution.files += r.Files
			caution.cats = append(caution.cats, r.ID)
			caution.names = append(caution.names, r.Name)
		}
	}
	for _, g := range []string{"system", "browsers", "apps", "developer"} {
		a := safe[g]
		if a == nil || a.size < 1<<20 {
			continue
		}
		recs = append(recs, Rec{ID: "junk-" + g, Kind: "clean", Icon: groupTitles[g][1], Title: groupTitles[g][0],
			Desc: listNames(a.names), Size: a.size, Count: a.files, Selected: true, Safety: "safe", Categories: a.cats})
	}
	if caution.size > 50<<20 {
		recs = append(recs, Rec{ID: "junk-caution", Kind: "clean", Icon: "box", Title: "Large caches you may want to keep",
			Desc: listNames(caution.names) + ". Safe to remove, but your next build or launch re-downloads them.",
			Size: caution.size, Count: caution.files, Safety: "caution", Categories: caution.cats})
	}
	if needAdmin > 0 {
		recs = append(recs, Rec{ID: "admin", Kind: "info", Icon: "shield", Title: "Some system junk needs administrator rights",
			Desc: "Restart Jigglypuff as administrator to include Windows temp files and the Windows Update cache.", Action: "restart-admin"})
	}
	if len(orphans) > 0 {
		var size int64
		var names []string
		high := 0
		for _, g := range orphans {
			size += g.Size
			names = append(names, g.Name)
			if g.Confidence == "high" {
				high++
			}
		}
		recs = append(recs, Rec{ID: "leftovers", Kind: "review", Icon: "puzzle", Page: "leftovers",
			Title: fmt.Sprintf("Leftovers from %d removed app%s", len(orphans), plural(len(orphans))),
			Desc:  listNames(names) + ". Review what's safe to delete.", Size: size, Count: len(orphans), Safety: "review"})
	}
	if len(old) > 0 {
		var size int64
		for _, f := range old {
			size += f.Size
		}
		recs = append(recs, Rec{ID: "old-downloads", Kind: "review", Icon: "download", Page: "large/old",
			Title: fmt.Sprintf("%d old installer%s and archive%s in Downloads", len(old), plural(len(old)), plural(len(old))),
			Desc:  fmt.Sprintf("Not opened for over %d days.", set.OldDownloadDays), Size: size, Count: len(old), Safety: "review"})
	}
	on := 0
	for _, it := range startup {
		if it.Enabled {
			on++
		}
	}
	if on >= 5 {
		recs = append(recs, Rec{ID: "startup", Kind: "review", Icon: "rocket", Page: "startup",
			Title: fmt.Sprintf("%d apps start with Windows", on), Desc: "Turning off the ones you don't need makes sign-in faster.", Count: on, Safety: "review"})
	}
	if drive != "" {
		name := strings.TrimRight(drive, `\`)
		recs = append(recs,
			Rec{ID: "drive-large", Kind: "review", Icon: "fileBig", Page: "large",
				Title: "See what's taking up space on " + name, Desc: "Find the biggest files anywhere on this drive.", Safety: "review"},
			Rec{ID: "drive-software", Kind: "review", Icon: "grid", Page: "apps",
				Title: "See the software installed on " + name, Desc: "Installed apps, games and portable programs on this drive.", Safety: "review"})
	}
	wold := filepath.Join(s.P.Paths().SystemDrive+string(filepath.Separator), "Windows.old")
	if system && platform.IsDir(wold) {
		recs = append(recs, Rec{ID: "windows-old", Kind: "info", Icon: "windows", Action: "storage",
			Title: "A previous Windows installation is taking up space",
			Desc:  "Windows removes it safely: open Storage settings → Temporary files → Previous Windows installation(s)."})
	}
	order := map[string]int{"clean": 0, "review": 1, "info": 2}
	sort.SliceStable(recs, func(i, j int) bool {
		if order[recs[i].Kind] != order[recs[j].Kind] {
			return order[recs[i].Kind] < order[recs[j].Kind]
		}
		if recs[i].Selected != recs[j].Selected {
			return recs[i].Selected
		}
		return recs[i].Size > recs[j].Size
	})
	var total int64
	for _, r := range recs {
		if r.Kind == "clean" && r.Selected {
			total += r.Size
		}
	}
	return map[string]any{
		"recs": recs, "recommended": total, "junkScanId": junkID, "orphanScanId": orphID, "oldScanId": oldID,
		"junk": junk, "orphans": orphans, "old": old, "drives": s.drives(), "time": time.Now(), "drive": drive,
	}, nil
}

func listNames(names []string) string {
	if len(names) <= 3 {
		return strings.Join(names, ", ")
	}
	return strings.Join(names[:3], ", ") + fmt.Sprintf(" and %d more", len(names)-3)
}

func plural(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}
