// Looks: themes the user can change down to every color, shape and button effect.
// The format is shared with the Android app (shared/looks/presets.json), so a theme code works on both.

export const SHAPE_KINDS = ["square", "rounded", "pill", "bevel", "squircle", "scoop", "notch", "leaf"] as const;
export type ShapeKind = (typeof SHAPE_KINDS)[number];
export interface Shape { kind: ShapeKind; radius: number }

export const PARTS = ["button", "card", "input", "chip", "nav", "dialog", "icon"] as const;
export type Part = (typeof PARTS)[number];

export const ROLES = ["bg", "surface", "surface2", "border", "text", "text2", "muted", "accent", "onAccent", "accent2",
  "danger", "onDanger", "good", "warning", "nav", "navActive"] as const;
export type Role = (typeof ROLES)[number];
export type Palette = Record<Role, string>;

export const BUTTON_STYLES = ["filled", "soft", "outline", "gradient", "glass", "neon", "raised", "pop", "flat"] as const;
export const PRESS = ["none", "shrink", "sink", "bounce", "jelly", "glow", "ripple"] as const;
export const HOVER = ["none", "lift", "glow", "brighten", "grow", "shine"] as const;
export const CARD_STYLES = ["solid", "outline", "elevated", "glass", "pop"] as const;
export const FONTS = ["system", "condensed", "serif", "mono"] as const;
export const TEXT_SIZES = [90, 100, 110, 120] as const;
export const DENSITIES = ["compact", "comfortable", "spacious"] as const;
export const MOTIONS = ["off", "calm", "normal", "lively"] as const;
export const BACKGROUNDS = ["plain", "gradient", "dots", "grid"] as const;

export interface Look {
  v: 1;
  preset: string;
  name: string;
  light: Palette;
  dark: Palette;
  shapes: Record<Part, Shape>;
  button: { style: (typeof BUTTON_STYLES)[number]; press: (typeof PRESS)[number]; hover: (typeof HOVER)[number]; border: number; shadow: number };
  card: { style: (typeof CARD_STYLES)[number]; shadow: number };
  font: (typeof FONTS)[number];
  textSize: (typeof TEXT_SIZES)[number];
  density: (typeof DENSITIES)[number];
  motion: (typeof MOTIONS)[number];
  background: (typeof BACKGROUNDS)[number];
}

export interface Preset extends Look { id: string; desc: string }

// Names shown in the editor ---------------------------------------------------------

export const PART_LABELS: Record<Part, [string, string]> = {
  button: ["Buttons", "Every button, from Clean to Cancel."],
  card: ["Cards and panels", "The boxes that hold lists and results."],
  input: ["Text boxes and menus", "Search boxes, number fields and drop-downs."],
  chip: ["Chips, tabs and badges", "Tab switchers, folder chips and labels like Safe."],
  nav: ["Sidebar items", "The sections on the left."],
  dialog: ["Dialogs and menus", "Pop-ups, confirmations and right-click menus."],
  icon: ["Icon tiles", "The squares behind icons and app logos."],
};

export const SHAPE_LABELS: Record<ShapeKind, string> = {
  square: "Square", rounded: "Rounded", pill: "Pill", bevel: "Cut corners", squircle: "Squircle", scoop: "Scooped", notch: "Notched", leaf: "Leaf",
};

export const ROLE_LABELS: Record<Role, [string, string]> = {
  bg: ["Background", "Behind everything"],
  surface: ["Cards", "Panels, lists and dialogs"],
  surface2: ["Raised areas", "Hovered rows, tab bars, chips"],
  border: ["Lines", "Borders and dividers"],
  text: ["Text", "Titles and main text"],
  text2: ["Secondary text", "Descriptions"],
  muted: ["Faint text", "Hints, sizes and dates"],
  accent: ["Accent", "Main buttons, links, ticks"],
  onAccent: ["Text on accent", "Labels on main buttons"],
  accent2: ["Second accent", "Gradients and glows"],
  danger: ["Danger", "Delete and uninstall buttons"],
  onDanger: ["Text on danger", "Labels on delete buttons"],
  good: ["Safe", "Safe badges and done ticks"],
  warning: ["Warning", "Caution badges and low space"],
  nav: ["Sidebar", "Behind the sections list"],
  navActive: ["Selected section", "The highlighted sidebar item"],
};

export const STYLE_LABELS: Record<string, string> = {
  filled: "Filled", soft: "Soft", outline: "Outline", gradient: "Gradient", glass: "Glass", neon: "Neon", raised: "3D", pop: "Pop", flat: "Text only",
  none: "None", shrink: "Shrink", sink: "Push down", bounce: "Bounce", jelly: "Jelly", glow: "Glow", ripple: "Ripple",
  lift: "Lift", brighten: "Brighten", grow: "Grow", shine: "Shine",
  solid: "Solid", elevated: "Floating",
  system: "System", condensed: "Condensed", serif: "Serif", mono: "Monospace",
  compact: "Compact", comfortable: "Comfortable", spacious: "Spacious",
  off: "Off", calm: "Calm", normal: "Normal", lively: "Lively",
  plain: "Plain", dots: "Dots", grid: "Grid",
};

// Colors -------------------------------------------------------------------------------

const HEX = /^#[0-9a-f]{6}$/;

export function normHex(s: unknown): string | null {
  if (typeof s !== "string") return null;
  let v = s.trim().toLowerCase();
  if (!v.startsWith("#")) v = "#" + v;
  if (/^#[0-9a-f]{3}$/.test(v)) v = "#" + v.slice(1).split("").map((c) => c + c).join("");
  return HEX.test(v) ? v : null;
}

function rgb(hex: string): [number, number, number] {
  const n = parseInt(hex.slice(1), 16);
  return [(n >> 16) & 255, (n >> 8) & 255, n & 255];
}

function toHex(r: number, g: number, b: number): string {
  const c = (v: number) => Math.max(0, Math.min(255, Math.round(v))).toString(16).padStart(2, "0");
  return "#" + c(r) + c(g) + c(b);
}

/** a blended toward b by t (0..1), in plain sRGB — the Android app blends the same way. */
export function mix(a: string, b: string, t: number): string {
  const [r1, g1, b1] = rgb(a);
  const [r2, g2, b2] = rgb(b);
  return toHex(r1 + (r2 - r1) * t, g1 + (g2 - g1) * t, b1 + (b2 - b1) * t);
}

export function luminance(hex: string): number {
  const f = (v: number) => {
    const c = v / 255;
    return c <= 0.03928 ? c / 12.92 : ((c + 0.055) / 1.055) ** 2.4;
  };
  const [r, g, b] = rgb(hex);
  return 0.2126 * f(r) + 0.7152 * f(g) + 0.0722 * f(b);
}

export function isDark(hex: string): boolean {
  return luminance(hex) < 0.2;
}

/** Readable text color on top of bg. */
export function contrastText(bg: string): string {
  return luminance(bg) > 0.4 ? "#111111" : "#ffffff";
}

function alpha(hex: string, a: number): string {
  const [r, g, b] = rgb(hex);
  return `rgba(${r}, ${g}, ${b}, ${a})`;
}

/** Fills in any color a palette leaves out, from the four it must have. */
export function completePalette(p: Partial<Record<Role, unknown>>, dark: boolean): Palette {
  const bg = normHex(p.bg) ?? (dark ? "#0f0f0e" : "#f4f4f2");
  const surface = normHex(p.surface) ?? (dark ? mix(bg, "#ffffff", 0.06) : mix(bg, "#ffffff", 0.7));
  const text = normHex(p.text) ?? (dark ? "#f0efec" : "#0b0b0b");
  const accent = normHex(p.accent) ?? (dark ? "#2f74c8" : "#256abf");
  const danger = normHex(p.danger) ?? (dark ? "#e5484d" : "#c73232");
  const out: Palette = {
    bg, surface, text, accent, danger,
    surface2: normHex(p.surface2) ?? mix(surface, text, 0.05),
    border: normHex(p.border) ?? mix(surface, text, 0.11),
    text2: normHex(p.text2) ?? mix(text, surface, 0.32),
    muted: normHex(p.muted) ?? mix(text, surface, 0.5),
    onAccent: normHex(p.onAccent) ?? contrastText(accent),
    accent2: normHex(p.accent2) ?? accent,
    onDanger: normHex(p.onDanger) ?? contrastText(danger),
    good: normHex(p.good) ?? (dark ? "#3fc23f" : "#0ca30c"),
    warning: normHex(p.warning) ?? (dark ? "#fab219" : "#e0a000"),
    nav: normHex(p.nav) ?? bg,
    navActive: normHex(p.navActive) ?? surface,
  };
  return out;
}

// Reading and checking looks ------------------------------------------------------------

const clampInt = (v: unknown, lo: number, hi: number, def: number) =>
  typeof v === "number" && Number.isFinite(v) ? Math.max(lo, Math.min(hi, Math.round(v))) : def;

function pick<T extends readonly (string | number)[]>(list: T, v: unknown, def: T[number]): T[number] {
  return (list as readonly unknown[]).includes(v) ? (v as T[number]) : def;
}

type Raw = Record<string, unknown>;
const obj = (v: unknown): Raw => (v && typeof v === "object" && !Array.isArray(v) ? (v as Raw) : {});

/** Turns anything (a saved look, a pasted code, a preset) into a valid look, using base for what's missing. */
export function sanitize(input: unknown, base: Look): Look {
  const r = obj(input);
  const shapes = obj(r.shapes);
  const b = obj(r.button);
  const c = obj(r.card);
  // A partial palette (like a preset's) derives what it leaves out, rather than mixing in another theme's colors.
  const pal = (key: "light" | "dark") => {
    const given = obj(r[key]);
    return Object.keys(given).length ? completePalette(given, key === "dark") : { ...base[key] };
  };
  const look: Look = {
    v: 1,
    preset: typeof r.preset === "string" ? r.preset.slice(0, 40) : typeof r.id === "string" ? r.id.slice(0, 40) : base.preset,
    name: typeof r.name === "string" && r.name.trim() ? r.name.trim().slice(0, 40) : base.name,
    light: pal("light"),
    dark: pal("dark"),
    shapes: {} as Record<Part, Shape>,
    button: {
      style: pick(BUTTON_STYLES, b.style, base.button.style),
      press: pick(PRESS, b.press, base.button.press),
      hover: pick(HOVER, b.hover, base.button.hover),
      border: clampInt(b.border, 0, 3, base.button.border),
      shadow: clampInt(b.shadow, 0, 3, base.button.shadow),
    },
    card: { style: pick(CARD_STYLES, c.style, base.card.style), shadow: clampInt(c.shadow, 0, 3, base.card.shadow) },
    font: pick(FONTS, r.font, base.font),
    textSize: pick(TEXT_SIZES, r.textSize, base.textSize),
    density: pick(DENSITIES, r.density, base.density),
    motion: pick(MOTIONS, r.motion, base.motion),
    background: pick(BACKGROUNDS, r.background, base.background),
  };
  for (const p of PARTS) {
    const s = obj(shapes[p]);
    look.shapes[p] = { kind: pick(SHAPE_KINDS, s.kind, base.shapes[p].kind), radius: clampInt(s.radius, 0, 32, base.shapes[p].radius) };
  }
  return look;
}

// Presets ------------------------------------------------------------------------------

/** Used if presets.json can't be loaded, and as the base everything else is checked against. */
export const FALLBACK: Look = {
  v: 1, preset: "classic", name: "Classic",
  light: completePalette({ bg: "#f4f4f2", surface: "#fdfdfc", surface2: "#f1f1ee", border: "#e3e3e0", text: "#0b0b0b", text2: "#52514e", muted: "#7d7b75", accent: "#256abf", accent2: "#3f8fe8" }, false),
  dark: completePalette({ bg: "#0f0f0e", surface: "#1a1a19", surface2: "#232321", border: "#2b2b29", text: "#f0efec", text2: "#c3c2b7", muted: "#8f8d86", accent: "#2f74c8", accent2: "#5b9cf0" }, true),
  shapes: {
    button: { kind: "rounded", radius: 8 }, card: { kind: "rounded", radius: 12 }, input: { kind: "rounded", radius: 8 }, chip: { kind: "pill", radius: 12 },
    nav: { kind: "rounded", radius: 8 }, dialog: { kind: "rounded", radius: 14 }, icon: { kind: "rounded", radius: 9 },
  },
  button: { style: "filled", press: "shrink", hover: "brighten", border: 1, shadow: 0 },
  card: { style: "solid", shadow: 1 },
  font: "system", textSize: 100, density: "comfortable", motion: "normal", background: "plain",
};

export let presets: Preset[] = [];

export async function loadPresets(): Promise<void> {
  try {
    const r = await fetch("presets.json", { cache: "no-cache" });
    const j = obj(await r.json());
    const list = Array.isArray(j.presets) ? j.presets : [];
    presets = list.slice(0, 40).map((p: unknown) => {
      const raw = obj(p);
      return { ...sanitize(raw, FALLBACK), id: String(raw.id ?? ""), desc: String(raw.desc ?? "") };
    }).filter((p: Preset) => p.id);
  } catch {
    presets = [];
  }
  if (!presets.length) presets = [{ ...FALLBACK, id: "classic", desc: "Calm grays with a blue accent." }];
}

export function presetLook(id: string): Look {
  const p = presets.find((x) => x.id === id) ?? presets[0];
  const { id: _id, desc: _desc, ...look } = p;
  return clone({ ...look, preset: p.id });
}

export function clone(l: Look): Look {
  return JSON.parse(JSON.stringify(l)) as Look;
}

/** True when a look still matches its preset exactly. */
export function isPristine(l: Look): boolean {
  const p = presets.find((x) => x.id === l.preset);
  if (!p) return false;
  return JSON.stringify(presetLook(p.id)) === JSON.stringify(l);
}

// Theme codes (copy on one device, paste on another) --------------------------------------

const CODE_PREFIX = "JIGGLYPUFF-LOOK:";

export function encode(l: Look): string {
  const bytes = new TextEncoder().encode(JSON.stringify(l));
  let bin = "";
  bytes.forEach((b) => (bin += String.fromCharCode(b)));
  return CODE_PREFIX + btoa(bin).replace(/\+/g, "-").replace(/\//g, "_").replace(/=+$/, "");
}

/** Reads a theme code (or plain JSON). Returns null if it isn't one. */
export function decode(text: string, base: Look): Look | null {
  const t = text.trim();
  if (t.length > 60000) return null;
  try {
    let json: string;
    if (t.toUpperCase().startsWith(CODE_PREFIX)) {
      const b64 = t.slice(CODE_PREFIX.length).replace(/\s+/g, "").replace(/-/g, "+").replace(/_/g, "/");
      const bin = atob(b64 + "===".slice((b64.length + 3) % 4));
      json = new TextDecoder().decode(Uint8Array.from(bin, (c) => c.charCodeAt(0)));
    } else if (t.startsWith("{")) json = t;
    else return null;
    const raw = JSON.parse(json);
    if (!raw || typeof raw !== "object" || (!("light" in raw) && !("shapes" in raw) && !("button" in raw))) return null;
    return sanitize(raw, base);
  } catch {
    return null;
  }
}

// Applying a look -------------------------------------------------------------------------

/** CSS border-radius and corner-shape for a shape. */
export function shapeCSS(s: Shape): { radius: string; corner: string } {
  const r = s.radius;
  switch (s.kind) {
    case "square": return { radius: "0px", corner: "round" };
    case "pill": return { radius: "999px", corner: "round" };
    case "leaf": return { radius: `${r}px 0`, corner: "round" };
    case "squircle": return { radius: `${Math.round(r * 1.6)}px`, corner: "squircle" };
    case "bevel": return { radius: `${r}px`, corner: "bevel" };
    case "scoop": return { radius: `${r}px`, corner: "scoop" };
    case "notch": return { radius: `${r}px`, corner: "notch" };
    default: return { radius: `${r}px`, corner: "round" };
  }
}

/** The CSS variables for a palette and shapes; used for the whole app and for preview tiles. */
export function lookVars(l: Look, p: Palette): Record<string, string> {
  const dark = isDark(p.bg);
  const v: Record<string, string> = {
    "--bg": p.bg, "--surface": p.surface, "--surface-2": p.surface2, "--surface-3": mix(p.surface2, p.text, 0.06),
    "--border": p.border, "--border-strong": mix(p.border, p.text, 0.18),
    "--text": p.text, "--text-2": p.text2, "--muted": p.muted,
    "--accent": p.accent, "--on-accent": p.onAccent, "--accent2": p.accent2,
    "--accent-hover": mix(p.accent, p.text, 0.14),
    "--accent-text": dark ? mix(p.accent, p.text, 0.3) : mix(p.accent, p.text, 0.08),
    "--accent-soft": alpha(p.accent, dark ? 0.18 : 0.12),
    "--accent-track": mix(p.surface, p.accent, dark ? 0.35 : 0.22),
    "--danger": p.danger, "--on-danger": p.onDanger, "--danger-hover": mix(p.danger, p.text, 0.14),
    "--danger-text": dark ? mix(p.danger, p.text, 0.35) : mix(p.danger, p.text, 0.1),
    "--good": p.good, "--good-text": dark ? mix(p.good, p.text, 0.15) : mix(p.good, p.text, 0.35),
    "--warning": p.warning, "--warning-text": dark ? p.warning : mix(p.warning, p.text, 0.45),
    "--critical": p.danger, "--s1": p.accent,
    "--nav": p.nav, "--nav-active": p.navActive, "--nav-fill": p.nav === p.bg ? "transparent" : p.nav,
    "--overlay": dark ? "rgba(0, 0, 0, 0.6)" : alpha(p.text, 0.3),
    "--shadow": dark ? "0 1px 2px rgba(0, 0, 0, 0.3)" : "0 1px 2px rgba(0, 0, 0, 0.04), 0 1px 3px rgba(0, 0, 0, 0.04)",
    "--shadow-lg": dark ? "0 16px 48px rgba(0, 0, 0, 0.55), 0 2px 8px rgba(0, 0, 0, 0.3)" : "0 12px 40px rgba(0, 0, 0, 0.18), 0 2px 8px rgba(0, 0, 0, 0.08)",
    "--btn-bw": `${l.button.border}px`,
    "--motion": String({ off: 0, calm: 1.6, normal: 1, lively: 0.75 }[l.motion]),
    "--dens": String({ compact: 0.82, comfortable: 1, spacious: 1.18 }[l.density]),
  };
  for (const part of PARTS) {
    const s = shapeCSS(l.shapes[part]);
    v[`--r-${part}`] = s.radius;
    v[`--cs-${part}`] = s.corner;
  }
  return v;
}

/** The data-* switches the stylesheet keys effects on. */
export function lookAttrs(l: Look): Record<string, string> {
  return {
    "data-btn": l.button.style, "data-press": l.button.press, "data-hover": l.button.hover, "data-bshadow": String(l.button.shadow),
    "data-card": l.card.style, "data-cshadow": String(l.card.shadow), "data-font": l.font, "data-density": l.density,
    "data-motion": l.motion, "data-bg": l.background,
  };
}

export function paletteFor(l: Look, mode: string): Palette {
  const dark = mode === "dark" || (mode !== "light" && matchMedia("(prefers-color-scheme: dark)").matches);
  return dark ? l.dark : l.light;
}

let scale = 1;

/** The text size as a zoom factor; menus and tooltips divide their positions by it. */
export function uiScale(): number {
  return scale;
}

export function applyLook(l: Look, mode: string): void {
  const root = document.documentElement;
  const p = paletteFor(l, mode);
  for (const [k, v] of Object.entries(lookVars(l, p))) root.style.setProperty(k, v);
  for (const [k, v] of Object.entries(lookAttrs(l))) root.setAttribute(k, v);
  const dark = isDark(p.bg);
  root.style.colorScheme = dark ? "dark" : "light";
  root.dataset.scheme = dark ? "dark" : "light";
  scale = l.textSize / 100;
  document.body.style.zoom = scale === 1 ? "" : String(scale);
  document.querySelector('meta[name="theme-color"]')?.setAttribute("content", p.bg);
}

/** Sets a look's variables on one element, so it previews that look inside the page. */
export function previewOn(el: HTMLElement, l: Look, p: Palette): HTMLElement {
  for (const [k, v] of Object.entries(lookVars(l, p))) el.style.setProperty(k, v);
  for (const [k, v] of Object.entries(lookAttrs(l))) el.setAttribute(k, v);
  return el;
}

// Press effects that need a script: jelly and ripple -----------------------------------------

const PRESSABLE = ".btn, .nav-item, .seg button";

document.addEventListener("pointerdown", (e) => {
  const t = e.target as HTMLElement | null;
  const el = t?.closest<HTMLElement>(PRESSABLE);
  if (!el || (el as HTMLButtonElement).disabled) return;
  const scope = el.closest<HTMLElement>("[data-press]");
  const press = scope?.getAttribute("data-press");
  if (document.documentElement.getAttribute("data-motion") === "off" || matchMedia("(prefers-reduced-motion: reduce)").matches) return;
  if (press === "jelly") {
    el.classList.remove("jelly");
    void el.offsetWidth; // restart the animation
    el.classList.add("jelly");
    const done = () => el.classList.remove("jelly");
    el.addEventListener("animationend", done, { once: true });
    setTimeout(done, 1500); // in case the animation never runs (hidden window)
  } else if (press === "ripple") {
    const r = el.getBoundingClientRect();
    const size = Math.max(r.width, r.height) * 2.2;
    const z = uiScale();
    const dot = document.createElement("span");
    dot.className = "ripple";
    dot.style.width = dot.style.height = `${size / z}px`;
    dot.style.left = `${(e.clientX - r.left) / z - size / z / 2}px`;
    dot.style.top = `${(e.clientY - r.top) / z - size / z / 2}px`;
    el.appendChild(dot);
    dot.addEventListener("animationend", () => dot.remove(), { once: true });
    setTimeout(() => dot.remove(), 1500);
  }
}, true);
