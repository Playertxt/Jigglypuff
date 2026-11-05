package com.kabir.jigglypuff.ui

import android.content.ClipData
import android.content.ClipboardManager
import android.content.Intent
import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.clickable
import androidx.compose.foundation.horizontalScroll
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxHeight
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.automirrored.outlined.ArrowBack
import androidx.compose.material.icons.automirrored.outlined.Undo
import androidx.compose.material.icons.outlined.Check
import androidx.compose.material.icons.outlined.ContentCopy
import androidx.compose.material.icons.outlined.ContentPaste
import androidx.compose.material.icons.outlined.Share
import androidx.compose.material3.HorizontalDivider
import androidx.compose.material3.Icon
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.Slider
import androidx.compose.material3.SliderDefaults
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableFloatStateOf
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberCoroutineScope
import androidx.compose.runtime.saveable.rememberSaveable
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.Brush
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.text.font.FontFamily
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import com.kabir.jigglypuff.AppModel
import com.kabir.jigglypuff.look.BACKGROUNDS
import com.kabir.jigglypuff.look.BUTTON_STYLES
import com.kabir.jigglypuff.look.CARD_STYLES
import com.kabir.jigglypuff.look.DENSITIES
import com.kabir.jigglypuff.look.FONTS
import com.kabir.jigglypuff.look.LShape
import com.kabir.jigglypuff.look.Look
import com.kabir.jigglypuff.look.LookNames
import com.kabir.jigglypuff.look.Looks
import com.kabir.jigglypuff.look.MOTIONS
import com.kabir.jigglypuff.look.PARTS
import com.kabir.jigglypuff.look.PRESS
import com.kabir.jigglypuff.look.ROLES
import com.kabir.jigglypuff.look.SHAPE_KINDS
import com.kabir.jigglypuff.look.TEXT_SIZES
import kotlinx.coroutines.launch

private val ACCENTS = listOf(
    "#256abf" to "#3f8fe8", "#b8246a" to "#e0569b", "#d946ef" to "#8b5cf6", "#7c3aed" to "#2563eb", "#0a6fd8" to "#00b3b8",
    "#0f9d8a" to "#22c55e", "#2f7d4f" to "#8fbf4d", "#e8502f" to "#e83e8c", "#e09a00" to "#ef4444", "#334155" to "#64748b",
)

private fun hexColor(h: String) = Color(android.graphics.Color.parseColor(h))

/** Pick a theme, then change any color, shape or button effect. Every change shows (and saves) right away. */
@Composable
fun AppearanceScreen(vm: AppModel, onBack: () -> Unit) {
    val ctx = LocalContext.current
    val snack = LocalSnack.current
    val scope = rememberCoroutineScope()
    val l = vm.look
    val dark = wantsDark(vm.settings.theme)
    var editing by rememberSaveable { mutableStateOf<String?>(null) }
    val which = editing ?: if (dark) "dark" else "light"
    val pristine = Looks.isPristine(l)
    val base = Looks.preset(l.preset)
    var pickRole by remember { mutableStateOf<String?>(null) }
    var pasting by remember { mutableStateOf(false) }
    var askReset by remember { mutableStateOf(false) }
    val c = LocalLook.current.c
    fun set(next: Look) = vm.setLook(next)
    fun pal(role: String, hex: String): Look = if (which == "dark") l.copy(dark = l.dark + (role to hex)) else l.copy(light = l.light + (role to hex))
    val palette = if (which == "dark") l.dark else l.light
    val basePalette = if (which == "dark") base.dark else base.light

    ScreenList {
        item {
            Row(verticalAlignment = Alignment.CenterVertically) {
                LookIconButton(onClick = onBack) { Icon(Icons.AutoMirrored.Outlined.ArrowBack, "Back") }
                Text("Appearance", style = MaterialTheme.typography.titleLarge, fontWeight = FontWeight.SemiBold, modifier = Modifier.weight(1f).padding(start = 4.dp))
                TextLinkButton(onClick = vm::undoLook, enabled = vm.lookUndoCount > 0) {
                    Icon(Icons.AutoMirrored.Outlined.Undo, null, Modifier.size(18.dp).padding(end = 4.dp))
                    Text("Undo")
                }
            }
        }
        item { Text("Pick a theme, then change any color, shape or button effect. Changes show up right away.", style = MaterialTheme.typography.bodyMedium, color = c.text2) }

        // Themes ---------------------------------------------------------------------------
        item { SectionTitle("Themes", "${Looks.presets.size} to start from") }
        items(Looks.presets.chunked(2)) { row ->
            Row(horizontalArrangement = Arrangement.spacedBy(12.dp)) {
                row.forEach { p ->
                    val on = pristine && l.preset == p.id
                    ThemeTile(p.look, p.look.name, p.desc, on, dark, Modifier.weight(1f)) { if (!on) set(p.look) }
                }
                if (row.size == 1) Spacer(Modifier.weight(1f))
            }
        }
        if (!pristine) item {
            Panel(hero = true) {
                RowItem("Your theme", "Based on ${base.name}, with your changes. Picking a theme replaces them; Undo brings them back.") {
                    SecondaryButton(onClick = { askReset = true }) { Text("Reset") }
                }
            }
        }

        item { SectionTitle("Light and dark") }
        item {
            Panel {
                Column(Modifier.padding(16.dp), verticalArrangement = Arrangement.spacedBy(8.dp)) {
                    LookSeg(listOf("system", "light", "dark"), vm.settings.theme, { it.replaceFirstChar { ch -> ch.uppercase() } }) {
                        editing = null
                        vm.updateSettings(vm.settings.copy(theme = it))
                    }
                    Text("Every theme has a light and a dark version. System follows your phone.", style = MaterialTheme.typography.bodySmall, color = c.text2)
                }
            }
        }

        // Buttons --------------------------------------------------------------------------
        item { SectionTitle("Buttons") }
        item {
            Panel {
                Column(Modifier.padding(16.dp), verticalArrangement = Arrangement.spacedBy(12.dp)) {
                    SubTitle("Style", "How main buttons like Clean and Remove look. Other buttons follow along.")
                    EffectGrid(BUTTON_STYLES, l.buttonStyle, { l.copy(buttonStyle = it) }, dark, ::set) { pick -> PrimaryButton(onClick = pick) { Text("Clean") } }
                    SubTitle("When pressed", "Tap a sample to feel it. Applies to buttons, tabs and cards.")
                    EffectGrid(PRESS, l.press, { l.copy(press = it) }, dark, ::set) { pick -> PrimaryButton(onClick = pick) { Text("Press") } }
                    SubTitle("Border", "How thick button outlines are.")
                    LookSeg(listOf(0, 1, 2, 3), l.buttonBorder, { "$it dp" }) { set(l.copy(buttonBorder = it)) }
                    SubTitle("Shadow", "A shadow under main buttons.")
                    LookSeg(listOf(0, 1, 2, 3), l.buttonShadow, { listOf("None", "Soft", "Medium", "Strong")[it] }) { set(l.copy(buttonShadow = it)) }
                    Text("Hover effects only apply in the PC app. Yours there: ${LookNames.of(l.hover)}.", style = MaterialTheme.typography.bodySmall, color = c.text2)
                }
            }
        }

        // Shapes ---------------------------------------------------------------------------
        item { SectionTitle("Shapes") }
        item {
            Panel {
                val kinds = PARTS.map { l.shape(it).kind }.toSet()
                ShapeRow("Everything", "Give every part the same shape in one go.", kinds.singleOrNull(), 8, null, vm) { k ->
                    set(l.copy(shapes = l.shapes.mapValues { it.value.copy(kind = k) }))
                }
                PARTS.forEach { part ->
                    HorizontalDivider(color = c.border)
                    val s = l.shape(part)
                    val (title, desc) = LookNames.parts[part] ?: (part to "")
                    ShapeRow(title, desc, s.kind, s.radius, { r -> l.copy(shapes = l.shapes + (part to s.copy(radius = r))) }, vm) { k ->
                        set(l.copy(shapes = l.shapes + (part to s.copy(kind = k))))
                    }
                }
            }
        }

        // Cards ----------------------------------------------------------------------------
        item { SectionTitle("Cards and panels") }
        item {
            Panel {
                Column(Modifier.padding(16.dp), verticalArrangement = Arrangement.spacedBy(12.dp)) {
                    EffectGrid(CARD_STYLES, l.cardStyle, { l.copy(cardStyle = it) }, dark, ::set) { _ ->
                        Box(Modifier.width(80.dp).height(40.dp)) { Panel(Modifier.fillMaxSize()) {} }
                    }
                    SubTitle("Shadow", "Glow uses your accent color.")
                    LookSeg(listOf(0, 1, 2, 3), l.cardShadow, { listOf("None", "Soft", "Deep", "Glow")[it] }) { set(l.copy(cardShadow = it)) }
                }
            }
        }

        // Colors ---------------------------------------------------------------------------
        item { SectionTitle("Colors") }
        item {
            Panel {
                Column(Modifier.padding(16.dp), verticalArrangement = Arrangement.spacedBy(12.dp)) {
                    LookSeg(listOf("light", "dark"), which, { if (it == "light") "Light colors" else "Dark colors" }) { editing = it }
                    SubTitle("Quick accent", "Sets the accent and second accent together.")
                    Row(Modifier.horizontalScroll(rememberScrollState()), horizontalArrangement = Arrangement.spacedBy(10.dp)) {
                        ACCENTS.forEach { (a0, b0) ->
                            val a = if (which == "dark") Looks.mix(a0, "#ffffff", 0.18) else a0
                            val b = if (which == "dark") Looks.mix(b0, "#ffffff", 0.18) else b0
                            val on = palette["accent"] == a
                            Box(
                                Modifier.size(34.dp).clip(CircleShape)
                                    .background(Brush.linearGradient(listOf(hexColor(a), hexColor(a), hexColor(b), hexColor(b)), start = androidx.compose.ui.geometry.Offset.Zero, end = androidx.compose.ui.geometry.Offset.Infinite))
                                    .border(if (on) 3.dp else 1.dp, if (on) c.text else c.border, CircleShape)
                                    .clickable {
                                        val m = (palette + mapOf("accent" to a, "accent2" to b, "onAccent" to Looks.contrastText(a)))
                                        set(if (which == "dark") l.copy(dark = m) else l.copy(light = m))
                                    },
                            )
                        }
                    }
                }
                ROLES.forEach { role ->
                    HorizontalDivider(color = c.border)
                    val hex = palette[role] ?: "#000000"
                    val changed = hex != basePalette[role]
                    val (name, desc) = LookNames.roles[role] ?: (role to "")
                    RowItem(name, "$desc · $hex", leading = {
                        Box(Modifier.size(36.dp).clip(lookShape("icon")).background(hexColor(hex)).border(1.dp, c.borderStrong, lookShape("icon")))
                    }, onClick = { pickRole = role }) {
                        if (changed) LookIconButton(onClick = { set(pal(role, basePalette[role] ?: hex)) }) {
                            Icon(Icons.AutoMirrored.Outlined.Undo, "Back to the theme's color", tint = c.text2)
                        }
                    }
                }
            }
        }

        // Text and layout -------------------------------------------------------------------
        item { SectionTitle("Text and layout") }
        item {
            Panel {
                Column(Modifier.padding(16.dp), verticalArrangement = Arrangement.spacedBy(12.dp)) {
                    SubTitle("Font", null)
                    LookSeg(FONTS, l.font, LookNames::of) { set(l.copy(font = it)) }
                    SubTitle("Text size", "Makes everything bigger or smaller.")
                    LookSeg(TEXT_SIZES, l.textSize, { "$it%" }) { set(l.copy(textSize = it)) }
                    SubTitle("Spacing", "How much room rows and buttons get.")
                    LookSeg(DENSITIES, l.density, LookNames::of) { set(l.copy(density = it)) }
                    SubTitle("Animations", "Off turns every animation off, including button effects.")
                    LookSeg(MOTIONS, l.motion, LookNames::of) { set(l.copy(motion = it)) }
                    SubTitle("Background", "A pattern behind the screens.")
                    LookSeg(BACKGROUNDS, l.background, LookNames::of) { set(l.copy(background = it)) }
                }
            }
        }

        // Share ------------------------------------------------------------------------------
        item { SectionTitle("Share") }
        item {
            Panel {
                Column(Modifier.padding(16.dp), verticalArrangement = Arrangement.spacedBy(12.dp)) {
                    SubTitle("Use this theme somewhere else", "A theme code holds every color, shape and effect. Paste it in Jigglypuff on another phone or on your PC (Appearance › Paste code).")
                    Row(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                        SecondaryButton(onClick = {
                            ctx.getSystemService(ClipboardManager::class.java)?.setPrimaryClip(ClipData.newPlainText("Jigglypuff theme", Looks.encode(l)))
                            scope.launch { snack.showSnackbar("Theme code copied") }
                        }) { Icon(Icons.Outlined.ContentCopy, null, Modifier.size(18.dp).padding(end = 4.dp)); Text("Copy code") }
                        SecondaryButton(onClick = { pasting = true }) { Icon(Icons.Outlined.ContentPaste, null, Modifier.size(18.dp).padding(end = 4.dp)); Text("Paste") }
                        LookIconButton(onClick = {
                            val send = Intent(Intent.ACTION_SEND).setType("text/plain").putExtra(Intent.EXTRA_TEXT, Looks.encode(l))
                            runCatching { ctx.startActivity(Intent.createChooser(send, "Share theme code")) }
                        }) { Icon(Icons.Outlined.Share, "Share theme code") }
                    }
                }
            }
        }
    }

    pickRole?.let { role ->
        val (name, _) = LookNames.roles[role] ?: (role to "")
        ColorPickerDialog("$name (${if (which == "dark") "dark" else "light"})", palette[role] ?: "#000000", onDismiss = { pickRole = null }) { hex ->
            pickRole = null
            set(pal(role, hex))
        }
    }
    if (pasting) PasteDialog(vm, onDismiss = { pasting = false }) { next ->
        pasting = false
        set(next)
        scope.launch { snack.showSnackbar("Using “${next.name}”") }
    }
    if (askReset) LookDialog(
        onDismissRequest = { askReset = false },
        title = { Text("Reset your changes?") },
        text = { Text("All colors, shapes and effects go back to ${base.name}. You can undo this.") },
        confirmButton = { PrimaryButton(onClick = { askReset = false; set(base) }) { Text("Reset") } },
        dismissButton = { TextLinkButton(onClick = { askReset = false }) { Text("Cancel") } },
    )
}

@Composable
private fun SubTitle(title: String, desc: String?) {
    Column {
        Text(title, style = MaterialTheme.typography.titleSmall, fontWeight = FontWeight.SemiBold)
        if (desc != null) Text(desc, style = MaterialTheme.typography.bodySmall, color = LocalLook.current.c.text2)
    }
}

/** A whole theme, drawn with its own colors and shapes. */
@Composable
private fun ThemeTile(look: Look, name: String, desc: String, on: Boolean, dark: Boolean, modifier: Modifier, onPick: () -> Unit) {
    val outer = LocalLook.current.c
    val frame = RoundedCornerShape(16.dp)
    Column(modifier.clip(frame).border(if (on) 2.dp else 1.dp, if (on) outer.accent else outer.border, frame).clickable(onClick = onPick)) {
        LookTheme(look, dark) {
            val c = LocalLook.current.c
            Row(Modifier.fillMaxWidth().height(104.dp).lookBackground().padding(10.dp), horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                Column(Modifier.width(16.dp).fillMaxHeight().clip(RoundedCornerShape(5.dp)).background(c.nav).padding(3.dp), verticalArrangement = Arrangement.spacedBy(3.dp)) {
                    repeat(4) { i -> Box(Modifier.fillMaxWidth().height(5.dp).clip(RoundedCornerShape(3.dp)).background(if (i == 0) c.accent else c.text.copy(alpha = 0.18f))) }
                }
                Panel(Modifier.weight(1f).fillMaxHeight()) {
                    Column(Modifier.fillMaxSize().padding(8.dp), verticalArrangement = Arrangement.spacedBy(5.dp)) {
                        Box(Modifier.fillMaxWidth(0.8f).height(6.dp).clip(RoundedCornerShape(3.dp)).background(c.text.copy(alpha = 0.22f)))
                        Box(Modifier.fillMaxWidth(0.5f).height(6.dp).clip(RoundedCornerShape(3.dp)).background(c.text.copy(alpha = 0.12f)))
                        Spacer(Modifier.weight(1f))
                        Row(horizontalArrangement = Arrangement.spacedBy(5.dp)) {
                            PrimaryButton(onClick = onPick, small = true) { Text("Clean") }
                            SecondaryButton(onClick = onPick, small = true) { Text("Review") }
                        }
                    }
                }
            }
            Column(Modifier.fillMaxWidth().background(c.surface).padding(horizontal = 10.dp, vertical = 8.dp)) {
                Row(verticalAlignment = Alignment.CenterVertically) {
                    Text(name, style = MaterialTheme.typography.titleSmall, fontWeight = FontWeight.SemiBold, color = c.text, maxLines = 1, overflow = TextOverflow.Ellipsis, modifier = Modifier.weight(1f, fill = false))
                    if (on) Icon(Icons.Outlined.Check, "In use", tint = c.accentText, modifier = Modifier.size(16.dp).padding(start = 2.dp))
                }
                Text(desc, style = MaterialTheme.typography.bodySmall, color = c.text2, maxLines = 2, minLines = 2, overflow = TextOverflow.Ellipsis)
            }
        }
    }
}

/** One tile per value, each showing the sample in the current look with that value. */
@Composable
private fun EffectGrid(values: List<String>, current: String, variant: (String) -> Look, dark: Boolean, onPick: (Look) -> Unit, sample: @Composable (pick: () -> Unit) -> Unit) {
    val outer = LocalLook.current.c
    val frame = RoundedCornerShape(14.dp)
    Column(verticalArrangement = Arrangement.spacedBy(8.dp)) {
        values.chunked(3).forEach { row ->
            Row(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                row.forEach { v ->
                    val on = v == current
                    val look = variant(v)
                    val pick = { onPick(look) }
                    Column(
                        Modifier.weight(1f).clip(frame).background(outer.bg).border(if (on) 2.dp else 1.dp, if (on) outer.accent else outer.border, frame)
                            .clickable(onClick = pick).padding(vertical = 12.dp),
                        horizontalAlignment = Alignment.CenterHorizontally, verticalArrangement = Arrangement.spacedBy(8.dp),
                    ) {
                        LookTheme(look, dark) { Box(Modifier.heightIn(min = 48.dp), contentAlignment = Alignment.Center) { sample(pick) } }
                        Text(LookNames.of(v), style = MaterialTheme.typography.labelMedium, color = if (on) outer.text else outer.text2, fontWeight = if (on) FontWeight.SemiBold else FontWeight.Normal)
                    }
                }
                repeat(3 - row.size) { Spacer(Modifier.weight(1f)) }
            }
        }
    }
}

/** Shape choices for one part, plus a corner size slider. [radius] = null hides the slider. */
@Composable
private fun ShapeRow(title: String, desc: String, kind: String?, radius: Int, onRadius: ((Int) -> Look)?, vm: AppModel, onKind: (String) -> Unit) {
    val c = LocalLook.current.c
    Column(Modifier.padding(horizontal = 16.dp, vertical = 12.dp), verticalArrangement = Arrangement.spacedBy(8.dp)) {
        Text(title, style = MaterialTheme.typography.bodyLarge)
        Text(desc, style = MaterialTheme.typography.bodySmall, color = c.text2)
        Row(Modifier.horizontalScroll(rememberScrollState()), horizontalArrangement = Arrangement.spacedBy(6.dp)) {
            SHAPE_KINDS.forEach { k ->
                val on = k == kind
                val frame = RoundedCornerShape(8.dp)
                Box(
                    Modifier.size(width = 44.dp, height = 38.dp).clip(frame)
                        .background(if (on) c.accent.copy(alpha = 0.14f) else c.surface)
                        .border(if (on) 2.dp else 1.dp, if (on) c.accent else c.border, frame)
                        .clickable { onKind(k) },
                    contentAlignment = Alignment.Center,
                ) {
                    val demo = remember(k, radius) { LShape(k, radius.coerceIn(if (k == "rounded" || k == "leaf") 6 else 5, 9)).toShape() }
                    Box(Modifier.size(width = 26.dp, height = 20.dp).clip(demo).background(c.accent))
                }
            }
        }
        Text(LookNames.shapes[kind] ?: "Mixed", style = MaterialTheme.typography.labelMedium, color = c.text2)
        if (onRadius != null) {
            val noRadius = kind == "square" || kind == "pill"
            var dragging by remember { mutableStateOf(false) }
            Row(verticalAlignment = Alignment.CenterVertically) {
                Slider(
                    value = radius.toFloat(), valueRange = 0f..32f, enabled = !noRadius, modifier = Modifier.weight(1f),
                    onValueChange = { v ->
                        if (!dragging) { dragging = true; vm.rememberLook() }
                        vm.setLook(onRadius(kotlin.math.round(v).toInt()), record = false)
                    },
                    onValueChangeFinished = { dragging = false },
                )
                Text(if (noRadius) "—" else "$radius dp", style = MaterialTheme.typography.labelMedium, color = c.text2, modifier = Modifier.width(48.dp).padding(start = 8.dp))
            }
        }
    }
}

/** Hue, saturation and brightness sliders, a color code field and some ready colors. */
@Composable
private fun ColorPickerDialog(title: String, initial: String, onDismiss: () -> Unit, onPick: (String) -> Unit) {
    val start = remember { FloatArray(3).also { android.graphics.Color.colorToHSV(android.graphics.Color.parseColor(initial), it) } }
    var h by remember { mutableFloatStateOf(start[0]) }
    var s by remember { mutableFloatStateOf(start[1]) }
    var v by remember { mutableFloatStateOf(start[2]) }
    fun hexOf(h: Float, s: Float, v: Float) = "#%06x".format(android.graphics.Color.HSVToColor(floatArrayOf(h, s, v)) and 0xffffff)
    var text by remember { mutableStateOf(initial) }
    val current = Looks.normHex(text) ?: hexOf(h, s, v)
    fun fromSliders() { text = hexOf(h, s, v) }
    val c = LocalLook.current.c
    LookDialog(
        onDismissRequest = onDismiss,
        title = { Text(title) },
        text = {
            Column(verticalArrangement = Arrangement.spacedBy(12.dp)) {
                Row(verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(12.dp)) {
                    Box(Modifier.size(52.dp).clip(lookShape("icon")).background(hexColor(current)).border(1.dp, c.borderStrong, lookShape("icon")))
                    OutlinedTextField(
                        value = text, singleLine = true, label = { Text("Color code") }, shape = lookShape("input"),
                        textStyle = MaterialTheme.typography.bodyLarge.copy(fontFamily = FontFamily.Monospace),
                        onValueChange = { t ->
                            text = t.take(7)
                            Looks.normHex(t)?.let { hex ->
                                val f = FloatArray(3)
                                android.graphics.Color.colorToHSV(android.graphics.Color.parseColor(hex), f)
                                h = f[0]; s = f[1]; v = f[2]
                            }
                        },
                    )
                }
                GradientSlider(h, 0f..360f, Brush.horizontalGradient((0..6).map { hexColor(hexOf(it * 60f, 1f, 1f)) })) { h = it; fromSliders() }
                GradientSlider(s, 0f..1f, Brush.horizontalGradient(listOf(hexColor(hexOf(h, 0f, v)), hexColor(hexOf(h, 1f, v))))) { s = it; fromSliders() }
                GradientSlider(v, 0f..1f, Brush.horizontalGradient(listOf(Color.Black, hexColor(hexOf(h, s, 1f))))) { v = it; fromSliders() }
                val ready = listOf("#ffffff", "#f4f4f2", "#1a1a19", "#0b0b0b", "#ff5a5f", "#ff7a50", "#ffd23f", "#39d98a", "#00c2c7", "#2f80ed", "#7b61ff", "#e879f9")
                ready.chunked(6).forEach { row ->
                    Row(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                        row.forEach { hex ->
                            Box(Modifier.size(30.dp).clip(CircleShape).background(hexColor(hex)).border(1.dp, c.borderStrong, CircleShape).clickable {
                                text = hex
                                val f = FloatArray(3)
                                android.graphics.Color.colorToHSV(android.graphics.Color.parseColor(hex), f)
                                h = f[0]; s = f[1]; v = f[2]
                            })
                        }
                    }
                }
            }
        },
        confirmButton = { PrimaryButton(onClick = { onPick(current) }) { Text("Use color") } },
        dismissButton = { TextLinkButton(onClick = onDismiss) { Text("Cancel") } },
    )
}

@Composable
private fun GradientSlider(value: Float, range: ClosedFloatingPointRange<Float>, track: Brush, onChange: (Float) -> Unit) {
    Box(contentAlignment = Alignment.Center) {
        Box(Modifier.fillMaxWidth().height(12.dp).clip(CircleShape).background(track))
        Slider(
            value = value, onValueChange = onChange, valueRange = range, modifier = Modifier.fillMaxWidth(),
            colors = SliderDefaults.colors(
                thumbColor = LocalLook.current.c.text, activeTrackColor = Color.Transparent, inactiveTrackColor = Color.Transparent,
                activeTickColor = Color.Transparent, inactiveTickColor = Color.Transparent,
            ),
        )
    }
}

@Composable
private fun PasteDialog(vm: AppModel, onDismiss: () -> Unit, onUse: (Look) -> Unit) {
    val ctx = LocalContext.current
    var text by remember {
        val clip = runCatching { ctx.getSystemService(ClipboardManager::class.java)?.primaryClip?.getItemAt(0)?.coerceToText(ctx)?.toString() }.getOrNull().orEmpty()
        mutableStateOf(if (clip.trim().uppercase().startsWith("JIGGLYPUFF-LOOK:")) clip.trim() else "")
    }
    var bad by remember { mutableStateOf(false) }
    LookDialog(
        onDismissRequest = onDismiss,
        title = { Text("Paste a theme code") },
        text = {
            Column(verticalArrangement = Arrangement.spacedBy(10.dp)) {
                Text("Paste a code copied from Jigglypuff on another phone or on your PC.")
                OutlinedTextField(
                    value = text, onValueChange = { text = it; bad = false }, minLines = 3, maxLines = 6, shape = lookShape("input"),
                    placeholder = { Text("JIGGLYPUFF-LOOK:…") }, isError = bad,
                    supportingText = if (bad) ({ Text("That isn't a Jigglypuff theme code.") }) else null,
                    textStyle = MaterialTheme.typography.bodySmall.copy(fontFamily = FontFamily.Monospace), modifier = Modifier.fillMaxWidth(),
                )
            }
        },
        confirmButton = {
            PrimaryButton(onClick = { Looks.decode(text, vm.look)?.let(onUse) ?: run { bad = true } }, enabled = text.isNotBlank()) { Text("Use this theme") }
        },
        dismissButton = { TextLinkButton(onClick = onDismiss) { Text("Cancel") } },
    )
}
