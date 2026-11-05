package com.kabir.jigglypuff.ui

import androidx.compose.animation.core.Animatable
import androidx.compose.animation.core.animateDpAsState
import androidx.compose.animation.core.animateFloatAsState
import androidx.compose.animation.core.keyframes
import androidx.compose.animation.core.snap
import androidx.compose.animation.core.spring
import androidx.compose.animation.core.tween
import androidx.compose.foundation.BorderStroke
import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.clickable
import androidx.compose.foundation.interaction.InteractionSource
import androidx.compose.foundation.interaction.MutableInteractionSource
import androidx.compose.foundation.interaction.collectIsPressedAsState
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.ColumnScope
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.RowScope
import androidx.compose.foundation.layout.WindowInsets
import androidx.compose.foundation.layout.defaultMinSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.navigationBars
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.windowInsetsPadding
import androidx.compose.foundation.shape.CutCornerShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.AlertDialog
import androidx.compose.material3.Icon
import androidx.compose.material3.LocalContentColor
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.ProvideTextStyle
import androidx.compose.material3.Text
import androidx.compose.material3.ripple
import androidx.compose.runtime.Composable
import androidx.compose.runtime.CompositionLocalProvider
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.remember
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.alpha
import androidx.compose.ui.draw.clip
import androidx.compose.ui.draw.drawBehind
import androidx.compose.ui.draw.shadow
import androidx.compose.ui.geometry.Offset
import androidx.compose.ui.geometry.Size
import androidx.compose.ui.graphics.Brush
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.Outline
import androidx.compose.ui.graphics.Path
import androidx.compose.ui.graphics.RectangleShape
import androidx.compose.ui.graphics.Shape
import androidx.compose.ui.graphics.SolidColor
import androidx.compose.ui.graphics.addOutline
import androidx.compose.ui.graphics.asAndroidPath
import androidx.compose.ui.graphics.drawOutline
import androidx.compose.ui.graphics.drawscope.DrawScope
import androidx.compose.ui.graphics.drawscope.drawIntoCanvas
import androidx.compose.ui.graphics.drawscope.translate
import androidx.compose.ui.graphics.graphicsLayer
import androidx.compose.ui.graphics.nativeCanvas
import androidx.compose.ui.graphics.toArgb
import androidx.compose.ui.graphics.vector.ImageVector
import androidx.compose.ui.semantics.Role
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.Density
import androidx.compose.ui.unit.Dp
import androidx.compose.ui.unit.LayoutDirection
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.max
import com.kabir.jigglypuff.look.LShape
import kotlinx.coroutines.launch
import kotlin.math.min

// Shapes ------------------------------------------------------------------------------------

/** The same shapes as the PC app's CSS corner-shape values. */
fun LShape.toShape(): Shape {
    val r = radius.dp
    return when (kind) {
        "square" -> RectangleShape
        "pill" -> RoundedCornerShape(50)
        "leaf" -> RoundedCornerShape(topStart = r, topEnd = 0.dp, bottomEnd = r, bottomStart = 0.dp)
        "bevel" -> CutCornerShape(r)
        "squircle" -> CornerPathShape(r * 1.6f, "squircle")
        "scoop" -> CornerPathShape(r, "scoop")
        "notch" -> CornerPathShape(r, "notch")
        else -> RoundedCornerShape(r)
    }
}

/** Corners drawn with a path: smooth squircle corners, scooped (concave) corners or square notches. */
private class CornerPathShape(private val r: Dp, private val kind: String) : Shape {
    override fun createOutline(size: Size, layoutDirection: LayoutDirection, density: Density): Outline {
        val w = size.width
        val h = size.height
        val rp = min(with(density) { r.toPx() }, min(w, h) / 2f)
        val p = Path()
        fun corner(fx: Float, fy: Float, cx: Float, cy: Float, tx: Float, ty: Float) {
            when (kind) {
                "squircle" -> p.cubicTo(fx + (cx - fx) * 0.82f, fy + (cy - fy) * 0.82f, tx + (cx - tx) * 0.82f, ty + (cy - ty) * 0.82f, tx, ty)
                "scoop" -> {
                    val ix = fx + tx - cx
                    val iy = fy + ty - cy
                    p.cubicTo(fx + (ix - fx) * 0.55f, fy + (iy - fy) * 0.55f, tx + (ix - tx) * 0.55f, ty + (iy - ty) * 0.55f, tx, ty)
                }
                else -> {
                    p.lineTo(fx + tx - cx, fy + ty - cy)
                    p.lineTo(tx, ty)
                }
            }
        }
        p.moveTo(rp, 0f)
        p.lineTo(w - rp, 0f)
        corner(w - rp, 0f, w, 0f, w, rp)
        p.lineTo(w, h - rp)
        corner(w, h - rp, w, h, w - rp, h)
        p.lineTo(rp, h)
        corner(rp, h, 0f, h, 0f, h - rp)
        p.lineTo(0f, rp)
        corner(0f, rp, 0f, 0f, rp, 0f)
        p.close()
        return Outline.Generic(p)
    }
}

@Composable
fun lookShape(part: String): Shape {
    val s = LocalLook.current.look.shape(part)
    return remember(s) { s.toShape() }
}

// Effects ----------------------------------------------------------------------------------

/** The look's press effect: shrink, push down, bounce, jelly or glow (ripple is the indication). */
@Composable
fun Modifier.pressEffect(interaction: InteractionSource, effect: String = LocalLook.current.look.press): Modifier {
    val m = LocalLook.current.motion
    val pressed by interaction.collectIsPressedAsState()
    val scale = remember { Animatable(1f) }
    val sx = remember { Animatable(1f) }
    val sy = remember { Animatable(1f) }
    val dy = remember { Animatable(0f) }
    LaunchedEffect(pressed, effect, m) {
        fun ms(v: Int) = (v * m).toInt()
        if (m == 0f) {
            scale.snapTo(if (pressed && effect in setOf("shrink", "bounce", "glow")) (if (effect == "bounce") 0.9f else 0.95f) else 1f)
            dy.snapTo(if (pressed && effect == "sink") 1f else 0f)
            return@LaunchedEffect
        }
        when (effect) {
            "shrink" -> scale.animateTo(if (pressed) 0.94f else 1f, tween(ms(110)))
            "sink" -> dy.animateTo(if (pressed) 1f else 0f, tween(ms(100)))
            "bounce" -> scale.animateTo(if (pressed) 0.9f else 1f, if (pressed) tween(ms(90)) else spring(dampingRatio = 0.3f, stiffness = 520f / m))
            "glow" -> scale.animateTo(if (pressed) 0.98f else 1f, tween(ms(100)))
            "jelly" -> if (pressed) {
                val d = ms(560)
                kotlinx.coroutines.coroutineScope {
                    launch { sx.animateTo(1f, keyframes { durationMillis = d; 1.14f at d * 25 / 100; 0.9f at d * 45 / 100; 1.05f at d * 65 / 100; 0.98f at d * 82 / 100 }) }
                    launch { sy.animateTo(1f, keyframes { durationMillis = d; 0.86f at d * 25 / 100; 1.1f at d * 45 / 100; 0.95f at d * 65 / 100; 1.02f at d * 82 / 100 }) }
                }
            }
            else -> {
                scale.snapTo(1f)
                dy.snapTo(0f)
            }
        }
    }
    return graphicsLayer {
        scaleX = scale.value * sx.value
        scaleY = scale.value * sy.value
        translationY = dy.value * 3.dp.toPx()
    }
}

/** A soft colored glow around a shape; works on see-through fills too. */
fun Modifier.glow(color: Color, radius: Dp, shape: Shape): Modifier = if (color.alpha == 0f || radius == 0.dp) this else drawBehind {
    val outline = shape.createOutline(size, layoutDirection, this)
    val path = Path().apply { addOutline(outline) }.asAndroidPath()
    drawIntoCanvas { canvas ->
        val paint = android.graphics.Paint(android.graphics.Paint.ANTI_ALIAS_FLAG).apply {
            style = android.graphics.Paint.Style.STROKE
            strokeWidth = 1.5.dp.toPx()
            this.color = color.copy(alpha = color.alpha * 0.9f).toArgb()
            setShadowLayer(radius.toPx(), 0f, 0f, color.toArgb())
        }
        canvas.nativeCanvas.drawPath(path, paint)
    }
}

private fun DrawScope.outlineAt(shape: Shape, dx: Float, dy: Float, color: Color) {
    val outline = shape.createOutline(size, layoutDirection, this)
    translate(dx, dy) { drawOutline(outline, color) }
}

// Buttons ------------------------------------------------------------------------------------

enum class BtnKind { Primary, Danger, Secondary, Tonal, Text }

/** Every button in the app: the look picks its shape, style and press effect. */
@Composable
fun LookButton(
    onClick: () -> Unit,
    modifier: Modifier = Modifier,
    enabled: Boolean = true,
    kind: BtnKind = BtnKind.Primary,
    small: Boolean = false,
    content: @Composable RowScope.() -> Unit,
) {
    val L = LocalLook.current
    val c = L.c
    val look = L.look
    val main = kind == BtnKind.Primary || kind == BtnKind.Danger
    val col = if (kind == BtnKind.Danger) c.danger else c.accent
    val on = if (kind == BtnKind.Danger) c.onDanger else c.onAccent
    val col2 = if (kind == BtnKind.Danger) lerp(c.danger, c.accent2, 0.35f) else c.accent2
    val colText = if (kind == BtnKind.Danger) c.dangerText else c.accentText
    val style = look.buttonStyle
    val bw = look.buttonBorder.dp
    val shape = lookShape("button")
    val interaction = remember { MutableInteractionSource() }
    val pressed by interaction.collectIsPressedAsState()

    var fill: Brush = SolidColor(c.surface)
    var opaque = true
    var fg = c.text
    var border: BorderStroke? = if (bw > 0.dp) BorderStroke(bw, c.borderStrong) else null
    var edge: Color? = null // 3D: a darker edge under the button
    var hard: Color? = null // Pop: a hard offset shadow
    var glowColor = Color.Transparent
    var weight = FontWeight.Medium
    when {
        kind == BtnKind.Text -> { fill = SolidColor(Color.Transparent); opaque = false; fg = c.accentText; border = null }
        main -> when (style) {
            "soft" -> { fill = SolidColor(col.copy(alpha = 0.16f)); opaque = false; fg = colText; border = null }
            "outline" -> { fill = SolidColor(Color.Transparent); opaque = false; fg = colText; border = BorderStroke(max(bw, 2.dp), col) }
            "gradient" -> { fill = Brush.linearGradient(listOf(col, col2)); fg = on; border = null }
            "glass" -> { fill = SolidColor(col.copy(alpha = 0.24f)); opaque = false; fg = lerp(col, c.text, 0.55f); border = BorderStroke(1.dp, col.copy(alpha = 0.45f)) }
            "neon" -> { fill = SolidColor(col.copy(alpha = 0.08f)); opaque = false; fg = col; border = BorderStroke(max(bw, 2.dp), col); glowColor = col.copy(alpha = 0.6f) }
            "raised" -> { fill = SolidColor(col); fg = on; border = null; edge = lerp(col, Color.Black, 0.38f) }
            "pop" -> { fill = SolidColor(col); fg = on; border = BorderStroke(max(bw, 2.dp), c.text); hard = c.text; weight = FontWeight.SemiBold }
            "flat" -> { fill = SolidColor(Color.Transparent); opaque = false; fg = colText; border = null; weight = FontWeight.SemiBold }
            else -> { fill = SolidColor(col); fg = on; border = null }
        }
        else -> {
            if (kind == BtnKind.Tonal) {
                fill = SolidColor(lerp(c.surface, c.accent, if (c.dark) 0.22f else 0.14f))
                fg = lerp(c.accent, c.text, 0.45f)
                border = null
            }
            when (style) {
                "soft" -> if (kind != BtnKind.Tonal) { fill = SolidColor(c.surface2); border = null }
                "outline", "flat" -> if (kind != BtnKind.Tonal) { fill = SolidColor(Color.Transparent); opaque = false; if (style == "flat") border = null }
                "glass" -> { fill = SolidColor(c.surface.copy(alpha = 0.55f)); opaque = false; border = BorderStroke(1.dp, c.text.copy(alpha = 0.14f)) }
                "neon" -> { fill = SolidColor(Color.Transparent); opaque = false; border = BorderStroke(max(bw, 1.dp), lerp(c.border, c.accent, 0.45f)); glowColor = c.accent.copy(alpha = 0.25f) }
                "raised" -> edge = c.borderStrong
                "pop" -> { border = BorderStroke(max(bw, 2.dp), c.text); hard = c.text; weight = FontWeight.SemiBold }
            }
        }
    }
    val m = L.motion
    val push by animateFloatAsState(if (pressed && (edge != null || hard != null)) 1f else 0f, if (m == 0f) snap() else tween((90 * m).toInt()), label = "push")
    val pressGlow by animateFloatAsState(if (pressed && look.press == "glow") 1f else 0f, if (m == 0f) snap() else tween((140 * m).toInt()), label = "glow")
    val elevation = if (main && opaque && edge == null && hard == null) listOf(0.dp, 2.dp, 6.dp, 12.dp)[look.buttonShadow] else 0.dp
    val glowAll = if (pressGlow > 0f) lerp(glowColor, col, pressGlow) else glowColor
    val glowRadius by animateDpAsState(if (pressGlow > 0f) 14.dp else if (glowColor.alpha > 0f) 8.dp else 0.dp, label = "glowR")

    Box(
        modifier
            .alpha(if (enabled) 1f else 0.45f)
            .pressEffect(interaction)
            .graphicsLayer {
                if (edge != null) translationY = push * 3.dp.toPx()
                if (hard != null) { translationX = push * 3.dp.toPx(); translationY = push * 3.dp.toPx() }
            }
            .drawBehind {
                edge?.let { outlineAt(shape, 0f, (4f - 3f * push).dp.toPx(), it) }
                hard?.let { outlineAt(shape, (3f - 3f * push).dp.toPx(), (3f - 3f * push).dp.toPx(), it) }
            }
            .glow(glowAll, glowRadius, shape)
            .shadow(elevation, shape, clip = false, ambientColor = col, spotColor = col)
            .clip(shape)
            .background(fill)
            .then(if (border != null) Modifier.border(border!!, shape) else Modifier)
            .clickable(
                interactionSource = interaction,
                indication = if (look.press == "ripple") ripple(color = fg) else null,
                enabled = enabled, role = Role.Button, onClick = onClick,
            )
            .defaultMinSize(minWidth = if (small) 0.dp else 48.dp, minHeight = if (small) 0.dp else (40 * L.dens).dp)
            .padding(horizontal = if (small) 10.dp else if (kind == BtnKind.Text) 12.dp else 18.dp, vertical = if (small) 4.dp else (8 * L.dens).dp),
        contentAlignment = Alignment.Center,
    ) {
        CompositionLocalProvider(LocalContentColor provides fg) {
            ProvideTextStyle((if (small) MaterialTheme.typography.labelSmall else MaterialTheme.typography.labelLarge).copy(fontWeight = weight)) {
                Row(verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.Center, content = content)
            }
        }
    }
}

@Composable
fun PrimaryButton(onClick: () -> Unit, modifier: Modifier = Modifier, enabled: Boolean = true, small: Boolean = false, content: @Composable RowScope.() -> Unit) =
    LookButton(onClick, modifier, enabled, BtnKind.Primary, small, content)

@Composable
fun DangerButton(onClick: () -> Unit, modifier: Modifier = Modifier, enabled: Boolean = true, content: @Composable RowScope.() -> Unit) =
    LookButton(onClick, modifier, enabled, BtnKind.Danger, content = content)

@Composable
fun SecondaryButton(onClick: () -> Unit, modifier: Modifier = Modifier, enabled: Boolean = true, small: Boolean = false, content: @Composable RowScope.() -> Unit) =
    LookButton(onClick, modifier, enabled, BtnKind.Secondary, small, content)

@Composable
fun TonalButton(onClick: () -> Unit, modifier: Modifier = Modifier, enabled: Boolean = true, content: @Composable RowScope.() -> Unit) =
    LookButton(onClick, modifier, enabled, BtnKind.Tonal, content = content)

@Composable
fun TextLinkButton(onClick: () -> Unit, modifier: Modifier = Modifier, enabled: Boolean = true, content: @Composable RowScope.() -> Unit) =
    LookButton(onClick, modifier, enabled, BtnKind.Text, content = content)

/** An icon-only button with the look's button shape and press effect. */
@Composable
fun LookIconButton(onClick: () -> Unit, modifier: Modifier = Modifier, enabled: Boolean = true, content: @Composable () -> Unit) {
    val interaction = remember { MutableInteractionSource() }
    val shape = lookShape("button")
    val look = LocalLook.current.look
    Box(
        modifier
            .size(44.dp)
            .alpha(if (enabled) 1f else 0.45f)
            .pressEffect(interaction)
            .clip(shape)
            .clickable(interaction, if (look.press == "none") null else ripple(), enabled = enabled, role = Role.Button, onClick = onClick),
        contentAlignment = Alignment.Center,
    ) { content() }
}

// Cards --------------------------------------------------------------------------------------

/** A card in the look's card style. [hero] tints it with the accent; [selected] outlines it. */
@Composable
fun Panel(
    modifier: Modifier = Modifier,
    hero: Boolean = false,
    selected: Boolean = false,
    onClick: (() -> Unit)? = null,
    content: @Composable ColumnScope.() -> Unit,
) {
    val L = LocalLook.current
    val c = L.c
    val look = L.look
    val shape = lookShape("card")
    var fill = if (hero) lerp(c.surface, c.accent, if (c.dark) 0.2f else 0.12f) else c.surface
    var border: BorderStroke? = BorderStroke(1.dp, c.border)
    var hard: Color? = null
    var opaque = true
    when (look.cardStyle) {
        "outline" -> { fill = fill.copy(alpha = 0.35f); opaque = false; border = BorderStroke(2.dp, c.borderStrong) }
        "elevated" -> border = null
        "glass" -> { fill = fill.copy(alpha = 0.6f); opaque = false; border = BorderStroke(1.dp, c.text.copy(alpha = 0.11f)) }
        "pop" -> { border = BorderStroke(2.dp, c.text); hard = c.text }
    }
    if (selected) border = BorderStroke(2.dp, c.accent)
    val elevation = if (opaque && hard == null) listOf(0.dp, 1.dp, 6.dp, 0.dp)[look.cardShadow] else 0.dp
    val interaction = remember { MutableInteractionSource() }
    Column(
        modifier
            .fillMaxWidth()
            .then(if (onClick != null) Modifier.pressEffect(interaction, if (look.press == "jelly") "shrink" else look.press) else Modifier)
            .drawBehind { hard?.let { outlineAt(shape, 4.dp.toPx(), 4.dp.toPx(), it) } }
            .glow(if (look.cardShadow == 3) c.accent.copy(alpha = 0.35f) else Color.Transparent, 14.dp, shape)
            .shadow(elevation, shape, clip = false)
            .clip(shape)
            .background(fill)
            .then(if (border != null) Modifier.border(border!!, shape) else Modifier)
            .then(if (onClick != null) Modifier.clickable(interaction, ripple(), onClick = onClick) else Modifier),
    ) {
        CompositionLocalProvider(LocalContentColor provides c.text) { content() }
    }
}

// Choices --------------------------------------------------------------------------------------

/** A row of options, one picked; shaped like the look's chips. */
@Composable
fun <T> LookSeg(options: List<T>, value: T, label: (T) -> String, modifier: Modifier = Modifier, onPick: (T) -> Unit) {
    val c = LocalLook.current.c
    val shape = lookShape("chip")
    Row(modifier.fillMaxWidth().clip(shape).background(c.surface2).padding(3.dp), horizontalArrangement = Arrangement.spacedBy(2.dp)) {
        options.forEach { o ->
            val on = o == value
            val interaction = remember { MutableInteractionSource() }
            Box(
                Modifier
                    .weight(1f)
                    .pressEffect(interaction)
                    .clip(shape)
                    .background(if (on) c.surface else Color.Transparent)
                    .clickable(interaction, null, role = Role.RadioButton) { onPick(o) }
                    .padding(vertical = 8.dp, horizontal = 4.dp),
                contentAlignment = Alignment.Center,
            ) {
                Text(label(o), style = MaterialTheme.typography.labelLarge, color = if (on) c.text else c.text2,
                    fontWeight = if (on) FontWeight.SemiBold else FontWeight.Normal, maxLines = 1, overflow = TextOverflow.Ellipsis)
            }
        }
    }
}

// Bottom bar ---------------------------------------------------------------------------------

@Composable
fun LookNavBar(tabs: List<Tab>, selected: Tab, onSelect: (Tab) -> Unit) {
    val L = LocalLook.current
    val c = L.c
    val shape = lookShape("nav")
    Column(Modifier.fillMaxWidth().background(c.nav).windowInsetsPadding(WindowInsets.navigationBars)) {
        if (c.nav == c.bg) Box(Modifier.fillMaxWidth().height(1.dp).background(c.border))
        Row(Modifier.fillMaxWidth().height(72.dp), verticalAlignment = Alignment.CenterVertically) {
            tabs.forEach { t ->
                val on = t == selected
                val interaction = remember { MutableInteractionSource() }
                Column(
                    Modifier.weight(1f).clickable(interaction, null, role = Role.Tab) { onSelect(t) },
                    horizontalAlignment = Alignment.CenterHorizontally, verticalArrangement = Arrangement.spacedBy(4.dp),
                ) {
                    Box(
                        Modifier
                            .pressEffect(interaction)
                            .clip(shape)
                            .background(if (on) c.navActive else Color.Transparent)
                            .padding(horizontal = 20.dp, vertical = 4.dp),
                    ) {
                        Icon(t.icon, null, tint = if (on) c.text else c.text2)
                    }
                    Text(t.label, style = MaterialTheme.typography.labelMedium, color = if (on) c.text else c.text2,
                        fontWeight = if (on) FontWeight.SemiBold else FontWeight.Medium, maxLines = 1)
                }
            }
        }
    }
}

// Background -------------------------------------------------------------------------------

/** The look's background: plain, a soft gradient, dots or a grid. */
@Composable
fun Modifier.lookBackground(): Modifier {
    val L = LocalLook.current
    val c = L.c
    return when (L.look.background) {
        "gradient" -> background(c.bg).drawBehind {
            drawRect(Brush.radialGradient(listOf(c.accent.copy(alpha = 0.18f), Color.Transparent), center = Offset.Zero, radius = size.maxDimension * 0.8f))
            drawRect(Brush.radialGradient(listOf(c.accent2.copy(alpha = 0.18f), Color.Transparent), center = Offset(size.width, size.height), radius = size.maxDimension * 0.75f))
        }
        "dots" -> background(c.bg).drawBehind {
            val step = 18.dp.toPx()
            val dot = c.text.copy(alpha = 0.14f)
            var y = step / 2
            while (y < size.height) {
                var x = step / 2
                while (x < size.width) { drawCircle(dot, 1.dp.toPx(), Offset(x, y)); x += step }
                y += step
            }
        }
        "grid" -> background(c.bg).drawBehind {
            val step = 28.dp.toPx()
            val line = c.accent.copy(alpha = 0.09f)
            var x = 0f
            while (x < size.width) { drawLine(line, Offset(x, 0f), Offset(x, size.height), 1.dp.toPx()); x += step }
            var y = 0f
            while (y < size.height) { drawLine(line, Offset(0f, y), Offset(size.width, y), 1.dp.toPx()); y += step }
        }
        else -> background(c.bg)
    }
}

// Dialogs ------------------------------------------------------------------------------------

@Composable
fun LookDialog(
    onDismissRequest: () -> Unit,
    confirmButton: @Composable () -> Unit,
    dismissButton: @Composable (() -> Unit)? = null,
    title: @Composable (() -> Unit)? = null,
    text: @Composable (() -> Unit)? = null,
) {
    val c = LocalLook.current.c
    AlertDialog(
        onDismissRequest = onDismissRequest, confirmButton = confirmButton, dismissButton = dismissButton, title = title, text = text,
        shape = lookShape("dialog"), containerColor = c.surface, titleContentColor = c.text, textContentColor = c.text2,
    )
}

/** An icon on a tile shaped like the look's icon tiles. */
@Composable
fun IconTile(icon: ImageVector, tint: Color, bg: Color, size: Dp = 40.dp) {
    Box(Modifier.size(size).clip(lookShape("icon")).background(bg), contentAlignment = Alignment.Center) {
        Icon(icon, null, tint = tint, modifier = Modifier.size(size * 0.55f))
    }
}
