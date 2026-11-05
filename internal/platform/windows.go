//go:build windows

package platform

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"
	"unsafe"
)

type winPlatform struct {
	paths *Paths
	reg   *winRegistry
	admin bool

	iconMu    sync.Mutex
	iconCache map[string][]byte
}

// NewWindows creates the real Windows platform for the given user folders.
func NewWindows(p *Paths) Platform {
	SetEnvOverrides(p)
	return &winPlatform{
		paths:     p,
		reg:       newWinRegistry(p.UserSID),
		admin:     IsElevated(),
		iconCache: map[string][]byte{},
	}
}

func (w *winPlatform) Kind() string     { return "windows" }
func (w *winPlatform) Paths() *Paths    { return w.paths }
func (w *winPlatform) Reg() Registry    { return w.reg }
func (w *winPlatform) IsAdmin() bool    { return w.admin }
func (w *winPlatform) UserName() string { return w.paths.UserName }

// Drives -----------------------------------------------------------------------

func (w *winPlatform) Drives() []Drive {
	buf := make([]uint16, 512)
	n, _, _ := procGetLogicalDriveStringsW.Call(uintptr(len(buf)), uintptr(unsafe.Pointer(&buf[0])))
	if n == 0 || int(n) > len(buf) {
		return nil
	}
	var drives []Drive
	start := 0
	for i := 0; i < int(n); i++ {
		if buf[i] != 0 {
			continue
		}
		root := syscall.UTF16ToString(buf[start:i])
		start = i + 1
		if root == "" {
			continue
		}
		rp := u16(root)
		if t, _, _ := procGetDriveTypeW.Call(uintptr(unsafe.Pointer(rp))); t != 3 { // DRIVE_FIXED
			continue
		}
		var free, total, totalFree uint64
		if r, _, _ := procGetDiskFreeSpaceExW.Call(uintptr(unsafe.Pointer(rp)), uintptr(unsafe.Pointer(&free)),
			uintptr(unsafe.Pointer(&total)), uintptr(unsafe.Pointer(&totalFree))); r == 0 || total == 0 {
			continue
		}
		label := make([]uint16, 261)
		procGetVolumeInformationW.Call(uintptr(unsafe.Pointer(rp)), uintptr(unsafe.Pointer(&label[0])), 261, 0, 0, 0, 0, 0)
		drives = append(drives, Drive{
			Root:   root,
			Label:  syscall.UTF16ToString(label),
			Total:  int64(total),
			Free:   int64(free),
			System: strings.EqualFold(strings.TrimRight(root, `\`), strings.TrimRight(w.paths.SystemDrive, `\`)),
		})
	}
	return drives
}

// Processes ----------------------------------------------------------------------

func (w *winPlatform) Processes() []Process {
	snap, err := syscall.CreateToolhelp32Snapshot(syscall.TH32CS_SNAPPROCESS, 0)
	if err != nil {
		return nil
	}
	defer syscall.CloseHandle(snap)
	var pe syscall.ProcessEntry32
	pe.Size = uint32(unsafe.Sizeof(pe))
	var out []Process
	buf := make([]uint16, 1024)
	for err = syscall.Process32First(snap, &pe); err == nil; err = syscall.Process32Next(snap, &pe) {
		p := Process{PID: int(pe.ProcessID), Name: syscall.UTF16ToString(pe.ExeFile[:])}
		if h, e := syscall.OpenProcess(0x1000, false, pe.ProcessID); e == nil { // QUERY_LIMITED_INFORMATION
			n := uint32(len(buf))
			if r, _, _ := procQueryFullProcessImageNameW.Call(uintptr(h), 0, uintptr(unsafe.Pointer(&buf[0])), uintptr(unsafe.Pointer(&n))); r != 0 {
				p.Path = syscall.UTF16ToString(buf[:n])
			}
			syscall.CloseHandle(h)
		}
		out = append(out, p)
	}
	return out
}

// Recycle Bin --------------------------------------------------------------------

type shQueryRBInfo struct {
	cbSize   uint32
	_        uint32
	size     int64
	numItems int64
}

func (w *winPlatform) RecycleBinInfo() (int64, int64, error) {
	info := shQueryRBInfo{cbSize: uint32(unsafe.Sizeof(shQueryRBInfo{}))}
	var hr uintptr
	withCOM(func() { hr, _, _ = procSHQueryRecycleBinW.Call(0, uintptr(unsafe.Pointer(&info))) })
	if hr != 0 {
		return 0, 0, fmt.Errorf("could not read the Recycle Bin (0x%x)", hr)
	}
	return info.size, info.numItems, nil
}

func (w *winPlatform) EmptyRecycleBin() error {
	size, _, _ := w.RecycleBinInfo()
	var hr uintptr
	withCOM(func() { hr, _, _ = procSHEmptyRecycleBinW.Call(0, 0, 0x1|0x2|0x4) })
	if hr != 0 && size > 0 {
		return fmt.Errorf("could not empty the Recycle Bin (0x%x)", hr)
	}
	return nil
}

type shFileOpStruct struct {
	hwnd                  uintptr
	wFunc                 uint32
	pFrom                 *uint16
	pTo                   *uint16
	fFlags                uint16
	fAnyOperationsAborted int32
	hNameMappings         uintptr
	lpszProgressTitle     *uint16
}

func (w *winPlatform) Recycle(paths []string) error {
	if len(paths) == 0 {
		return nil
	}
	var buf []uint16
	for _, p := range paths {
		if !filepath.IsAbs(p) || strings.ContainsAny(p, "*?") {
			return fmt.Errorf("invalid path %q", p)
		}
		u, err := syscall.UTF16FromString(p)
		if err != nil {
			return err
		}
		buf = append(buf, u...) // includes the terminating NUL
	}
	buf = append(buf, 0) // list ends with a double NUL
	const (
		foDelete          = 3
		fofSilent         = 0x0004
		fofNoConfirmation = 0x0010
		fofAllowUndo      = 0x0040
		fofNoErrorUI      = 0x0400
		fofWantNukeWarn   = 0x4000 // ask before deleting something that can't be recycled
	)
	op := shFileOpStruct{wFunc: foDelete, pFrom: &buf[0], fFlags: fofAllowUndo | fofNoConfirmation | fofSilent | fofNoErrorUI | fofWantNukeWarn}
	var ret uintptr
	withCOM(func() { ret, _, _ = procSHFileOperationW.Call(uintptr(unsafe.Pointer(&op))) })
	if op.fAnyOperationsAborted != 0 {
		return errors.New("the move to the Recycle Bin was cancelled")
	}
	if ret != 0 {
		return fmt.Errorf("could not move to the Recycle Bin (code 0x%x)", ret)
	}
	return nil
}

func (w *winPlatform) RecycleLimit(path string) (int64, bool) {
	vol := make([]uint16, 300)
	if r, _, _ := procGetVolumePathNameW.Call(uintptr(unsafe.Pointer(u16(path))), uintptr(unsafe.Pointer(&vol[0])), uintptr(len(vol))); r == 0 {
		return 0, true
	}
	root := syscall.UTF16ToString(vol)
	if t, _, _ := procGetDriveTypeW.Call(uintptr(unsafe.Pointer(u16(root)))); t != 3 {
		return 0, true // removable and network drives have no Recycle Bin
	}
	name := make([]uint16, 64)
	var total int64
	for _, d := range w.Drives() {
		if strings.EqualFold(d.Root, root) {
			total = d.Total
		}
	}
	if r, _, _ := procGetVolumeNameForVolumeMountPointW.Call(uintptr(unsafe.Pointer(u16(root))), uintptr(unsafe.Pointer(&name[0])), uintptr(len(name))); r != 0 {
		vname := syscall.UTF16ToString(name) // \\?\Volume{GUID}\
		if i, j := strings.IndexByte(vname, '{'), strings.IndexByte(vname, '}'); i >= 0 && j > i {
			m := ValueMap(must(w.reg.Values(`HKCU\Software\Microsoft\Windows\CurrentVersion\Explorer\BitBucket\Volume\` + vname[i:j+1])))
			if n, ok := Num(m, "NukeOnDelete"); ok && n == 1 {
				return 0, true
			}
			if n, ok := Num(m, "MaxCapacity"); ok && n > 0 {
				return int64(n) * 1024 * 1024, false
			}
		}
	}
	if total > 0 {
		return total / 20, false // Windows' default is roughly 5% of the drive
	}
	return 0, false
}

// Explorer & launching -------------------------------------------------------------

func explorerExe() string {
	win := os.Getenv("SystemRoot")
	if win == "" {
		win = `C:\Windows`
	}
	return filepath.Join(win, "explorer.exe")
}

func (w *winPlatform) Reveal(path string) error {
	if !Exists(path) {
		return w.OpenFolder(filepath.Dir(path))
	}
	cmd := exec.Command(explorerExe())
	cmd.SysProcAttr = &syscall.SysProcAttr{CmdLine: `"` + explorerExe() + `" /select,"` + path + `"`}
	return cmd.Start()
}

func (w *winPlatform) OpenFolder(path string) error {
	if !IsDir(path) {
		return fmt.Errorf("folder not found: %s", path)
	}
	cmd := exec.Command(explorerExe())
	cmd.SysProcAttr = &syscall.SysProcAttr{CmdLine: `"` + explorerExe() + `" "` + path + `"`}
	return cmd.Start()
}

func (w *winPlatform) Launch(target string) error {
	_, err := shellExecute(context.Background(), "open", target, "", false)
	return err
}

type shellExecuteInfo struct {
	cbSize         uint32
	fMask          uint32
	hwnd           uintptr
	lpVerb         *uint16
	lpFile         *uint16
	lpParameters   *uint16
	lpDirectory    *uint16
	nShow          int32
	hInstApp       uintptr
	lpIDList       uintptr
	lpClass        *uint16
	hkeyClass      uintptr
	dwHotKey       uint32
	hIconOrMonitor uintptr
	hProcess       uintptr
}

// ErrCancelled means the user said "No" to a UAC prompt.
var ErrCancelled = errors.New("cancelled by the user")

// shellExecute runs ShellExecuteEx. With wait=true it waits for the started
// process and returns its exit code.
func shellExecute(ctx context.Context, verb, file, params string, wait bool) (int, error) {
	sei := shellExecuteInfo{fMask: 0x40 | 0x100 | 0x400, nShow: 1} // NOCLOSEPROCESS | NOASYNC | FLAG_NO_UI
	sei.cbSize = uint32(unsafe.Sizeof(sei))
	sei.lpVerb = u16(verb)
	sei.lpFile = u16(file)
	if params != "" {
		sei.lpParameters = u16(params)
	}
	var ok uintptr
	var callErr error
	withCOM(func() { ok, _, callErr = procShellExecuteExW.Call(uintptr(unsafe.Pointer(&sei))) })
	if ok == 0 {
		if errno, isErrno := callErr.(syscall.Errno); isErrno && errno == errorCancelled {
			return -1, ErrCancelled
		}
		return -1, fmt.Errorf("could not start %s: %v", filepath.Base(file), callErr)
	}
	if sei.hProcess == 0 {
		return 0, nil
	}
	h := syscall.Handle(sei.hProcess)
	defer syscall.CloseHandle(h)
	if !wait {
		return 0, nil
	}
	for {
		ev, _ := syscall.WaitForSingleObject(h, 300)
		if ev == syscall.WAIT_OBJECT_0 {
			break
		}
		if ctx.Err() != nil {
			return -1, ctx.Err()
		}
	}
	var code uint32
	syscall.GetExitCodeProcess(h, &code)
	return int(code), nil
}

// Icons ------------------------------------------------------------------------------

type iconInfo struct {
	fIcon    int32
	xHotspot uint32
	yHotspot uint32
	hbmMask  uintptr
	hbmColor uintptr
}

type bitmapObj struct {
	bmType       int32
	bmWidth      int32
	bmHeight     int32
	bmWidthBytes int32
	bmPlanes     uint16
	bmBitsPixel  uint16
	bmBits       uintptr
}

type bitmapInfoHeader struct {
	biSize          uint32
	biWidth         int32
	biHeight        int32
	biPlanes        uint16
	biBitCount      uint16
	biCompression   uint32
	biSizeImage     uint32
	biXPelsPerMeter int32
	biYPelsPerMeter int32
	biClrUsed       uint32
	biClrImportant  uint32
}

func (w *winPlatform) Icon(path string, index, size int) ([]byte, error) {
	if size <= 0 || size > 256 {
		size = 64
	}
	key := fmt.Sprintf("%s|%d|%d", strings.ToLower(path), index, size)
	w.iconMu.Lock()
	if b, ok := w.iconCache[key]; ok {
		w.iconMu.Unlock()
		if b == nil {
			return nil, errors.New("no icon")
		}
		return b, nil
	}
	w.iconMu.Unlock()

	var data []byte
	var err error
	ext := strings.ToLower(filepath.Ext(path))
	if ext == ".png" || ext == ".jpg" || ext == ".jpeg" {
		var fi os.FileInfo
		if fi, err = os.Stat(path); err == nil && fi.Size() < 2<<20 {
			data, err = os.ReadFile(path)
		} else if err == nil {
			err = errors.New("image too large")
		}
	} else {
		data, err = extractIconPNG(path, index, size)
	}
	w.iconMu.Lock()
	if len(w.iconCache) > 2000 {
		w.iconCache = map[string][]byte{}
	}
	w.iconCache[key] = data
	w.iconMu.Unlock()
	return data, err
}

func extractIconPNG(path string, index, size int) ([]byte, error) {
	if !Exists(path) {
		return nil, errors.New("file not found")
	}
	var large, small uintptr
	var hr uintptr
	withCOM(func() {
		hr, _, _ = procSHDefExtractIconW.Call(uintptr(unsafe.Pointer(u16(path))), uintptr(index), 0,
			uintptr(unsafe.Pointer(&large)), uintptr(unsafe.Pointer(&small)), uintptr(size|(16<<16)))
	})
	if small != 0 {
		procDestroyIcon.Call(small)
	}
	if hr != 0 || large == 0 {
		if large != 0 {
			procDestroyIcon.Call(large)
		}
		return nil, errors.New("no icon")
	}
	defer procDestroyIcon.Call(large)
	return hiconToPNG(large)
}

func hiconToPNG(hicon uintptr) ([]byte, error) {
	var ii iconInfo
	if r, _, _ := procGetIconInfo.Call(hicon, uintptr(unsafe.Pointer(&ii))); r == 0 {
		return nil, errors.New("GetIconInfo failed")
	}
	defer func() {
		if ii.hbmColor != 0 {
			procDeleteObject.Call(ii.hbmColor)
		}
		if ii.hbmMask != 0 {
			procDeleteObject.Call(ii.hbmMask)
		}
	}()
	if ii.hbmColor == 0 {
		return nil, errors.New("monochrome icon")
	}
	var bm bitmapObj
	if r, _, _ := procGetObjectW.Call(ii.hbmColor, unsafe.Sizeof(bm), uintptr(unsafe.Pointer(&bm))); r == 0 {
		return nil, errors.New("GetObject failed")
	}
	wd, ht := int(bm.bmWidth), int(bm.bmHeight)
	if wd <= 0 || ht <= 0 || wd > 512 || ht > 512 {
		return nil, errors.New("unexpected icon size")
	}
	hdc, _, _ := procCreateCompatibleDC.Call(0)
	if hdc == 0 {
		return nil, errors.New("CreateCompatibleDC failed")
	}
	defer procDeleteDC.Call(hdc)
	bi := make([]byte, 40+256*4)
	setHdr := func() {
		h := (*bitmapInfoHeader)(unsafe.Pointer(&bi[0]))
		*h = bitmapInfoHeader{biSize: 40, biWidth: int32(wd), biHeight: -int32(ht), biPlanes: 1, biBitCount: 32}
	}
	setHdr()
	px := make([]byte, wd*ht*4)
	if r, _, _ := procGetDIBits.Call(hdc, ii.hbmColor, 0, uintptr(ht), uintptr(unsafe.Pointer(&px[0])), uintptr(unsafe.Pointer(&bi[0])), 0); r == 0 {
		return nil, errors.New("GetDIBits failed")
	}
	hasAlpha := false
	for i := 3; i < len(px); i += 4 {
		if px[i] != 0 {
			hasAlpha = true
			break
		}
	}
	if !hasAlpha {
		mask := make([]byte, wd*ht*4)
		setHdr()
		r := uintptr(0)
		if ii.hbmMask != 0 {
			r, _, _ = procGetDIBits.Call(hdc, ii.hbmMask, 0, uintptr(ht), uintptr(unsafe.Pointer(&mask[0])), uintptr(unsafe.Pointer(&bi[0])), 0)
		}
		for i := 0; i < wd*ht; i++ {
			if r == 0 || mask[i*4] == 0 {
				px[i*4+3] = 255
			}
		}
	}
	img := image.NewNRGBA(image.Rect(0, 0, wd, ht))
	for i := 0; i < wd*ht; i++ {
		img.Pix[i*4+0] = px[i*4+2]
		img.Pix[i*4+1] = px[i*4+1]
		img.Pix[i*4+2] = px[i*4+0]
		img.Pix[i*4+3] = px[i*4+3]
	}
	var out bytes.Buffer
	if err := png.Encode(&out, img); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

// FileInfo reads CompanyName and FileDescription from an executable's
// version resource.
func (w *winPlatform) FileInfo(path string) (string, string) {
	if !Exists(path) {
		return "", ""
	}
	p := u16(path)
	size, _, _ := procGetFileVersionInfoSizeW.Call(uintptr(unsafe.Pointer(p)), 0)
	if size == 0 || size > 4<<20 {
		return "", ""
	}
	buf := make([]byte, size)
	base := uintptr(unsafe.Pointer(&buf[0]))
	if r, _, _ := procGetFileVersionInfoW.Call(uintptr(unsafe.Pointer(p)), 0, size, base); r == 0 {
		return "", ""
	}
	query := func(sub string) (int, int, bool) {
		var ptr uintptr
		var l uint32
		r, _, _ := procVerQueryValueW.Call(base, uintptr(unsafe.Pointer(u16(sub))), uintptr(unsafe.Pointer(&ptr)), uintptr(unsafe.Pointer(&l)))
		if r == 0 || ptr < base || ptr >= base+uintptr(len(buf)) {
			return 0, 0, false
		}
		return int(ptr - base), int(l), true
	}
	var langs []string
	if off, l, ok := query(`\VarFileInfo\Translation`); ok {
		for i := 0; i+4 <= l && i < 32 && off+i+4 <= len(buf); i += 4 {
			lang := binary.LittleEndian.Uint16(buf[off+i:])
			cp := binary.LittleEndian.Uint16(buf[off+i+2:])
			langs = append(langs, fmt.Sprintf("%04x%04x", lang, cp))
		}
	}
	langs = append(langs, "040904b0", "040904e4", "04090000", "000004b0")
	read := func(name string) string {
		for _, lg := range langs {
			off, l, ok := query(`\StringFileInfo\` + lg + `\` + name)
			if !ok || l <= 0 {
				continue
			}
			end := off + l*2
			if end > len(buf) {
				end = len(buf)
			}
			if s := strings.TrimSpace(utf16BytesToString(buf[off:end])); s != "" {
				return s
			}
		}
		return ""
	}
	return read("CompanyName"), read("FileDescription")
}

// Uninstallers -------------------------------------------------------------------------

type jobAccounting struct {
	totalUserTime, totalKernelTime, periodUserTime, periodKernelTime int64
	totalPageFaults, totalProcesses, activeProcesses, terminated     uint32
}

// RunUninstaller starts cmdline inside a Windows job object so Jigglypuff can wait
// for the uninstaller *and* every helper process it spawns (NSIS and Inno
// Setup uninstallers copy themselves to %TEMP% and restart).
func (w *winPlatform) RunUninstaller(ctx context.Context, cmdline string) (int, error) {
	exe, args := ParseCommandLine(cmdline)
	if exe == "" {
		return -1, errors.New("the uninstall command is empty")
	}
	job, _, _ := procCreateJobObjectW.Call(0, 0)
	if job != 0 {
		defer syscall.CloseHandle(syscall.Handle(job))
	}
	cl, err := syscall.UTF16FromString(cmdline)
	if err != nil {
		return -1, err
	}
	var dir *uint16
	if d := filepath.Dir(exe); filepath.IsAbs(exe) && IsDir(d) {
		dir = u16(d)
	}
	var si syscall.StartupInfo
	si.Cb = uint32(unsafe.Sizeof(si))
	var pi syscall.ProcessInformation
	err = syscall.CreateProcess(nil, &cl[0], nil, nil, false, createSuspended, nil, dir, &si, &pi)
	if err != nil {
		if errno, ok := err.(syscall.Errno); ok && errno == errorElevationRequired {
			return shellExecute(ctx, "runas", exe, args, true)
		}
		return -1, fmt.Errorf("could not start the uninstaller: %v", err)
	}
	if job != 0 {
		procAssignProcessToJobObject.Call(job, uintptr(pi.Process))
	}
	procResumeThread.Call(uintptr(pi.Thread))
	syscall.CloseHandle(pi.Thread)
	defer syscall.CloseHandle(pi.Process)

	for {
		ev, _ := syscall.WaitForSingleObject(pi.Process, 300)
		if ev == syscall.WAIT_OBJECT_0 {
			break
		}
		if ctx.Err() != nil {
			return -1, ctx.Err()
		}
	}
	var code uint32
	syscall.GetExitCodeProcess(pi.Process, &code)
	if job == 0 {
		return int(code), nil
	}
	for {
		var info jobAccounting
		r, _, _ := procQueryInformationJobObject.Call(job, 1, uintptr(unsafe.Pointer(&info)), unsafe.Sizeof(info), 0)
		if r == 0 || info.activeProcesses == 0 {
			break
		}
		select {
		case <-ctx.Done():
			return int(code), ctx.Err()
		case <-time.After(400 * time.Millisecond):
		}
	}
	return int(code), nil
}

func (w *winPlatform) CreateRestorePoint(description string) (string, error) {
	script := "try { Checkpoint-Computer -Description " + psQuote(description) +
		" -RestorePointType 'APPLICATION_UNINSTALL' -ErrorAction Stop -WarningVariable jigglypuffWarn -WarningAction SilentlyContinue;" +
		" if ($jigglypuffWarn) { 'WARN:' + ($jigglypuffWarn -join ' ') } else { 'OK' } } catch { 'ERR:' + $_.Exception.Message }"
	out, err := powershell(nil, 4*time.Minute, script)
	out = strings.TrimSpace(out)
	switch {
	case err != nil:
		return "", err
	case strings.HasPrefix(out, "OK"):
		return "created", nil
	case strings.HasPrefix(out, "WARN:"):
		if strings.Contains(out, "1440") || strings.Contains(strings.ToLower(out), "past") {
			return "skipped — Windows already made a restore point in the last 24 hours", nil
		}
		return "skipped — " + strings.TrimPrefix(out, "WARN:"), nil
	case strings.HasPrefix(out, "ERR:"):
		msg := strings.TrimSpace(strings.TrimPrefix(out, "ERR:"))
		if strings.Contains(strings.ToLower(msg), "disabled") || strings.Contains(strings.ToLower(msg), "service cannot be started") {
			msg = "System Protection is turned off for this PC"
		}
		return "", errors.New(msg)
	}
	return "", errors.New("unexpected answer from Windows: " + firstLine(out))
}

// Store apps --------------------------------------------------------------------------------

func (w *winPlatform) StoreApps() ([]StoreApp, error) {
	script := `$names=@{}
try { foreach($a in Get-StartApps){ $id=[string]$a.AppID; $i=$id.IndexOf('!'); if($i -gt 0){ $f=$id.Substring(0,$i); if(-not $names.ContainsKey($f)){ $names[$f]=[string]$a.Name } } } } catch {}
$list = foreach($p in (Get-AppxPackage -PackageTypeFilter Main)){
  if($p.IsFramework -or $p.NonRemovable -or ([string]$p.SignatureKind -eq 'System')){ continue }
  $d=$names[$p.PackageFamilyName]
  if(-not $d){ continue }
  [pscustomobject]@{name=[string]$p.Name;fullName=[string]$p.PackageFullName;family=[string]$p.PackageFamilyName;publisher=[string]$p.Publisher;version=[string]$p.Version;location=[string]$p.InstallLocation;displayName=$d}
}
ConvertTo-Json -InputObject @($list) -Compress -Depth 3`
	out, err := powershell(nil, 90*time.Second, script)
	if err != nil {
		return nil, err
	}
	out = strings.TrimSpace(out)
	if i := strings.IndexAny(out, "[{"); i > 0 {
		out = out[i:]
	}
	var apps []StoreApp
	if out == "" || out == "null" {
		return nil, nil
	}
	if err := json.Unmarshal([]byte(out), &apps); err != nil {
		return nil, fmt.Errorf("could not read Store apps: %v", err)
	}
	return apps, nil
}

func (w *winPlatform) RemoveStoreApp(fullName string) error {
	_, err := powershell(nil, 5*time.Minute, "Remove-AppxPackage -Package "+psQuote(fullName)+" -ErrorAction Stop")
	return err
}

// Services, tasks, shortcuts -----------------------------------------------------------------

var (
	procOpenSCManagerW     = modadvapi32.NewProc("OpenSCManagerW")
	procOpenServiceW       = modadvapi32.NewProc("OpenServiceW")
	procControlService     = modadvapi32.NewProc("ControlService")
	procQueryServiceStatus = modadvapi32.NewProc("QueryServiceStatus")
	procDeleteService      = modadvapi32.NewProc("DeleteService")
	procCloseServiceHandle = modadvapi32.NewProc("CloseServiceHandle")
)

type serviceStatus struct {
	serviceType, currentState, controlsAccepted, win32ExitCode, specificExitCode, checkPoint, waitHint uint32
}

func (w *winPlatform) StopAndDeleteService(name string) error {
	scm, _, e := procOpenSCManagerW.Call(0, 0, 0x0001) // SC_MANAGER_CONNECT
	if scm == 0 {
		return fmt.Errorf("could not open the service manager: %v", e)
	}
	defer procCloseServiceHandle.Call(scm)
	svc, _, e := procOpenServiceW.Call(scm, uintptr(unsafe.Pointer(u16(name))), 0x0004|0x0020|0x10000) // QUERY_STATUS | STOP | DELETE
	if svc == 0 {
		return fmt.Errorf("could not open service %s: %v", name, e)
	}
	defer procCloseServiceHandle.Call(svc)
	var st serviceStatus
	if r, _, _ := procControlService.Call(svc, 1 /* SERVICE_CONTROL_STOP */, uintptr(unsafe.Pointer(&st))); r != 0 {
		for i := 0; i < 40; i++ {
			if r, _, _ := procQueryServiceStatus.Call(svc, uintptr(unsafe.Pointer(&st))); r == 0 || st.currentState == 1 /* STOPPED */ {
				break
			}
			time.Sleep(250 * time.Millisecond)
		}
	}
	if r, _, e := procDeleteService.Call(svc); r == 0 {
		return fmt.Errorf("could not delete service %s: %v", name, e)
	}
	return nil
}

func (w *winPlatform) DeleteScheduledTask(taskPath string) error {
	out, err := hiddenCmd(nil, system32("schtasks.exe"), "/Delete", "/TN", taskPath, "/F").CombinedOutput()
	if err != nil {
		return fmt.Errorf("could not delete scheduled task: %s", strings.TrimSpace(string(out)))
	}
	return nil
}

func (w *winPlatform) CreateShortcut(lnkPath, target, args, workDir, description string) error {
	script := "$s=(New-Object -ComObject WScript.Shell).CreateShortcut(" + psQuote(lnkPath) + ");" +
		"$s.TargetPath=" + psQuote(target) + ";$s.Arguments=" + psQuote(args) + ";" +
		"$s.WorkingDirectory=" + psQuote(workDir) + ";$s.Description=" + psQuote(description) + ";" +
		"$s.IconLocation=" + psQuote(target+",0") + ";$s.Save()"
	_, err := powershell(nil, 30*time.Second, script)
	return err
}
