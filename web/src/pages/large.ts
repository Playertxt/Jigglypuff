import { cancelJob, FileItem, post, Progress, runJob } from "../api.js";
import { baseName, fmtBytes, fmtDate, h, icon, plural, relTime, shortPath } from "../dom.js";
import { Page, rerender, store } from "../store.js";
import { btn, checkbox, empty, pickFolder, recycleLabel, reviewAndClean, scanningCard, showError } from "../ui.js";

let tab: "large" | "old" = "large";
let scanning = false;
let jobId = "";
let update: ((p: Progress | null) => void) | null = null;
let folder = "";
let minMB = 0;
let appData = false;
let days = 0;
const selLarge = new Set<string>();
const selOld = new Set<string>();

function settingsDefaults(): void {
  if (!minMB) minMB = store.settings?.largeMinMB ?? 200;
  if (!days) days = store.settings?.oldDownloadDays ?? 60;
  if (!folder) folder = store.settings?.largeFolder || store.info?.paths.userProfile || "";
}

async function scanLarge(): Promise<void> {
  if (scanning) return;
  scanning = true;
  rerender("large");
  try {
    const job = await runJob<{ scanId: string; files: FileItem[] | null; folder: string }>("/api/scan/large", { folder, minMB, appData }, (p) => update?.(p), (id) => (jobId = id));
    if (job.state === "done" && job.result) {
      store.large = { scanId: job.result.scanId, files: job.result.files ?? [], folder: job.result.folder };
      selLarge.clear();
    }
  } catch (e) {
    showError(e);
  } finally {
    scanning = false;
    update = null;
    rerender("large");
  }
}

async function scanOld(): Promise<void> {
  if (scanning) return;
  scanning = true;
  rerender("large");
  try {
    const job = await runJob<{ scanId: string; files: FileItem[] | null; days: number }>("/api/scan/old", { days }, (p) => update?.(p), (id) => (jobId = id));
    if (job.state === "done" && job.result) {
      store.old = { scanId: job.result.scanId, files: job.result.files ?? [], days: job.result.days };
      selOld.clear();
    }
  } catch (e) {
    showError(e);
  } finally {
    scanning = false;
    update = null;
    rerender("large");
  }
}

const typeLabel: Record<string, string> = { video: "Video", archive: "Archive", disk: "Disk image", installer: "Installer", image: "Image", audio: "Audio", document: "Document", other: "File" };

export function fileRow(f: FileItem, on: boolean, set: (v: boolean) => void, extra?: HTMLElement | null): HTMLElement {
  return h("div", { class: "row click", onclick: () => set(!on) },
    checkbox(on, set, { label: f.name }),
    h("div", { class: "ico sm" }, icon(f.type)),
    h("div", { class: "grow" },
      h("div", { class: "title" }, h("span", { class: "t selectable", title: f.path }, f.name), extra ?? null),
      h("div", { class: "desc path", title: f.dir }, `${typeLabel[f.type] ?? "File"} · ${shortPath(f.dir, 80)}`)),
    h("div", { class: "when", title: fmtDate(f.modified) }, relTime(f.modified)),
    h("div", { class: "size" }, fmtBytes(f.size)),
    btn("", { kind: "ghost", size: "sm", iconOnly: true, icon: "external", title: "Show in File Explorer",
      onclick: (e) => { e.stopPropagation(); post("/api/open", { path: f.path, reveal: true }).catch(showError); } }));
}

async function remove(kind: "large" | "old"): Promise<void> {
  const data = kind === "large" ? store.large : store.old;
  const sel = kind === "large" ? selLarge : selOld;
  if (!data || !sel.size) return;
  const out = await reviewAndClean({ source: kind === "large" ? "large" : "old-downloads", scanId: data.scanId, ids: [...sel],
    title: kind === "large" ? "Removed large files" : "Removed old downloads" });
  if (out) {
    const gone = new Set(out.done.map((p) => p.toLowerCase()));
    data.files = data.files.filter((f) => !gone.has(f.path.toLowerCase()));
    sel.clear();
    store.smart = null;
    store.history = null;
    rerender("large");
  }
}

function listView(kind: "large" | "old"): HTMLElement[] {
  const data = kind === "large" ? store.large : store.old;
  const sel = kind === "large" ? selLarge : selOld;
  if (!data) return [];
  const files = data.files;
  if (!files.length) {
    return [h("div", { class: "card" }, kind === "large"
      ? empty("check", "No large files found", `Nothing bigger than ${minMB >= 1024 ? minMB / 1024 + " GB" : minMB + " MB"} in this folder.`)
      : empty("check", "Downloads look tidy", `No installers or archives older than ${store.old?.days ?? days} days.`))];
  }
  const total = files.reduce((a, f) => a + f.size, 0);
  const selSize = files.filter((f) => sel.has(f.id)).reduce((a, f) => a + f.size, 0);
  const set = (id: string) => (v: boolean) => {
    if (v) sel.add(id);
    else sel.delete(id);
    rerender("large");
  };
  const out: HTMLElement[] = [
    h("div", { class: "toolbar" }, h("span", null, `${plural(files.length, "file")} · ${fmtBytes(total)}`), h("span", { class: "spacer" }),
      h("button", { class: "link", onclick: () => { files.forEach((f) => sel.add(f.id)); rerender("large"); } }, "Select all"),
      h("span", { class: "sep" }, "·"),
      h("button", { class: "link", onclick: () => { sel.clear(); rerender("large"); } }, "None")),
    h("div", { class: "card" }, files.slice(0, 500).map((f) => fileRow(f, sel.has(f.id), set(f.id)))),
  ];
  if (sel.size) {
    out.push(h("div", { class: "actionbar" },
      h("div", { class: "grow" }, `${plural(sel.size, "file")} selected `, h("span", null, `· ${fmtBytes(selSize)}`)),
      btn("Clear", { kind: "ghost", onclick: () => { sel.clear(); rerender("large"); } }),
      btn(recycleLabel(), { kind: "danger", icon: "trash", onclick: () => remove(kind) })));
  }
  return out;
}

export const largePage: Page = {
  id: "large",
  enter(arg) {
    settingsDefaults();
    if (arg === "old") tab = "old";
    else if (arg === "large") tab = "large";
    if (tab === "old" && !store.old && !scanning) scanOld();
  },
  render(root) {
    settingsDefaults();
    root.append(h("div", { class: "page-head" },
      h("div", { class: "titles" }, h("h1", null, "Large & old files"),
        h("p", { class: "subtitle" }, "Find big files and forgotten downloads. Nothing is removed unless you pick it."))));
    root.append(h("div", { class: "seg", role: "tablist", style: "margin-bottom:14px" },
      h("button", { type: "button", class: tab === "large" ? "on" : "", onclick: () => { tab = "large"; rerender("large"); } }, icon("fileBig"), "Large files"),
      h("button", { type: "button", class: tab === "old" ? "on" : "", onclick: () => { tab = "old"; if (!store.old && !scanning) scanOld(); else rerender("large"); } }, icon("download"), "Old downloads",
        store.old ? h("span", { class: "n" }, String(store.old.files.length)) : null)));
    if (tab === "large") {
      const sizeSel = h("select", { class: "input", "aria-label": "Minimum size", onchange: (e: Event) => { minMB = Number((e.target as HTMLSelectElement).value); } },
        [50, 100, 200, 500, 1024, 5120].map((v) => h("option", { value: String(v) }, `Bigger than ${v >= 1024 ? v / 1024 + " GB" : v + " MB"}`)));
      sizeSel.value = String([50, 100, 200, 500, 1024, 5120].includes(minMB) ? minMB : 200);
      root.append(h("div", { class: "card pad", style: "display:flex;gap:12px;align-items:center;flex-wrap:wrap;margin-bottom:14px" },
        h("div", { class: "field" }, "Look in"),
        h("span", { class: "chip", title: folder }, icon("folder"), h("span", null, baseName(folder) || folder)),
        btn("Change…", { size: "sm", onclick: async () => { const f = await pickFolder("Where should Jigglypuff look?", folder); if (f) { folder = f; rerender("large"); } } }),
        sizeSel,
        h("label", { class: "field" }, checkbox(appData, (v) => (appData = v), { label: "Include app data" }), "Include app data"),
        h("span", { style: "flex:1" }),
        btn(store.large ? "Scan again" : "Scan", { kind: "primary", icon: "search", disabled: scanning, onclick: scanLarge })));
    } else {
      const daySel = h("select", { class: "input", "aria-label": "Older than", onchange: (e: Event) => { days = Number((e.target as HTMLSelectElement).value); scanOld(); } },
        [30, 60, 90, 180, 365].map((v) => h("option", { value: String(v) }, `Not opened for ${v} days`)));
      daySel.value = String([30, 60, 90, 180, 365].includes(days) ? days : 60);
      root.append(h("div", { class: "card pad", style: "display:flex;gap:12px;align-items:center;flex-wrap:wrap;margin-bottom:14px" },
        h("div", { class: "field" }, icon("download"), "Installers, archives and disk images in Downloads"), h("span", { style: "flex:1" }), daySel,
        btn("", { kind: "ghost", iconOnly: true, icon: "refresh", title: "Scan again", disabled: scanning, onclick: scanOld })));
    }
    if (scanning) {
      const card = scanningCard(tab === "large" ? "Looking for large files…" : "Checking Downloads…", () => jobId && cancelJob(jobId));
      update = (p) => card.update(p && { ...p, message: p.message ? `${p.done.toLocaleString()} files checked · ${p.message}` : "" });
      root.append(card.el);
      return;
    }
    if (tab === "large" && !store.large) {
      root.append(h("div", { class: "card" }, empty("fileBig", "Find what's taking up space", "Pick a folder or a whole drive, then scan. Program and Windows folders are always skipped.",
        btn("Scan your user folder", { kind: "primary", icon: "search", onclick: scanLarge }))));
      return;
    }
    root.append(...listView(tab));
  },
};
