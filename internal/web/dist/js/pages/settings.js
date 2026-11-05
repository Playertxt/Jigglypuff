import { post } from "../api.js";
import { h, icon, shortPath } from "../dom.js";
import { applyTheme } from "../theme.js";
import { go, rerender, store } from "../store.js";
import { btn, pickFolder, showError, toast, toggle } from "../ui.js";
async function save(patch, quiet = false) {
    if (!store.settings)
        return;
    const next = { ...store.settings, ...patch };
    try {
        store.settings = await post("/api/settings", next);
        if (store.info)
            store.info.settings = store.settings;
        applyTheme(store.settings.theme);
        if (!quiet)
            toast("Saved");
    }
    catch (e) {
        showError(e);
    }
    rerender("settings");
}
function row(label, help, control) {
    return h("div", { class: "settings-row" }, h("div", { class: "grow" }, h("div", { class: "label" }, label), h("div", { class: "help" }, help)), control);
}
function numberInput(value, min, max, onsave, suffix) {
    const input = h("input", { class: "input num", type: "number", min: String(min), max: String(max), value: String(value) });
    input.addEventListener("change", () => {
        const v = Math.round(Number(input.value));
        if (Number.isFinite(v) && v >= min && v <= max)
            onsave(v);
        else
            input.value = String(value);
    });
    return h("div", { class: "field" }, input, h("span", null, suffix));
}
export const settingsPage = {
    id: "settings",
    render(root) {
        const s = store.settings;
        const info = store.info;
        root.append(h("div", { class: "page-head" }, h("div", { class: "titles" }, h("h1", null, "Settings"), h("p", { class: "subtitle" }, "Choose how careful Jigglypuff should be."))));
        if (!s || !info)
            return;
        const theme = h("div", { class: "seg" }, ["system", "light", "dark"].map((t) => h("button", { type: "button", class: s.theme === t ? "on" : "", onclick: () => save({ theme: t }, true) }, t[0].toUpperCase() + t.slice(1))));
        root.append(h("div", { class: "section-title" }, "Appearance"), h("div", { class: "card" }, row("Theme", "System follows your Windows setting.", theme), row("Colors, shapes and effects", "Pick one of 10 themes or change any color, button shape and effect.", btn("Customize", { size: "sm", icon: "palette", onclick: () => go("appearance") }))));
        root.append(h("div", { class: "section-title" }, "Deleting"), h("div", { class: "card" }, row("Send personal files to the Recycle Bin", "Leftovers, large files and duplicates can be restored from the Recycle Bin. Junk and cache files are always deleted, because apps recreate them.", toggle(s.recycleUserFiles, (v) => save({ recycleUserFiles: v }), "Send personal files to the Recycle Bin")), row("Keep recent temporary files", "Temporary files changed within this time are skipped, in case an app or installer still needs them.", numberInput(s.tempMinAgeHours, 0, 720, (v) => save({ tempMinAgeHours: v }), "hours")), row("Create a restore point before uninstalling", "Lets you roll Windows back if an uninstaller misbehaves. Windows allows one every 24 hours.", toggle(s.restorePoint, (v) => save({ restorePoint: v }), "Create a restore point before uninstalling"))));
        const ex = s.exclusions ?? [];
        root.append(h("div", { class: "section-title" }, "Protected folders"), h("div", { class: "card" }, h("div", { class: "settings-row" }, h("div", { class: "grow" }, h("div", { class: "label" }, "Never touch these folders"), h("div", { class: "help" }, "Jigglypuff never lists or deletes anything inside them. Windows and program folders are always protected.")), btn("Add folder", { size: "sm", icon: "plus", onclick: async () => { const f = await pickFolder("Protect a folder"); if (f && !ex.includes(f))
                save({ exclusions: [...ex, f] }); } })), ex.map((f) => h("div", { class: "settings-row", style: "padding-top:10px;padding-bottom:10px" }, icon("lock"), h("div", { class: "grow path selectable", title: f }, shortPath(f, 90)), btn("Remove", { kind: "ghost", size: "sm", onclick: () => save({ exclusions: ex.filter((x) => x !== f) }) })))));
        root.append(h("div", { class: "section-title" }, "Scanning"), h("div", { class: "card" }, row("Large files", "The smallest file the large file finder lists.", numberInput(s.largeMinMB, 10, 1048576, (v) => save({ largeMinMB: v }), "MB")), row("Old downloads", "Installers and archives in Downloads untouched for this long are suggested.", numberInput(s.oldDownloadDays, 1, 3650, (v) => save({ oldDownloadDays: v }), "days")), row("Duplicates", "Smaller files are ignored when looking for duplicates.", numberInput(s.dupMinKB, 1, 4194304, (v) => save({ dupMinKB: v }), "KB"))));
        root.append(h("div", { class: "section-title" }, "Shortcuts"), h("div", { class: "card" }, row("Open Jigglypuff from Start or the desktop", "Jigglypuff is a single file and doesn't install anything. A shortcut makes it easy to find.", h("div", { class: "actions" }, btn("Add to Start", { size: "sm", onclick: () => post("/api/shortcut", { where: "startmenu" }).then(() => toast("Added to the Start menu")).catch(showError) }), btn("Add to desktop", { size: "sm", onclick: () => post("/api/shortcut", { where: "desktop" }).then(() => toast("Shortcut added to your desktop")).catch(showError) })))));
        root.append(h("div", { class: "section-title" }, "About"), h("div", { class: "card" }, row(`Jigglypuff ${info.version}`, info.admin ? "Running as administrator, so system files and apps for all users can be cleaned." : "Running without administrator rights. Some system items are locked.", h("span", { style: "display:inline-flex;gap:6px;align-items:center;color:var(--text-2)" }, icon(info.admin ? "shieldCheck" : "lock"), info.admin ? "Administrator" : "Limited")), row("Jigglypuff's data", "Settings, history and registry backups. Delete this folder to reset Jigglypuff.", btn("Open folder", { size: "sm", icon: "folder", onclick: () => post("/api/launch", { target: "data" }).catch(showError) }))));
    },
};
