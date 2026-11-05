package com.kabir.jigglypuff.ui

import androidx.compose.foundation.isSystemInDarkTheme
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.darkColorScheme
import androidx.compose.material3.lightColorScheme
import androidx.compose.runtime.Composable
import androidx.compose.ui.graphics.Color

// Jigglypuff pink, matching the app icon.
private val Light = lightColorScheme(
    primary = Color(0xFFB8246A), onPrimary = Color.White,
    primaryContainer = Color(0xFFFFD8E7), onPrimaryContainer = Color(0xFF3E0021),
    secondary = Color(0xFF74565F), onSecondary = Color.White,
    secondaryContainer = Color(0xFFFFD9E2), onSecondaryContainer = Color(0xFF2B151C),
    tertiary = Color(0xFF7C5635), tertiaryContainer = Color(0xFFFFDCC2),
    background = Color(0xFFFFF8F9), onBackground = Color(0xFF22191C),
    surface = Color(0xFFFFF8F9), onSurface = Color(0xFF22191C),
    surfaceVariant = Color(0xFFF2DDE2), onSurfaceVariant = Color(0xFF514347),
    surfaceContainerLowest = Color.White, surfaceContainerLow = Color(0xFFFFF0F3),
    surfaceContainer = Color(0xFFFBEAEE), surfaceContainerHigh = Color(0xFFF5E4E8), surfaceContainerHighest = Color(0xFFEFDFE3),
    outline = Color(0xFF837377), outlineVariant = Color(0xFFD5C2C6),
    error = Color(0xFFBA1A1A), errorContainer = Color(0xFFFFDAD6),
)

private val Dark = darkColorScheme(
    primary = Color(0xFFFFB0CD), onPrimary = Color(0xFF640038),
    primaryContainer = Color(0xFF8C0E50), onPrimaryContainer = Color(0xFFFFD8E7),
    secondary = Color(0xFFE2BDC6), onSecondary = Color(0xFF422931),
    secondaryContainer = Color(0xFF5A3F47), onSecondaryContainer = Color(0xFFFFD9E2),
    tertiary = Color(0xFFEFBD94), tertiaryContainer = Color(0xFF623F20),
    background = Color(0xFF191113), onBackground = Color(0xFFEFDFE2),
    surface = Color(0xFF191113), onSurface = Color(0xFFEFDFE2),
    surfaceVariant = Color(0xFF514347), onSurfaceVariant = Color(0xFFD5C2C6),
    surfaceContainerLowest = Color(0xFF130C0E), surfaceContainerLow = Color(0xFF22191C),
    surfaceContainer = Color(0xFF261D20), surfaceContainerHigh = Color(0xFF31282A), surfaceContainerHighest = Color(0xFF3C3235),
    outline = Color(0xFF9E8C91), outlineVariant = Color(0xFF514347),
    error = Color(0xFFFFB4AB), errorContainer = Color(0xFF93000A),
)

/** Colors for safety badges and the storage meter. */
object Tones {
    val safe = Color(0xFF2E8B57)
    val caution = Color(0xFFC77700)
    val bad = Color(0xFFD32F2F)
}

@Composable
fun JigglypuffTheme(theme: String, content: @Composable () -> Unit) {
    val dark = when (theme) {
        "light" -> false
        "dark" -> true
        else -> isSystemInDarkTheme()
    }
    MaterialTheme(colorScheme = if (dark) Dark else Light, content = content)
}
