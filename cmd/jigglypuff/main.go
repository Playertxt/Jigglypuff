// Command jigglypuff is a minimal, careful PC cleaner for Windows.
//
// Normal start (double-click): the un-elevated launcher asks Windows for
// administrator rights (UAC) and starts an elevated copy of itself that runs
// the cleaning engine behind a local, key-protected web server. The launcher
// then opens the interface in a chrome-less Microsoft Edge app window — as the
// normal user, so the browser itself never runs with admin rights.
package main

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"jigglypuff/internal/platform"
	"jigglypuff/internal/server"
	"jigglypuff/internal/web"
)

var version = "1.0.0"

const preferredPort = 47815

func main() {
	serve := flag.Bool("serve", false, "internal: run the elevated server")
	ctxFile := flag.String("ctx", "", "internal: launcher context file")
	handshake := flag.String("handshake", "", "internal: where to report the server address")
	demo := flag.String("demo", "", "run against a simulated PC stored in this folder")
	port := flag.Int("port", preferredPort, "preferred port")
	keep := flag.Bool("keep", false, "keep running when the window closes")
	noWindow := flag.Bool("no-window", false, "don't open a window")
	showVersion := flag.Bool("version", false, "print the version")
	flag.Parse()

	if *showVersion {
		fmt.Println("Jigglypuff", version)
		return
	}
	if *demo != "" || runtime.GOOS != "windows" {
		runDemo(*demo, *port, *keep, *noWindow)
		return
	}
	if *serve {
		runElevatedServer(*ctxFile, *handshake)
		return
	}
	launch(*noWindow)
}

func fatal(msg string, err error) {
	log.Printf("%s: %v", msg, err)
	platform.MessageBox("Jigglypuff", fmt.Sprintf("%s\n\n%v", msg, err))
	os.Exit(1)
}

func setupLog(dir string) {
	os.MkdirAll(dir, 0o755)
	f := filepath.Join(dir, "jigglypuff.log")
	if fi, err := os.Stat(f); err == nil && fi.Size() > 1<<20 {
		os.Remove(f)
	}
	if lf, err := os.OpenFile(f, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644); err == nil {
		log.SetOutput(io.MultiWriter(lf, os.Stderr))
	}
}

func randomName(prefix string) string {
	b := make([]byte, 6)
	rand.Read(b)
	return prefix + hex.EncodeToString(b)
}

type instance struct {
	Port  int    `json:"port"`
	Token string `json:"token"`
}

func instanceURL(i instance) string { return fmt.Sprintf("http://127.0.0.1:%d/#k=%s", i.Port, i.Token) }

func browserProfile(p *platform.Paths) string { return filepath.Join(p.JigglypuffData, "Window") }

// launch runs as the signed-in user.
func launch(noWindow bool) {
	paths := platform.CurrentPaths()
	setupLog(paths.JigglypuffData)
	log.Printf("Jigglypuff %s starting (admin=%v)", version, platform.IsElevated())

	// Already running? Just open another window onto it.
	if b, err := os.ReadFile(filepath.Join(paths.JigglypuffData, "instance.json")); err == nil {
		var inst instance
		if json.Unmarshal(b, &inst) == nil && inst.Port > 0 && server.PingInstance(inst.Port, inst.Token) {
			if !noWindow {
				openWindow(instanceURL(inst), paths, inst)
			}
			return
		}
	}

	if !platform.IsElevated() {
		inst, err := startElevated(paths)
		if err == nil {
			if !noWindow {
				openWindow(instanceURL(inst), paths, inst)
			}
			return
		}
		log.Printf("continuing without administrator rights: %v", err)
	}
	runServer(paths, noWindow, !platform.IsElevated())
}

// startElevated starts "Jigglypuff.exe --serve" with a UAC prompt and waits for
// it to report its address.
func startElevated(paths *platform.Paths) (instance, error) {
	ctx := filepath.Join(paths.Temp, randomName("jigglypuff-ctx-")+".json")
	hs := ctx + ".ready"
	b, _ := json.Marshal(paths)
	if err := os.WriteFile(ctx, b, 0o600); err != nil {
		return instance{}, err
	}
	defer os.Remove(ctx)
	defer os.Remove(hs)
	alive, err := platform.ElevateSelf(fmt.Sprintf(`--serve --ctx="%s" --handshake="%s"`, ctx, hs))
	if err != nil {
		return instance{}, err
	}
	deadline := time.Now().Add(90 * time.Second)
	for time.Now().Before(deadline) {
		if b, err := os.ReadFile(hs); err == nil {
			var inst instance
			if json.Unmarshal(b, &inst) == nil && inst.Port > 0 {
				return inst, nil
			}
		}
		if !alive() {
			return instance{}, fmt.Errorf("the administrator copy of Jigglypuff stopped before it was ready")
		}
		time.Sleep(150 * time.Millisecond)
	}
	return instance{}, fmt.Errorf("the administrator copy of Jigglypuff didn't start")
}

// openWindow shows the interface and falls back to the default browser when
// no app window connects.
func openWindow(url string, paths *platform.Paths, inst instance) {
	if err := platform.OpenAppWindow(url, browserProfile(paths)); err != nil {
		log.Printf("app window failed: %v", err)
	}
	for i := 0; i < 100; i++ {
		if server.ClientConnected(inst.Port, inst.Token) {
			return
		}
		time.Sleep(250 * time.Millisecond)
	}
	log.Printf("no window connected; opening the default browser")
	platform.OpenDefaultBrowser(url)
}

// runElevatedServer is the administrator copy started by startElevated.
func runElevatedServer(ctxFile, handshake string) {
	own := platform.CurrentPaths()
	paths := own
	if b, err := os.ReadFile(ctxFile); err == nil {
		var user platform.Paths
		if json.Unmarshal(b, &user) == nil && user.UserProfile != "" {
			// System folders always come from this process; only the
			// signed-in user's folders are taken from the launcher.
			user.SystemDrive, user.WinDir = own.SystemDrive, own.WinDir
			user.ProgramFiles, user.ProgramFilesX86 = own.ProgramFiles, own.ProgramFilesX86
			user.ProgramData, user.CommonFiles, user.CommonFilesX86 = own.ProgramData, own.CommonFiles, own.CommonFilesX86
			user.CommonStartMenu, user.CommonStartup, user.PublicDesktop, user.Public = own.CommonStartMenu, own.CommonStartup, own.PublicDesktop, own.Public
			paths = &user
		}
	}
	setupLog(paths.JigglypuffData)
	srv := newServer(paths, false, nil)
	if err := srv.Listen(); err != nil {
		fatal("Jigglypuff couldn't start its engine", err)
	}
	srv.WriteInstanceFile()
	defer srv.RemoveInstanceFile()
	if handshake != "" {
		b, _ := json.Marshal(instance{Port: srv.Port(), Token: srv.Token()})
		os.WriteFile(handshake, b, 0o600)
	}
	log.Printf("elevated server on port %d", srv.Port())
	if err := srv.Serve(); err != nil {
		log.Printf("server stopped: %v", err)
	}
}

// runServer runs the engine in this process (already elevated, or the user
// declined the UAC prompt).
func runServer(paths *platform.Paths, noWindow, limited bool) {
	var restart func() (string, error)
	if limited {
		restart = func() (string, error) {
			inst, err := startElevated(paths)
			if err != nil {
				if err == platform.ErrCancelled {
					return "", fmt.Errorf("administrator rights weren't granted")
				}
				return "", err
			}
			return instanceURL(inst), nil
		}
	}
	srv := newServer(paths, false, restart)
	if err := srv.Listen(); err != nil {
		fatal("Jigglypuff couldn't start", err)
	}
	srv.WriteInstanceFile()
	defer srv.RemoveInstanceFile()
	if !noWindow {
		go openWindow(srv.URL(), paths, instance{Port: srv.Port(), Token: srv.Token()})
	}
	if err := srv.Serve(); err != nil {
		log.Printf("server stopped: %v", err)
	}
}

func newServer(paths *platform.Paths, keep bool, restart func() (string, error)) *server.Server {
	return server.New(server.Options{
		Platform:     platform.NewWindows(paths),
		Web:          web.FS(),
		Version:      version,
		Port:         preferredPort,
		KeepAlive:    keep,
		RestartAdmin: restart,
	})
}

// runDemo serves Jigglypuff against a simulated PC (any OS).
func runDemo(dir string, port int, keep, noWindow bool) {
	if dir == "" || dir == "1" || strings.EqualFold(dir, "true") {
		dir = filepath.Join(os.TempDir(), "jigglypuff-demo")
	}
	p, err := platform.NewDemo(dir)
	if err != nil {
		log.Fatal(err)
	}
	webFS := web.FS()
	if dir := os.Getenv("JIGGLYPUFF_WEB_DIR"); dir != "" {
		webFS = os.DirFS(dir) // development: serve the interface from disk
	}
	var restart func() (string, error)
	if !p.IsAdmin() {
		restart = func() (string, error) { return "", fmt.Errorf("the demo can't restart as administrator") }
	}
	srv := server.New(server.Options{Platform: p, Web: webFS, Version: version + "-demo", Port: port, KeepAlive: keep, RestartAdmin: restart})
	if err := srv.Listen(); err != nil {
		log.Fatal(err)
	}
	fmt.Println(srv.URL())
	if !noWindow {
		platform.OpenAppWindow(srv.URL(), filepath.Join(p.Paths().JigglypuffData, "Window"))
	}
	if err := srv.Serve(); err != nil {
		log.Fatal(err)
	}
}
