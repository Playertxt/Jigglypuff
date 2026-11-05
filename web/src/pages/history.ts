import { get, HistoryEntry, post } from "../api.js";
import { fmtBytes, fmtDateTime, h, icon, plural, relTime, shortPath } from "../dom.js";
import { Page, rerender, store } from "../store.js";
import { btn, confirmDialog, empty, showError, toast } from "../ui.js";

let loading = false;
const open = new Set<string>();

async function load(): Promise<void> {
  loading = true;
  try {
    store.history = await get<HistoryEntry[]>("/api/history");
  } catch (e) {
    showError(e);
  } finally {
    loading = false;
    rerender("history");
  }
}

const sourceIcon: Record<string, string> = { junk: "broom", uninstall: "grid", "app-leftovers": "grid", leftovers: "package", large: "fileBig", "old-downloads": "download", dups: "copy", startup: "power", "broken-entry": "grid" };

function entry(e: HistoryEntry): HTMLElement[] {
  const isOpen = open.has(e.id);
  const bits = [e.items ? plural(e.items, "item") : "", e.recycled ? `${e.recycled} in Recycle Bin` : "", e.inUse ? `${e.inUse} in use` : "", e.failed ? `${e.failed} failed` : ""].filter(Boolean);
  const details = e.details ?? [];
  const rows = [h("div", { class: "row" + (details.length || e.backupDir ? " click" : ""), onclick: () => { if (isOpen) open.delete(e.id); else open.add(e.id); rerender("history"); } },
    h("div", { class: "ico" }, icon(sourceIcon[e.source] ?? "check")),
    h("div", { class: "grow" }, h("div", { class: "title" }, h("span", { class: "t" }, e.title)),
      h("div", { class: "desc", title: fmtDateTime(e.time) }, [relTime(e.time), ...bits].join(" · "))),
    e.freed > 0 ? h("div", { class: "size" }, fmtBytes(e.freed)) : null,
    details.length || e.backupDir ? h("span", { class: "chev" + (isOpen ? " open" : "") }, icon("chevRight")) : null)];
  if (isOpen) {
    if (e.backupDir) {
      rows.push(h("div", { class: "row sub" }, icon("undo"), h("div", { class: "grow" }, h("div", { class: "title" }, "Registry backups and removed tasks"),
        h("div", { class: "desc path" }, "Double-click a .reg file in this folder to restore it.")),
        btn("Open backups", { size: "sm", icon: "folder", onclick: () => post("/api/open", { path: e.backupDir }).catch(showError) })));
    }
    for (const d of details.slice(0, 120)) rows.push(h("div", { class: "row sub", style: "min-height:36px" }, h("div", { class: "grow path selectable", title: d }, shortPath(d, 110))));
    if (details.length > 120) rows.push(h("div", { class: "row sub", style: "min-height:36px;color:var(--muted)" }, `…and ${details.length - 120} more`));
  }
  return rows;
}

export const historyPage: Page = {
  id: "history",
  enter() {
    load();
  },
  render(root) {
    const list = store.history;
    root.append(h("div", { class: "page-head" },
      h("div", { class: "titles" }, h("h1", null, "History"), h("p", { class: "subtitle" }, "Everything Jigglypuff has removed, and where the backups are.")),
      h("div", { class: "actions" },
        btn("Open backups", { kind: "ghost", icon: "folder", onclick: () => post("/api/launch", { target: "backups" }).catch(showError) }),
        btn("Clear history", { kind: "ghost", disabled: !list?.length, onclick: async () => {
          if (await confirmDialog("Clear the history?", "This only clears the list. Backups stay in Jigglypuff's backup folder.", "Clear", "danger")) {
            await post("/api/history/clear");
            store.history = [];
            toast("History cleared");
            rerender("history");
          }
        } }))));
    if (!list) {
      root.append(h("div", { class: "card" }, Array.from({ length: 4 }, () => h("div", { class: "skeleton" }))));
      return;
    }
    if (!list.length) {
      root.append(h("div", { class: "card" }, empty("clock", "Nothing cleaned yet", "When you clean something, it shows up here with its backups.")));
      return;
    }
    const freed = list.reduce((a, e) => a + e.freed, 0);
    root.append(h("div", { class: "toolbar" }, `${plural(list.length, "clean-up")} · ${fmtBytes(freed)} freed in total`));
    root.append(h("div", { class: "card" }, list.flatMap(entry)));
    void loading;
  },
};
