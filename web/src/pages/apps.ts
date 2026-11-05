import { App, cancelJob, get, iconURL, Leftover, post, runJob, UninstallResult } from "../api.js";
import { avatarColor, debounce, fmtBytes, fmtDate, h, icon, plural, shortPath } from "../dom.js";
import { currentPage, Page, rerender, store } from "../store.js";
import { badge, btn, checkbox, confidenceBadge, empty, openDialog, openMenu, reviewAndClean, showError, toast } from "../ui.js";

let tab: "desktop" | "store" = "desktop";
let query = "";
let sort: "name" | "size" | "date" = "name";
let loading = false;
let loadingStore = false;

export function onAppSizes(sizes: Record<string, number>): void {
  if (!store.apps) return;
  for (const a of store.apps) {
    if (sizes[a.id] !== undefined) {
      a.size = sizes[a.id];
      a.sizeKnown = true;
    }
  }
  if (currentPage() === "apps" && tab === "desktop") rerender("apps");
}

async function loadApps(refresh = false): Promise<void> {
  loading = true;
  rerender("apps");
  try {
    store.apps = await get<App[]>("/api/apps" + (refresh ? "?refresh=1" : ""));
  } catch (e) {
    showError(e);
  } finally {
    loading = false;
    rerender("apps");
  }
}

async function loadStore(refresh = false): Promise<void> {
  loadingStore = true;
  rerender("apps");
  try {
    store.storeApps = await get<App[]>("/api/apps/store" + (refresh ? "?refresh=1" : ""));
  } catch (e) {
    showError(e);
    store.storeApps = [];
  } finally {
    loadingStore = false;
    rerender("apps");
  }
}

function appIcon(a: App): HTMLElement {
  const letter = h("div", { class: "app-icon", style: `background:${avatarColor(a.name)}` }, (a.name.trim()[0] ?? "?").toUpperCase());
  if (!a.hasIcon) return letter;
  const img = h("img", { src: iconURL({ app: a.id }), alt: "", loading: "lazy" });
  const box = h("div", { class: "app-icon" }, img);
  img.addEventListener("error", () => box.replaceWith(letter));
  return box;
}

// Uninstall wizard --------------------------------------------------------------------

function describe(l: Leftover): { main: string; icon: string } {
  switch (l.kind) {
    case "regkey": return { main: l.path, icon: "key" };
    case "regvalue": return { main: /\\Run$/i.test(l.path) ? `Startup entry “${l.name}”` : `${l.path}\\${l.name}`, icon: "key" };
    case "service": return { main: `Service “${l.name || l.path}”`, icon: "cog" };
    case "task": return { main: `Scheduled task ${l.path}`, icon: "calendar" };
    case "shortcut": return { main: l.path, icon: "shortcut" };
    case "file": return { main: l.path, icon: "file" };
    default: return { main: l.path, icon: "folder" };
  }
}

export function leftoverRow(l: Leftover, on: boolean, set: (v: boolean) => void): HTMLElement {
  const d = describe(l);
  return h("div", { class: "row click", onclick: () => set(!on) },
    checkbox(on, set, { label: d.main }),
    h("div", { class: "ico sm" }, icon(d.icon)),
    h("div", { class: "grow" }, h("div", { class: "title" }, h("span", { class: "t path selectable", title: d.main, style: "color:var(--text);font-size:13.5px" }, shortPath(d.main, 64))),
      h("div", { class: "desc" }, l.reason + (l.files > 0 ? ` · ${plural(l.files, "file")}` : ""))),
    confidenceBadge(l.confidence),
    h("div", { class: "size" + (l.size ? "" : " muted") }, l.size ? fmtBytes(l.size) : "—"));
}

async function uninstallFlow(app: App, force: boolean): Promise<void> {
  const d = openDialog({ label: "Uninstall " + app.name, wide: true });
  const admin = !!store.info?.admin;
  let restore = (store.settings?.restorePoint ?? true) && admin && app.kind === "desktop";
  let jobId = "";
  let result: UninstallResult | null = null;
  const removedAll = async () => {
    store.apps = null;
    store.storeApps = null;
    store.history = null;
    if (currentPage() === "apps") {
      if (tab === "store") loadStore(true);
      else loadApps(true);
    }
  };

  const spinnerState = (title: string, text: string, foot?: HTMLElement[]) => {
    const p = h("p", null, text);
    d.set(h("div", { class: "wizard-center" }, h("div", { class: "spinner lg" }), h("h3", null, title), p), foot ?? null);
    return p;
  };

  const confirm = () => {
    const what = force
      ? `Use this when the normal uninstall doesn't work. Jigglypuff removes ${app.name}'s program folder, its settings and its entry in Installed apps. You'll see the full list before anything is deleted.`
      : app.kind === "store"
        ? `Windows removes this Microsoft Store app for your account. Jigglypuff then checks for anything left behind.`
        : `Jigglypuff runs ${app.name}'s own uninstaller. When it finishes, Jigglypuff looks for leftover files and settings and shows them to you before anything is deleted.`;
    const rp = app.kind === "desktop" && !force
      ? h("label", { class: "row", style: "padding:12px 0 4px;border:0;min-height:0;gap:12px" },
        checkbox(restore, (v) => (restore = v), { disabled: !admin, label: "Create a restore point first" }),
        h("div", { class: "grow" }, h("div", { class: "title" }, "Create a restore point first"),
          h("div", { class: "desc wrap" }, admin ? "Lets you roll Windows back if something goes wrong. Takes a few seconds." : "Needs administrator rights.")))
      : null;
    d.set([h("h3", null, force ? `Force remove ${app.name}?` : `Uninstall ${app.name}?`), h("p", null, what), rp,
      force ? h("div", { class: "note warn" }, icon("alert"), h("div", null, "Force remove can't run the app's own clean-up steps, such as removing drivers it installed.")) : null],
    [btn("Cancel", { onclick: () => d.close() }), btn(force ? "Find everything" : "Uninstall", { kind: force ? "danger" : "primary", onclick: () => (force ? findLeftovers(true) : run()) })]);
  };

  const run = async () => {
    d.dismissable = false;
    const skip = btn("Skip waiting", { kind: "ghost", onclick: () => jobId && cancelJob(jobId) });
    skip.style.visibility = "hidden";
    const text = spinnerState("Starting…", "", [skip]);
    const title = d.body.querySelector("h3")!;
    const timer = setTimeout(() => (skip.style.visibility = "visible"), 8000);
    try {
      const job = await runJob<UninstallResult>(`/api/apps/${app.id}/uninstall`, { restorePoint: restore }, (p) => {
        if (p.phase === "restore-point") {
          title.textContent = "Creating a restore point…";
          text.textContent = "Windows is saving a snapshot of your system. This can take a minute.";
        } else if (p.phase === "uninstalling") {
          title.textContent = app.kind === "store" ? `Removing ${app.name}…` : `Waiting for ${app.name}'s uninstaller…`;
          text.textContent = app.kind === "store" ? "Windows is removing the app." : "Follow the steps in the uninstaller's window. If you don't see it, it may be behind this one.";
        } else if (p.phase === "verifying") {
          title.textContent = "Checking…";
          text.textContent = `Making sure ${app.name} is gone.`;
        }
      }, (id) => (jobId = id));
      result = job.result ?? null;
    } catch (e) {
      result = { status: "failed", exitCode: -1, restorePoint: "", message: e instanceof Error ? e.message : String(e) };
    } finally {
      clearTimeout(timer);
      d.dismissable = true;
    }
    const r = result!;
    if (r.status === "failed") {
      d.set([h("div", { class: "result-icon warn" }, icon("alert")), h("h3", null, "The uninstall didn't work"), h("p", null, r.message || "The uninstaller reported a problem."),
        restoreLine(r)], [btn("Close", { onclick: () => d.close() }), app.kind === "desktop" ? btn("Force remove…", { kind: "danger", onclick: () => { force = true; confirm(); } }) : null]);
      return;
    }
    if (r.status === "still-installed") {
      d.set([h("div", { class: "result-icon warn" }, icon("alert")), h("h3", null, `${app.name} is still installed`), h("p", null, r.message || "The uninstaller may have been cancelled."), restoreLine(r)],
        [btn("Close", { onclick: () => d.close() }), btn("Force remove…", { kind: "danger", onclick: () => { force = true; confirm(); } }), btn("Look for leftovers", { kind: "primary", onclick: () => findLeftovers(false) })]);
      removedAll();
      return;
    }
    findLeftovers(false);
  };

  const restoreLine = (r: UninstallResult | null) => {
    if (!r || !r.restorePoint || r.restorePoint === "off") return null;
    const ok = r.restorePoint === "created";
    return h("div", { class: ok ? "note info" : "note warn", style: "margin-bottom:10px" }, icon(ok ? "check" : "info"),
      h("div", null, ok ? "A restore point was created before uninstalling." : `Restore point ${r.restorePoint}.`));
  };

  const findLeftovers = async (forceMode: boolean) => {
    d.dismissable = false;
    spinnerState(forceMode ? `Finding everything ${app.name} installed…` : "Looking for leftovers…", "Checking program folders, app data, shortcuts, the registry, services and scheduled tasks.");
    try {
      const job = await runJob<{ scanId: string; items: Leftover[] }>(`/api/apps/${app.id}/leftovers`, { force: forceMode });
      d.dismissable = true;
      const res = job.result;
      if (!res) {
        d.close();
        return;
      }
      if (!res.items.length) {
        d.set([h("div", { class: "result-icon" }, icon("check")), h("h3", null, forceMode ? "Nothing found" : `${app.name} is completely gone`),
          h("p", null, forceMode ? `Jigglypuff didn't find any files or settings that belong to ${app.name}.` : "Jigglypuff didn't find anything left behind."), restoreLine(result)],
        [btn("Done", { kind: "primary", onclick: () => d.close() })]);
        if (!forceMode) toast(`${app.name} was uninstalled`);
        removedAll();
        return;
      }
      showList(res.scanId, res.items, forceMode);
    } catch (e) {
      d.dismissable = true;
      d.set([h("div", { class: "result-icon warn" }, icon("alert")), h("h3", null, "Couldn't check for leftovers"), h("p", null, e instanceof Error ? e.message : String(e))],
        [btn("Close", { kind: "primary", onclick: () => d.close() })]);
    }
  };

  const showList = (scanId: string, items: Leftover[], forceMode: boolean) => {
    const sel = new Set(items.filter((l) => l.confidence === "high").map((l) => l.id));
    const total = items.reduce((a, l) => a + l.size, 0);
    const list = h("div", { class: "card", style: "max-height:330px;overflow-y:auto;box-shadow:none;margin:12px 0" });
    const go = btn("", { kind: "danger", icon: "trash" });
    const paint = () => {
      list.replaceChildren(...items.map((l) => leftoverRow(l, sel.has(l.id), (v) => { if (v) sel.add(l.id); else sel.delete(l.id); paint(); })));
      const size = items.filter((l) => sel.has(l.id)).reduce((a, l) => a + l.size, 0);
      go.lastChild!.textContent = sel.size ? `Remove ${sel.size} selected${size ? ` · ${fmtBytes(size)}` : ""}` : "Remove selected";
      go.disabled = sel.size === 0;
    };
    go.append("");
    go.onclick = async () => {
      d.close();
      const out = await reviewAndClean({ source: "app-leftovers", scanId, ids: [...sel], title: `Removed ${app.name}${forceMode ? "" : " leftovers"}` },
        { title: forceMode ? `Remove ${app.name} completely?` : `Remove ${app.name}'s leftovers?` });
      if (out) {
        toast(`${app.name} is gone`);
        removedAll();
      }
    };
    d.set([
      h("h3", null, forceMode ? `Everything ${app.name} left on your PC` : `${app.name} left ${plural(items.length, "thing")} behind`),
      h("p", null, `${fmtBytes(total)} in total. Likely leftovers are already ticked — untick anything you want to keep.`),
      restoreLine(result),
      list,
    ], [btn("Keep everything", { onclick: () => { d.close(); removedAll(); } }), go]);
    paint();
  };

  confirm();
}

function moreMenu(e: MouseEvent, a: App): void {
  const items = [] as Array<{ label: string; icon?: string; danger?: boolean; onclick: () => void }>;
  if (a.installLocation) items.push({ label: "Open install folder", icon: "folder", onclick: () => post("/api/open", { path: a.installLocation }).catch(showError) });
  if (a.kind === "desktop" && !a.broken) items.push({ label: "Force remove…", icon: "trash", danger: true, onclick: () => uninstallFlow(a, true) });
  if (a.kind === "desktop" && a.broken) items.push({ label: "Remove from the list only", icon: "x", onclick: () => removeEntry(a) });
  if (!items.length) return;
  openMenu(e.currentTarget as HTMLElement, items);
}

async function removeEntry(a: App): Promise<void> {
  const out = await reviewAndClean({ source: "broken-entry", ids: [a.id], appId: a.id, title: `Removed ${a.name} from Installed apps` },
    { title: `Remove ${a.name} from the list?`, intro: "This only removes the broken entry from Installed apps. Use Force remove to also delete its files." });
  if (out) loadApps(true);
}

function appRow(a: App): HTMLElement {
  const meta = [a.publisher, a.version ? `v${a.version}` : "", a.installDate ? `installed ${fmtDate(a.installDate)}` : ""].filter(Boolean).join(" · ");
  const action = a.broken
    ? btn("Force remove", { size: "sm", onclick: () => uninstallFlow(a, true) })
    : btn("Uninstall", { size: "sm", disabled: !a.canUninstall, title: a.canUninstall ? undefined : "This app can't be uninstalled", onclick: () => uninstallFlow(a, false) });
  return h("div", { class: "row" },
    appIcon(a),
    h("div", { class: "grow" },
      h("div", { class: "title" }, h("span", { class: "t" }, a.name), a.broken ? badge("Uninstaller missing", "warn") : null),
      h("div", { class: "desc", title: meta }, meta || " ")),
    h("div", { class: "size" + (a.sizeKnown ? "" : " muted") }, a.sizeKnown && a.size > 0 ? fmtBytes(a.size) : "—"),
    action,
    btn("", { kind: "ghost", size: "sm", iconOnly: true, icon: "more", title: "More", onclick: (e) => moreMenu(e, a) }));
}

const onSearch = debounce((v: string) => {
  query = v;
  rerender("apps");
  const el = document.getElementById("app-search") as HTMLInputElement | null;
  el?.focus();
  el?.setSelectionRange(v.length, v.length);
}, 150);

export const appsPage: Page = {
  id: "apps",
  enter(arg) {
    if (arg === "store") tab = "store";
    if (!store.apps && !loading) loadApps();
  },
  render(root) {
    const list = tab === "desktop" ? store.apps : store.storeApps;
    const search = h("div", { class: "search" }, icon("search"),
      h("input", { class: "input", id: "app-search", type: "search", placeholder: "Search apps", value: query, oninput: (e: Event) => onSearch((e.target as HTMLInputElement).value) }));
    const sorter = h("select", { class: "input", "aria-label": "Sort", onchange: (e: Event) => { sort = (e.target as HTMLSelectElement).value as typeof sort; rerender("apps"); } },
      h("option", { value: "name" }, "Sort by name"), h("option", { value: "size" }, "Sort by size"), h("option", { value: "date" }, "Sort by install date"));
    sorter.value = sort;
    root.append(h("div", { class: "page-head" },
      h("div", { class: "titles" }, h("h1", null, "Apps"),
        h("p", { class: "subtitle" }, "Uninstall apps completely. Jigglypuff runs each app's own uninstaller, then removes what it leaves behind.")),
      h("div", { class: "actions" }, search, sorter,
        btn("", { kind: "ghost", iconOnly: true, icon: "refresh", title: "Refresh", onclick: () => (tab === "desktop" ? loadApps(true) : loadStore(true)) }))));
    const seg = h("div", { class: "seg", role: "tablist", style: "margin-bottom:14px" },
      h("button", { type: "button", class: tab === "desktop" ? "on" : "", role: "tab", onclick: () => { tab = "desktop"; rerender("apps"); } },
        "Desktop apps", store.apps ? h("span", { class: "n" }, String(store.apps.length)) : null),
      h("button", { type: "button", class: tab === "store" ? "on" : "", role: "tab", onclick: () => { tab = "store"; if (!store.storeApps && !loadingStore) loadStore(); else rerender("apps"); } },
        "Microsoft Store apps", store.storeApps ? h("span", { class: "n" }, String(store.storeApps.length)) : null));
    root.append(seg);
    const busy = tab === "desktop" ? loading : loadingStore;
    if (!list) {
      root.append(h("div", { class: "card" }, Array.from({ length: 6 }, () => h("div", { class: "skeleton" }))));
      if (tab === "store" && busy) root.append(h("p", { style: "color:var(--muted);font-size:13px;margin-top:10px" }, "Asking Windows for your Store apps… this takes a few seconds."));
      return;
    }
    const q = query.trim().toLowerCase();
    let shown = list.filter((a) => !q || a.name.toLowerCase().includes(q) || a.publisher.toLowerCase().includes(q));
    shown = [...shown].sort((x, y) => sort === "size" ? y.size - x.size : sort === "date" ? (y.installDate ?? "").localeCompare(x.installDate ?? "") : x.name.localeCompare(y.name));
    const total = list.reduce((a, x) => a + (x.sizeKnown ? x.size : 0), 0);
    root.append(h("div", { class: "toolbar" }, h("span", null, `${plural(list.length, "app")}${total ? ` · ${fmtBytes(total)}` : ""}`),
      tab === "store" ? h("span", null, "· only apps you can remove are listed") : null));
    if (!shown.length) {
      root.append(h("div", { class: "card" }, empty("search", q ? "No matching apps" : "No apps here", q ? `Nothing matches “${query}”.` : "Jigglypuff didn't find any apps in this list.")));
      return;
    }
    root.append(h("div", { class: "card" }, shown.map(appRow)));
  },
};
