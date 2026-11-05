package platform

import (
	"strconv"
	"strings"
)

// ParseCommandLine splits a Windows command line into the program path and
// the remaining arguments. It copes with the unquoted paths containing spaces
// that many uninstall entries use, e.g. C:\Program Files\App\uninst.exe /S.
func ParseCommandLine(cmd string) (exe, args string) {
	cmd = strings.TrimSpace(cmd)
	if cmd == "" {
		return "", ""
	}
	if cmd[0] == '"' {
		if j := strings.IndexByte(cmd[1:], '"'); j >= 0 {
			return cmd[1 : 1+j], strings.TrimSpace(cmd[2+j:])
		}
		return strings.Trim(cmd, `"`), ""
	}
	lower := strings.ToLower(cmd)
	best := -1
	for _, ext := range []string{".exe", ".com", ".bat", ".cmd"} {
		from := 0
		for {
			i := strings.Index(lower[from:], ext)
			if i < 0 {
				break
			}
			end := from + i + len(ext)
			if end == len(cmd) || cmd[end] == ' ' || cmd[end] == '\t' {
				if best < 0 || end < best {
					best = end
				}
				break
			}
			from = end
		}
	}
	if best > 0 {
		return cmd[:best], strings.TrimSpace(cmd[best:])
	}
	if i := strings.IndexAny(cmd, " \t"); i >= 0 {
		return cmd[:i], strings.TrimSpace(cmd[i:])
	}
	return cmd, ""
}

// ParseIconLocation splits `"C:\app.exe",3` into the path and icon index.
func ParseIconLocation(s string) (path string, index int) {
	s = strings.TrimSpace(s)
	if s == "" {
		return "", 0
	}
	if i := strings.LastIndexByte(s, ','); i > 0 {
		if n, err := strconv.Atoi(strings.TrimSpace(s[i+1:])); err == nil {
			return strings.Trim(strings.TrimSpace(s[:i]), `"`), n
		}
	}
	return strings.Trim(s, `"`), 0
}
