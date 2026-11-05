import { store } from "./store.js";

const BG = { light: "#f4f4f2", dark: "#0f0f0e" };

export function applyTheme(theme: string): void {
  const root = document.documentElement;
  if (theme === "light" || theme === "dark") root.dataset.theme = theme;
  else delete root.dataset.theme;
  const dark = theme === "dark" || (theme !== "light" && matchMedia("(prefers-color-scheme: dark)").matches);
  document.querySelector('meta[name="theme-color"]')?.setAttribute("content", dark ? BG.dark : BG.light);
}

matchMedia("(prefers-color-scheme: dark)").addEventListener("change", () => applyTheme(store.settings?.theme ?? "system"));

