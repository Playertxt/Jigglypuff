package engine

import (
	"path/filepath"
	"regexp"
	"strings"
	"unicode"
)

// Norm lower-cases s and keeps only letters and digits, so that
// "Notepad++", "notepad" and "NOTEPAD" all compare equal.
func Norm(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range strings.ToLower(s) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
		}
	}
	return b.String()
}

var (
	reBrackets = regexp.MustCompile(`\([^)]*\)|\[[^\]]*\]`)
	reVersion  = regexp.MustCompile(`(?i)\bv?\d+([._]\d+)+[a-z0-9\-]*\b|\b(version|ver)\s*\d+\b`)
	reArch     = regexp.MustCompile(`(?i)\b(x64|x86|x86_64|amd64|arm64|64-bit|32-bit|64 bit|32 bit|win64|win32)\b`)
	reCorp     = regexp.MustCompile(`(?i)[,.]?\s+\b(inc|incorporated|llc|l\.l\.c|ltd|limited|corp|corporation|co|company|gmbh|ag|s\.?a|s\.?r\.?o|srl|b\.?v|ab|oy|plc|pty|k\.?k|sas|spa|s\.?l)\b\.?`)
	reBizWords = regexp.MustCompile(`(?i)\b(technologies|technology|software|systems|labs|studio|studios|games|interactive|entertainment|group|holdings|foundation|project|team|community|development|video communications|communications)\b`)
	reSpaces   = regexp.MustCompile(`\s+`)
)

// genericWords are never used on their own to match folders.
var genericWords = map[string]bool{
	"app": true, "apps": true, "application": true, "update": true, "updater": true, "setup": true,
	"install": true, "installer": true, "uninstall": true, "tools": true, "tool": true, "driver": true,
	"drivers": true, "microsoft": true, "windows": true, "common": true, "system": true, "runtime": true,
	"service": true, "services": true, "software": true, "data": true, "cache": true, "user": true,
	"users": true, "local": true, "program": true, "programs": true, "files": true, "temp": true,
	"logs": true, "log": true, "config": true, "settings": true, "desktop": true, "studio": true,
	"player": true, "manager": true, "edition": true, "launcher": true, "helper": true, "client": true,
	"pro": true, "free": true, "plus": true, "home": true, "sdk": true, "bin": true, "lib": true,
	"core": true, "package": true, "packages": true, "default": true, "shared": true, "support": true,
	"library": true, "framework": true, "host": true, "agent": true, "web": true, "online": true,
	"cloud": true, "media": true, "audio": true, "video": true, "game": true, "games": true,
	"utility": true, "utilities": true, "assistant": true, "center": true, "centre": true,
	"components": true, "component": true, "plugin": true, "plugins": true, "extension": true,
	"extensions": true, "module": true, "modules": true, "the": true, "and": true, "for": true,
	"new": true, "lite": true, "light": true, "beta": true, "preview": true, "insiders": true,
	"drive": true, "photos": true, "photo": true, "mail": true, "store": true, "music": true, "notes": true,
	"tasks": true, "documents": true, "desktop app": true, "workspace": true,
}

// cleanName strips versions, architecture tags and brackets from a product name.
func cleanName(s string) string {
	s = reBrackets.ReplaceAllString(s, " ")
	s = reVersion.ReplaceAllString(s, " ")
	s = reArch.ReplaceAllString(s, " ")
	s = strings.NewReplacer(" - ", " ", "_", " ", "–", " ").Replace(s)
	return strings.TrimSpace(reSpaces.ReplaceAllString(s, " "))
}

// PublisherTokens returns normalised forms of a publisher name, e.g.
// "Epic Games, Inc." → ["epicgames", "epic"].
func PublisherTokens(pub string) []string {
	pub = strings.TrimSpace(reBrackets.ReplaceAllString(pub, " "))
	if pub == "" {
		return nil
	}
	legal := strings.TrimSpace(reCorp.ReplaceAllString(pub, ""))
	var out []string
	addTok(&out, Norm(legal), 3)
	short := strings.TrimSpace(reBizWords.ReplaceAllString(legal, ""))
	addTok(&out, Norm(short), 4)
	if f := strings.Fields(short); len(f) > 1 {
		addTok(&out, Norm(f[0]), 5)
	}
	return out
}

// AppTokens returns the names an app's folders and registry keys are likely
// to use: the cleaned display name, the name without the publisher prefix,
// the install folder name and the main executable name.
func AppTokens(name, publisher, installDir string, exes []string) []string {
	var out []string
	cn := cleanName(name)
	addTok(&out, Norm(cn), 3)
	words := strings.Fields(cn)
	pubToks := PublisherTokens(publisher)
	// The publisher's first word, even a generic one like "Microsoft", is
	// stripped from the front of the app name ("Microsoft Visual Studio Code").
	pubFirst := ""
	if f := strings.Fields(strings.TrimSpace(reCorp.ReplaceAllString(publisher, ""))); len(f) > 0 {
		pubFirst = Norm(f[0])
	}
	if len(words) > 1 {
		first := Norm(words[0])
		strip := first != "" && first == pubFirst
		for _, pt := range pubToks {
			if first == pt || strings.HasPrefix(pt, first) {
				strip = true
			}
		}
		// The rest of the name must be distinctive on its own ("chrome",
		// "visualstudiocode"); short leftovers such as "drive" are too vague.
		// The publisher's own name is never an app token: folders named after
		// the vendor are handled as vendor folders, which are shared.
		if strip {
			addTok(&out, Norm(strings.Join(words[1:], " ")), 6)
		}
	}
	if installDir != "" {
		base := filepath.Base(platformClean(installDir))
		addTok(&out, Norm(cleanName(base)), 3)
		addTok(&out, Norm(base), 3)
	}
	for _, e := range exes {
		b := strings.TrimSuffix(strings.ToLower(filepath.Base(e)), strings.ToLower(filepath.Ext(e)))
		if isGenericExe(b) {
			continue
		}
		addTok(&out, Norm(b), 4)
	}
	return out
}

func platformClean(p string) string {
	p = strings.TrimRight(strings.TrimSpace(strings.Trim(p, `"`)), `\/`)
	return p
}

func addTok(list *[]string, t string, minLen int) {
	if len([]rune(t)) < minLen || genericWords[t] {
		return
	}
	for _, x := range *list {
		if x == t {
			return
		}
	}
	*list = append(*list, t)
}

func isGenericExe(b string) bool {
	switch {
	case strings.HasPrefix(b, "unins"), strings.HasPrefix(b, "uninst"), strings.HasPrefix(b, "setup"),
		strings.HasPrefix(b, "install"), b == "update", b == "updater", b == "msiexec", b == "rundll32",
		strings.HasPrefix(b, "au_"), b == "helper", b == "launcher", b == "maintenancetool", b == "cmd",
		b == "powershell", b == "explorer", b == "elevate", b == "app", b == "main":
		return true
	}
	return false
}

// Match strengths.
const (
	MatchNone   = 0
	MatchWeak   = 1
	MatchStrong = 2
)

// MatchName compares a folder or key name with app tokens.
func MatchName(name string, tokens []string) int {
	f := Norm(name)
	if len(f) < 3 {
		return MatchNone
	}
	best := MatchNone
	for _, t := range tokens {
		if len(t) < 3 {
			continue
		}
		switch {
		case f == t:
			return MatchStrong
		case len(t) >= 5 && strings.HasPrefix(f, t) && isVersionTail(f[len(t):]):
			best = MatchStrong
		case len(t) >= 5 && strings.HasPrefix(f, t):
			if best < MatchWeak {
				best = MatchWeak
			}
		case len(f) >= 5 && strings.HasPrefix(t, f) && len(f)*10 >= len(t)*6:
			if best < MatchWeak {
				best = MatchWeak
			}
		}
	}
	return best
}

// isVersionTail reports whether s looks like a version suffix ("2023", "64").
func isVersionTail(s string) bool {
	if s == "" {
		return true
	}
	digits := 0
	for _, r := range s {
		if unicode.IsDigit(r) {
			digits++
		}
	}
	return digits*2 >= len(s) && len(s) <= 10
}
