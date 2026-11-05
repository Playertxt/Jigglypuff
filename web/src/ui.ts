// Shared interface components.
import { CleanResult, get, Plan, PlanRequest, post, Progress, runJob } from "./api.js";
import { Child, clear, fmtBytes, fmtNum, h, icon, plural, shortPath } from "./dom.js";
import { currentDrive, driveName, setDrive, store } from "./store.js";
import { uiScale } from "./look.js";

type BtnKind = "primary" | "danger" | "ghost" | "default";

export function btn(label: Child, opts: { kind?: BtnKind; icon?: string; onclick?: (e: MouseEvent) => void; disabled?: boolean; size?: "sm" | "lg"; title?: string; iconOnly?: boolean } = {}): HTMLButtonElement {
  const cls = ["btn"];
  if (opts.kind && opts.kind !== "default") cls.push(opts.kind);
  if (opts.size) cls.push(opts.size);
  if (opts.iconOnly) cls.push("icon");
  return h("button", { class: cls.join(" "), type: "button", onclick: opts.onclick, disabled: opts.disabled, title: opts.title, "aria-label": opts.iconOnly ? opts.title : undefined },
    opts.icon ? icon(opts.icon) : null, opts.iconOnly ? null : label);
}

export function badge(text: string, tone: "good" | "warn" | "bad" | "neutral" | "plain" = "neutral", title?: string): HTMLElement {
  return h("span", { class: `badge ${tone === "neutral" ? "" : tone}`, title }, text);
}

export function safetyBadge(safety: string): HTMLElement {
  switch (safety) {
    case "safe": return badge("Safe", "good", "Apps recreate these files when needed");
    case "caution": return badge("Caution", "warn", "Safe to remove, but they take time to download again");
    default: return badge("Review first", "warn", "Check these before removing them");
  }
}

export function confidenceBadge(c: string): HTMLElement {
  return c === "high"
    ? badge("Likely leftover", "good", "Nothing installed uses it and it hasn't been touched in a long time")
    : badge("Check first", "warn", "Probably a leftover, but make sure you don't need it");
}

export function checkbox(checked: boolean, onchange: (v: boolean) => void, opts: { indeterminate?: boolean; disabled?: boolean; label?: string } = {}): HTMLInputElement {
  const cb = h("input", { type: "checkbox", class: "cb", checked, disabled: opts.disabled, "aria-label": opts.label });
  cb.indeterminate = !!opts.indeterminate;
  cb.addEventListener("click", (e) => e.stopPropagation());
  cb.addEventListener("change", () => onchange(cb.checked));
  return cb;
}

export function toggle(checked: boolean, onchange: (v: boolean) => void, label?: string): HTMLInputElement {
  const t = h("input", { type: "checkbox", class: "switch", role: "switch", checked, "aria-label": label });
  t.addEventListener("click", (e) => e.stopPropagation());
  t.addEventListener("change", () => onchange(t.checked));
  return t;
}

export function progressBar(pct: number | null): HTMLElement {
  const bar = h("div", { style: `width:${Math.max(2, Math.min(100, pct ?? 30))}%` });
  return h("div", { class: "progress" + (pct === null ? " indet" : "") }, bar);
}

export function setProgress(el: HTMLElement | null, pct: number | null): void {
  if (!el) return;
  el.classList.toggle("indet", pct === null);
  const bar = el.firstElementChild as HTMLElement | null;
  if (bar && pct !== null) bar.style.width = `${Math.max(2, Math.min(100, pct))}%`;
}

export function pctOf(p: Progress | null | undefined): number | null {
  if (!p || !p.total || p.total <= 0) return null;
  return (p.done / p.total) * 100;
}

/** A card shown while a scan runs. Returns update(progress). */
export function scanningCard(title: string, onCancel?: () => void): { el: HTMLElement; update: (p: Progress | null) => void } {
  const msg = h("div", { class: "msg" }, "Starting…");
  const bar = progressBar(null);
  const el = h("div", { class: "card scanning" },
    h("div", { class: "spinner" }),
    h("div", { class: "grow" }, h("div", { style: "font-weight:500;margin-bottom:10px" }, title), bar, msg),
    onCancel ? btn("Stop", { kind: "ghost", size: "sm", onclick: onCancel }) : null);
  return {
    el,
    update(p) {
      if (!p) return;
      setProgress(bar, pctOf(p));
      const text = p.message ? shortPath(p.message, 90) : "";
      msg.textContent = text + (p.bytes > 0 ? ` · ${fmtBytes(p.bytes)} found` : "");
    },
  };
}

export function empty(iconName: string, title: string, text: Child, ...actions: Child[]): HTMLElement {
  return h("div", { class: "empty" }, h("div", { class: "ico" }, icon(iconName)), h("h3", null, title), h("p", null, text),
    actions.length ? h("div", { class: "actions" }, ...actions) : null);
}

/** Says which drive a page is limited to, with a way back to every drive. */
export function scopeNote(what: string): HTMLElement | null {
  const d = currentDrive();
  if (!d) return null;
  return h("div", { class: "scope-note" }, icon("drive"),
    h("span", null, `${what} on ${d.label || "Local Disk"} (${driveName(d.root)}) only ·`),
    h("a", { href: "#", onclick: (e: Event) => { e.preventDefault(); setDrive(""); } }, "Show all drives"));
}

export function note(text: Child, tone: "info" | "warn" = "info"): HTMLElement {
  return h("div", { class: `note ${tone}` }, icon(tone === "warn" ? "alert" : "info"), h("div", null, text));
}

// Toasts ---------------------------------------------------------------------------

export function toast(msg: string, kind: "ok" | "err" = "ok"): void {
  const box = document.getElementById("toasts");
  if (!box) return;
  const t = h("div", { class: "toast" + (kind === "err" ? " err" : "") }, icon(kind === "err" ? "alert" : "check"), msg);
  box.appendChild(t);
  setTimeout(() => t.remove(), kind === "err" ? 6000 : 3200);
}

export function showError(e: unknown): void {
  toast(e instanceof Error ? e.message : String(e), "err");
}

// Dialogs ---------------------------------------------------------------------------

export interface DialogHandle {
  body: HTMLElement;
  foot: HTMLElement;
  set(body: Child, foot?: Child): void;
  close(): void;
  dismissable: boolean;
  closed: Promise<void>;
}

export function openDialog(opts: { wide?: boolean; dismissable?: boolean; label?: string } = {}): DialogHandle {
  const body = h("div", { class: "dialog-body" });
  const foot = h("div", { class: "dialog-foot" });
  const dlg = h("div", { class: "dialog" + (opts.wide ? " wide" : ""), role: "dialog", "aria-modal": "true", "aria-label": opts.label }, body, foot);
  const overlay = h("div", { class: "overlay" }, dlg);
  let resolveClosed: () => void = () => {};
  const closed = new Promise<void>((r) => (resolveClosed = r));
  const handle: DialogHandle = {
    body, foot, closed, dismissable: opts.dismissable ?? true,
    set(b, f) {
      clear(body, b);
      clear(foot, f ?? null);
      foot.style.display = f ? "" : "none";
      const first = (foot.querySelector(".btn.primary, .btn.danger") ?? foot.querySelector(".btn")) as HTMLElement | null;
      first?.focus();
    },
    close() {
      overlay.remove();
      document.removeEventListener("keydown", onKey, true);
      resolveClosed();
    },
  };
  const onKey = (e: KeyboardEvent) => {
    if (e.key === "Escape" && handle.dismissable) {
      e.stopPropagation();
      handle.close();
    }
  };
  overlay.addEventListener("mousedown", (e) => {
    if (e.target === overlay && handle.dismissable) handle.close();
  });
  document.addEventListener("keydown", onKey, true);
  document.body.appendChild(overlay);
  return handle;
}

/** A simple question with two buttons. */
export function confirmDialog(title: string, text: Child, okLabel: string, kind: BtnKind = "primary", extra?: Child): Promise<boolean> {
  return new Promise((resolve) => {
    const d = openDialog({ label: title });
    let answered = false;
    const answer = (v: boolean) => {
      answered = true;
      d.close();
      resolve(v);
    };
    d.set([h("h3", null, title), h("p", null, text), extra ?? null],
      [btn("Cancel", { onclick: () => answer(false) }), btn(okLabel, { kind, onclick: () => answer(true) })]);
    d.closed.then(() => { if (!answered) resolve(false); });
  });
}

// Menus & tooltips ---------------------------------------------------------------------

export interface MenuItem { label: string; icon?: string; danger?: boolean; onclick: () => void }

export function openMenu(anchor: HTMLElement, items: MenuItem[]): void {
  document.querySelectorAll(".menu").forEach((m) => m.remove());
  const menu = h("div", { class: "menu", role: "menu" }, items.map((it) =>
    h("button", { type: "button", role: "menuitem", class: it.danger ? "danger" : "", onclick: () => { close(); it.onclick(); } },
      it.icon ? icon(it.icon) : null, it.label)));
  document.body.appendChild(menu);
  // With a text size other than 100% the page is zoomed: work in unzoomed pixels.
  const z = uiScale();
  const b = anchor.getBoundingClientRect();
  const r = { top: b.top / z, bottom: b.bottom / z, right: b.right / z };
  const vw = window.innerWidth / z;
  const vh = window.innerHeight / z;
  const mw = menu.offsetWidth;
  const mh = menu.offsetHeight;
  let top = r.bottom + 6;
  if (top + mh > vh - 8) top = r.top - mh - 6;
  menu.style.top = `${Math.max(8, top)}px`;
  menu.style.left = `${Math.max(8, Math.min(vw - mw - 8, r.right - mw))}px`;
  const close = () => {
    menu.remove();
    document.removeEventListener("mousedown", outside, true);
    document.removeEventListener("keydown", key, true);
  };
  const outside = (e: MouseEvent) => { if (!menu.contains(e.target as Node)) close(); };
  const key = (e: KeyboardEvent) => { if (e.key === "Escape") close(); };
  setTimeout(() => {
    document.addEventListener("mousedown", outside, true);
    document.addEventListener("keydown", key, true);
  });
  (menu.querySelector("button") as HTMLElement | null)?.focus();
}

let tipEl: HTMLElement | null = null;
export function attachTip(el: HTMLElement, content: () => Child): void {
  const show = (x: number, y: number) => {
    tipEl?.remove();
    tipEl = h("div", { class: "tip", role: "tooltip" }, content());
    document.body.appendChild(tipEl);
    const z = uiScale();
    const w = tipEl.offsetWidth;
    tipEl.style.left = `${Math.min(window.innerWidth / z - w - 8, Math.max(8, x / z - w / 2))}px`;
    tipEl.style.top = `${y / z - tipEl.offsetHeight - 10}px`;
  };
  const hide = () => { tipEl?.remove(); tipEl = null; };
  el.addEventListener("pointermove", (e) => show(e.clientX, el.getBoundingClientRect().top));
  el.addEventListener("pointerleave", hide);
  el.addEventListener("focus", () => { const r = el.getBoundingClientRect(); show(r.left + r.width / 2, r.top); });
  el.addEventListener("blur", hide);
}

// Folder picker ---------------------------------------------------------------------

interface FsEntry { name: string; path: string; kind?: string }

export function pickFolder(title = "Choose a folder", start?: string): Promise<string | null> {
  return new Promise((resolve) => {
    const d = openDialog({ wide: true, label: title });
    let current = start ?? "";
    let picked: string | null = null;
    let places: FsEntry[] = [];
    let drives: FsEntry[] = [];
    const placeIcon: Record<string, string> = { home: "user", desktop: "monitor", downloads: "download", documents: "file", pictures: "image", videos: "video", music: "music", drive: "drive" };
    const placesEl = h("div", { class: "places" });
    const crumb = h("div", { class: "crumb" });
    const dirs = h("div", { class: "dirs" });
    const choose = btn("Select this folder", { kind: "primary", onclick: () => { picked = current; d.close(); } });
    const up = btn("Up", { kind: "ghost", size: "sm", icon: "chevRight", onclick: () => parent && open(parent) });
    let parent = "";
    const renderPlaces = () => {
      clear(placesEl, [...places, ...drives].map((p) =>
        h("button", { type: "button", class: p.path === current ? "on" : "", onclick: () => open(p.path) }, icon(placeIcon[p.kind ?? ""] ?? "folder"), h("span", null, p.name))));
    };
    const open = async (path: string) => {
      try {
        const r = await get<{ path: string; parent: string; dirs: FsEntry[] | null }>(`/api/fs?path=${encodeURIComponent(path)}`);
        current = r.path;
        parent = r.parent;
        clear(crumb, icon("folder"), h("span", { title: r.path }, "‎" + r.path));
        const list = r.dirs ?? [];
        clear(dirs, list.length ? list.map((x) => h("button", { type: "button", ondblclick: () => open(x.path), onclick: () => open(x.path) }, icon("folder"), h("span", null, x.name)))
          : h("div", { style: "padding:16px;color:var(--muted);font-size:13px" }, "No folders inside"));
        up.disabled = !parent;
        choose.disabled = false;
        renderPlaces();
      } catch (e) {
        showError(e);
      }
    };
    d.set([h("h3", null, title), h("p", null, "Pick a place on the left, then open folders on the right."),
      h("div", { class: "picker" }, placesEl, h("div", { class: "right" }, crumb, dirs))],
      [h("div", { class: "left" }, up), btn("Cancel", { onclick: () => d.close() }), choose]);
    up.querySelector("svg")?.setAttribute("style", "transform:rotate(-90deg)");
    choose.disabled = true;
    get<{ places: FsEntry[]; drives: FsEntry[] | null }>("/api/fs").then((r) => {
      places = r.places;
      drives = r.drives ?? [];
      renderPlaces();
      open(current || places[0]?.path || "");
    }).catch(showError);
    d.closed.then(() => resolve(picked));
  });
}

// Reviewing and cleaning -------------------------------------------------------------------

export interface CleanOutcome { result: CleanResult; done: string[] }

const kindIcon: Record<string, string> = { file: "file", folder: "folder", shortcut: "shortcut", regkey: "key", regvalue: "key", service: "cog", task: "calendar", recyclebin: "trash" };

/**
 * Builds a deletion plan, shows exactly what it contains, and only runs it
 * after an explicit click. Resolves with the outcome, or null when cancelled.
 */
export async function reviewAndClean(req: PlanRequest, opts: { title?: string; intro?: string } = {}): Promise<CleanOutcome | null> {
  let plan: Plan;
  try {
    plan = await post<Plan>("/api/plans", req);
  } catch (e) {
    showError(e);
    return null;
  }
  const s = plan.summary;
  return new Promise((resolve) => {
    const d = openDialog({ label: "Confirm" });
    let outcome: CleanOutcome | null = null;
    const facts: HTMLElement[] = [];
    const fact = (ic: string, title: string, text: string) =>
      facts.push(h("div", { class: "fact" }, h("div", { class: "ico" }, icon(ic)), h("div", null, h("b", null, title), h("span", null, text))));
    const junkish = plan.source === "junk";
    if (s.recycle > 0) fact("undo", `${plural(s.recycle, "item")} go${s.recycle === 1 ? "es" : ""} to the Recycle Bin`, "You can restore them from there if you change your mind.");
    if (s.permanent > 0) fact("trash", `${plural(s.permanent, "item")} deleted permanently`,
      junkish ? "Temporary and cache files that Windows and your apps recreate when needed." : "These can't be restored afterwards.");
    if (s.registry > 0) fact("key", `${plural(s.registry, "registry entry", "registry entries")} removed`, "Each one is backed up first, so it can be restored.");
    if (s.system > 0) fact("cog", `${plural(s.system, "service or task", "services and tasks")} removed`, "Backed up first. They belong to the app you're removing.");
    const groups = (s.groups ?? []).filter((g) => g.label);
    const preview = s.preview ?? [];
    const title = opts.title ?? (s.items === 1 ? "Delete 1 item?" : `Delete ${fmtNum(s.items)} items?`);
    const allRecycle = s.recycle > 0 && s.permanent === 0 && s.registry === 0 && s.system === 0;
    const go = btn(allRecycle ? "Move to Recycle Bin" : s.bytes > 0 ? `Delete ${fmtBytes(s.bytes)}` : "Delete", { kind: "danger", icon: "trash", onclick: () => run() });
    d.set([
      h("h3", null, title),
      opts.intro ? h("p", null, opts.intro) : null,
      s.bytes > 0 ? h("div", null, h("div", { class: "big-figure" }, fmtBytes(s.bytes)), h("div", { style: "color:var(--text-2);margin-bottom:4px" }, "will be freed")) : null,
      h("div", { class: "facts" }, facts),
      groups.length > 1 ? h("div", { class: "list-box" }, groups.map((g) => h("div", { class: "li" }, h("div", { class: "grow", style: "color:var(--text)" }, g.label), h("span", { style: "color:var(--muted)" }, plural(g.items, "item")), h("span", { class: "size" }, fmtBytes(g.bytes))))) : null,
      ...(s.notes ?? []).map((n) => note(n, "warn")),
      preview.length ? h("details", { class: "more" }, h("summary", null, icon("chevRight"), `Show ${preview.length < s.items ? `the ${preview.length} largest of ` : "all "}${plural(s.items, "item")}`),
        h("div", { class: "list-box" }, preview.map((p) => h("div", { class: "li", title: p.path }, icon(kindIcon[p.kind] ?? "file"),
          h("div", { class: "grow path" }, shortPath(p.path + (p.name ? `\\${p.name}` : ""), 80)), p.size > 0 ? h("span", { class: "size" }, fmtBytes(p.size)) : null)))) : null,
    ], [btn("Cancel", { onclick: () => d.close() }), go]);

    const run = async () => {
      d.dismissable = false;
      const msg = h("p", { class: "path", style: "min-height:20px" }, "Starting…");
      const bar = progressBar(0);
      const count = h("div", { style: "font-variant-numeric:tabular-nums;color:var(--text-2);margin-top:8px;font-size:13px" });
      d.set(h("div", { class: "wizard-center" }, h("div", { class: "spinner lg" }), h("h3", null, "Cleaning up…"), msg, h("div", { style: "max-width:380px;margin:14px auto 0" }, bar), count));
      try {
        const job = await runJob<{ result: CleanResult; done: string[] }>(`/api/plans/${plan.id}/execute`, {}, (p) => {
          msg.textContent = shortPath(p.message ?? "", 70);
          setProgress(bar, pctOf(p));
          count.textContent = `${fmtNum(p.done)} of ${fmtNum(p.total)} · ${fmtBytes(p.bytes)} freed`;
        });
        outcome = job.result ?? null;
        showResult(outcome);
      } catch (e) {
        d.dismissable = true;
        d.set([h("div", { class: "result-icon warn" }, icon("alert")), h("h3", null, "Something went wrong"), h("p", null, e instanceof Error ? e.message : String(e))],
          [btn("Close", { kind: "primary", onclick: () => d.close() })]);
      }
    };

    const showResult = (o: CleanOutcome | null) => {
      d.dismissable = true;
      const r = o?.result;
      if (!r) {
        d.close();
        return;
      }
      const lines: Child[] = [];
      const done = r.removed + r.recycled;
      if (r.removed > 0) lines.push(h("div", null, `${plural(r.removed, "item")} removed`));
      if (r.recycled > 0) lines.push(h("div", null, `${plural(r.recycled, "item")} moved to the Recycle Bin`));
      if (r.inUse > 0) lines.push(h("div", { class: "inline-warn" }, icon("info"), `${plural(r.inUse, "file")} ${r.inUse === 1 ? "was" : "were"} in use and skipped. Close the apps using them and clean again.`));
      if (r.skipped > 0) lines.push(h("div", { style: "color:var(--text-2)" }, `${plural(r.skipped, "item")} skipped (already gone, changed since the scan, or protected)`));
      if (r.failed > 0) lines.push(h("div", { class: "inline-warn" }, icon("alert"), `${plural(r.failed, "item")} couldn't be removed`));
      const problems = r.errors ?? [];
      d.set([
        h("div", { class: "result-icon" + (r.failed > 0 && done === 0 ? " warn" : "") }, icon(r.failed > 0 && done === 0 ? "alert" : "check")),
        h("h3", null, r.freed > 0 ? `Freed ${fmtBytes(r.freed)}` : done > 0 ? "All done" : "Nothing was removed"),
        h("div", { style: "display:flex;flex-direction:column;gap:6px;margin:8px 0 12px;color:var(--text)" }, lines),
        problems.length ? h("details", { class: "more" }, h("summary", null, icon("chevRight"), "Show details"),
          h("div", { class: "list-box" }, problems.map((p) => h("div", { class: "li" }, h("div", { class: "grow path", title: p }, p))))) : null,
      ], [
        r.backupDir ? h("div", { class: "left" }, btn("Open backups", { kind: "ghost", size: "sm", icon: "folder", onclick: () => post("/api/launch", { target: "backups" }).catch(showError) })) : null,
        btn("Done", { kind: "primary", onclick: () => d.close() }),
      ]);
    };
    d.closed.then(() => resolve(outcome));
  });
}

/** Whether personal files go to the Recycle Bin (the default). */
export function recycleLabel(): string {
  return store.settings?.recycleUserFiles === false ? "Delete" : "Move to Recycle Bin";
}
