// Package server exposes Jigglypuff's engine to its window through a small HTTP
// API on 127.0.0.1. Every API call must carry a random per-session key, and
// requests from other origins or host names are refused, so web pages can't
// talk to it.
package server

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"jigglypuff/internal/engine"
	"jigglypuff/internal/platform"
)

// Options configure a server.
type Options struct {
	Platform     platform.Platform
	Web          fs.FS
	Version      string
	Port         int  // preferred port, 0 = any
	KeepAlive    bool // don't exit when the window closes (development)
	RestartAdmin func() (string, error)
}

// Server is Jigglypuff's local back end.
type Server struct {
	P       platform.Platform
	eng     *engine.Engine
	opts    Options
	state   *store
	hub     *hub
	jobs    *jobs
	token   string
	ln      net.Listener
	port    int
	srv     *http.Server
	started time.Time
	quit    chan struct{}
	once    sync.Once
}

// New creates a server.
func New(o Options) *Server {
	st := newStore(o.Platform.Paths().JigglypuffData)
	h := newHub()
	b := make([]byte, 24)
	rand.Read(b)
	s := &Server{P: o.Platform, opts: o, state: st, hub: h, jobs: newJobs(h), token: hex.EncodeToString(b),
		started: time.Now(), quit: make(chan struct{})}
	s.eng = engine.New(o.Platform, st.loadSettings())
	return s
}

// Listen binds to 127.0.0.1.
func (s *Server) Listen() error {
	var err error
	for _, port := range []int{s.opts.Port, s.opts.Port + 1, s.opts.Port + 2, 0} {
		if s.opts.Port == 0 && port != 0 {
			continue
		}
		s.ln, err = net.Listen("tcp", "127.0.0.1:"+strconv.Itoa(port))
		if err == nil {
			break
		}
	}
	if err != nil {
		return err
	}
	s.port = s.ln.Addr().(*net.TCPAddr).Port
	return nil
}

// URL is the address the window opens, including the session key.
func (s *Server) URL() string { return fmt.Sprintf("http://127.0.0.1:%d/#k=%s", s.port, s.token) }

// Port and Token are used for the single-instance handshake.
func (s *Server) Port() int             { return s.port }
func (s *Server) Token() string         { return s.token }
func (s *Server) Done() <-chan struct{} { return s.quit }

// Shutdown stops the server.
func (s *Server) Shutdown() {
	s.once.Do(func() {
		close(s.quit)
		if s.srv != nil {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			s.srv.Shutdown(ctx)
		}
	})
}

// Serve handles requests until the window has been closed for a few seconds
// (and nothing is being deleted), or Shutdown is called.
func (s *Server) Serve() error {
	mux := http.NewServeMux()
	s.routes(mux)
	s.srv = &http.Server{Handler: s.guard(mux), ReadHeaderTimeout: 10 * time.Second}
	if !s.opts.KeepAlive {
		go s.watchLifetime()
	}
	err := s.srv.Serve(s.ln)
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}

func (s *Server) watchLifetime() {
	t := time.NewTicker(time.Second)
	defer t.Stop()
	for {
		select {
		case <-s.quit:
			return
		case <-t.C:
		}
		s.hub.mu.Lock()
		n, ever, left := len(s.hub.clients), s.hub.everConnected, s.hub.lastLeft
		s.hub.mu.Unlock()
		idle := (!ever && time.Since(s.started) > 3*time.Minute) || (ever && n == 0 && time.Since(left) > 8*time.Second)
		if !idle {
			continue
		}
		s.jobs.cancelScans()
		if s.jobs.busy() {
			continue // finish deleting first
		}
		s.Shutdown()
		return
	}
}

// guard enforces the host, origin and session key checks.
func (s *Server) guard(next http.Handler) http.Handler {
	hosts := map[string]bool{fmt.Sprintf("127.0.0.1:%d", s.port): true, fmt.Sprintf("localhost:%d", s.port): true}
	origins := map[string]bool{}
	for h := range hosts {
		origins["http://"+h] = true
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !hosts[r.Host] {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		if o := r.Header.Get("Origin"); o != "" && !origins[o] {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("X-Frame-Options", "DENY")
		if strings.HasPrefix(r.URL.Path, "/api/") {
			key := r.Header.Get("X-Jigglypuff-Key")
			if key == "" && r.Method == http.MethodGet {
				key = r.URL.Query().Get("k")
			}
			if key != s.token {
				http.Error(w, "forbidden", http.StatusForbidden)
				return
			}
			w.Header().Set("Cache-Control", "no-store")
		} else {
			w.Header().Set("Content-Security-Policy", "default-src 'self'; img-src 'self' data:; style-src 'self' 'unsafe-inline'; script-src 'self'; connect-src 'self'; frame-ancestors 'none'; base-uri 'none'; form-action 'none'")
		}
		next.ServeHTTP(w, r)
	})
}

// Helpers ---------------------------------------------------------------------

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Println("encode:", err)
	}
}

func writeErr(w http.ResponseWriter, code int, err error) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
}

func readJSON(r *http.Request, v any) error {
	if r.Body == nil {
		return nil
	}
	defer r.Body.Close()
	dec := json.NewDecoder(http.MaxBytesReader(nil, r.Body, 8<<20))
	if err := dec.Decode(v); err != nil && err.Error() != "EOF" {
		return fmt.Errorf("bad request: %v", err)
	}
	return nil
}

func (s *Server) backupDir() string {
	return filepath.Join(s.P.Paths().JigglypuffData, "Backups", time.Now().Format("2006-01-02 150405"))
}

// PingInstance checks whether a Jigglypuff server answers on port with token.
func PingInstance(port int, token string) bool {
	c := &http.Client{Timeout: 1500 * time.Millisecond}
	req, _ := http.NewRequest(http.MethodGet, fmt.Sprintf("http://127.0.0.1:%d/api/ping", port), nil)
	req.Header.Set("X-Jigglypuff-Key", token)
	resp, err := c.Do(req)
	if err != nil {
		return false
	}
	resp.Body.Close()
	return resp.StatusCode == 200
}

// ClientConnected reports whether a window has connected yet.
func ClientConnected(port int, token string) bool {
	c := &http.Client{Timeout: 1500 * time.Millisecond}
	req, _ := http.NewRequest(http.MethodGet, fmt.Sprintf("http://127.0.0.1:%d/api/ping", port), nil)
	req.Header.Set("X-Jigglypuff-Key", token)
	resp, err := c.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	var v struct {
		Clients int  `json:"clients"`
		Ever    bool `json:"ever"`
	}
	json.NewDecoder(resp.Body).Decode(&v)
	return v.Ever
}

// RemoveInstanceFile deletes instance.json if it still describes this server.
func (s *Server) RemoveInstanceFile() {
	f := filepath.Join(s.P.Paths().JigglypuffData, "instance.json")
	b, err := os.ReadFile(f)
	if err != nil {
		return
	}
	var v struct {
		Token string `json:"token"`
	}
	if json.Unmarshal(b, &v) == nil && v.Token == s.token {
		os.Remove(f)
	}
}

// WriteInstanceFile records where this server listens, for single-instance use.
func (s *Server) WriteInstanceFile() string {
	f := filepath.Join(s.P.Paths().JigglypuffData, "instance.json")
	b, _ := json.Marshal(map[string]any{"port": s.port, "token": s.token, "pid": os.Getpid()})
	os.MkdirAll(filepath.Dir(f), 0o755)
	os.WriteFile(f, b, 0o600)
	return f
}
