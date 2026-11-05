import { store } from "./store.js";
import { applyLook, FALLBACK } from "./look.js";
export function applyTheme(theme) {
    const root = document.documentElement;
    if (theme === "light" || theme === "dark")
        root.dataset.theme = theme;
    else
        delete root.dataset.theme;
    applyLook(store.look ?? FALLBACK, theme);
}
matchMedia("(prefers-color-scheme: dark)").addEventListener("change", () => applyTheme(store.settings?.theme ?? "system"));
