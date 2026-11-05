import { get, iconURL, post, StartupItem } from "../api.js";
import { avatarColor, h, icon, shortPath } from "../dom.js";
import { Page, rerender, store } from "../store.js";
import { badge, btn, empty, reviewAndClean, showError, toast, toggle } from "../ui.js";

let loading = false;

async function load(): Promise<void> {
  loading = true;
  rerender("startup");
  try {
    store.startup = await get<StartupItem[]>("/api/startup");
  } catch (e) {
    showError(e);
  } finally {
    loading = false;
    rerender("startup");
  }
}

async function setEnabled(it: StartupItem, on: boolean): Promise<void> {
  try {
    store.startup = await post<StartupItem[]>(`/api/startup/${it.id}`, { enabled: on });
    toast(on ? `${it.name} will start with Windows` : `${it.name} won't start with Windows anymore`);
  } catch (e) {
    showError(e);
  }
  rerender("startup");
}

async function removeBroken(it: StartupItem): Promise<void> {
  const out = await reviewAndClean({ source: "startup", ids: [it.id], title: `Removed startup entry ${it.name}` },
    { title: `Remove “${it.name}” from startup?`, intro: "Its program no longer exists, so this entry does nothing except slow down sign-in a little." });
  if (out) load();
}

function itemIcon(it: StartupItem): HTMLElement {
  const letter = h("div", { class: "app-icon", style: `background:${avatarColor(it.name)}` }, (it.name.trim()[0] ?? "?").toUpperCase());
  if (!it.path || it.missing) return letter;
  const img = h("img", { src: iconURL({ path: it.path }), alt: "", loading: "lazy" });
  const box = h("div", { class: "app-icon" }, img);
  img.addEventListener("error", () => box.replaceWith(letter));
  return box;
}

function row(it: StartupItem): HTMLElement {
  const where = `${it.scope === "machine" ? "All users" : "You"} · ${it.source === "folder" ? "Startup folder" : "Registry"}`;
  return h("div", { class: "row" },
    itemIcon(it),
    h("div", { class: "grow" },
      h("div", { class: "title" }, h("span", { class: "t" }, it.name), it.missing ? badge("Program missing", "warn") : null),
      h("div", { class: "desc", title: it.command }, [it.publisher, where].filter(Boolean).join(" · ")),
      h("div", { class: "desc path mono", title: it.command, style: "font-size:11.5px" }, shortPath(it.command, 96))),
    it.missing ? btn("Remove", { size: "sm", icon: "trash", onclick: () => removeBroken(it) }) : null,
    h("label", { class: "field", style: "min-width:76px;justify-content:flex-end" }, h("span", null, it.enabled ? "On" : "Off"),
      toggle(it.enabled, (v) => setEnabled(it, v), `Start ${it.name} with Windows`)));
}

export const startupPage: Page = {
  id: "startup",
  enter() {
    if (!store.startup && !loading) load();
  },
  render(root) {
    const items = store.startup;
    root.append(h("div", { class: "page-head" },
      h("div", { class: "titles" }, h("h1", null, "Startup apps"),
        h("p", { class: "subtitle" }, "Apps that open when you sign in. Turning one off doesn't uninstall it, and you can turn it back on anytime.")),
      h("div", { class: "actions" }, btn("", { kind: "ghost", iconOnly: true, icon: "refresh", title: "Refresh", onclick: load }))));
    if (!items) {
      root.append(h("div", { class: "card" }, Array.from({ length: 5 }, () => h("div", { class: "skeleton" }))));
      return;
    }
    if (!items.length) {
      root.append(h("div", { class: "card" }, empty("power", "Nothing starts with Windows", "No startup apps were found.")));
      return;
    }
    const on = items.filter((i) => i.enabled).length;
    root.append(h("div", { class: "toolbar" }, h("span", null, `${on} of ${items.length} turned on`), h("span", { class: "spacer" }),
      h("span", { style: "display:inline-flex;gap:6px;align-items:center" }, icon("info"), "Changes take effect the next time you sign in.")));
    root.append(h("div", { class: "card" }, items.map(row)));
  },
};
