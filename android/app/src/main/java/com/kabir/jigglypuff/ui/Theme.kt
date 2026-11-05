package com.kabir.jigglypuff.ui

import androidx.compose.foundation.isSystemInDarkTheme
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Typography
import androidx.compose.material3.darkColorScheme
import androidx.compose.material3.lightColorScheme
import androidx.compose.runtime.Composable
import androidx.compose.runtime.CompositionLocalProvider
import androidx.compose.runtime.Immutable
import androidx.compose.runtime.remember
import androidx.compose.runtime.staticCompositionLocalOf
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.platform.LocalDensity
import androidx.compose.ui.text.TextStyle
import androidx.compose.ui.text.font.FontFamily
import androidx.compose.ui.unit.Density
import com.kabir.jigglypuff.look.Look
import com.kabir.jigglypuff.look.Looks

/** A look's colors for one mode, ready for Compose. */
@Immutable
data class LookColors(
    val bg: Color, val surface: Color, val surface2: Color, val border: Color, val text: Color, val text2: Color, val muted: Color,
    val accent: Color, val onAccent: Color, val accent2: Color, val danger: Color, val onDanger: Color, val good: Color, val warning: Color,
    val nav: Color, val navActive: Color, val dark: Boolean,
) {
    val borderStrong get() = lerp(border, text, 0.18f)
    /** Accent that reads well as text on the background. */
    val accentText get() = if (dark) lerp(accent, text, 0.3f) else lerp(accent, text, 0.08f)
    val dangerText get() = if (dark) lerp(danger, text, 0.35f) else lerp(danger, text, 0.1f)

    companion object {
        fun of(p: Map<String, String>): LookColors {
            fun c(k: String) = Color(android.graphics.Color.parseColor(p[k] ?: "#ff00ff"))
            return LookColors(c("bg"), c("surface"), c("surface2"), c("border"), c("text"), c("text2"), c("muted"), c("accent"), c("onAccent"),
                c("accent2"), c("danger"), c("onDanger"), c("good"), c("warning"), c("nav"), c("navActive"), Looks.isDark(p["bg"] ?: "#ffffff"))
        }
    }
}

fun lerp(a: Color, b: Color, t: Float) = androidx.compose.ui.graphics.lerp(a, b, t)

/** The look in use, with its colors for the current mode. */
@Immutable
data class ResolvedLook(val look: Look, val c: LookColors) {
    /** Spacing factor for rows and buttons. */
    val dens: Float get() = when (look.density) { "compact" -> 0.82f; "spacious" -> 1.18f; else -> 1f }
    /** Animation length factor; 0 turns animations off. */
    val motion: Float get() = when (look.motion) { "off" -> 0f; "calm" -> 1.6f; "lively" -> 0.75f; else -> 1f }
}

val LocalLook = staticCompositionLocalOf { ResolvedLook(Looks.FALLBACK, LookColors.of(Looks.FALLBACK.light)) }
private val LocalPhoneFontScale = staticCompositionLocalOf<Float?> { null }

/** Colors for safety badges and the storage meter; they follow the look. */
object Tones {
    val safe: Color @Composable get() = LocalLook.current.c.good
    val caution: Color @Composable get() = LocalLook.current.c.warning
    val bad: Color @Composable get() = LocalLook.current.c.danger
}

fun fontFamily(font: String): FontFamily = when (font) {
    "condensed" -> FontFamily(androidx.compose.ui.text.font.Typeface(android.graphics.Typeface.create("sans-serif-condensed", android.graphics.Typeface.NORMAL)))
    "serif" -> FontFamily.Serif
    "mono" -> FontFamily.Monospace
    else -> FontFamily.Default
}

private fun Typography.withFont(f: FontFamily): Typography {
    fun TextStyle.w() = copy(fontFamily = f)
    return Typography(
        displayLarge.w(), displayMedium.w(), displaySmall.w(), headlineLarge.w(), headlineMedium.w(), headlineSmall.w(),
        titleLarge.w(), titleMedium.w(), titleSmall.w(), bodyLarge.w(), bodyMedium.w(), bodySmall.w(), labelLarge.w(), labelMedium.w(), labelSmall.w(),
    )
}

fun colorScheme(c: LookColors) = if (c.dark) darkColorScheme(
    primary = c.accent, onPrimary = c.onAccent, primaryContainer = lerp(c.surface, c.accent, 0.3f), onPrimaryContainer = c.text,
    secondary = lerp(c.accent, c.text, 0.3f), onSecondary = c.onAccent,
    secondaryContainer = lerp(c.surface, c.accent, 0.22f), onSecondaryContainer = lerp(c.accent, c.text, 0.45f),
    tertiary = c.accent2, tertiaryContainer = lerp(c.surface, c.warning, 0.26f), onTertiaryContainer = c.text,
    background = c.bg, onBackground = c.text, surface = c.bg, onSurface = c.text, surfaceVariant = c.surface2, onSurfaceVariant = c.text2,
    surfaceContainerLowest = c.surface, surfaceContainerLow = c.surface, surfaceContainer = c.surface, surfaceContainerHigh = c.surface2,
    surfaceContainerHighest = lerp(c.surface2, c.text, 0.06f), surfaceBright = c.surface2, surfaceDim = c.bg, surfaceTint = c.accent,
    outline = c.borderStrong, outlineVariant = c.border, error = c.danger, onError = c.onDanger,
    errorContainer = lerp(c.surface, c.danger, 0.25f), onErrorContainer = c.text, inverseSurface = c.text, inverseOnSurface = c.bg,
    inversePrimary = lerp(c.accent, c.bg, 0.4f),
) else lightColorScheme(
    primary = c.accent, onPrimary = c.onAccent, primaryContainer = lerp(c.surface, c.accent, 0.18f), onPrimaryContainer = c.text,
    secondary = lerp(c.accent, c.text, 0.3f), onSecondary = c.onAccent,
    secondaryContainer = lerp(c.surface, c.accent, 0.14f), onSecondaryContainer = lerp(c.accent, c.text, 0.45f),
    tertiary = c.accent2, tertiaryContainer = lerp(c.surface, c.warning, 0.22f), onTertiaryContainer = c.text,
    background = c.bg, onBackground = c.text, surface = c.bg, onSurface = c.text, surfaceVariant = c.surface2, onSurfaceVariant = c.text2,
    surfaceContainerLowest = c.surface, surfaceContainerLow = c.surface, surfaceContainer = c.surface, surfaceContainerHigh = c.surface2,
    surfaceContainerHighest = lerp(c.surface2, c.text, 0.06f), surfaceBright = c.surface, surfaceDim = c.surface2, surfaceTint = c.accent,
    outline = c.borderStrong, outlineVariant = c.border, error = c.danger, onError = c.onDanger,
    errorContainer = lerp(c.surface, c.danger, 0.18f), onErrorContainer = c.text, inverseSurface = c.text, inverseOnSurface = c.bg,
    inversePrimary = lerp(c.accent, c.bg, 0.4f),
)

/** True when the light/dark setting (and the phone, for "system") asks for dark colors. */
@Composable
fun wantsDark(theme: String) = when (theme) {
    "light" -> false
    "dark" -> true
    else -> isSystemInDarkTheme()
}

/** Shows content in a look: its colors, font and text size, plus everything the styled components read. */
@Composable
fun LookTheme(look: Look, dark: Boolean, content: @Composable () -> Unit) {
    val colors = remember(look, dark) { LookColors.of(if (dark) look.dark else look.light) }
    val resolved = remember(look, colors) { ResolvedLook(look, colors) }
    val typography = remember(look.font) { Typography().withFont(fontFamily(look.font)) }
    val d = LocalDensity.current
    // The phone's own font scale, so a preview inside the app doesn't scale text twice.
    val base = LocalPhoneFontScale.current ?: d.fontScale
    CompositionLocalProvider(
        LocalLook provides resolved, LocalPhoneFontScale provides base,
        LocalDensity provides Density(d.density, base * look.textSize / 100f),
    ) {
        MaterialTheme(colorScheme = colorScheme(colors), typography = typography) {
            CompositionLocalProvider(androidx.compose.material3.LocalContentColor provides colors.text, content = content)
        }
    }
}

@Composable
fun JigglypuffTheme(theme: String, look: Look, content: @Composable () -> Unit) = LookTheme(look, wantsDark(theme), content)
