package platform

import (
	"os"
	"strings"
	"sync"
)

var (
	envMu        sync.RWMutex
	envOverrides = map[string]string{}
)

// SetEnvOverrides makes %VAR% expansion use the signed-in user's folders even
// when Jigglypuff's server runs elevated under another account.
func SetEnvOverrides(p *Paths) {
	envMu.Lock()
	defer envMu.Unlock()
	set := func(k, v string) {
		if v != "" {
			envOverrides[strings.ToUpper(k)] = v
		}
	}
	set("USERPROFILE", p.UserProfile)
	set("APPDATA", p.AppData)
	set("LOCALAPPDATA", p.LocalAppData)
	set("TEMP", p.Temp)
	set("TMP", p.Temp)
	set("USERNAME", p.UserName)
	set("PUBLIC", p.Public)
}

func lookupEnv(name string) string {
	envMu.RLock()
	v, ok := envOverrides[strings.ToUpper(name)]
	envMu.RUnlock()
	if ok {
		return v
	}
	return os.Getenv(name)
}

// expandEnv expands Windows style %VARIABLE% references. Unknown variables are
// left untouched.
func expandEnv(s string) string {
	if !strings.Contains(s, "%") {
		return s
	}
	var b strings.Builder
	for {
		i := strings.IndexByte(s, '%')
		if i < 0 {
			b.WriteString(s)
			break
		}
		j := strings.IndexByte(s[i+1:], '%')
		if j < 0 {
			b.WriteString(s)
			break
		}
		name := s[i+1 : i+1+j]
		if v := lookupEnv(name); name != "" && v != "" {
			b.WriteString(s[:i])
			b.WriteString(v)
		} else {
			b.WriteString(s[:i+1+j+1])
		}
		s = s[i+1+j+1:]
	}
	return b.String()
}

// ExpandEnv is the exported form of expandEnv.
func ExpandEnv(s string) string { return expandEnv(s) }
