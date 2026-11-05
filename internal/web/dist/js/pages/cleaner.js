import { cancelJob, post, runJob } from "../api.js";
import { fmtBytes, fmtNum, h, icon, plural, shortPath } from "../dom.js";
import { rerender, store } from "../store.js";
import { badge, btn, checkbox, empty, reviewAndClean, safetyBadge, scanningCard, scopeNote, showError } from "../ui.js";
let scanning = false;
let jobId = "";
let selected = new Set();
let expanded = new Set();
let seenScan = "";
let update = null;
const CAT_ICON = {
    "user-temp": "clock", "win-temp": "monitor", "wu-cache": "download", "delivery-opt": "refresh", "error-reports": "alert",
    "crash-dumps": "alert", "win-logs": "file", thumbnails: "image", "inet-cache": "globe", "shader-cache": "sparkle",
    "broken-shortcuts": "shortcut", "recycle-bin": "trash", "app-caches": "grid", "store-caches": "grid", steam: "grid",
};
function catIcon(c) {
    if (CAT_ICON[c.id])
        return CAT_ICON[c.id];
    if (c.id.startsWith("browser-"))
        return "globe";
    if (c.id.startsWith("dev-"))
        return c.safety === "caution" ? "box" : "code";
    return c.group;
}
const GROUPS = [
    ["system", "Windows", "monitor"], ["browsers", "Browsers", "globe"], ["apps", "Apps", "grid"], ["developer", "Developer tools", "code"],
];
export async function scanJunk() {
    if (scanning)
        return;
    scanning = true;
    rerender("cleaner");
    try {
        const job = await runJob("/api/scan/junk", { drive: store.drive }, (p) => update?.(p), (id) => (jobId = id));
        if (job.state === "done" && job.result)
            store.junk = job.result;
    }
    catch (e) {
        showError(e);
    }
    finally {
        scanning = false;
        update = null;
        rerender("cleaner");
    }
}
function syncSelection() {
    const j = store.junk;
    if (!j || j.scanId === seenScan)
        return;
    seenScan = j.scanId;
    selected = new Set(j.categories.filter((c) => c.default && c.available && c.size > 0 && c.special !== "recyclebin").map((c) => c.id));
}
function selectable(c) {
    return c.available && (c.size > 0 || c.files > 0);
}
async function clean() {
    const j = store.junk;
    if (!j)
        return;
    const out = await reviewAndClean({ source: "junk", scanId: j.scanId, ids: [...selected], title: "Cleaned junk files" });
    if (out) {
        store.junk = null;
        store.smart = null;
        store.history = null;
        scanJunk();
    }
}
function runningNote(c) {
    if (!c.running?.length)
        return null;
    const app = c.group === "browsers" ? c.name.replace(/ cache$/, "") : c.running.map((r) => r.replace(/\.exe$/i, "")).join(", ");
    return h("div", { class: "inline-warn" }, icon("alert"), `${app} is open. Close it first to clean everything.`);
}
function categoryRow(c) {
    const can = selectable(c);
    const on = selected.has(c.id) && can;
    const open = expanded.has(c.id);
    const locs = c.locations ?? [];
    const set = (v) => {
        if (v)
            selected.add(c.id);
        else
            selected.delete(c.id);
        rerender("cleaner");
    };
    const desc = [c.desc];
    const rows = [h("div", { class: "row click" + (can ? "" : " disabled"), onclick: () => can && set(!on) }, checkbox(on, set, { disabled: !can, label: c.name }), h("div", { class: "ico" }, icon(catIcon(c))), h("div", { class: "grow" }, h("div", { class: "title" }, h("span", { class: "t" }, c.name)), h("div", { class: "desc", title: c.desc }, ...desc), runningNote(c), c.skipped > 0 && c.id.includes("temp")
            ? h("div", { style: "color:var(--muted);font-size:12.5px;margin-top:2px" }, `${plural(c.skipped, "recent file")} kept, in case an app still needs ${c.skipped === 1 ? "it" : "them"}`)
            : c.note && !c.running?.length && c.size > 0 ? h("div", { style: "color:var(--muted);font-size:12.5px;margin-top:2px" }, c.note) : null), !c.available ? badge("Needs admin", "plain") : safetyBadge(c.safety), h("div", { class: "size" + (c.size ? "" : " muted") }, c.available ? (c.size || c.files ? fmtBytes(c.size) : "Clean") : "—"), locs.length ? btn("", { kind: "ghost", size: "sm", iconOnly: true, icon: "chevRight", title: open ? "Hide details" : "Show details",
            onclick: (e) => { e.stopPropagation(); if (open)
                expanded.delete(c.id);
            else
                expanded.add(c.id); rerender("cleaner"); } }) : h("div", { style: "width:28px" }))];
    const chev = rows[0].querySelector(".btn.icon svg");
    if (chev && open)
        chev.classList.add("chev", "open");
    else
        chev?.classList.add("chev");
    if (open) {
        for (const l of locs.slice(0, 30)) {
            rows.push(h("div", { class: "row sub" }, icon(c.special === "shortcuts" ? "shortcut" : "folder"), h("div", { class: "grow" }, h("div", { class: "title path selectable", title: l.path }, shortPath(l.path, 86)), l.label ? h("div", { class: "desc" }, l.label) : null), h("div", { class: "when" }, plural(l.files, "file")), h("div", { class: "size" }, fmtBytes(l.size)), btn("", { kind: "ghost", size: "sm", iconOnly: true, icon: "external", title: "Show in File Explorer",
                onclick: () => post("/api/open", { path: l.path, reveal: c.special === "shortcuts" }).catch(showError) })));
        }
    }
    return rows;
}
export const cleanerPage = {
    id: "cleaner",
    enter() {
        if (!store.junk && !scanning)
            scanJunk();
    },
    render(root) {
        syncSelection();
        const j = store.junk;
        const cats = j?.categories ?? [];
        const total = cats.filter((c) => selected.has(c.id) && selectable(c)).reduce((a, c) => a + c.size, 0);
        const count = cats.filter((c) => selected.has(c.id) && selectable(c)).length;
        root.append(h("div", { class: "page-head" }, h("div", { class: "titles" }, h("h1", null, "Junk cleaner"), h("p", { class: "subtitle" }, "Temporary files, caches and logs that Windows and your apps rebuild on their own.")), h("div", { class: "actions" }, btn("Scan again", { kind: "ghost", icon: "refresh", disabled: scanning, onclick: scanJunk }), btn(total > 0 ? `Clean ${fmtBytes(total)}` : "Clean selected", { kind: "primary", icon: "broom", disabled: scanning || count === 0, onclick: clean }))));
        const scope = scopeNote("Junk");
        if (scope)
            root.append(scope);
        if (scanning) {
            const card = scanningCard("Looking for junk…", () => jobId && cancelJob(jobId));
            update = card.update;
            root.append(card.el);
            return;
        }
        if (!j) {
            root.append(h("div", { class: "card" }, empty("broom", "Find junk files", "Scanning only looks. You choose what to remove afterwards.", btn("Scan now", { kind: "primary", icon: "search", onclick: scanJunk }))));
            return;
        }
        const found = cats.reduce((a, c) => a + (c.available ? c.size : 0), 0);
        const files = cats.reduce((a, c) => a + (c.available ? c.files : 0), 0);
        const setAll = (pred) => {
            selected = new Set(cats.filter((c) => selectable(c) && pred(c)).map((c) => c.id));
            rerender("cleaner");
        };
        root.append(h("div", { class: "toolbar" }, h("span", null, `Found ${fmtBytes(found)} in ${plural(files, "file")}`), h("span", { class: "spacer" }), h("span", null, "Select:"), h("button", { class: "link", onclick: () => setAll((c) => c.default && c.special !== "recyclebin") }, "Recommended"), h("span", { class: "sep" }, "·"), h("button", { class: "link", onclick: () => setAll(() => true) }, "All"), h("span", { class: "sep" }, "·"), h("button", { class: "link", onclick: () => setAll(() => false) }, "None")));
        for (const [gid, label, ic] of GROUPS) {
            const list = cats.filter((c) => c.group === gid);
            if (!list.length)
                continue;
            const sel = list.filter((c) => selected.has(c.id) && selectable(c));
            const avail = list.filter(selectable);
            const gsize = list.reduce((a, c) => a + (c.available ? c.size : 0), 0);
            const setGroup = (v) => {
                for (const c of avail) {
                    if (v)
                        selected.add(c.id);
                    else
                        selected.delete(c.id);
                }
                rerender("cleaner");
            };
            root.append(h("div", { class: "card", style: "margin-bottom:14px" }, h("div", { class: "group-head" }, checkbox(sel.length > 0 && sel.length === avail.length, setGroup, { indeterminate: sel.length > 0 && sel.length < avail.length, disabled: !avail.length, label: label }), icon(ic), h("span", null, label), h("span", { class: "size" }, fmtBytes(gsize))), list.flatMap(categoryRow)));
        }
        if (count > 0) {
            root.append(h("div", { class: "actionbar" }, h("div", { class: "grow" }, `${fmtBytes(total)} selected `, h("span", null, `· ${fmtNum(count)} ${count === 1 ? "category" : "categories"}`)), btn(`Clean ${fmtBytes(total)}`, { kind: "primary", icon: "broom", onclick: clean })));
        }
    },
};
