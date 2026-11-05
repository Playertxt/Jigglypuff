//go:build windows

package platform

import (
	"encoding/binary"
	"errors"
	"fmt"
	"strings"
	"syscall"
	"time"
	"unicode/utf16"
	"unsafe"
)

const (
	keyQueryValue       = 0x0001
	keySetValue         = 0x0002
	keyCreateSubKey     = 0x0004
	keyEnumerateSubKeys = 0x0008
	keyWow6464Key       = 0x0100
	accessDelete        = 0x10000
	keyRead             = 0x20019

	hkeyClassesRoot  = 0x80000000
	hkeyCurrentUser  = 0x80000001
	hkeyLocalMachine = 0x80000002
	hkeyUsers        = 0x80000003
)

type winRegistry struct {
	// userSID, when set, makes HKCU resolve to HKU\<sid>. The elevated server
	// uses this so it always works on the hive of the person who started Jigglypuff.
	userSID string
}

func newWinRegistry(sid string) *winRegistry {
	r := &winRegistry{userSID: sid}
	if sid != "" && !r.Exists(`HKU\`+sid) {
		r.userSID = ""
	}
	return r
}

func (r *winRegistry) resolve(key string) (uintptr, string, error) {
	hive, p := SplitKey(key)
	switch hive {
	case "HKLM", "HKEY_LOCAL_MACHINE":
		return hkeyLocalMachine, p, nil
	case "HKCU", "HKEY_CURRENT_USER":
		if r.userSID != "" {
			return hkeyUsers, JoinKey(r.userSID, p), nil
		}
		return hkeyCurrentUser, p, nil
	case "HKU", "HKEY_USERS":
		return hkeyUsers, p, nil
	case "HKCR", "HKEY_CLASSES_ROOT":
		return hkeyClassesRoot, p, nil
	}
	return 0, "", fmt.Errorf("unknown registry hive %q", hive)
}

// regExePath returns the key as reg.exe expects it.
func (r *winRegistry) regExePath(key string) string {
	hive, p := SplitKey(key)
	switch hive {
	case "HKCU", "HKEY_CURRENT_USER":
		if r.userSID != "" {
			return JoinKey("HKU", r.userSID, p)
		}
		return JoinKey("HKCU", p)
	case "HKEY_LOCAL_MACHINE":
		return JoinKey("HKLM", p)
	case "HKEY_USERS":
		return JoinKey("HKU", p)
	}
	return JoinKey(hive, p)
}

func (r *winRegistry) open(key string, access uint32) (uintptr, error) {
	root, p, err := r.resolve(key)
	if err != nil {
		return 0, err
	}
	var h uintptr
	ret, _, _ := procRegOpenKeyExW.Call(root, uintptr(unsafe.Pointer(u16(p))), 0,
		uintptr(access|keyWow6464Key), uintptr(unsafe.Pointer(&h)))
	if ret != 0 {
		return 0, syscall.Errno(ret)
	}
	return h, nil
}

func regClose(h uintptr) { procRegCloseKey.Call(h) }

type keyInfo struct {
	subKeys, maxSubKeyLen, values, maxValueNameLen, maxValueLen uint32
	lastWrite                                                   syscall.Filetime
}

func queryInfo(h uintptr) (keyInfo, error) {
	var ki keyInfo
	ret, _, _ := procRegQueryInfoKeyW.Call(h, 0, 0, 0,
		uintptr(unsafe.Pointer(&ki.subKeys)), uintptr(unsafe.Pointer(&ki.maxSubKeyLen)), 0,
		uintptr(unsafe.Pointer(&ki.values)), uintptr(unsafe.Pointer(&ki.maxValueNameLen)),
		uintptr(unsafe.Pointer(&ki.maxValueLen)), 0, uintptr(unsafe.Pointer(&ki.lastWrite)))
	if ret != 0 {
		return ki, syscall.Errno(ret)
	}
	return ki, nil
}

func (r *winRegistry) SubKeys(key string) ([]string, error) {
	h, err := r.open(key, keyEnumerateSubKeys|keyQueryValue)
	if err != nil {
		return nil, err
	}
	defer regClose(h)
	ki, _ := queryInfo(h)
	buf := make([]uint16, ki.maxSubKeyLen+2)
	names := make([]string, 0, ki.subKeys)
	for i := uint32(0); i < 100000; i++ {
		n := uint32(len(buf))
		ret, _, _ := procRegEnumKeyExW.Call(h, uintptr(i), uintptr(unsafe.Pointer(&buf[0])),
			uintptr(unsafe.Pointer(&n)), 0, 0, 0, 0)
		if ret == errorNoMoreItems {
			break
		}
		if ret == errorMoreData {
			buf = make([]uint16, len(buf)*2+256)
			i--
			continue
		}
		if ret != 0 {
			return names, syscall.Errno(ret)
		}
		names = append(names, syscall.UTF16ToString(buf[:n]))
	}
	return names, nil
}

func (r *winRegistry) Values(key string) ([]RegValue, error) {
	h, err := r.open(key, keyQueryValue)
	if err != nil {
		return nil, err
	}
	defer regClose(h)
	ki, _ := queryInfo(h)
	nameBuf := make([]uint16, ki.maxValueNameLen+2)
	dataBuf := make([]byte, ki.maxValueLen+8)
	vals := make([]RegValue, 0, ki.values)
	for i := uint32(0); i < 100000; i++ {
		nameLen := uint32(len(nameBuf))
		dataLen := uint32(len(dataBuf))
		var typ uint32
		ret, _, _ := procRegEnumValueW.Call(h, uintptr(i), uintptr(unsafe.Pointer(&nameBuf[0])),
			uintptr(unsafe.Pointer(&nameLen)), 0, uintptr(unsafe.Pointer(&typ)),
			uintptr(unsafe.Pointer(&dataBuf[0])), uintptr(unsafe.Pointer(&dataLen)))
		if ret == errorNoMoreItems {
			break
		}
		if ret == errorMoreData {
			nameBuf = make([]uint16, len(nameBuf)*2+256)
			dataBuf = make([]byte, len(dataBuf)*2+1024)
			i--
			continue
		}
		if ret != 0 {
			return vals, syscall.Errno(ret)
		}
		if dataLen > uint32(len(dataBuf)) {
			dataLen = uint32(len(dataBuf))
		}
		vals = append(vals, decodeValue(syscall.UTF16ToString(nameBuf[:nameLen]), typ, dataBuf[:dataLen]))
	}
	return vals, nil
}

func decodeValue(name string, typ uint32, data []byte) RegValue {
	v := RegValue{Name: name, Type: typ}
	switch typ {
	case RegSZ, RegExpandSZ:
		v.S = utf16BytesToString(data)
		if typ == RegExpandSZ {
			v.S = expandEnv(v.S)
		}
	case RegMultiSZ:
		u := bytesToU16(data)
		var parts []string
		start := 0
		for i, c := range u {
			if c == 0 {
				if i > start {
					parts = append(parts, string(utf16.Decode(u[start:i])))
				}
				start = i + 1
			}
		}
		v.S = strings.Join(parts, "\n")
	case RegDWORD:
		if len(data) >= 4 {
			v.N = uint64(binary.LittleEndian.Uint32(data))
		}
		v.S = fmt.Sprint(v.N)
	case RegQWORD:
		if len(data) >= 8 {
			v.N = binary.LittleEndian.Uint64(data)
		}
		v.S = fmt.Sprint(v.N)
	default:
		v.B = append([]byte(nil), data...)
	}
	return v
}

func bytesToU16(b []byte) []uint16 {
	u := make([]uint16, len(b)/2)
	for i := range u {
		u[i] = binary.LittleEndian.Uint16(b[i*2:])
	}
	return u
}

func utf16BytesToString(b []byte) string {
	u := bytesToU16(b)
	for i, c := range u {
		if c == 0 {
			u = u[:i]
			break
		}
	}
	return string(utf16.Decode(u))
}

func (r *winRegistry) LastWrite(key string) (time.Time, error) {
	h, err := r.open(key, keyQueryValue)
	if err != nil {
		return time.Time{}, err
	}
	defer regClose(h)
	ki, err := queryInfo(h)
	if err != nil {
		return time.Time{}, err
	}
	return time.Unix(0, ki.lastWrite.Nanoseconds()), nil
}

func (r *winRegistry) Exists(key string) bool {
	h, err := r.open(key, keyQueryValue)
	if err != nil {
		return false
	}
	regClose(h)
	return true
}

func splitLastKey(key string) (parent, name string) {
	key = strings.TrimRight(key, `\`)
	i := strings.LastIndexByte(key, '\\')
	if i <= 0 {
		return "", ""
	}
	return key[:i], key[i+1:]
}

func (r *winRegistry) DeleteTree(key string) error {
	parent, name := splitLastKey(key)
	if parent == "" || name == "" {
		return errors.New("refusing to delete a registry hive")
	}
	h, err := r.open(parent, accessDelete|keyEnumerateSubKeys|keyQueryValue|keySetValue)
	if err != nil {
		return err
	}
	defer regClose(h)
	np := uintptr(unsafe.Pointer(u16(name)))
	ret, _, _ := procRegDeleteTreeW.Call(h, np)
	if ret != 0 && ret != errorFileNotFound {
		return syscall.Errno(ret)
	}
	if r.Exists(key) {
		ret, _, _ = procRegDeleteKeyExW.Call(h, np, keyWow6464Key, 0)
		if ret != 0 && ret != errorFileNotFound {
			return syscall.Errno(ret)
		}
	}
	return nil
}

func (r *winRegistry) DeleteValue(key, name string) error {
	h, err := r.open(key, keySetValue|keyQueryValue)
	if err != nil {
		return err
	}
	defer regClose(h)
	ret, _, _ := procRegDeleteValueW.Call(h, uintptr(unsafe.Pointer(u16(name))))
	if ret != 0 && ret != errorFileNotFound {
		return syscall.Errno(ret)
	}
	return nil
}

func (r *winRegistry) SetBinary(key, name string, data []byte) error {
	root, p, err := r.resolve(key)
	if err != nil {
		return err
	}
	var h uintptr
	var disp uint32
	ret, _, _ := procRegCreateKeyExW.Call(root, uintptr(unsafe.Pointer(u16(p))), 0, 0, 0,
		uintptr(keySetValue|keyQueryValue|keyWow6464Key), 0, uintptr(unsafe.Pointer(&h)), uintptr(unsafe.Pointer(&disp)))
	if ret != 0 {
		return syscall.Errno(ret)
	}
	defer regClose(h)
	var dp uintptr
	if len(data) > 0 {
		dp = uintptr(unsafe.Pointer(&data[0]))
	}
	ret, _, _ = procRegSetValueExW.Call(h, uintptr(unsafe.Pointer(u16(name))), 0, RegBinary, dp, uintptr(len(data)))
	if ret != 0 {
		return syscall.Errno(ret)
	}
	return nil
}

func (r *winRegistry) Export(key, file string) error {
	args := []string{"export", r.regExePath(key), file, "/y"}
	if hive, _ := SplitKey(key); hive == "HKLM" || hive == "HKEY_LOCAL_MACHINE" {
		args = append(args, "/reg:64")
	}
	out, err := hiddenCmd(nil, system32("reg.exe"), args...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("registry backup failed: %s", strings.TrimSpace(string(out)))
	}
	return nil
}
