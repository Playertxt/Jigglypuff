package server

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"runtime/debug"
	"sync"
	"time"

	"jigglypuff/internal/engine"
)

func newID() string {
	b := make([]byte, 8)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// Job is a running or finished background task.
type Job struct {
	ID          string          `json:"id"`
	Type        string          `json:"type"`
	State       string          `json:"state"` // running | done | error | cancelled
	Progress    engine.Progress `json:"progress"`
	Error       string          `json:"error,omitempty"`
	Started     time.Time       `json:"started"`
	Result      any             `json:"result,omitempty"`
	destructive bool
	cancel      context.CancelFunc
	lastSent    time.Time
}

type jobs struct {
	mu  sync.Mutex
	all map[string]*Job
	hub *hub
}

func newJobs(h *hub) *jobs { return &jobs{all: map[string]*Job{}, hub: h} }

// start runs fn in the background. Destructive jobs are never cancelled when
// the window closes; Jigglypuff waits for them before exiting.
func (js *jobs) start(typ string, destructive bool, fn func(ctx context.Context, progress engine.ProgressFunc) (any, error)) *Job {
	ctx, cancel := context.WithCancel(context.Background())
	j := &Job{ID: newID(), Type: typ, State: "running", Started: time.Now(), destructive: destructive, cancel: cancel}
	js.mu.Lock()
	js.all[j.ID] = j
	// Forget old finished jobs.
	for id, o := range js.all {
		if o.State != "running" && time.Since(o.Started) > time.Hour {
			delete(js.all, id)
		}
	}
	js.mu.Unlock()
	js.publish(j, true)
	go func() {
		var res any
		var err error
		func() {
			defer func() {
				if r := recover(); r != nil {
					err = fmt.Errorf("unexpected problem: %v", r)
					debug.PrintStack()
				}
			}()
			res, err = fn(ctx, func(p engine.Progress) {
				js.mu.Lock()
				j.Progress = p
				js.mu.Unlock()
				js.publish(j, false)
			})
		}()
		js.mu.Lock()
		j.Result = res
		switch {
		case ctx.Err() != nil && !destructive:
			j.State = "cancelled"
		case err != nil:
			j.State, j.Error = "error", err.Error()
		default:
			j.State = "done"
		}
		js.mu.Unlock()
		cancel()
		js.publish(j, true)
	}()
	return j
}

func (js *jobs) publish(j *Job, force bool) {
	js.mu.Lock()
	if !force && time.Since(j.lastSent) < 120*time.Millisecond {
		js.mu.Unlock()
		return
	}
	j.lastSent = time.Now()
	msg := map[string]any{"id": j.ID, "type": j.Type, "state": j.State, "progress": j.Progress, "error": j.Error}
	js.mu.Unlock()
	js.hub.broadcast("job", msg)
}

func (js *jobs) get(id string) (*Job, bool) {
	js.mu.Lock()
	defer js.mu.Unlock()
	j, ok := js.all[id]
	return j, ok
}

func (js *jobs) snapshot(id string) ([]byte, bool) {
	js.mu.Lock()
	defer js.mu.Unlock()
	j, ok := js.all[id]
	if !ok {
		return nil, false
	}
	b, err := json.Marshal(j)
	if err != nil {
		b, _ = json.Marshal(map[string]any{"id": j.ID, "state": "error", "error": err.Error()})
	}
	return b, true
}

func (js *jobs) cancel(id string) bool {
	js.mu.Lock()
	defer js.mu.Unlock()
	if j, ok := js.all[id]; ok && j.State == "running" {
		j.cancel()
		return true
	}
	return false
}

// busy reports whether a destructive job is still running.
func (js *jobs) busy() bool {
	js.mu.Lock()
	defer js.mu.Unlock()
	for _, j := range js.all {
		if j.State == "running" && j.destructive {
			return true
		}
	}
	return false
}

func (js *jobs) cancelScans() {
	js.mu.Lock()
	defer js.mu.Unlock()
	for _, j := range js.all {
		if j.State == "running" && !j.destructive {
			j.cancel()
		}
	}
}

// hub fans server-sent events out to every open window.
type hub struct {
	mu            sync.Mutex
	clients       map[chan []byte]bool
	everConnected bool
	lastLeft      time.Time
}

func newHub() *hub { return &hub{clients: map[chan []byte]bool{}} }

func (h *hub) add() chan []byte {
	ch := make(chan []byte, 64)
	h.mu.Lock()
	h.clients[ch] = true
	h.everConnected = true
	h.mu.Unlock()
	return ch
}

func (h *hub) remove(ch chan []byte) {
	h.mu.Lock()
	delete(h.clients, ch)
	h.lastLeft = time.Now()
	h.mu.Unlock()
}

func (h *hub) count() int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return len(h.clients)
}

func (h *hub) broadcast(event string, data any) {
	b, err := json.Marshal(data)
	if err != nil {
		return
	}
	msg := []byte("event: " + event + "\ndata: " + string(b) + "\n\n")
	h.mu.Lock()
	defer h.mu.Unlock()
	for ch := range h.clients {
		select {
		case ch <- msg:
		default: // slow client: drop the update, the next one will catch up
		}
	}
}
