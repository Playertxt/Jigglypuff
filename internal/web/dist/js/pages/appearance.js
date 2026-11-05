import { post } from "../api.js";
import { h, icon } from "../dom.js";
import { BACKGROUNDS, BUTTON_STYLES, CARD_STYLES, clone, decode, DENSITIES, encode, FONTS, HOVER, isPristine, MOTIONS, normHex, PART_LABELS, PARTS, presetLook, presets, PRESS, previewOn, ROLE_LABELS, ROLES, SHAPE_KINDS, SHAPE_LABELS, shapeCSS, STYLE_LABELS, TEXT_SIZES, contrastText, mix, paletteFor, } from "../look.js";
import { rerender, store } from "../store.js";
import { applyTheme } from "../theme.js";
import { btn, confirmDialog, openDialog, showError, toast } from "../ui.js";
const undo = [];
let saveTimer = 0;
let editing = null;
function look() {
    return store.look;
}
function mode() {
    return store.settings?.theme ?? "system";
}
function showingDark() {
    return paletteFor(look(), mode()) === look().dark;
}
function setLook(next, opts = {}) {
    if (opts.record !== false) {
        undo.push(clone(look()));
        if (undo.length > 30)
            undo.shift();
    }
    store.look = next;
    applyTheme(mode());
    clearTimeout(saveTimer);
    saveTimer = window.setTimeout(save, 450);
    if (opts.render !== false)
        rerender("appearance");
}
function edit(fn, opts = {}) {
    const next = clone(look());
    fn(next);
    setLook(next, opts);
}
async function save() {
    if (!store.settings)
        return;
    try {
        store.settings = await post("/api/settings", { ...store.settings, look: store.look });
        if (store.info)
            store.info.settings = store.settings;
    }
    catch (e) {
        showError(e);
    }
}
function themeTile(id, name, desc, l, on, onpick) {
    const p = showingDark() ? l.dark : l.light;
    const tile = h("div", { class: "look-tile" + (on ? " on" : ""), role: "button", tabindex: "0", "aria-pressed": String(on), title: `Use ${name}`,
        onclick: onpick, onkeydown: (e) => { if (e.key === "Enter" || e.key === " ") {
            e.preventDefault();
            onpick();
        } }, "data-id": id }, h("div", { class: "tile-canvas" }, h("div", { class: "tile-nav" }, h("i"), h("i"), h("i"), h("i")), h("div", { class: "card" }, h("div", { class: "line" }), h("div", { class: "line short" }), h("div", { class: "btns" }, h("span", { class: "btn primary" }, "Clean"), h("span", { class: "btn" }, "Review")))), h("div", { class: "tile-meta" }, h("b", null, name, on ? icon("check") : null), h("span", null, desc)));
    return previewOn(tile, l, p);
}
function effectTile(label, on, variant, sample, onpick) {
    const p = paletteFor(variant, mode());
    const tile = h("div", { class: "effect-tile" + (on ? " on" : ""), role: "button", tabindex: "0", "aria-pressed": String(on),
        onclick: onpick, onkeydown: (e) => { if (e.key === "Enter" || e.key === " ") {
            e.preventDefault();
            onpick();
        } } }, sample, h("span", null, label));
    return previewOn(tile, variant, p);
}
function seg(options, value, onpick, label = (v) => STYLE_LABELS[String(v)] ?? String(v)) {
    return h("div", { class: "seg" }, options.map((o) => h("button", { type: "button", class: o === value ? "on" : "", "aria-pressed": String(o === value), onclick: () => onpick(o) }, label(o))));
}
function row(label, help, control) {
    return h("div", { class: "settings-row" }, h("div", { class: "grow" }, h("div", { class: "label" }, label), h("div", { class: "help" }, help)), control);
}
function shapeDemo(kind, radius) {
    const s = shapeCSS({ kind, radius: Math.max(radius, kind === "rounded" || kind === "leaf" ? 6 : 5) });
    const el = h("span", { class: "shape-demo" });
    el.style.borderRadius = s.radius.replace(/(\d+)px/g, (_m, n) => `${Math.min(9, Number(n))}px`);
    el.style.setProperty("corner-shape", s.corner);
    return el;
}
function shapePicker(value, radius, onpick) {
    return h("div", { class: "shape-pick", role: "group" }, SHAPE_KINDS.map((k) => h("button", { type: "button", class: k === value ? "on" : "", title: SHAPE_LABELS[k], "aria-label": SHAPE_LABELS[k], "aria-pressed": String(k === value), onclick: () => onpick(k) }, shapeDemo(k, radius))));
}
function radiusSlider(value, disabled, oninput, onchange) {
    const out = h("output", null, `${value}px`);
    const input = h("input", { type: "range", min: "0", max: "32", step: "1", value: String(value), "aria-label": "Corner size" });
    input.addEventListener("input", () => { out.textContent = `${input.value}px`; oninput(Number(input.value)); });
    input.addEventListener("change", () => onchange(Number(input.value)));
    return h("label", { class: "radius" + (disabled ? " off" : ""), title: disabled ? "This shape has no corner size" : "Corner size" }, input, out);
}
const ACCENTS = [
    ["#256abf", "#3f8fe8"], ["#b8246a", "#e0569b"], ["#d946ef", "#8b5cf6"], ["#7c3aed", "#2563eb"], ["#0a6fd8", "#00b3b8"],
    ["#0f9d8a", "#22c55e"], ["#2f7d4f", "#8fbf4d"], ["#e8502f", "#e83e8c"], ["#e09a00", "#ef4444"], ["#334155", "#64748b"],
];
function colorRow(role, pal, base, which) {
    const value = pal[role];
    const changed = value !== base[role];
    const swatch = h("input", { type: "color", value, "aria-label": ROLE_LABELS[role][0] });
    const hex = h("input", { class: "input hex", value, spellcheck: "false", maxlength: "7", "aria-label": `${ROLE_LABELS[role][0]} hex code` });
    const live = (v) => edit((l) => { l[which][role] = v; }, { record: false, render: false });
    let recorded = false;
    const record = () => {
        if (!recorded) {
            undo.push(clone(look()));
            recorded = true;
        }
    };
    swatch.addEventListener("input", () => { record(); hex.value = swatch.value; live(swatch.value); });
    swatch.addEventListener("change", () => { recorded = false; rerender("appearance"); });
    hex.addEventListener("change", () => {
        const v = normHex(hex.value);
        if (!v) {
            hex.value = value;
            toast("Use a color code like #ff66aa", "err");
            return;
        }
        edit((l) => { l[which][role] = v; });
    });
    return h("div", { class: "color-row" }, swatch, h("div", { class: "grow" }, h("div", { class: "label" }, ROLE_LABELS[role][0], changed ? h("span", { class: "changed-dot", title: "Changed from the theme" }) : null), h("div", { class: "help" }, ROLE_LABELS[role][1])), hex, changed ? btn("", { kind: "ghost", size: "sm", icon: "undo", iconOnly: true, title: "Back to the theme's color", onclick: () => edit((l) => { l[which][role] = base[role]; }) })
        : h("span", { style: "width:28px;flex:none" }));
}
function pasteDialog() {
    const d = openDialog({ label: "Paste a theme code" });
    const ta = h("textarea", { class: "input", placeholder: "JIGGLYPUFF-LOOK:…", spellcheck: "false", "aria-label": "Theme code" });
    const apply = () => {
        const l = decode(ta.value, look());
        if (!l) {
            toast("That isn't a Jigglypuff theme code", "err");
            return;
        }
        d.close();
        setLook(l);
        toast(`Using “${l.name}”`);
    };
    d.set([h("h3", null, "Paste a theme code"), h("p", null, "Paste a code copied from Jigglypuff on another PC or on your phone (More › Appearance › Copy theme code)."), ta], [btn("Cancel", { onclick: () => d.close() }), btn("Use this theme", { kind: "primary", onclick: apply })]);
    navigator.clipboard?.readText().then((t) => { if (!ta.value && t.trim().toUpperCase().startsWith("JIGGLYPUFF-LOOK:"))
        ta.value = t.trim(); }).catch(() => { });
    ta.focus();
}
async function copyCode() {
    try {
        await navigator.clipboard.writeText(encode(look()));
        toast("Theme code copied. Paste it in Jigglypuff on another device.");
    }
    catch {
        const d = openDialog({ label: "Theme code" });
        const ta = h("textarea", { class: "input", readonly: true }, encode(look()));
        d.set([h("h3", null, "Theme code"), h("p", null, "Copy this code and paste it in Jigglypuff on another device."), ta], [btn("Done", { kind: "primary", onclick: () => d.close() })]);
        ta.select();
    }
}
export const appearancePage = {
    id: "appearance",
    render(root) {
        if (!store.look || !store.settings)
            return;
        const l = look();
        const pristine = isPristine(l);
        const which = editing ?? (showingDark() ? "dark" : "light");
        const base = presetLook(l.preset);
        root.append(h("div", { class: "page-head" }, h("div", { class: "titles" }, h("h1", null, "Appearance"), h("p", { class: "subtitle" }, "Pick a theme, then change any color, shape or button effect. Changes show up right away.")), h("div", { class: "actions" }, btn("Undo", { kind: "ghost", icon: "undo", disabled: !undo.length, onclick: () => { const prev = undo.pop(); if (prev)
                setLook(prev, { record: false }); } }), btn("Paste code", { icon: "download", onclick: pasteDialog }), btn("Copy code", { icon: "copy", onclick: copyCode }))));
        root.append(h("div", { class: "section-title" }, "Themes"), h("div", { class: "look-grid" }, presets.map((p) => {
            const pl = presetLook(p.id);
            const on = l.preset === p.id && pristine;
            return themeTile(p.id, p.name, p.desc, pl, on, () => { if (!on)
                setLook(pl); });
        }), !pristine ? themeTile("custom", "Your theme", `Based on ${presets.find((p) => p.id === l.preset)?.name ?? "a theme"}, with your changes.`, l, true, () => { }) : null));
        if (!pristine) {
            root.append(h("div", { class: "toolbar", style: "margin-top:10px" }, h("span", null, "You've changed this theme. Picking another theme replaces your changes; Undo brings them back."), h("span", { class: "spacer" }), btn(`Reset to ${base.name}`, { size: "sm", onclick: async () => {
                    if (await confirmDialog("Reset your changes?", `All colors, shapes and effects go back to ${base.name}. You can undo this.`, "Reset"))
                        setLook(presetLook(l.preset));
                } })));
        }
        root.append(h("div", { class: "section-title" }, "Light and dark"), h("div", { class: "card" }, row("Mode", "Every theme has a light and a dark version. System follows your Windows setting.", seg(["system", "light", "dark"], store.settings.theme, async (t) => {
            try {
                store.settings = await post("/api/settings", { ...store.settings, theme: t, look: store.look });
                if (store.info)
                    store.info.settings = store.settings;
                editing = null;
                applyTheme(t);
            }
            catch (e) {
                showError(e);
            }
            rerender("appearance");
        }, (t) => t[0].toUpperCase() + t.slice(1)))));
        const variant = (fn) => { const v = clone(l); fn(v); return v; };
        const sampleBtn = (text) => h("span", { class: "btn primary" }, text);
        root.append(h("div", { class: "section-title" }, "Buttons"), h("div", { class: "card" }, h("div", { class: "settings-sub" }, "Style", h("span", null, "How main buttons like Clean and Remove look. Other buttons follow along.")), h("div", { class: "effect-grid" }, BUTTON_STYLES.map((s) => effectTile(STYLE_LABELS[s], l.button.style === s, variant((x) => { x.button.style = s; }), sampleBtn("Clean"), () => edit((x) => { x.button.style = s; })))), h("div", { class: "settings-sub" }, "When pressed", h("span", null, "Click a sample to feel it. Applies to buttons, tabs and the sidebar.")), h("div", { class: "effect-grid" }, PRESS.map((s) => effectTile(STYLE_LABELS[s], l.button.press === s, variant((x) => { x.button.press = s; }), sampleBtn("Press me"), () => edit((x) => { x.button.press = s; })))), h("div", { class: "settings-sub" }, "When the pointer is over it", h("span", null, "Point at a sample to see it.")), h("div", { class: "effect-grid" }, HOVER.map((s) => effectTile(STYLE_LABELS[s], l.button.hover === s, variant((x) => { x.button.hover = s; }), sampleBtn("Point at me"), () => edit((x) => { x.button.hover = s; })))), h("div", { style: "border-top:1px solid var(--border)" }, row("Border", "How thick button outlines are.", seg([0, 1, 2, 3], l.button.border, (v) => edit((x) => { x.button.border = v; }), (v) => `${v} px`)), row("Shadow", "A shadow under main buttons.", seg([0, 1, 2, 3], l.button.shadow, (v) => edit((x) => { x.button.shadow = v; }), (v) => ["None", "Soft", "Medium", "Strong"][v])))));
        const kinds = new Set(PARTS.map((p) => l.shapes[p].kind));
        const allSame = kinds.size === 1 ? [...kinds][0] : null;
        root.append(h("div", { class: "section-title" }, "Shapes"), h("div", { class: "card" }, h("div", { class: "shape-row" }, h("div", { class: "grow" }, h("div", { class: "label" }, "Everything"), h("div", { class: "help" }, "Give every part the same shape in one go.")), shapePicker(allSame ?? "", 8, (k) => edit((x) => { for (const p of PARTS)
            x.shapes[p].kind = k; }))), PARTS.map((part) => {
            const s = l.shapes[part];
            const noRadius = s.kind === "square" || s.kind === "pill";
            return h("div", { class: "shape-row" }, h("div", { class: "grow" }, h("div", { class: "label" }, PART_LABELS[part][0]), h("div", { class: "help" }, PART_LABELS[part][1])), shapePicker(s.kind, s.radius, (k) => edit((x) => { x.shapes[part].kind = k; })), radiusSlider(s.radius, noRadius, (v) => edit((x) => { x.shapes[part].radius = v; }, { record: false, render: false }), (v) => { undo.push(variant((x) => { x.shapes[part].radius = s.radius; })); edit((x) => { x.shapes[part].radius = v; }, { record: false }); }));
        })));
        const miniCard = () => h("div", { class: "card" });
        root.append(h("div", { class: "section-title" }, "Cards and panels"), h("div", { class: "card" }, h("div", { class: "effect-grid" }, CARD_STYLES.map((s) => effectTile(STYLE_LABELS[s], l.card.style === s, variant((x) => { x.card.style = s; }), miniCard(), () => edit((x) => { x.card.style = s; })))), h("div", { style: "border-top:1px solid var(--border)" }, row("Shadow", "Glow uses your accent color.", seg([0, 1, 2, 3], l.card.shadow, (v) => edit((x) => { x.card.shadow = v; }), (v) => ["None", "Soft", "Deep", "Glow"][v])))));
        const pal = l[which];
        root.append(h("div", { class: "section-title" }, "Colors"), h("div", { class: "card" }, row("Editing", "Light and dark mode each have their own colors.", seg(["light", "dark"], which, (v) => { editing = v; rerender("appearance"); }, (v) => (v === "light" ? "Light colors" : "Dark colors"))), h("div", { class: "settings-row" }, h("div", { class: "grow" }, h("div", { class: "label" }, "Quick accent"), h("div", { class: "help" }, "Sets the accent and second accent together.")), h("div", { class: "swatches" }, ACCENTS.map(([a, a2]) => {
            const shown = which === "dark" ? mix(a, "#ffffff", 0.18) : a;
            const b = h("button", { type: "button", class: pal.accent === shown ? "on" : "", title: shown, "aria-label": `Accent ${shown}`,
                onclick: () => edit((x) => { x[which].accent = shown; x[which].accent2 = which === "dark" ? mix(a2, "#ffffff", 0.18) : a2; x[which].onAccent = contrastText(shown); }) });
            b.style.background = `linear-gradient(135deg, ${shown} 50%, ${which === "dark" ? mix(a2, "#ffffff", 0.18) : a2} 50%)`;
            return b;
        }))), h("div", { class: "color-grid" }, ROLES.map((r) => colorRow(r, pal, base[which], which)))));
        root.append(h("div", { class: "section-title" }, "Text and layout"), h("div", { class: "card" }, row("Font", "Condensed uses Bahnschrift, which comes with Windows.", seg(FONTS, l.font, (v) => edit((x) => { x.font = v; }))), row("Text size", "Makes everything bigger or smaller.", seg(TEXT_SIZES, l.textSize, (v) => edit((x) => { x.textSize = v; }), (v) => `${v}%`)), row("Spacing", "How much room rows and buttons get.", seg(DENSITIES, l.density, (v) => edit((x) => { x.density = v; }))), row("Animations", "Off turns every animation off, including button effects.", seg(MOTIONS, l.motion, (v) => edit((x) => { x.motion = v; }))), row("Background", "A pattern behind the pages.", seg(BACKGROUNDS, l.background, (v) => edit((x) => { x.background = v; })))));
        root.append(h("div", { class: "section-title" }, "Share"), h("div", { class: "card" }, row("Use this theme somewhere else", "A theme code holds every color, shape and effect. Paste it in Jigglypuff on another PC or in the Android app.", h("div", { class: "actions" }, btn("Copy code", { size: "sm", icon: "copy", onclick: copyCode }), btn("Paste code", { size: "sm", icon: "download", onclick: pasteDialog })))));
    },
};
