import { cancelJob, runJob } from "../api.js";
import { fmtBytes, h, icon, plural, relTime } from "../dom.js";
import { rerender, store } from "../store.js";
import { btn, checkbox, confidenceBadge, empty, note, reviewAndClean, scanningCard, scopeNote, showError } from "../ui.js";
import { leftoverRow } from "./apps.js";
let scanning = false;
let jobId = "";
let selected = new Set();
let expanded = new Set();
let update = null;
export async function scanLeftovers() {
    if (scanning)
        return;
    scanning = true;
    rerender("leftovers");
    try {
        const job = await runJob("/api/scan/orphans", { drive: store.drive }, (p) => update?.(p), (id) => (jobId = id));
        if (job.state === "done" && job.result) {
            store.orphans = { scanId: job.result.scanId, groups: job.result.groups ?? [] };
            selected = new Set();
        }
    }
    catch (e) {
        showError(e);
    }
    finally {
        scanning = false;
        update = null;
        rerender("leftovers");
    }
}
async function remove() {
    const o = store.orphans;
    if (!o || !selected.size)
        return;
    const out = await reviewAndClean({ source: "leftovers", scanId: o.scanId, ids: [...selected], title: "Removed leftovers of uninstalled apps" }, { title: "Remove these leftovers?" });
    if (out) {
        store.smart = null;
        store.history = null;
        selected = new Set();
        scanLeftovers();
    }
}
function groupCard(g) {
    const ids = g.items.map((l) => l.id);
    const n = ids.filter((id) => selected.has(id)).length;
    const open = expanded.has(g.key);
    const setAll = (v) => {
        for (const id of ids) {
            if (v)
                selected.add(id);
            else
                selected.delete(id);
        }
        rerender("leftovers");
    };
    const toggleOpen = () => {
        if (open)
            expanded.delete(g.key);
        else
            expanded.add(g.key);
        rerender("leftovers");
    };
    const last = relTime(g.lastUsed);
    const kinds = new Set(g.items.map((l) => (l.kind === "regkey" || l.kind === "regvalue" ? "registry" : "files")));
    const what = [kinds.has("files") ? "files" : "", kinds.has("registry") ? "registry settings" : ""].filter(Boolean).join(" and ");
    const head = h("div", { class: "row click", onclick: toggleOpen }, checkbox(n === ids.length, setAll, { indeterminate: n > 0 && n < ids.length, label: g.name }), h("div", { class: "ico" }, icon("package")), h("div", { class: "grow" }, h("div", { class: "title" }, h("span", { class: "t" }, g.name)), h("div", { class: "desc" }, `${plural(g.items.length, "item")} · ${what}${last ? ` · last changed ${last}` : ""}`)), confidenceBadge(g.confidence), h("div", { class: "size" + (g.size ? "" : " muted") }, g.size ? fmtBytes(g.size) : "—"), h("span", { class: "chev" + (open ? " open" : "") }, icon("chevRight")));
    const card = h("div", { class: "card" }, head);
    if (open) {
        for (const l of g.items) {
            const row = leftoverRow(l, selected.has(l.id), (v) => {
                if (v)
                    selected.add(l.id);
                else
                    selected.delete(l.id);
                rerender("leftovers");
            });
            row.classList.add("sub");
            card.append(row);
        }
    }
    return card;
}
export const leftoversPage = {
    id: "leftovers",
    enter() {
        if (!store.orphans && !scanning)
            scanLeftovers();
    },
    render(root) {
        const o = store.orphans;
        const groups = o?.groups ?? [];
        const items = groups.flatMap((g) => g.items);
        const selSize = items.filter((l) => selected.has(l.id)).reduce((a, l) => a + l.size, 0);
        root.append(h("div", { class: "page-head" }, h("div", { class: "titles" }, h("h1", null, "Leftovers"), h("p", { class: "subtitle" }, "Folders and settings left behind by apps you've already uninstalled.")), h("div", { class: "actions" }, btn("Scan again", { kind: "ghost", icon: "refresh", disabled: scanning, onclick: scanLeftovers }), btn(selected.size ? `Remove ${selected.size} selected` : "Remove selected", { kind: "danger", icon: "trash", disabled: scanning || !selected.size, onclick: remove }))));
        const scope = scopeNote("Leftovers");
        if (scope)
            root.append(scope);
        if (scanning) {
            const card = scanningCard("Looking for leftovers…", () => jobId && cancelJob(jobId));
            update = card.update;
            root.append(card.el);
            return;
        }
        if (!o) {
            root.append(h("div", { class: "card" }, empty("package", "Find leftovers", "Jigglypuff compares what's on your disk with what's installed.", btn("Scan now", { kind: "primary", icon: "search", onclick: scanLeftovers }))));
            return;
        }
        root.append(note("Jigglypuff only lists folders that no installed app, service, shortcut or running program uses, and that haven't changed in the last 30 days. Folders go to the Recycle Bin and registry settings are backed up, so you can undo this."));
        if (!groups.length) {
            root.append(h("div", { class: "card", style: "margin-top:12px" }, empty("check", "No leftovers found", "Apps you removed didn't leave anything Jigglypuff is sure about.")));
            return;
        }
        const total = groups.reduce((a, g) => a + g.size, 0);
        root.append(h("div", { class: "toolbar", style: "margin-top:16px" }, h("span", null, `${plural(groups.length, "app")} · ${fmtBytes(total)}`), h("span", { class: "spacer" }), h("span", null, "Select:"), h("button", { class: "link", onclick: () => { selected = new Set(items.filter((l) => l.confidence === "high").map((l) => l.id)); rerender("leftovers"); } }, "Likely leftovers"), h("span", { class: "sep" }, "·"), h("button", { class: "link", onclick: () => { selected = new Set(items.map((l) => l.id)); rerender("leftovers"); } }, "All"), h("span", { class: "sep" }, "·"), h("button", { class: "link", onclick: () => { selected = new Set(); rerender("leftovers"); } }, "None")));
        root.append(...groups.map(groupCard));
        if (selected.size) {
            root.append(h("div", { class: "actionbar" }, h("div", { class: "grow" }, `${plural(selected.size, "item")} selected `, h("span", null, selSize ? `· ${fmtBytes(selSize)}` : "")), btn("Clear", { kind: "ghost", onclick: () => { selected = new Set(); rerender("leftovers"); } }), btn("Remove selected", { kind: "danger", icon: "trash", onclick: remove })));
        }
    },
};
