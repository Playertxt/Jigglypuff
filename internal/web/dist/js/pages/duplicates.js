import { cancelJob, post, runJob } from "../api.js";
import { baseName, fmtBytes, fmtDate, h, icon, plural, relTime, shortPath } from "../dom.js";
import { currentDrive, folderLabel, onDriveChange, rerender, store } from "../store.js";
import { badge, btn, checkbox, empty, pickFolder, recycleLabel, reviewAndClean, scanningCard, showError, toast } from "../ui.js";
let scanning = false;
let jobId = "";
let update = null;
let folders = [];
let minKB = 0;
let selected = new Set();
let showAll = false;
function defaults() {
    if (!folders.length) {
        const p = store.info?.paths ?? {};
        const d = currentDrive();
        folders = d && !d.system ? [d.path]
            : (store.settings?.dupFolders?.length ? store.settings.dupFolders : [p.desktop, p.documents, p.downloads, p.pictures, p.music, p.videos]).filter(Boolean);
    }
    if (!minKB)
        minKB = store.settings?.dupMinKB ?? 1024;
}
onDriveChange(() => {
    folders = [];
    selected = new Set();
});
function autoSelect(groups, keep) {
    selected = new Set();
    for (const g of groups) {
        const sorted = [...g.files].sort((a, b) => a.modified.localeCompare(b.modified));
        const keepId = keep === "oldest" ? sorted[0].id : sorted[sorted.length - 1].id;
        for (const f of g.files)
            if (f.id !== keepId)
                selected.add(f.id);
    }
}
async function scan() {
    if (scanning || !folders.length)
        return;
    scanning = true;
    rerender("duplicates");
    try {
        const job = await runJob("/api/scan/dups", { folders, minKB }, (p) => update?.(p), (id) => (jobId = id));
        if (job.state === "done" && job.result) {
            store.dups = { scanId: job.result.scanId, groups: job.result.groups ?? [], folders: job.result.folders };
            autoSelect(store.dups.groups, "oldest");
            showAll = false;
        }
    }
    catch (e) {
        showError(e);
    }
    finally {
        scanning = false;
        update = null;
        rerender("duplicates");
    }
}
async function remove() {
    const d = store.dups;
    if (!d || !selected.size)
        return;
    const out = await reviewAndClean({ source: "dups", scanId: d.scanId, ids: [...selected], title: "Removed duplicate copies" }, { title: "Remove the duplicate copies?", intro: "One copy of every file stays where it is." });
    if (out) {
        const gone = new Set(out.done.map((p) => p.toLowerCase()));
        d.groups = d.groups.map((g) => ({ ...g, files: g.files.filter((f) => !gone.has(f.path.toLowerCase())) })).filter((g) => g.files.length > 1);
        for (const g of d.groups)
            g.wasted = g.size * (g.files.length - 1);
        selected = new Set([...selected].filter((id) => d.groups.some((g) => g.files.some((f) => f.id === id))));
        store.history = null;
        rerender("duplicates");
    }
}
function groupCard(g) {
    const first = g.files[0];
    const card = h("div", { class: "card" }, h("div", { class: "group-head", style: "font-weight:500;color:var(--text);font-size:14px" }, h("div", { class: "ico sm" }, icon(first.type)), h("span", { style: "overflow:hidden;text-overflow:ellipsis;white-space:nowrap" }, first.name), h("span", { style: "color:var(--muted);font-weight:400;font-size:13px;white-space:nowrap" }, `${g.files.length} copies · ${fmtBytes(g.size)} each`), h("span", { class: "size" }, `${fmtBytes(g.wasted)} extra`)));
    for (const f of g.files) {
        const on = selected.has(f.id);
        const set = (v) => {
            if (v) {
                const others = g.files.filter((x) => x.id !== f.id && !selected.has(x.id));
                if (!others.length) {
                    toast("Keep at least one copy of every file", "err");
                    rerender("duplicates");
                    return;
                }
                selected.add(f.id);
            }
            else
                selected.delete(f.id);
            rerender("duplicates");
        };
        card.append(h("div", { class: "row click", onclick: () => set(!on) }, checkbox(on, set, { label: f.path }), h("div", { class: "ico sm" }, icon("folder")), h("div", { class: "grow" }, h("div", { class: "title" }, h("span", { class: "t path selectable", style: "color:var(--text);font-size:13.5px", title: f.path }, shortPath(f.dir, 80)), on ? null : badge("Keep", "good")), h("div", { class: "desc" }, `${f.name} · changed ${fmtDate(f.modified)}`)), h("div", { class: "when" }, relTime(f.modified)), btn("", { kind: "ghost", size: "sm", iconOnly: true, icon: "external", title: "Show in File Explorer",
            onclick: (e) => { e.stopPropagation(); post("/api/open", { path: f.path, reveal: true }).catch(showError); } })));
    }
    return card;
}
export const duplicatesPage = {
    id: "duplicates",
    enter() {
        defaults();
    },
    render(root) {
        defaults();
        root.append(h("div", { class: "page-head" }, h("div", { class: "titles" }, h("h1", null, "Duplicates"), h("p", { class: "subtitle" }, "Find identical files and keep just one copy. Files are compared by their content, not their names."))));
        const sizeSel = h("select", { class: "input", "aria-label": "Minimum size", onchange: (e) => { minKB = Number(e.target.value); } }, [[100, "100 KB"], [1024, "1 MB"], [10240, "10 MB"], [102400, "100 MB"]].map(([v, l]) => h("option", { value: String(v) }, `Bigger than ${l}`)));
        sizeSel.value = String([100, 1024, 10240, 102400].includes(minKB) ? minKB : 1024);
        root.append(h("div", { class: "card pad", style: "margin-bottom:14px" }, h("div", { class: "field", style: "margin-bottom:10px" }, "Look in these folders"), h("div", { style: "display:flex;gap:8px;flex-wrap:wrap;align-items:center" }, folders.map((f) => h("span", { class: "chip", title: f }, icon("folder"), h("span", null, folderLabel(f) || baseName(f)), h("button", { type: "button", title: "Remove", "aria-label": `Remove ${baseName(f)}`, onclick: () => { folders = folders.filter((x) => x !== f); rerender("duplicates"); } }, icon("x")))), btn("Add folder", { size: "sm", icon: "plus", onclick: async () => { const f = await pickFolder("Add a folder to check"); if (f && !folders.includes(f)) {
                folders.push(f);
                rerender("duplicates");
            } } })), h("div", { style: "display:flex;gap:12px;align-items:center;margin-top:14px;flex-wrap:wrap" }, sizeSel, h("span", { style: "flex:1" }), btn(store.dups ? "Scan again" : "Find duplicates", { kind: "primary", icon: "search", disabled: scanning || !folders.length, onclick: scan }))));
        if (scanning) {
            const card = scanningCard("Comparing files…", () => jobId && cancelJob(jobId));
            update = (p) => card.update(p && { ...p, message: p.phase === "listing" ? `${p.done.toLocaleString()} files found · ${p.message}` : `Comparing · ${p.message}`, bytes: 0 });
            root.append(card.el);
            return;
        }
        const d = store.dups;
        if (!d) {
            root.append(h("div", { class: "card" }, empty("copy", "Find copies of the same file", "Photos saved twice, downloads you got again, backups of backups. Jigglypuff keeps one copy of each.")));
            return;
        }
        if (!d.groups.length) {
            root.append(h("div", { class: "card" }, empty("check", "No duplicates found", "Every file in these folders is unique.")));
            return;
        }
        const wasted = d.groups.reduce((a, g) => a + g.wasted, 0);
        const selFiles = d.groups.flatMap((g) => g.files.filter((f) => selected.has(f.id)));
        const selSize = selFiles.reduce((a, f) => a + f.size, 0);
        root.append(h("div", { class: "toolbar" }, h("span", null, `${plural(d.groups.length, "file")} with copies · ${fmtBytes(wasted)} could be freed`), h("span", { class: "spacer" }), h("span", null, "Select copies:"), h("button", { class: "link", onclick: () => { autoSelect(d.groups, "oldest"); rerender("duplicates"); } }, "Keep the oldest"), h("span", { class: "sep" }, "·"), h("button", { class: "link", onclick: () => { autoSelect(d.groups, "newest"); rerender("duplicates"); } }, "Keep the newest"), h("span", { class: "sep" }, "·"), h("button", { class: "link", onclick: () => { selected = new Set(); rerender("duplicates"); } }, "None")));
        const shown = showAll ? d.groups : d.groups.slice(0, 150);
        root.append(...shown.map(groupCard));
        if (shown.length < d.groups.length) {
            root.append(h("div", { style: "text-align:center;margin-top:14px" }, btn(`Show all ${d.groups.length}`, { onclick: () => { showAll = true; rerender("duplicates"); } })));
        }
        if (selected.size) {
            root.append(h("div", { class: "actionbar" }, h("div", { class: "grow" }, `${plural(selFiles.length, "copy", "copies")} selected `, h("span", null, `· ${fmtBytes(selSize)}`)), btn("Clear", { kind: "ghost", onclick: () => { selected = new Set(); rerender("duplicates"); } }), btn(recycleLabel(), { kind: "danger", icon: "trash", onclick: remove })));
        }
    },
};
