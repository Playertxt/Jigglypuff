import { dispatchJobEvent, eventsURL, get, Info, initKey, Job, post } from "./api.js";
import { clear, h, icon } from "./dom.js";
import { currentPage, go, onNavigate, registerPage, rerender, setRoot, store } from "./store.js";
import { btn, showError } from "./ui.js";
import { applyTheme } from "./theme.js";
import { overviewPage } from "./pages/overview.js";
import { cleanerPage } from "./pages/cleaner.js";
import { appsPage, onAppSizes } from "./pages/apps.js";
import { leftoversPage } from "./pages/leftovers.js";
import { largePage } from "./pages/large.js";
import { duplicatesPage } from "./pages/duplicates.js";
import { startupPage } from "./pages/startup.js";
import { historyPage } from "./pages/history.js";
import { settingsPage } from "./pages/settings.js";

const NAV: Array<{ id: string; label: string; icon: string } | null> = [
  { id: "overview", label: "Overview", icon: "home" },
  { id: "cleaner", label: "Junk cleaner", icon: "broom" },
  null,
  { id: "apps", label: "Apps", icon: "grid" },
  { id: "leftovers", label: "Leftovers", icon: "package" },
  { id: "startup", label: "Startup apps", icon: "power" },
  null,
  { id: "large", label: "Large & old files", icon: "fileBig" },
  { id: "duplicates", label: "Duplicates", icon: "copy" },
];

for (const p of [overviewPage, cleanerPage, appsPage, leftoversPage, largePage, duplicatesPage, startupPage, historyPage, settingsPage]) registerPage(p);

function sidebar(info: Info): HTMLElement {
  const items = new Map<string, HTMLElement>();
  const item = (n: { id: string; label: string; icon: string }) => {
    const b = h("button", { type: "button", class: "nav-item", onclick: () => go(n.id), "data-id": n.id, title: n.label },
      icon(n.icon), h("span", { class: "label" }, n.label));
    items.set(n.id, b);
    return b;
  };
  const mark = h("div", { class: "brand-mark" }, icon("broom"));
  const adminPill = info.admin
    ? h("div", { class: "admin-pill ok", title: "Jigglypuff can clean system files and remove apps for all users" }, icon("shieldCheck"), h("span", null, "Running as administrator"))
    : h("div", { class: "admin-pill limited" },
      h("div", { style: "display:flex;gap:8px;align-items:center" }, icon("lock"), h("span", null, "Limited mode")),
      info.canElevate ? btn("Restart as admin", { size: "sm", onclick: restartAdmin }) : null);
  const el = h("nav", { class: "sidebar", "aria-label": "Sections" },
    h("div", { class: "brand" }, mark, h("span", null, "Jigglypuff")),
    NAV.map((n) => (n ? item(n) : h("div", { class: "nav-gap" }))),
    h("div", { class: "sidebar-foot" },
      item({ id: "history", label: "History", icon: "clock" }),
      item({ id: "settings", label: "Settings", icon: "sliders" }),
      adminPill));
  const mark2 = (id: string) => items.forEach((b, k) => b.classList.toggle("active", k === id));
  onNavigate(mark2);
  mark2(currentPage());
  return el;
}

async function restartAdmin(): Promise<void> {
  try {
    const r = await post<{ url: string }>("/api/restart-admin");
    location.href = r.url;
    setTimeout(() => location.reload(), 300);
  } catch (e) {
    showError(e);
  }
}

function connectEvents(): void {
  let failures = 0;
  let banner: HTMLElement | null = null;
  const es = new EventSource(eventsURL());
  es.addEventListener("hello", () => {
    failures = 0;
    banner?.remove();
    banner = null;
  });
  es.addEventListener("job", (e) => dispatchJobEvent(JSON.parse((e as MessageEvent).data) as Job));
  es.addEventListener("appsizes", (e) => onAppSizes(JSON.parse((e as MessageEvent).data) as Record<string, number>));
  es.onerror = () => {
    failures++;
    if (failures >= 4 && !banner) {
      banner = h("div", { class: "disconnected" }, h("div", { class: "empty" },
        h("div", { class: "ico" }, icon("power")), h("h3", null, "Jigglypuff has stopped"),
        h("p", null, "Close this window and open Jigglypuff again.")));
      document.body.appendChild(banner);
    }
  };
}

async function boot(): Promise<void> {
  const app = document.getElementById("app")!;
  if (!initKey()) {
    clear(app, h("div", { class: "disconnected" }, h("div", { class: "empty" }, h("div", { class: "ico" }, icon("lock")),
      h("h3", null, "Open Jigglypuff from its app"), h("p", null, "This page only works inside the Jigglypuff window."))));
    return;
  }
  let info: Info;
  try {
    info = await get<Info>("/api/info");
  } catch (e) {
    clear(app, h("div", { class: "disconnected" }, h("div", { class: "empty" }, h("div", { class: "ico" }, icon("alert")),
      h("h3", null, "Can't reach Jigglypuff"), h("p", null, String(e instanceof Error ? e.message : e)))));
    return;
  }
  store.info = info;
  store.settings = info.settings;
  applyTheme(info.settings.theme);
  const main = h("main", { class: "main", id: "main" });
  clear(app, h("div", { class: "app" }, sidebar(info), main));
  setRoot(main);
  connectEvents();
  go("overview");
  document.addEventListener("keydown", (e) => {
    if (e.key === "F5" || (e.ctrlKey && e.key.toLowerCase() === "r")) {
      e.preventDefault();
      rerender();
    }
  });
  document.addEventListener("contextmenu", (e) => {
    const t = e.target as HTMLElement;
    if (!t.closest("input, .selectable, .path")) e.preventDefault();
  });
}

boot();
