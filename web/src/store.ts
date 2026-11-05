// Shared state and navigation.
import type { App, DupGroup, FileItem, HistoryEntry, Info, JunkResult, LeftoverGroup, Settings, SmartResult, StartupItem } from "./api.js";

export const store = {
  info: null as Info | null,
  settings: null as Settings | null,
  smart: null as SmartResult | null,
  junk: null as { scanId: string; categories: JunkResult[] } | null,
  orphans: null as { scanId: string; groups: LeftoverGroup[] } | null,
  old: null as { scanId: string; files: FileItem[]; days: number } | null,
  large: null as { scanId: string; files: FileItem[]; folder: string } | null,
  dups: null as { scanId: string; groups: DupGroup[]; folders: string[] } | null,
  apps: null as App[] | null,
  storeApps: null as App[] | null,
  startup: null as StartupItem[] | null,
  history: null as HistoryEntry[] | null,
};

export interface Page {
  id: string;
  /** Called when the page is opened (arg comes from links like "large/old"). */
  enter?(arg?: string): void;
  render(root: HTMLElement): void;
}

const pages = new Map<string, Page>();
let current = "overview";
let root: HTMLElement | null = null;
const navListeners: Array<(id: string) => void> = [];

export function registerPage(p: Page): void {
  pages.set(p.id, p);
}

export function setRoot(el: HTMLElement): void {
  root = el;
}

export function currentPage(): string {
  return current;
}

export function onNavigate(fn: (id: string) => void): void {
  navListeners.push(fn);
}

/** Opens a page. "large/old" opens the large-files page on its second tab. */
export function go(id: string, arg?: string): void {
  const [pid, sub] = id.split("/");
  current = pid;
  pages.get(pid)?.enter?.(arg ?? sub);
  if (root) root.scrollTop = 0;
  rerender();
  for (const fn of navListeners) fn(pid);
}

/** Re-renders the visible page, keeping the scroll position. */
export function rerender(id?: string): void {
  if (id && id !== current) return;
  const p = pages.get(current);
  if (!p || !root) return;
  const top = root.scrollTop;
  const page = document.createElement("div");
  page.className = "page";
  p.render(page);
  root.replaceChildren(page);
  root.scrollTop = top;
}
