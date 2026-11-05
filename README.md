# Jigglypuff

A careful, minimal PC cleaner for Windows 10 and 11. Jigglypuff is a single portable file (`Jigglypuff.exe`) and installs nothing.

## Features

- **Overview**: one scan, then a short list of recommendations. Everything marked *Safe* is pre-ticked. Anything that needs your judgement is listed separately.
- **Junk cleaner**:
  - Windows: temp files, Windows temp, Windows Update cache, Delivery Optimization, error reports, crash dumps, old logs, thumbnail and shader caches, and broken shortcuts.
  - Browser caches: Chrome, Edge, Brave, Vivaldi, Opera and Firefox. History, passwords and logins are never touched.
  - App caches: Discord, Slack, VS Code, Steam, Store apps and others.
  - Developer caches: npm, Yarn, pnpm, Bun, pip, uv, Poetry, NuGet, Gradle, Maven, Go, Cargo, Electron and Playwright.
  - The Recycle Bin.
- **Apps**: uninstall desktop and Microsoft Store apps completely.
  1. Jigglypuff creates a restore point first (optional).
  2. It runs the app's own uninstaller and waits for every helper process the uninstaller starts.
  3. It hunts down what the app left behind: folders, shortcuts, registry keys, startup entries, services and scheduled tasks.
  - **Force remove** handles apps whose uninstaller is broken or missing.
- **Leftovers**: finds folders and settings of apps you uninstalled long ago. It ignores anything an installed app, service, shortcut, scheduled task or running program still uses, and anything changed in the last 30 days.
- **Large & old files**: finds the biggest files in any folder, plus old installers, archives and disk images sitting in Downloads.
- **Duplicates**: compares file contents (SHA-256) and keeps at least one copy of every file.
- **Startup apps**: turns apps off or on the same reversible way Task Manager does, and removes entries whose program is gone.
- **History**: lists every clean-up and links to its backups.

## Safety

- **Scanning never changes anything.** Deleting always starts with a confirmation dialog that lists exactly what will be removed and where it goes.
- **Your confirmation is final, and it is all Jigglypuff acts on.** The engine builds a deletion plan on its side, shows you that plan, and runs only that plan, once. It checks every item again just before deleting it.
- **A built-in guard refuses certain items outright**, whatever the interface asks for:
  - Windows and system folders, Program Files roots and your user folders.
  - Microsoft's registry keys and GPU driver keys (Khronos, NVIDIA, AMD, Intel).
  - Windows services and Windows' own scheduled tasks.
  - Anything inside a folder you protect in *Settings → Protected folders*.
- **Jigglypuff never follows links.** It skips symlinks, junctions and OneDrive placeholders, so it never deletes cloud files.
- **Your personal files can be restored.** Leftovers, large files and duplicates go to the **Recycle Bin**. You can switch this off in Settings. Items too big for the Recycle Bin are flagged in the dialog.
- **Registry keys, services and scheduled tasks are backed up before removal** to `%LOCALAPPDATA%\Jigglypuff\Backups`. To restore a registry key, double-click its `.reg` file. To restore a task, run `schtasks /Create /XML <file> /TN <name>`.
- **Files that are in use or were recently changed are skipped.** Temp files newer than 24 hours (configurable) are skipped too.

## Running

1. Double-click `Jigglypuff.exe`.
2. Windows asks for administrator rights. These are needed to clean system files and remove apps installed for all users.
   - If you say *No*, Jigglypuff runs in **limited mode**, and you can restart it as administrator later.
3. The interface opens in a chrome-less **Microsoft Edge app window**. If Edge is missing, Chrome, Brave or your default browser is used instead.
   - The window runs **without** admin rights; only Jigglypuff's engine is elevated.

Jigglypuff talks to its window over `127.0.0.1` with a random per-session key. Other programs and web pages can't control it.

If SmartScreen says "Windows protected your PC" (the file isn't code-signed), click **More info → Run anyway**.

Jigglypuff's own data (settings, history, backups and the window's browser profile) lives in `%LOCALAPPDATA%\Jigglypuff`. To reset Jigglypuff, delete that folder.

## Building from source

Requirements:

- Go 1.24 or newer.
- TypeScript (`npm i -g typescript`), only if you change the interface.

The compiled interface and the Windows resources (icon, manifest, version info) are already included, so this is enough:

```
go build -trimpath -ldflags "-s -w -H windowsgui" -o Jigglypuff.exe ./cmd/jigglypuff
```

To rebuild everything on Windows, run `tools\build.ps1`. On Linux or macOS, run `tools/build.sh`, which also needs `rsvg-convert` and `llvm-windres`.

To try the interface on any OS against a simulated PC (sparse fake files, in-memory registry):

```
go run ./cmd/jigglypuff --demo ./demo-pc --keep
```

To run the tests:

```
go test ./internal/...
```

## Layout

| Path | What |
|---|---|
| `cmd/jigglypuff` | Entry point: launcher, UAC elevation, single instance |
| `internal/platform` | Windows APIs (registry, Recycle Bin, icons, uninstallers, restore points, Store apps…) and the demo PC |
| `internal/engine` | Scanners, leftover matching, the safety guard and the deletion engine |
| `internal/server` | Local HTTP API, background jobs, deletion plans, history and settings |
| `web/` | The interface (TypeScript + CSS), compiled into `internal/web/dist` and embedded in the exe |
