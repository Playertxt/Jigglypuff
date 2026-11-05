package engine

import (
	"context"
	"sort"
	"testing"
)

func TestScanJunkOnOneDrive(t *testing.T) {
	e := demoEngine(t)
	all, err := e.ScanJunk(context.Background(), "", noProgress)
	if err != nil {
		t.Fatal(err)
	}
	onC, err := e.ScanJunk(context.Background(), `C:\`, noProgress)
	if err != nil {
		t.Fatal(err)
	}
	onD, err := e.ScanJunk(context.Background(), `D:\`, noProgress)
	if err != nil {
		t.Fatal(err)
	}
	sum := func(rs []*JunkResult) (size int64, bin int64) {
		for _, r := range rs {
			if r.Special == "recyclebin" {
				bin = r.Size
			}
			size += r.Size
			for _, it := range r.Items {
				if r.Drive != "" && !e.OnDrive(it.Path, r.Drive) {
					t.Errorf("%s: %s is not on %s", r.ID, it.Path, r.Drive)
				}
			}
		}
		return
	}
	sa, ba := sum(all)
	sc, bc := sum(onC)
	sd, bd := sum(onD)
	if sa == 0 || sc != sa {
		t.Errorf("all demo junk is on C: — all=%d C=%d", sa, sc)
	}
	if sd != 0 || bd != 0 {
		t.Errorf("D: has no junk and an empty Recycle Bin, got %d bytes (bin %d)", sd, bd)
	}
	if ba == 0 || bc != ba {
		t.Errorf("Recycle Bin: all=%d C=%d", ba, bc)
	}
}

func TestProgramsOnDrive(t *testing.T) {
	e := demoEngine(t)
	apps := e.InstalledApps()
	var blender *App
	for _, a := range apps {
		if a.Name == "Blender 4.2" {
			blender = a
		}
		if a.Name == "Google Chrome" && a.Drive != `C:\` {
			t.Errorf("Chrome should be on C:, got %q", a.Drive)
		}
	}
	if blender == nil || blender.Drive != `D:\` {
		t.Fatalf("Blender should be an installed app on D:, got %+v", blender)
	}

	progs := e.ProgramsOnDrive(context.Background(), `D:\`, apps, noProgress)
	got := map[string]*Program{}
	var names []string
	for _, p := range progs {
		got[p.Name] = p
		names = append(names, p.Name)
		t.Logf("%-8s %-14s %8.1f MB  %s  exe=%s", p.Kind, p.Name, float64(p.Size)/1e6, p.Path, p.Exe)
	}
	sort.Strings(names)
	for name, kind := range map[string]string{"Hades": "game", "Celeste": "game", "GIMPPortable": "program", "Ffmpeg": "program", "Rufus": "program"} {
		p := got[name]
		if p == nil {
			t.Errorf("%s not found; found %v", name, names)
			continue
		}
		if p.Kind != kind || p.Drive != `D:\` || p.Exe == "" || p.Size == 0 {
			t.Errorf("%s: kind=%s drive=%s exe=%q size=%d", name, p.Kind, p.Drive, p.Exe, p.Size)
		}
	}
	for _, not := range []string{"Blender Foundation", "Photos", "Backups", "SteamLibrary"} {
		if got[not] != nil {
			t.Errorf("%s should not be listed as a program", not)
		}
	}
	if len(progs) > 1 && progs[0].Size < progs[len(progs)-1].Size {
		t.Errorf("programs should be sorted biggest first")
	}
}
