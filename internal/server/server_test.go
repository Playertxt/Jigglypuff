package server

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"jigglypuff/internal/engine"
	"jigglypuff/internal/platform"
)

func startTest(t *testing.T) (*Server, string) {
	t.Helper()
	p, err := platform.NewDemo(filepath.Join(t.TempDir(), "pc"))
	if err != nil {
		t.Fatal(err)
	}
	web := fstest.MapFS{"index.html": {Data: []byte("<!doctype html>")}}
	s := New(Options{Platform: p, Web: web, Version: "test", KeepAlive: true})
	if err := s.Listen(); err != nil {
		t.Fatal(err)
	}
	go s.Serve()
	t.Cleanup(s.Shutdown)
	return s, fmt.Sprintf("http://127.0.0.1:%d", s.Port())
}

func call(t *testing.T, method, url, key string, body any, out any) int {
	t.Helper()
	var r io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		r = bytes.NewReader(b)
	}
	req, _ := http.NewRequest(method, url, r)
	if key != "" {
		req.Header.Set("X-Jigglypuff-Key", key)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if out != nil {
		json.NewDecoder(resp.Body).Decode(out)
	}
	return resp.StatusCode
}

func waitJob(t *testing.T, base, key, id string) map[string]any {
	t.Helper()
	for i := 0; i < 600; i++ {
		var j map[string]any
		call(t, "GET", base+"/api/jobs/"+id, key, nil, &j)
		if j["state"] != "running" {
			return j
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatal("job did not finish")
	return nil
}

func TestSecurityChecks(t *testing.T) {
	s, base := startTest(t)
	if code := call(t, "GET", base+"/api/info", "", nil, nil); code != 403 {
		t.Errorf("no key: got %d", code)
	}
	if code := call(t, "GET", base+"/api/info", "wrong", nil, nil); code != 403 {
		t.Errorf("wrong key: got %d", code)
	}
	if code := call(t, "GET", base+"/api/info", s.Token(), nil, nil); code != 200 {
		t.Errorf("right key: got %d", code)
	}
	req, _ := http.NewRequest("POST", base+"/api/scan/junk", nil)
	req.Header.Set("X-Jigglypuff-Key", s.Token())
	req.Header.Set("Origin", "https://evil.example")
	resp, _ := http.DefaultClient.Do(req)
	if resp.StatusCode != 403 {
		t.Errorf("foreign origin: got %d", resp.StatusCode)
	}
	req, _ = http.NewRequest("GET", base+"/api/info", nil)
	req.Host = "evil.example"
	req.Header.Set("X-Jigglypuff-Key", s.Token())
	resp, _ = http.DefaultClient.Do(req)
	if resp.StatusCode != 403 {
		t.Errorf("DNS rebinding host: got %d", resp.StatusCode)
	}
	// POST must use the header; a key in the URL only works for GET (images, events).
	if code := call(t, "POST", base+"/api/scan/junk?k="+s.Token(), "", nil, nil); code != 403 {
		t.Errorf("POST with key in URL: got %d", code)
	}
}

func TestPlansAreExactAndSingleUse(t *testing.T) {
	s, base := startTest(t)
	key := s.Token()
	var start map[string]string
	call(t, "POST", base+"/api/scan/junk", key, map[string]any{}, &start)
	j := waitJob(t, base, key, start["jobId"])
	res := j["result"].(map[string]any)
	scanID := res["scanId"].(string)

	// Stale scan ids are refused.
	var errResp map[string]string
	if code := call(t, "POST", base+"/api/plans", key, PlanRequest{Source: "junk", ScanID: "old", IDs: []string{"user-temp"}}, &errResp); code != 409 {
		t.Errorf("stale scan: got %d", code)
	}
	var plan Plan
	if code := call(t, "POST", base+"/api/plans", key, PlanRequest{Source: "junk", ScanID: scanID, IDs: []string{"user-temp"}}, &plan); code != 200 {
		t.Fatalf("plan: %d", code)
	}
	if plan.Summary.Items == 0 || plan.Summary.Bytes == 0 || plan.Summary.Recycle != 0 {
		t.Errorf("unexpected summary %+v", plan.Summary)
	}
	var exec map[string]string
	call(t, "POST", base+"/api/plans/"+plan.ID+"/execute", key, nil, &exec)
	done := waitJob(t, base, key, exec["jobId"])
	out := done["result"].(map[string]any)["result"].(map[string]any)
	if int(out["removed"].(float64)) != plan.Summary.Items {
		t.Errorf("removed %v of %d", out["removed"], plan.Summary.Items)
	}
	// The same plan can't run twice.
	if code := call(t, "POST", base+"/api/plans/"+plan.ID+"/execute", key, nil, nil); code != 404 {
		t.Errorf("second run: got %d", code)
	}
	var hist []HistoryEntry
	call(t, "GET", base+"/api/history", key, nil, &hist)
	if len(hist) != 1 || hist[0].Freed == 0 {
		t.Errorf("history not recorded: %+v", hist)
	}
}

func TestDuplicatesKeepOneCopy(t *testing.T) {
	s, base := startTest(t)
	key := s.Token()
	var start map[string]string
	call(t, "POST", base+"/api/scan/dups", key, map[string]any{}, &start)
	j := waitJob(t, base, key, start["jobId"])
	b, _ := json.Marshal(j["result"])
	var res struct {
		ScanID string             `json:"scanId"`
		Groups []*engine.DupGroup `json:"groups"`
	}
	json.Unmarshal(b, &res)
	if len(res.Groups) == 0 {
		t.Fatal("no duplicate groups")
	}
	var all []string
	for _, f := range res.Groups[0].Files {
		all = append(all, f.ID)
	}
	var e map[string]string
	if code := call(t, "POST", base+"/api/plans", key, PlanRequest{Source: "dups", ScanID: res.ScanID, IDs: all}, &e); code != 409 || !strings.Contains(e["error"], "keep at least one") {
		t.Errorf("selecting every copy must be refused: %d %v", code, e)
	}
	var plan Plan
	if code := call(t, "POST", base+"/api/plans", key, PlanRequest{Source: "dups", ScanID: res.ScanID, IDs: all[1:]}, &plan); code != 200 {
		t.Fatalf("plan: %d", code)
	}
	if plan.Summary.Recycle != len(all)-1 {
		t.Errorf("duplicates should go to the Recycle Bin: %+v", plan.Summary)
	}
}

func TestGuardRejectsTamperedTargets(t *testing.T) {
	s, _ := startTest(t)
	p := s.P.Paths()
	g := s.eng.Guard()
	for _, tg := range []engine.Target{
		{Kind: "folder", Path: p.WinDir, Purpose: engine.PurposeLeftover},
		{Kind: "folder", Path: p.UserProfile, Purpose: engine.PurposeLeftover},
		{Kind: "folder", Path: p.Documents, Purpose: engine.PurposeLeftover},
		{Kind: "file", Path: filepath.Join(p.WinDir, "System32", "kernel32.dll"), Purpose: engine.PurposeJunk, Root: filepath.Join(p.WinDir, "System32")},
		{Kind: "regkey", Path: `HKLM\SOFTWARE\Microsoft`, Purpose: engine.PurposeLeftover},
		{Kind: "regkey", Path: `HKLM\SYSTEM\CurrentControlSet\Services\Spooler`, Purpose: engine.PurposeLeftover},
		{Kind: "service", Path: "Spooler", Purpose: engine.PurposeLeftover},
		{Kind: "task", Path: `\Microsoft\Windows\Defrag\ScheduledDefrag`, Purpose: engine.PurposeLeftover},
		{Kind: "file", Path: filepath.Join(p.JigglypuffData, "settings.json"), Purpose: engine.PurposeUserFile},
	} {
		if err := s.eng.Validate(g, tg); err == nil {
			t.Errorf("guard allowed %s %s", tg.Kind, tg.Path)
		}
	}
	// Execute re-checks even if someone slips a target in.
	res := s.eng.Execute(context.Background(), []engine.Target{{Kind: "folder", Path: p.Documents, Purpose: engine.PurposeLeftover}}, t.TempDir(), func(engine.Progress) {})
	if res.Removed != 0 || res.Skipped != 1 {
		t.Errorf("execute must skip protected targets: %+v", res)
	}
	if _, err := os.Stat(p.Documents); err != nil {
		t.Errorf("Documents must still exist")
	}
}
