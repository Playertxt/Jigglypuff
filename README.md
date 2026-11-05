# Jigglypuff

A careful, minimal cleaner for Windows 10 and 11, and now for Android phones. On Windows, Jigglypuff is a single portable file (`Jigglypuff.exe`) and installs nothing. On Android it's `Jigglypuff.apk` — see [Android app](#android-app).

## Features

- **Pick a drive to scan**: the *Drive to scan* menu in the sidebar (or a click on a drive card on the Overview) limits every scan to one drive: junk, leftovers, old downloads, large files, duplicates, the Recycle Bin and the software list. *All drives* scans everything, as before.
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
  - **Every program on a drive**: each app shows which drive it's installed on. With a drive picked, the page lists everything installed there. The *Games & portable programs* tab adds software that Windows' installed-apps list doesn't know about: Steam and Xbox games, portable apps and tools unzipped into a folder, each with its size and a button to open its folder.
- **Leftovers**: finds folders and settings of apps you uninstalled long ago. It ignores anything an installed app, service, shortcut, scheduled task or running program still uses, and anything changed in the last 30 days.
- **Large & old files**: finds the biggest files in any folder, plus old installers, archives and disk images sitting in Downloads.
- **Duplicates**: compares file contents (SHA-256) and keeps at least one copy of every file.
- **Startup apps**: turns apps off or on the same reversible way Task Manager does, and removes entries whose program is gone.
- **History**: lists every clean-up and links to its backups.
- **Appearance** (PC and Android): make Jigglypuff look the way you like. Changes show up right away and are saved.
  - **10 themes** to start from, each with its own colors, shapes and button effects: Classic, Jigglypuff, Bubblegum, Midnight Neon, Paper, Forest, Sunset, Ocean Glass, Terminal and Bold Pop. Every theme has a light and a dark version.
  - **Shapes**: give buttons, cards, text boxes, chips, the sidebar or bottom bar, dialogs and icon tiles their own shape (square, rounded, pill, cut corners, squircle, scooped, notched or leaf) and corner size.
  - **Button effects**: 9 styles (filled, soft, outline, gradient, glass, neon, 3D, pop, text only), 7 press effects (none, shrink, push down, bounce, jelly, glow, ripple) and, on the PC, 6 hover effects (lift, glow, brighten, grow, shine). Each one is a live sample you can click.
  - **Every color**: background, cards, lines, three text shades, accent, second accent, danger, safe, warning, sidebar and more, separately for light and dark mode.
  - **Text and layout**: font, text size, spacing, animation speed and a background pattern.
  - **Theme codes**: *Copy code* turns your whole theme into a code; *Paste code* in Jigglypuff on another PC or phone gives it the same look. Codes work between the Windows and Android apps. The 10 themes live in [`shared/looks/presets.json`](shared/looks/presets.json), which both apps build in.
  - **Undo** takes back the last 30 changes.

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

## Android app

`Jigglypuff.apk` runs on Android 11 and newer. Download it on your phone and open it; Android asks you to allow installing apps from your browser or file manager the first time.

The first time you scan, Jigglypuff asks for two permissions on Android's own screens:
- **All files access**: to look through your storage.
- **Usage access** (optional): for app sizes, caches and when you last opened each app.

What it does:
- **Storage to scan**: the menu at the top picks internal storage, an SD card or a USB drive (or all of them). Tap a storage card on the Overview to do the same.
- **Overview**: one scan, then what's safe to clean (pre-ticked) and what's worth a look.
- **Clean**: thumbnail caches, temporary and log files, empty folders, installers of apps you already have, and **leftovers** of removed apps (`Android/media/<package>` and folders no installed app uses).
- **Apps**: every app on the chosen storage with its size and last use, plus **Unused** apps, the **Caches** that are biggest, and **APK files** saved on the storage. Uninstalling goes through Android's own prompt.
- **Files**: large files, old downloads, and duplicates (compared by SHA-256; one copy is always kept).
- **More**: Jigglypuff's **trash** (removed files can be put back for 7 days), history, permissions and settings.
- **Appearance** (in More): the same 10 themes, shapes, button styles and press effects, colors and theme codes as the PC app. See *Appearance* under [Features](#features).

What Android doesn't allow, so it isn't there: clearing other apps' caches directly (Jigglypuff opens each app's storage page instead), silent uninstalls, and anything inside other apps' private folders.

Safety: everything removed is listed in a confirmation first. A guard refuses Android's own folders, the storage roots, other apps' private folders and your protected folders, and every item is checked again just before it's removed. Photos, documents and other personal files go to a hidden trash folder (with `.nomedia`, so they don't show up in your gallery) instead of being deleted.

Building it needs the Android SDK (API 37) and JDK 17 or newer:

```
cd android
gradlew assembleRelease
```

Release builds are signed with the key in `~/.jigglypuff/keystore.properties` when it exists.

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
| `android/` | The Android app (Kotlin + Jetpack Compose): `engine/` scans and cleans, `look/` is the theme format, `ui/` is the interface |
| `shared/looks` | The 10 themes, built into both apps |
