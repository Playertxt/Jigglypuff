// Talking to Jigglypuff's engine. Every request carries the per-session key.

export interface Progress { phase: string; message: string; done: number; total: number; bytes: number }
export interface Drive { root: string; label: string; total: number; free: number; system: boolean; path: string }
export interface Settings {
  theme: "system" | "light" | "dark"; recycleUserFiles: boolean; tempMinAgeHours: number; restorePoint: boolean;
  exclusions: string[] | null; largeMinMB: number; oldDownloadDays: number; dupMinKB: number; dupFolders: string[] | null;
  largeFolder: string; largeAppData: boolean; look?: unknown;
}
export interface Info {
  version: string; user: string; admin: boolean; kind: string; canElevate: boolean; settings: Settings; drives: Drive[];
  dataDir: string; paths: Record<string, string>; os: string;
}
export interface JunkLocation { path: string; label?: string; size: number; files: number }
export interface JunkResult {
  id: string; name: string; group: string; desc: string; note?: string; safety: string; admin: boolean; default: boolean;
  recycle: boolean; available: boolean; size: number; files: number; skipped: number; running?: string[] | null;
  locations: JunkLocation[] | null; special?: string;
}
export interface Leftover {
  id: string; kind: string; path: string; name?: string; size: number; files: number; confidence: string; reason: string;
  lastUsed: string; hasExe?: boolean;
}
export interface LeftoverGroup { key: string; name: string; items: Leftover[]; size: number; confidence: string; lastUsed: string }
export interface App {
  id: string; kind: string; name: string; publisher: string; version: string; installDate?: string; size: number;
  sizeKnown: boolean; installLocation?: string; scope: string; arch?: string; broken: boolean; canUninstall: boolean; hasIcon: boolean;
  drive?: string;
}
export interface Program { id: string; name: string; kind: "program" | "game"; path: string; drive: string; exe?: string; size: number; partial?: boolean; modified: string }
export interface FileItem { id: string; path: string; name: string; dir: string; size: number; modified: string; type: string }
export interface DupGroup { id: string; size: number; wasted: number; files: FileItem[] }
export interface StartupItem {
  id: string; name: string; entry: string; command: string; path: string; source: string; scope: string; enabled: boolean;
  publisher: string; description: string; missing: boolean;
}
export interface Rec {
  id: string; kind: "clean" | "review" | "info"; icon: string; title: string; desc: string; size: number; count: number;
  selected: boolean; safety: string; page?: string; categories?: string[]; action?: string;
}
export interface SmartResult {
  recs: Rec[]; recommended: number; junkScanId: string; orphanScanId: string; oldScanId: string;
  junk: JunkResult[]; orphans: LeftoverGroup[] | null; old: FileItem[] | null; drives: Drive[]; time: string; drive: string;
}
export interface PlanGroup { label: string; items: number; bytes: number }
export interface PlanPreview { kind: string; path: string; name?: string; size: number; recycle: boolean }
export interface Plan {
  id: string; title: string; source: string; appId?: string;
  summary: { items: number; bytes: number; permanent: number; recycle: number; registry: number; system: number;
    groups: PlanGroup[] | null; notes: string[] | null; preview: PlanPreview[] | null };
}
export interface CleanResult {
  freed: number; removed: number; recycled: number; failed: number; inUse: number; skipped: number;
  errors: string[] | null; backupDir?: string;
}
export interface UninstallResult { status: string; exitCode: number; restorePoint: string; message?: string }
export interface HistoryEntry {
  id: string; time: string; title: string; source: string; freed: number; items: number; recycled: number; failed: number;
  inUse: number; skipped: number; backupDir?: string; details?: string[] | null;
}
export interface Job<T = unknown> { id: string; type: string; state: string; progress: Progress; error?: string; result?: T }
export interface PlanRequest { source: string; scanId?: string; ids: string[]; appId?: string; title?: string }

let KEY = "";

export function initKey(): boolean {
  const m = location.hash.match(/k=([0-9a-f]+)/);
  if (m) {
    KEY = m[1];
    try { sessionStorage.setItem("jigglypuff.k", KEY); } catch { /* storage may be unavailable */ }
    history.replaceState(null, "", location.pathname);
  } else {
    try { KEY = sessionStorage.getItem("jigglypuff.k") ?? ""; } catch { KEY = ""; }
  }
  return KEY !== "";
}

async function call<T>(method: string, path: string, body?: unknown): Promise<T> {
  const res = await fetch(path, {
    method,
    headers: { "X-Jigglypuff-Key": KEY, ...(body !== undefined ? { "Content-Type": "application/json" } : {}) },
    body: body !== undefined ? JSON.stringify(body) : undefined,
  });
  const text = await res.text();
  let data: unknown = null;
  try { data = text ? JSON.parse(text) : null; } catch { data = null; }
  if (!res.ok) {
    const msg = (data as { error?: string } | null)?.error ?? (text || res.statusText);
    throw new Error(msg);
  }
  return data as T;
}

export const get = <T>(path: string) => call<T>("GET", path);
export const post = <T>(path: string, body?: unknown) => call<T>("POST", path, body ?? {});

export function iconURL(q: Record<string, string>): string {
  const p = new URLSearchParams({ k: KEY, ...q });
  return `/api/icon?${p.toString()}`;
}

export function eventsURL(): string {
  return `/api/events?k=${KEY}`;
}

// Jobs ------------------------------------------------------------------------------

type Listener = (j: Job) => void;
const listeners = new Map<string, Listener>();

export function dispatchJobEvent(j: Job): void {
  listeners.get(j.id)?.(j);
}

export interface RunningJob<T> { id: string; done: Promise<Job<T>> }

export async function startJob<T>(path: string, body: unknown, onProgress?: (p: Progress) => void): Promise<RunningJob<T>> {
  const { jobId } = await post<{ jobId: string }>(path, body);
  let finished = false;
  const done = new Promise<Job<T>>((resolve, reject) => {
    const finish = async () => {
      if (finished) return;
      finished = true;
      listeners.delete(jobId);
      clearInterval(poll);
      try {
        resolve(await get<Job<T>>(`/api/jobs/${jobId}`));
      } catch (e) {
        reject(e);
      }
    };
    listeners.set(jobId, (j) => {
      if (j.state === "running") onProgress?.(j.progress);
      else finish();
    });
    const poll = window.setInterval(async () => {
      try {
        const j = await get<Job<T>>(`/api/jobs/${jobId}`);
        if (j.state !== "running") finish();
        else onProgress?.(j.progress);
      } catch { /* keep waiting */ }
    }, 2000);
  });
  return { id: jobId, done };
}

export function cancelJob(id: string): Promise<unknown> {
  return post(`/api/jobs/${id}/cancel`);
}

/** Runs a job and returns its result, throwing on errors. */
export async function runJob<T>(path: string, body: unknown, onProgress?: (p: Progress) => void, onStart?: (id: string) => void): Promise<Job<T>> {
  const j = await startJob<T>(path, body, onProgress);
  onStart?.(j.id);
  const res = await j.done;
  if (res.state === "error") throw new Error(res.error || "Something went wrong");
  return res;
}
