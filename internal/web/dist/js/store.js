export const store = {
    info: null,
    settings: null,
    smart: null,
    junk: null,
    orphans: null,
    old: null,
    large: null,
    dups: null,
    apps: null,
    storeApps: null,
    startup: null,
    history: null,
    drive: "",
    software: null,
    look: null,
};
const driveListeners = [];
export function onDriveChange(fn) {
    driveListeners.push(fn);
}
export function setDrive(d) {
    if (d === store.drive)
        return;
    store.drive = d;
    store.smart = store.junk = store.orphans = store.old = store.large = store.dups = store.software = null;
    for (const fn of driveListeners)
        fn(d);
    rerender();
}
export function currentDrive() {
    return store.info?.drives.find((x) => x.root === store.drive) ?? null;
}
export function folderLabel(path) {
    const d = store.info?.drives.find((x) => x.path.toLowerCase() === path.toLowerCase() || x.root.toLowerCase() === path.toLowerCase());
    if (d)
        return `${d.label || "Local Disk"} (${driveName(d.root)})`;
    const parts = path.split(/[\\/]/).filter(Boolean);
    return parts[parts.length - 1] ?? path;
}
export function driveName(root) {
    return root.replace(/\\$/, "");
}
const pages = new Map();
let current = "overview";
let root = null;
const navListeners = [];
export function registerPage(p) {
    pages.set(p.id, p);
}
export function setRoot(el) {
    root = el;
}
export function currentPage() {
    return current;
}
export function onNavigate(fn) {
    navListeners.push(fn);
}
export function go(id, arg) {
    const [pid, sub] = id.split("/");
    current = pid;
    pages.get(pid)?.enter?.(arg ?? sub);
    if (root)
        root.scrollTop = 0;
    rerender();
    for (const fn of navListeners)
        fn(pid);
}
export function rerender(id) {
    if (id && id !== current)
        return;
    const p = pages.get(current);
    if (!p || !root)
        return;
    const top = root.scrollTop;
    const page = document.createElement("div");
    page.className = "page";
    p.render(page);
    root.replaceChildren(page);
    root.scrollTop = top;
}
