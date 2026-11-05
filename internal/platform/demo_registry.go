package platform

import (
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"
	"sync"
	"time"
)

// fakeRegistry is an in-memory registry used by the demo PC.
type fakeRegistry struct {
	mu   sync.Mutex
	root *fakeKey
}

type fakeKey struct {
	name      string
	values    []RegValue
	subs      map[string]*fakeKey
	lastWrite time.Time
}

func newFakeRegistry() *fakeRegistry {
	return &fakeRegistry{root: &fakeKey{subs: map[string]*fakeKey{}}}
}

func normHive(key string) []string {
	hive, path := SplitKey(key)
	switch hive {
	case "HKEY_LOCAL_MACHINE":
		hive = "HKLM"
	case "HKEY_CURRENT_USER":
		hive = "HKCU"
	}
	parts := []string{hive}
	for _, p := range strings.Split(path, `\`) {
		if p != "" {
			parts = append(parts, p)
		}
	}
	return parts
}

func (r *fakeRegistry) find(key string, create bool) *fakeKey {
	k := r.root
	for _, p := range normHive(key) {
		next := k.subs[strings.ToLower(p)]
		if next == nil {
			if !create {
				return nil
			}
			next = &fakeKey{name: p, subs: map[string]*fakeKey{}, lastWrite: time.Now()}
			k.subs[strings.ToLower(p)] = next
		}
		k = next
	}
	return k
}

func (r *fakeRegistry) set(key string, v RegValue) {
	r.mu.Lock()
	defer r.mu.Unlock()
	k := r.find(key, true)
	for i, x := range k.values {
		if strings.EqualFold(x.Name, v.Name) {
			k.values[i] = v
			return
		}
	}
	k.values = append(k.values, v)
}

func (r *fakeRegistry) str(key, name, value string) {
	r.set(key, RegValue{Name: name, Type: RegSZ, S: value})
}

func (r *fakeRegistry) dword(key, name string, n uint64) {
	r.set(key, RegValue{Name: name, Type: RegDWORD, N: n, S: fmt.Sprint(n)})
}

func (r *fakeRegistry) touch(key string, t time.Time) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.find(key, true).lastWrite = t
}

func (r *fakeRegistry) SubKeys(key string) ([]string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	k := r.find(key, false)
	if k == nil {
		return nil, errors.New("not found")
	}
	var out []string
	for _, s := range k.subs {
		out = append(out, s.name)
	}
	sort.Strings(out)
	return out, nil
}

func (r *fakeRegistry) Values(key string) ([]RegValue, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	k := r.find(key, false)
	if k == nil {
		return nil, errors.New("not found")
	}
	return append([]RegValue(nil), k.values...), nil
}

func (r *fakeRegistry) LastWrite(key string) (time.Time, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	k := r.find(key, false)
	if k == nil {
		return time.Time{}, errors.New("not found")
	}
	return k.lastWrite, nil
}

func (r *fakeRegistry) Exists(key string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.find(key, false) != nil
}

func (r *fakeRegistry) DeleteTree(key string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	parts := normHive(key)
	if len(parts) < 2 {
		return errors.New("refusing to delete a hive")
	}
	parent := r.root
	for _, p := range parts[:len(parts)-1] {
		parent = parent.subs[strings.ToLower(p)]
		if parent == nil {
			return nil
		}
	}
	delete(parent.subs, strings.ToLower(parts[len(parts)-1]))
	return nil
}

func (r *fakeRegistry) DeleteValue(key, name string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	k := r.find(key, false)
	if k == nil {
		return nil
	}
	kept := k.values[:0]
	for _, v := range k.values {
		if !strings.EqualFold(v.Name, name) {
			kept = append(kept, v)
		}
	}
	k.values = kept
	return nil
}

func (r *fakeRegistry) SetBinary(key, name string, data []byte) error {
	r.set(key, RegValue{Name: name, Type: RegBinary, B: append([]byte(nil), data...)})
	return nil
}

func (r *fakeRegistry) Export(key, file string) error {
	r.mu.Lock()
	k := r.find(key, false)
	var b strings.Builder
	b.WriteString("Windows Registry Editor Version 5.00\r\n\r\n")
	var dump func(path string, k *fakeKey)
	dump = func(path string, k *fakeKey) {
		fmt.Fprintf(&b, "[%s]\r\n", path)
		for _, v := range k.values {
			fmt.Fprintf(&b, "%q=%q\r\n", v.Name, v.S)
		}
		b.WriteString("\r\n")
		for _, s := range k.subs {
			dump(path+`\`+s.name, s)
		}
	}
	if k != nil {
		dump(key, k)
	}
	r.mu.Unlock()
	if k == nil {
		return errors.New("key not found")
	}
	return os.WriteFile(file, []byte(b.String()), 0o644)
}
