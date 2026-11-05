// Shared state and navigation.
import type { App, Drive, DupGroup, FileItem, HistoryEntry, Info, JunkResult, LeftoverGroup, Program, Settings, SmartResult, StartupItem } from "./api.js";

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
  /** The drive every scan looks at (a root like D:\); "" means every drive. */
  drive: "",
  software: null as { drive: string; programs: Program[] } | null,
};

const driveListeners: Array<(d: string) => void> = [];

/** Runs fn whenever a different drive is picked. */
export function onDriveChange(fn: (d: string) => void): void {
  driveListeners.push(fn);
}

/** Picks the drive to scan. Results from the old drive no longer apply, so they're dropped. */
export function setDrive(d: string): void {
  if (d === store.drive) return;
  store.drive = d;
  store.smart = store.junk = store.orphans = store.old = store.large = store.dups = store.software = null;
  for (const fn of driveListeners) fn(d);
  rerender();
}

/** The selected drive, or null for every drive. */
export function currentDrive(): Drive | null {
  return store.info?.drives.find((x) => x.root === store.drive) ?? null;
}

/** "Data (D:)" when path is a whole drive, else the folder's own name. */
export function folderLabel(path: string): string {
  const d = store.info?.drives.find((x) => x.path.toLowerCase() === path.toLowerCase() || x.root.toLowerCase() === path.toLowerCase());
  if (d) return `${d.label || "Local Disk"} (${driveName(d.root)})`;
  const parts = path.split(/[\\/]/).filter(Boolean);
  return parts[parts.length - 1] ?? path;
}

/** "D:" for a drive root like D:\ */
export function driveName(root: string): string {
  return root.replace(/\\$/, "");
}

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
