package com.kabir.jigglypuff.ui

import android.content.Context
import android.text.format.DateUtils
import androidx.compose.foundation.Image
import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.ColumnScope
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.RowScope
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.LazyListScope
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.outlined.Close
import androidx.compose.material3.Button
import androidx.compose.material3.Card
import androidx.compose.material3.CardDefaults
import androidx.compose.material3.Icon
import androidx.compose.material3.LinearProgressIndicator
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedButton
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.runtime.Composable
import androidx.compose.runtime.collectAsState
import androidx.compose.runtime.getValue
import androidx.compose.runtime.produceState
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.ImageBitmap
import androidx.compose.ui.graphics.asImageBitmap
import androidx.compose.ui.graphics.vector.ImageVector
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import androidx.core.graphics.drawable.toBitmap
import com.kabir.jigglypuff.AppModel
import com.kabir.jigglypuff.engine.Safety
import com.kabir.jigglypuff.engine.fmtSize
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.withContext

/** A screen's scrolling list with the usual side padding. */
@Composable
fun ScreenList(content: LazyListScope.() -> Unit) {
    LazyColumn(
        contentPadding = PaddingValues(start = 16.dp, end = 16.dp, top = 8.dp, bottom = 24.dp),
        verticalArrangement = Arrangement.spacedBy(12.dp),
        content = content,
    )
}

@Composable
fun SectionTitle(text: String, meta: String? = null) {
    Row(Modifier.padding(top = 8.dp, start = 4.dp), verticalAlignment = Alignment.Bottom) {
        Text(text, style = MaterialTheme.typography.titleMedium, fontWeight = FontWeight.SemiBold)
        if (meta != null) Text("  ·  $meta", style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
    }
}

@Composable
fun Panel(modifier: Modifier = Modifier, content: @Composable ColumnScope.() -> Unit) {
    Card(
        modifier.fillMaxWidth(),
        shape = RoundedCornerShape(20.dp),
        colors = CardDefaults.cardColors(containerColor = MaterialTheme.colorScheme.surfaceContainerLow),
        content = content,
    )
}

/** A list row: icon, title, description, and whatever goes on the right. */
@Composable
fun RowItem(
    title: String,
    desc: String?,
    leading: @Composable (() -> Unit)? = null,
    onClick: (() -> Unit)? = null,
    trailing: @Composable RowScope.() -> Unit = {},
) {
    Row(
        Modifier
            .fillMaxWidth()
            .heightIn(min = 60.dp)
            .then(if (onClick != null) Modifier.clickable(onClick = onClick) else Modifier)
            .padding(horizontal = 14.dp, vertical = 10.dp),
        verticalAlignment = Alignment.CenterVertically,
        horizontalArrangement = Arrangement.spacedBy(12.dp),
    ) {
        leading?.invoke()
        Column(Modifier.weight(1f)) {
            Text(title, style = MaterialTheme.typography.bodyLarge, maxLines = 2, overflow = TextOverflow.Ellipsis)
            if (!desc.isNullOrEmpty()) {
                Text(desc, style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant, maxLines = 2, overflow = TextOverflow.Ellipsis)
            }
        }
        trailing()
    }
}

@Composable
fun IconBox(icon: ImageVector, tint: Color = MaterialTheme.colorScheme.onSecondaryContainer, bg: Color = MaterialTheme.colorScheme.secondaryContainer) {
    Box(Modifier.size(40.dp).clip(RoundedCornerShape(12.dp)).background(bg), contentAlignment = Alignment.Center) {
        Icon(icon, null, tint = tint, modifier = Modifier.size(22.dp))
    }
}

@Composable
fun Badge(text: String, color: Color) {
    Surface(shape = RoundedCornerShape(50), color = color.copy(alpha = 0.14f)) {
        Row(Modifier.padding(horizontal = 8.dp, vertical = 2.dp), verticalAlignment = Alignment.CenterVertically) {
            Box(Modifier.size(6.dp).clip(CircleShape).background(color))
            Spacer(Modifier.width(5.dp))
            Text(text, color = color, fontSize = 11.5.sp, fontWeight = FontWeight.Medium)
        }
    }
}

@Composable
fun SafetyBadge(s: Safety) = when (s) {
    Safety.SAFE -> Badge("Safe", Tones.safe)
    Safety.CAUTION -> Badge("Caution", Tones.caution)
    Safety.REVIEW -> Badge("Review first", Tones.caution)
}

@Composable
fun SizeText(bytes: Long, muted: Boolean = false) {
    Text(
        bytes.fmtSize(),
        style = MaterialTheme.typography.bodyMedium, fontWeight = FontWeight.Medium,
        color = if (muted) MaterialTheme.colorScheme.onSurfaceVariant else MaterialTheme.colorScheme.onSurface,
    )
}

/** Shown while a scan runs, with the folder it's looking at and a Stop button. */
@Composable
fun ScanningCard(vm: AppModel, title: String) {
    val p by vm.progress.collectAsState()
    Panel {
        Column(Modifier.padding(18.dp), verticalArrangement = Arrangement.spacedBy(10.dp)) {
            Text(title, style = MaterialTheme.typography.titleMedium)
            Text("Only looking — nothing is changed.", style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
            if (p.total > 0) LinearProgressIndicator(progress = { p.done.toFloat() / p.total }, modifier = Modifier.fillMaxWidth())
            else LinearProgressIndicator(Modifier.fillMaxWidth())
            Text(
                buildString {
                    if (p.count > 0) append("${p.count} files checked · ")
                    append(p.message)
                },
                style = MaterialTheme.typography.bodySmall, maxLines = 1, overflow = TextOverflow.Ellipsis,
                color = MaterialTheme.colorScheme.onSurfaceVariant,
            )
            OutlinedButton(onClick = vm::cancel) { Text("Stop") }
        }
    }
}

@Composable
fun EmptyState(icon: ImageVector, title: String, text: String, action: String? = null, onAction: () -> Unit = {}) {
    Panel {
        Column(Modifier.fillMaxWidth().padding(28.dp), horizontalAlignment = Alignment.CenterHorizontally, verticalArrangement = Arrangement.spacedBy(8.dp)) {
            IconBox(icon)
            Text(title, style = MaterialTheme.typography.titleMedium)
            Text(text, style = MaterialTheme.typography.bodyMedium, color = MaterialTheme.colorScheme.onSurfaceVariant, textAlign = androidx.compose.ui.text.style.TextAlign.Center)
            if (action != null) Button(onClick = onAction, modifier = Modifier.padding(top = 6.dp)) { Text(action) }
        }
    }
}

/** A note with an optional action, e.g. a missing permission. */
@Composable
fun Note(icon: ImageVector, text: String, action: String? = null, warn: Boolean = false, onDismiss: (() -> Unit)? = null, onAction: () -> Unit = {}) {
    Surface(
        shape = RoundedCornerShape(16.dp),
        color = if (warn) MaterialTheme.colorScheme.tertiaryContainer else MaterialTheme.colorScheme.primaryContainer,
        modifier = Modifier.fillMaxWidth(),
    ) {
        Row(Modifier.padding(start = 14.dp, end = 6.dp, top = 10.dp, bottom = 10.dp), verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(10.dp)) {
            Icon(icon, null, modifier = Modifier.size(20.dp))
            Text(text, style = MaterialTheme.typography.bodyMedium, modifier = Modifier.weight(1f))
            if (action != null) TextButton(onClick = onAction) { Text(action) }
            if (onDismiss != null) androidx.compose.material3.IconButton(onClick = onDismiss) { Icon(Icons.Outlined.Close, "Dismiss") }
        }
    }
}

/** Loads an app's icon off the main thread. */
@Composable
fun AppIcon(pkg: String, label: String, sizeDp: Int = 40) {
    val ctx = LocalContext.current
    val bmp by produceState<ImageBitmap?>(null, pkg) { value = withContext(Dispatchers.IO) { loadIcon(ctx, pkg) } }
    val b = bmp
    if (b != null) Image(b, null, Modifier.size(sizeDp.dp).clip(RoundedCornerShape(10.dp)))
    else Box(Modifier.size(sizeDp.dp).clip(RoundedCornerShape(10.dp)).background(avatarColor(label)), contentAlignment = Alignment.Center) {
        Text(label.trim().take(1).uppercase(), color = Color.White, fontWeight = FontWeight.SemiBold)
    }
}

private val iconCache = android.util.LruCache<String, ImageBitmap>(200)

private fun loadIcon(ctx: Context, pkg: String): ImageBitmap? {
    iconCache.get(pkg)?.let { return it }
    return runCatching { ctx.packageManager.getApplicationIcon(pkg).toBitmap(96, 96).asImageBitmap() }.getOrNull()?.also { iconCache.put(pkg, it) }
}

/** Loads an APK file's icon. */
@Composable
fun ApkIcon(path: String, label: String) {
    val ctx = LocalContext.current
    val bmp by produceState<ImageBitmap?>(null, path) {
        value = withContext(Dispatchers.IO) {
            runCatching {
                @Suppress("DEPRECATION")
                val info = ctx.packageManager.getPackageArchiveInfo(path, 0)!!
                val ai = info.applicationInfo!!
                ai.sourceDir = path
                ai.publicSourceDir = path
                ai.loadIcon(ctx.packageManager).toBitmap(96, 96).asImageBitmap()
            }.getOrNull()
        }
    }
    val b = bmp
    if (b != null) Image(b, null, Modifier.size(40.dp).clip(RoundedCornerShape(10.dp)))
    else Box(Modifier.size(40.dp).clip(RoundedCornerShape(10.dp)).background(avatarColor(label)), contentAlignment = Alignment.Center) {
        Text(label.trim().take(1).uppercase(), color = Color.White, fontWeight = FontWeight.SemiBold)
    }
}

fun avatarColor(s: String): Color {
    val palette = listOf(0xFFD0487F, 0xFF7B61FF, 0xFF2F80ED, 0xFF16A085, 0xFFE67E22, 0xFFC0392B, 0xFF8E44AD, 0xFF2D9CDB)
    return Color(palette[(s.hashCode() and 0x7fffffff) % palette.size])
}

fun relTime(t: Long): String = when {
    t <= 0 -> "never"
    System.currentTimeMillis() - t < 60_000 -> "just now"
    else -> DateUtils.getRelativeTimeSpanString(t, System.currentTimeMillis(), DateUtils.MINUTE_IN_MILLIS).toString()
}

fun plural(n: Int, word: String) = "$n $word" + if (n == 1) "" else "s"

/** A path shortened for display: "Internal storage › Download › file.zip". */
fun prettyPath(vm: AppModel, path: String): String {
    val v = vm.volumes.firstOrNull { path.startsWith(it.root.path + "/") || path == it.root.path }
    val rel = if (v != null) path.removePrefix(v.root.path).trimStart('/') else path
    // With a single storage its name adds nothing.
    val head = if (vm.volumes.size > 1) v?.name ?: "" else ""
    val parts = rel.split('/').filter { it.isNotEmpty() }
    val tail = if (parts.size > 3) listOf("…") + parts.takeLast(3) else parts
    return (listOf(head) + tail).filter { it.isNotEmpty() }.joinToString(" › ")
}
