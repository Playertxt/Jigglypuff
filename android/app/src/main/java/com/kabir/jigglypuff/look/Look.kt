package com.kabir.jigglypuff.look

import android.content.Context
import org.json.JSONObject
import kotlin.math.max
import kotlin.math.min
import kotlin.math.pow
import kotlin.math.roundToInt

/*
 * Looks: themes the user can change down to every color, shape and button effect.
 * The format is shared with the Windows app (shared/looks/presets.json), so a theme code works on both.
 * The color math matches web/src/look.ts exactly.
 */

val SHAPE_KINDS = listOf("square", "rounded", "pill", "bevel", "squircle", "scoop", "notch", "leaf")
val PARTS = listOf("button", "card", "input", "chip", "nav", "dialog", "icon")
val ROLES = listOf("bg", "surface", "surface2", "border", "text", "text2", "muted", "accent", "onAccent", "accent2",
    "danger", "onDanger", "good", "warning", "nav", "navActive")
val BUTTON_STYLES = listOf("filled", "soft", "outline", "gradient", "glass", "neon", "raised", "pop", "flat")
val PRESS = listOf("none", "shrink", "sink", "bounce", "jelly", "glow", "ripple")
val HOVER = listOf("none", "lift", "glow", "brighten", "grow", "shine")
val CARD_STYLES = listOf("solid", "outline", "elevated", "glass", "pop")
val FONTS = listOf("system", "condensed", "serif", "mono")
val TEXT_SIZES = listOf(90, 100, 110, 120)
val DENSITIES = listOf("compact", "comfortable", "spacious")
val MOTIONS = listOf("off", "calm", "normal", "lively")
val BACKGROUNDS = listOf("plain", "gradient", "dots", "grid")

data class LShape(val kind: String, val radius: Int)

data class Look(
    val preset: String,
    val name: String,
    val light: Map<String, String>,
    val dark: Map<String, String>,
    val shapes: Map<String, LShape>,
    val buttonStyle: String,
    val press: String,
    val hover: String,
    val buttonBorder: Int,
    val buttonShadow: Int,
    val cardStyle: String,
    val cardShadow: Int,
    val font: String,
    val textSize: Int,
    val density: String,
    val motion: String,
    val background: String,
) {
    fun shape(part: String): LShape = shapes[part] ?: LShape("rounded", 12)

    fun json(): JSONObject = JSONObject()
        .put("v", 1).put("preset", preset).put("name", name)
        .put("light", JSONObject(light.toSortedRoles())).put("dark", JSONObject(dark.toSortedRoles()))
        .put("shapes", JSONObject().apply { for (p in PARTS) put(p, JSONObject().put("kind", shape(p).kind).put("radius", shape(p).radius)) })
        .put("button", JSONObject().put("style", buttonStyle).put("press", press).put("hover", hover).put("border", buttonBorder).put("shadow", buttonShadow))
        .put("card", JSONObject().put("style", cardStyle).put("shadow", cardShadow))
        .put("font", font).put("textSize", textSize).put("density", density).put("motion", motion).put("background", background)

    private fun Map<String, String>.toSortedRoles(): Map<String, String> = LinkedHashMap<String, String>().also { m -> for (r in ROLES) this[r]?.let { m[r] = it } }
}

data class Preset(val id: String, val desc: String, val look: Look)

object Looks {
    // Colors -----------------------------------------------------------------------------

    private val HEX = Regex("^#[0-9a-f]{6}$")

    fun normHex(s: Any?): String? {
        if (s !is String) return null
        var v = s.trim().lowercase()
        if (!v.startsWith("#")) v = "#$v"
        if (Regex("^#[0-9a-f]{3}$").matches(v)) v = "#" + v.drop(1).map { "$it$it" }.joinToString("")
        return if (HEX.matches(v)) v else null
    }

    private fun rgb(hex: String): IntArray {
        val n = hex.drop(1).toInt(16)
        return intArrayOf((n shr 16) and 255, (n shr 8) and 255, n and 255)
    }

    private fun toHex(r: Double, g: Double, b: Double): String {
        fun c(v: Double) = min(255, max(0, jsRound(v))).toString(16).padStart(2, '0')
        return "#" + c(r) + c(g) + c(b)
    }

    /** JavaScript's Math.round, so both apps blend to the same colors. */
    private fun jsRound(v: Double): Int = kotlin.math.floor(v + 0.5).toInt()

    /** a blended toward b by t (0..1), in plain sRGB. */
    fun mix(a: String, b: String, t: Double): String {
        val x = rgb(a)
        val y = rgb(b)
        return toHex(x[0] + (y[0] - x[0]) * t, x[1] + (y[1] - x[1]) * t, x[2] + (y[2] - x[2]) * t)
    }

    fun luminance(hex: String): Double {
        fun f(v: Int): Double {
            val c = v / 255.0
            return if (c <= 0.03928) c / 12.92 else ((c + 0.055) / 1.055).pow(2.4)
        }
        val c = rgb(hex)
        return 0.2126 * f(c[0]) + 0.7152 * f(c[1]) + 0.0722 * f(c[2])
    }

    fun isDark(hex: String) = luminance(hex) < 0.2

    fun contrastText(bg: String) = if (luminance(bg) > 0.4) "#111111" else "#ffffff"

    /** Fills in any color a palette leaves out, from the four it must have. */
    fun completePalette(p: Map<String, Any?>, dark: Boolean): Map<String, String> {
        val bg = normHex(p["bg"]) ?: if (dark) "#0f0f0e" else "#f4f4f2"
        val surface = normHex(p["surface"]) ?: if (dark) mix(bg, "#ffffff", 0.06) else mix(bg, "#ffffff", 0.7)
        val text = normHex(p["text"]) ?: if (dark) "#f0efec" else "#0b0b0b"
        val accent = normHex(p["accent"]) ?: if (dark) "#2f74c8" else "#256abf"
        val danger = normHex(p["danger"]) ?: if (dark) "#e5484d" else "#c73232"
        return mapOf(
            "bg" to bg, "surface" to surface,
            "surface2" to (normHex(p["surface2"]) ?: mix(surface, text, 0.05)),
            "border" to (normHex(p["border"]) ?: mix(surface, text, 0.11)),
            "text" to text,
            "text2" to (normHex(p["text2"]) ?: mix(text, surface, 0.32)),
            "muted" to (normHex(p["muted"]) ?: mix(text, surface, 0.5)),
            "accent" to accent,
            "onAccent" to (normHex(p["onAccent"]) ?: contrastText(accent)),
            "accent2" to (normHex(p["accent2"]) ?: accent),
            "danger" to danger,
            "onDanger" to (normHex(p["onDanger"]) ?: contrastText(danger)),
            "good" to (normHex(p["good"]) ?: if (dark) "#3fc23f" else "#0ca30c"),
            "warning" to (normHex(p["warning"]) ?: if (dark) "#fab219" else "#e0a000"),
            "nav" to (normHex(p["nav"]) ?: bg),
            "navActive" to (normHex(p["navActive"]) ?: surface),
        )
    }

    // Reading and checking looks --------------------------------------------------------------

    private fun JSONObject.map(): Map<String, Any?> = keys().asSequence().associateWith { opt(it) }

    private fun clampInt(v: Any?, lo: Int, hi: Int, def: Int): Int =
        if (v is Number && v.toDouble().isFinite()) min(hi, max(lo, jsRound(v.toDouble()))) else def

    private fun <T> pick(list: List<T>, v: Any?, def: T): T {
        @Suppress("UNCHECKED_CAST")
        val x = if (v is Number && list.firstOrNull() is Int) v.toInt() as T else v as? T
        return if (x != null && x in list) x else def
    }

    /** Turns anything (a saved look, a pasted code, a preset) into a valid look, using base for what's missing. */
    fun sanitize(r: JSONObject, base: Look): Look {
        val shapes = r.optJSONObject("shapes") ?: JSONObject()
        val b = r.optJSONObject("button") ?: JSONObject()
        val c = r.optJSONObject("card") ?: JSONObject()
        fun pal(key: String): Map<String, String> {
            val given = r.optJSONObject(key)
            return if (given != null && given.length() > 0) completePalette(given.map(), key == "dark") else if (key == "dark") base.dark else base.light
        }
        val name = r.optString("name").trim().take(40)
        val preset = (r.opt("preset") as? String ?: r.opt("id") as? String)?.take(40)
        return Look(
            preset = preset ?: base.preset,
            name = name.ifEmpty { base.name },
            light = pal("light"),
            dark = pal("dark"),
            shapes = PARTS.associateWith { p ->
                val s = shapes.optJSONObject(p) ?: JSONObject()
                LShape(pick(SHAPE_KINDS, s.opt("kind"), base.shape(p).kind), clampInt(s.opt("radius"), 0, 32, base.shape(p).radius))
            },
            buttonStyle = pick(BUTTON_STYLES, b.opt("style"), base.buttonStyle),
            press = pick(PRESS, b.opt("press"), base.press),
            hover = pick(HOVER, b.opt("hover"), base.hover),
            buttonBorder = clampInt(b.opt("border"), 0, 3, base.buttonBorder),
            buttonShadow = clampInt(b.opt("shadow"), 0, 3, base.buttonShadow),
            cardStyle = pick(CARD_STYLES, c.opt("style"), base.cardStyle),
            cardShadow = clampInt(c.opt("shadow"), 0, 3, base.cardShadow),
            font = pick(FONTS, r.opt("font"), base.font),
            textSize = pick(TEXT_SIZES, r.opt("textSize"), base.textSize),
            density = pick(DENSITIES, r.opt("density"), base.density),
            motion = pick(MOTIONS, r.opt("motion"), base.motion),
            background = pick(BACKGROUNDS, r.opt("background"), base.background),
        )
    }

    // Presets ----------------------------------------------------------------------------------

    /** Used if presets.json can't be read, and as the base everything else is checked against. */
    val FALLBACK = Look(
        preset = "jigglypuff", name = "Jigglypuff",
        light = completePalette(mapOf("bg" to "#fff8f9", "surface" to "#fff0f3", "surface2" to "#f5e4e8", "border" to "#ead6db", "text" to "#22191c",
            "text2" to "#514347", "muted" to "#837377", "accent" to "#b8246a", "accent2" to "#e0569b", "danger" to "#ba1a1a", "good" to "#2e8b57",
            "warning" to "#c77700", "nav" to "#fbeaee", "navActive" to "#ffd9e2"), false),
        dark = completePalette(mapOf("bg" to "#191113", "surface" to "#22191c", "surface2" to "#31282a", "border" to "#3c3235", "text" to "#efdfe2",
            "text2" to "#d5c2c6", "muted" to "#9e8c91", "accent" to "#ffb0cd", "onAccent" to "#640038", "accent2" to "#ff7eb3", "danger" to "#ffb4ab",
            "onDanger" to "#690005", "good" to "#5cc98a", "warning" to "#f0b44c", "nav" to "#261d20", "navActive" to "#5a3f47"), true),
        shapes = mapOf("button" to LShape("pill", 20), "card" to LShape("rounded", 20), "input" to LShape("rounded", 12), "chip" to LShape("pill", 12),
            "nav" to LShape("pill", 16), "dialog" to LShape("rounded", 28), "icon" to LShape("rounded", 12)),
        buttonStyle = "filled", press = "jelly", hover = "lift", buttonBorder = 1, buttonShadow = 0, cardStyle = "solid", cardShadow = 0,
        font = "system", textSize = 100, density = "comfortable", motion = "normal", background = "plain",
    )

    /** The theme a new install starts with: pink, like the icon. */
    const val DEFAULT_PRESET = "jigglypuff"

    @Volatile
    var presets: List<Preset> = listOf(Preset(FALLBACK.preset, "Soft pink and round, like the app icon.", FALLBACK))
        private set

    fun loadPresets(ctx: Context) {
        val list = runCatching {
            val j = JSONObject(ctx.assets.open("presets.json").bufferedReader().use { it.readText() })
            val a = j.getJSONArray("presets")
            (0 until minOf(a.length(), 40)).mapNotNull { i ->
                val o = a.getJSONObject(i)
                val id = o.optString("id")
                if (id.isEmpty()) null else Preset(id, o.optString("desc"), sanitize(o, FALLBACK).copy(preset = id))
            }
        }.getOrDefault(emptyList())
        if (list.isNotEmpty()) presets = list
    }

    fun preset(id: String): Look = (presets.firstOrNull { it.id == id } ?: presets.firstOrNull { it.id == DEFAULT_PRESET } ?: presets.first()).look

    fun isPristine(l: Look): Boolean = presets.any { it.id == l.preset } && preset(l.preset) == l

    // Theme codes ---------------------------------------------------------------------------------

    private const val CODE_PREFIX = "JIGGLYPUFF-LOOK:"

    fun encode(l: Look): String {
        val b64 = java.util.Base64.getUrlEncoder().withoutPadding().encodeToString(l.json().toString().toByteArray(Charsets.UTF_8))
        return CODE_PREFIX + b64
    }

    /** Reads a theme code (or plain JSON). Returns null if it isn't one. */
    fun decode(text: String, base: Look): Look? {
        val t = text.trim()
        if (t.length > 60000) return null
        return runCatching {
            val json = when {
                t.uppercase().startsWith(CODE_PREFIX) -> {
                    val b64 = t.drop(CODE_PREFIX.length).filterNot { it.isWhitespace() }.replace('+', '-').replace('/', '_').trimEnd('=')
                    String(java.util.Base64.getUrlDecoder().decode(b64), Charsets.UTF_8)
                }
                t.startsWith("{") -> t
                else -> return null
            }
            val o = JSONObject(json)
            if (!o.has("light") && !o.has("shapes") && !o.has("button")) null else sanitize(o, base)
        }.getOrNull()
    }

    // Saving ---------------------------------------------------------------------------------------

    fun load(ctx: Context): Look {
        val saved = ctx.getSharedPreferences("look", Context.MODE_PRIVATE).getString("look", null)
        val base = preset(DEFAULT_PRESET)
        return saved?.let { s -> runCatching { sanitize(JSONObject(s), base) }.getOrNull() } ?: base
    }

    fun save(ctx: Context, l: Look) {
        ctx.getSharedPreferences("look", Context.MODE_PRIVATE).edit().putString("look", l.json().toString()).apply()
    }
}

/** Names shown in the editor. */
object LookNames {
    val parts = mapOf(
        "button" to ("Buttons" to "Every button, from Clean to Cancel."),
        "card" to ("Cards and panels" to "The boxes that hold lists and results."),
        "input" to ("Text boxes" to "Where you type, like theme codes."),
        "chip" to ("Chips and badges" to "The storage picker, tabs and labels like Safe."),
        "nav" to ("Bottom bar" to "The highlight behind the selected tab."),
        "dialog" to ("Dialogs" to "Confirmations and pop-ups."),
        "icon" to ("Icon tiles" to "The squares behind icons and app logos."),
    )
    val shapes = mapOf("square" to "Square", "rounded" to "Rounded", "pill" to "Pill", "bevel" to "Cut corners", "squircle" to "Squircle",
        "scoop" to "Scooped", "notch" to "Notched", "leaf" to "Leaf")
    val roles = mapOf(
        "bg" to ("Background" to "Behind everything"),
        "surface" to ("Cards" to "Panels, lists and dialogs"),
        "surface2" to ("Raised areas" to "Lists inside dialogs, tracks"),
        "border" to ("Lines" to "Borders and dividers"),
        "text" to ("Text" to "Titles and main text"),
        "text2" to ("Secondary text" to "Descriptions"),
        "muted" to ("Faint text" to "Hints and outlines"),
        "accent" to ("Accent" to "Main buttons, ticks, switches"),
        "onAccent" to ("Text on accent" to "Labels on main buttons"),
        "accent2" to ("Second accent" to "Gradients and glows"),
        "danger" to ("Danger" to "Remove and delete buttons"),
        "onDanger" to ("Text on danger" to "Labels on remove buttons"),
        "good" to ("Safe" to "Safe badges and done ticks"),
        "warning" to ("Warning" to "Caution badges"),
        "nav" to ("Bottom bar" to "Behind the tabs at the bottom"),
        "navActive" to ("Selected tab" to "The highlight on the open tab"),
    )
    val values = mapOf(
        "filled" to "Filled", "soft" to "Soft", "outline" to "Outline", "gradient" to "Gradient", "glass" to "Glass", "neon" to "Neon", "raised" to "3D",
        "pop" to "Pop", "flat" to "Text only", "none" to "None", "shrink" to "Shrink", "sink" to "Push down", "bounce" to "Bounce", "jelly" to "Jelly",
        "glow" to "Glow", "ripple" to "Ripple", "lift" to "Lift", "brighten" to "Brighten", "grow" to "Grow", "shine" to "Shine",
        "solid" to "Solid", "elevated" to "Floating", "system" to "System", "condensed" to "Condensed", "serif" to "Serif", "mono" to "Mono",
        "compact" to "Compact", "comfortable" to "Comfy", "spacious" to "Roomy", "off" to "Off", "calm" to "Calm", "normal" to "Normal", "lively" to "Lively",
        "plain" to "Plain", "dots" to "Dots", "grid" to "Grid",
    )
    fun of(v: String) = values[v] ?: v.replaceFirstChar { it.uppercase() }
}
