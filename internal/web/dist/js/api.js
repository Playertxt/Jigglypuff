let KEY = "";
export function initKey() {
    const m = location.hash.match(/k=([0-9a-f]+)/);
    if (m) {
        KEY = m[1];
        try {
            sessionStorage.setItem("jigglypuff.k", KEY);
        }
        catch { }
        history.replaceState(null, "", location.pathname);
    }
    else {
        try {
            KEY = sessionStorage.getItem("jigglypuff.k") ?? "";
        }
        catch {
            KEY = "";
        }
    }
    return KEY !== "";
}
async function call(method, path, body) {
    const res = await fetch(path, {
        method,
        headers: { "X-Jigglypuff-Key": KEY, ...(body !== undefined ? { "Content-Type": "application/json" } : {}) },
        body: body !== undefined ? JSON.stringify(body) : undefined,
    });
    const text = await res.text();
    let data = null;
    try {
        data = text ? JSON.parse(text) : null;
    }
    catch {
        data = null;
    }
    if (!res.ok) {
        const msg = data?.error ?? (text || res.statusText);
        throw new Error(msg);
    }
    return data;
}
export const get = (path) => call("GET", path);
export const post = (path, body) => call("POST", path, body ?? {});
export function iconURL(q) {
    const p = new URLSearchParams({ k: KEY, ...q });
    return `/api/icon?${p.toString()}`;
}
export function eventsURL() {
    return `/api/events?k=${KEY}`;
}
const listeners = new Map();
export function dispatchJobEvent(j) {
    listeners.get(j.id)?.(j);
}
export async function startJob(path, body, onProgress) {
    const { jobId } = await post(path, body);
    let finished = false;
    const done = new Promise((resolve, reject) => {
        const finish = async () => {
            if (finished)
                return;
            finished = true;
            listeners.delete(jobId);
            clearInterval(poll);
            try {
                resolve(await get(`/api/jobs/${jobId}`));
            }
            catch (e) {
                reject(e);
            }
        };
        listeners.set(jobId, (j) => {
            if (j.state === "running")
                onProgress?.(j.progress);
            else
                finish();
        });
        const poll = window.setInterval(async () => {
            try {
                const j = await get(`/api/jobs/${jobId}`);
                if (j.state !== "running")
                    finish();
                else
                    onProgress?.(j.progress);
            }
            catch { }
        }, 2000);
    });
    return { id: jobId, done };
}
export function cancelJob(id) {
    return post(`/api/jobs/${id}/cancel`);
}
export async function runJob(path, body, onProgress, onStart) {
    const j = await startJob(path, body, onProgress);
    onStart?.(j.id);
    const res = await j.done;
    if (res.state === "error")
        throw new Error(res.error || "Something went wrong");
    return res;
}
