import { CleanResult, Drive, post, Progress, Rec, runJob, SmartResult, cancelJob } from "../api.js";
import { fmtBytes, h, icon, plural, relTime, shortPath, sizeParts } from "../dom.js";
import { go, Page, rerender, store } from "../store.js";
import { attachTip, badge, btn, checkbox, pctOf, progressBar, reviewAndClean, safetyBadge, setProgress, showError, toast } from "../ui.js";

let scanning = false;
let jobId = "";
let progress: Progress | null = null;
let selected = new Set<string>();
let cleaned: CleanResult | null = null;
let progressEls: { bar: HTMLElement; msg: HTMLElement } | null = null;

// Each recommendation keeps its color no matter what else is selected.
const SLOT: Record<string, string> = {
  "junk-system": "var(--s1)", "junk-browsers": "var(--s2)", "junk-apps": "var(--s3)",
  "junk-developer": "var(--s4)", "junk-caution": "var(--s5)", "recycle-bin": "var(--s6)",
};

export async function smartScan(): Promise<void> {
  if (scanning) return;
  scanning = true;
  cleaned = null;
  progress = null;
  rerender("overview");
  try {
    const job = await runJob<SmartResult>("/api/scan/smart", {}, (p) => {
      progress = p;
      if (progressEls) {
        setProgress(progressEls.bar, pctOf(p) === null ? null : Math.max(4, pctOf(p)!));
        progressEls.msg.textContent = shortPath(p.message || "", 80);
      }
    }, (id) => (jobId = id));
    if (job.state === "done" && job.result) {
      const r = job.result;
      store.smart = r;
      store.junk = { scanId: r.junkScanId, categories: r.junk ?? [] };
      store.orphans = { scanId: r.orphanScanId, groups: r.orphans ?? [] };
      store.old = { scanId: r.oldScanId, files: r.old ?? [], days: store.settings?.oldDownloadDays ?? 60 };
      if (store.info) store.info.drives = r.drives;
      selected = new Set(r.recs.filter((x) => x.kind === "clean" && x.selected).map((x) => x.id));
    }
  } catch (e) {
    showError(e);
  } finally {
    scanning = false;
    progressEls = null;
    rerender("overview");
  }
}

function selectedTotal(recs: Rec[]): number {
  return recs.filter((r) => r.kind === "clean" && selected.has(r.id)).reduce((a, r) => a + r.size, 0);
}

async function cleanSelected(): Promise<void> {
  const smart = store.smart;
  if (!smart) return;
  const recs = smart.recs.filter((r) => r.kind === "clean" && selected.has(r.id));
  const ids = recs.flatMap((r) => r.categories ?? []);
  if (!ids.length) return;
  const total = selectedTotal(smart.recs);
  const out = await reviewAndClean({ source: "junk", scanId: smart.junkScanId, ids, title: "Cleaned recommended items" },
    { title: `Clean ${fmtBytes(total)}?`, intro: "Here's exactly what Jigglypuff will remove. Your documents, photos and settings are not touched." });
  if (out && out.result.removed + out.result.recycled > 0) {
    cleaned = out.result;
    store.junk = null;
    store.history = null;
    for (const r of recs) selected.delete(r.id);
    rerender("overview");
  }
}

function hero(): HTMLElement {
  const smart = store.smart;
  if (scanning) {
    const bar = progressBar(pctOf(progress));
    const msg = h("div", { class: "scanning msg", style: "padding:0;margin-top:10px" }, shortPath(progress?.message || "Starting…", 80));
    progressEls = { bar, msg };
    return h("div", { class: "card hero" },
      h("div", { class: "grow" }, h("div", { class: "hero-title" }, "Scanning your PC…"),
        h("div", { class: "hero-label", style: "margin:0 0 16px" }, "Only looking — nothing is changed."), h("div", { style: "max-width:520px" }, bar), msg),
      h("div", { class: "actions" }, btn("Stop", { onclick: () => jobId && cancelJob(jobId) })));
  }
  if (cleaned) {
    const [v, u] = sizeParts(cleaned.freed);
    return h("div", { class: "card hero done" },
      h("div", { class: "grow" }, h("div", { class: "hero-figure" }, v, h("small", null, u)),
        h("div", { class: "hero-label" }, cleaned.freed > 0 ? "freed. Your PC is a little lighter." : "Cleaned."),
        cleaned.inUse > 0 ? h("div", { class: "inline-warn", style: "margin-top:10px" }, icon("info"), `${plural(cleaned.inUse, "file")} in use ${cleaned.inUse === 1 ? "was" : "were"} skipped.`) : null),
      h("div", { class: "actions" }, btn("Scan again", { kind: "primary", size: "lg", icon: "refresh", onclick: smartScan })));
  }
  if (!smart) {
    return h("div", { class: "card hero" },
      h("div", { class: "grow" }, h("div", { class: "hero-title" }, "Let's see what can go"),
        h("div", { class: "hero-label", style: "max-width:520px" }, "Jigglypuff looks for junk files, leftovers of removed apps and forgotten downloads. Scanning only looks — nothing is removed until you confirm.")),
      h("div", { class: "actions" }, btn("Scan my PC", { kind: "primary", size: "lg", icon: "search", onclick: smartScan })));
  }
  const total = selectedTotal(smart.recs);
  const [v, u] = sizeParts(total);
  const segs = smart.recs.filter((r) => r.kind === "clean" && selected.has(r.id) && r.size > 0);
  const bar = h("div", { class: "segbar", role: "img", "aria-label": segs.map((r) => `${r.title} ${fmtBytes(r.size)}`).join(", ") },
    segs.map((r) => {
      const seg = h("div", { tabindex: "0", style: `flex:${r.size} 1 0;background:${SLOT[r.id] ?? "var(--s1)"}`, "aria-label": `${r.title}: ${fmtBytes(r.size)}` });
      attachTip(seg, () => [h("b", null, fmtBytes(r.size)), h("span", null, h("span", { class: "k", style: `background:${SLOT[r.id]}` }), `${r.title} · ${Math.round((r.size / Math.max(total, 1)) * 100)}%`)]);
      return seg;
    }));
  const found = smart.recs.filter((r) => r.kind === "clean").reduce((a, r) => a + r.size, 0);
  return h("div", { class: "card hero" },
    h("div", { class: "grow" },
      h("div", { class: "hero-figure" }, v, h("small", null, u)),
      h("div", { class: "hero-label" }, total > 0 ? "ready to clean safely" : "selected — tick something below to clean it"),
      segs.length ? bar : null,
      h("div", { style: "color:var(--muted);font-size:12.5px;margin-top:12px" }, `${fmtBytes(found)} of junk found · scanned ${relTime(smart.time)}`)),
    h("div", { class: "actions" },
      btn(total > 0 ? `Clean ${fmtBytes(total)}` : "Clean", { kind: "primary", size: "lg", icon: "broom", disabled: total === 0, onclick: cleanSelected }),
      btn("Scan again", { kind: "ghost", icon: "refresh", onclick: smartScan })));
}

async function runAction(action: string): Promise<void> {
  try {
    if (action === "restart-admin") {
      const r = await post<{ url: string }>("/api/restart-admin");
      location.href = r.url;
    } else {
      await post("/api/launch", { target: action });
      toast("Opened Windows Storage settings");
    }
  } catch (e) {
    showError(e);
  }
}

function recRow(r: Rec): HTMLElement {
  const ico = h("div", { class: "ico" }, icon(r.icon));
  if (r.kind === "clean") {
    const on = selected.has(r.id);
    const toggleSel = (v: boolean) => {
      if (v) selected.add(r.id);
      else selected.delete(r.id);
      rerender("overview");
    };
    return h("div", { class: "row click", onclick: () => toggleSel(!on) },
      checkbox(on, toggleSel, { label: r.title }),
      h("div", { class: "swatch", style: `background:${on ? SLOT[r.id] ?? "var(--s1)" : "var(--surface-3)"}` }),
      ico,
      h("div", { class: "grow" }, h("div", { class: "title" }, h("span", { class: "t" }, r.title)), h("div", { class: "desc", title: r.desc }, r.desc)),
      safetyBadge(r.safety),
      h("div", { class: "size" }, fmtBytes(r.size)),
      btn("Details", { kind: "ghost", size: "sm", onclick: (e) => { e.stopPropagation(); go("cleaner"); } }));
  }
  if (r.kind === "review") {
    return h("div", { class: "row click", onclick: () => r.page && go(r.page) },
      h("div", { style: "width:18px" }), h("div", { style: "width:10px" }), ico,
      h("div", { class: "grow" }, h("div", { class: "title" }, h("span", { class: "t" }, r.title)), h("div", { class: "desc", title: r.desc }, r.desc)),
      r.size > 0 ? h("div", { class: "size muted" }, fmtBytes(r.size)) : null,
      btn("Review", { size: "sm", icon: "arrowRight", onclick: (e) => { e.stopPropagation(); r.page && go(r.page); } }));
  }
  return h("div", { class: "row" },
    h("div", { style: "width:18px" }), h("div", { style: "width:10px" }), ico,
    h("div", { class: "grow" }, h("div", { class: "title" }, h("span", { class: "t" }, r.title)), h("div", { class: "desc wrap" }, r.desc)),
    r.action ? btn(r.action === "restart-admin" ? "Restart as admin" : "Open settings", { size: "sm", onclick: () => runAction(r.action!) }) : null);
}

function driveCard(d: Drive): HTMLElement {
  const used = d.total - d.free;
  const freeShare = d.free / Math.max(d.total, 1);
  const GB = 1024 ** 3;
  const tone = d.free < 5 * GB || freeShare < 0.04 ? "bad" : d.free < 15 * GB || freeShare < 0.08 ? "warn" : "";
  const name = `${d.label || "Local Disk"} (${d.root.replace(/\\$/, "")})`;
  return h("div", { class: "card drive" },
    h("div", { class: "name" }, icon("drive"), h("span", null, name), tone ? h("span", { style: "margin-left:auto" }, badge("Low on space", tone === "bad" ? "bad" : "warn")) : null),
    h("div", { class: `meter ${tone}`, role: "meter", "aria-valuenow": String(Math.round((used / d.total) * 100)), "aria-valuemin": "0", "aria-valuemax": "100", "aria-label": `${name} used` },
      h("div", { style: `width:${(used / Math.max(d.total, 1)) * 100}%` })),
    h("div", { class: "free" }, h("span", null, `${fmtBytes(d.free)} free`), h("span", null, `of ${fmtBytes(d.total)}`)));
}

export const overviewPage: Page = {
  id: "overview",
  render(root) {
    const smart = store.smart;
    const user = store.info?.user;
    root.append(h("div", { class: "page-head" }, h("div", { class: "titles" },
      h("h1", null, user ? `Hi, ${user}` : "Overview"),
      h("p", { class: "subtitle" }, "Here's how your PC is doing. Jigglypuff never deletes anything without showing you first."))));
    root.append(hero());
    const drives = store.info?.drives ?? [];
    const sys = drives.find((d) => d.system);
    if (sys && (sys.free < 15 * 1024 ** 3 || sys.free / sys.total < 0.08) && !scanning) {
      root.append(h("div", { class: "note warn", style: "margin-top:12px" }, icon("alert"),
        h("div", null, `Your system drive has only ${fmtBytes(sys.free)} free. Windows needs some free space to update and run smoothly.`)));
    }
    if (smart && !scanning) {
      const clean = smart.recs.filter((r) => r.kind === "clean");
      const other = smart.recs.filter((r) => r.kind !== "clean");
      if (clean.length && !cleaned) {
        root.append(h("h2", null, "Ready to clean", h("span", { class: "meta" }, "· tick what you want removed")),
          h("div", { class: "card" }, clean.map(recRow)));
      }
      if (other.length) {
        root.append(h("h2", null, "Worth a look", h("span", { class: "meta" }, "· these need your judgement")), h("div", { class: "card" }, other.map(recRow)));
      }
      if (!smart.recs.length && !cleaned) {
        root.append(h("div", { class: "card", style: "margin-top:12px" }, h("div", { class: "empty" },
          h("div", { class: "ico" }, icon("check")), h("h3", null, "Your PC looks clean"), h("p", null, "Jigglypuff didn't find anything worth removing right now."))));
      }
    }
    if (drives.length) {
      root.append(h("h2", null, "Drives"), h("div", { class: "drives" }, drives.map(driveCard)));
    }
  },
};
