package com.kabir.jigglypuff.ui

import androidx.compose.foundation.BorderStroke
import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxHeight
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.automirrored.outlined.ArrowForward
import androidx.compose.material.icons.outlined.Apps
import androidx.compose.material.icons.outlined.CheckCircle
import androidx.compose.material.icons.outlined.ContentCopy
import androidx.compose.material.icons.outlined.DeleteSweep
import androidx.compose.material.icons.outlined.Description
import androidx.compose.material.icons.outlined.Download
import androidx.compose.material.icons.outlined.FolderOff
import androidx.compose.material.icons.outlined.HourglassEmpty
import androidx.compose.material.icons.outlined.Image
import androidx.compose.material.icons.outlined.Inventory2
import androidx.compose.material.icons.outlined.Lock
import androidx.compose.material.icons.outlined.Memory
import androidx.compose.material.icons.outlined.Search
import androidx.compose.material.icons.outlined.Warning
import androidx.compose.material.icons.outlined.Widgets
import androidx.compose.material3.Checkbox
import androidx.compose.material3.HorizontalDivider
import androidx.compose.material3.Icon
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.vector.ImageVector
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import com.kabir.jigglypuff.AppModel
import com.kabir.jigglypuff.engine.Device
import com.kabir.jigglypuff.engine.JunkCategory
import com.kabir.jigglypuff.engine.Safety
import com.kabir.jigglypuff.engine.Target
import com.kabir.jigglypuff.engine.Volume
import com.kabir.jigglypuff.engine.fmtSize
import java.util.concurrent.TimeUnit

fun junkIcon(id: String): ImageVector = when (id) {
    "thumbnails" -> Icons.Outlined.Image
    "temp" -> Icons.Outlined.Description
    "empty" -> Icons.Outlined.FolderOff
    "apk-installed" -> Icons.Outlined.Inventory2
    else -> Icons.Outlined.Widgets
}

/** Safe categories are ticked by default, like on Windows. */
fun defaultJunk(cats: List<JunkCategory>?) = cats.orEmpty().filter { it.safety == Safety.SAFE }.map { it.id }.toSet()

fun junkTargets(cats: List<JunkCategory>, ids: Set<String>) =
    cats.filter { it.id in ids }.flatMap { c -> c.items.map { Target(it.path, it.size, c.trash, it.isDir) } }

@Composable
fun OverviewScreen(vm: AppModel) {
    val start = rememberStart()
    val ctx = androidx.compose.ui.platform.LocalContext.current
    val nav = LocalNav.current
    var sel by remember(vm.scannedAt) { mutableStateOf(defaultJunk(vm.junk)) }
    var confirm by remember { mutableStateOf<CleanRequest?>(null) }
    val where = vm.selected?.name

    ScreenList {
        item {
            Column(Modifier.padding(horizontal = 4.dp, vertical = 4.dp)) {
                Text("Let's tidy up", style = MaterialTheme.typography.headlineSmall, fontWeight = FontWeight.SemiBold)
                Text("Jigglypuff never deletes anything without showing you first.", style = MaterialTheme.typography.bodyMedium, color = MaterialTheme.colorScheme.onSurfaceVariant)
            }
        }
        if (!vm.hasAllFiles) item {
            Note(Icons.Outlined.Lock, "Jigglypuff needs All files access to look through your storage.", "Allow", warn = true) { start(Device.allFilesIntent(ctx)) }
        }
        item {
            when {
                vm.busy == "overview" -> ScanningCard(vm, if (where != null) "Scanning $where…" else "Scanning your phone…")
                vm.scannedAt == 0L -> Hero(
                    title = if (where != null) "Let's see what can go from $where" else "Let's see what can go",
                    text = "Jigglypuff looks for junk files, leftovers of removed apps and forgotten downloads. Scanning only looks — nothing is removed until you confirm.",
                    button = if (where != null) "Scan $where" else "Scan my phone",
                    enabled = vm.hasAllFiles && vm.busy == null,
                ) { vm.scanOverview() }
                else -> {
                    val cats = vm.junk.orEmpty()
                    val total = cats.filter { it.id in sel }.sumOf { it.size }
                    ResultHero(vm, total, cats.sumOf { it.size }, sel.isNotEmpty() && total >= 0) {
                        confirm = CleanRequest("Clean ${total.fmtSize()}?", "Cleaned recommended items", junkTargets(cats, sel),
                            "Here's exactly what Jigglypuff will remove. Your photos, documents and app data are not touched.")
                    }
                }
            }
        }
        if (vm.scannedAt > 0 && vm.busy != "overview") {
            val cats = vm.junk.orEmpty()
            if (cats.isNotEmpty()) {
                item { SectionTitle("Ready to clean", "tick what you want removed") }
                item {
                    Panel {
                        cats.forEachIndexed { i, c ->
                            if (i > 0) HorizontalDivider(color = MaterialTheme.colorScheme.outlineVariant.copy(alpha = 0.5f))
                            RowItem(c.title, "${plural(c.items.size, "item")} · ${c.desc}", leading = { IconBox(junkIcon(c.id)) },
                                onClick = { sel = if (c.id in sel) sel - c.id else sel + c.id }) {
                                Column(horizontalAlignment = Alignment.End) {
                                    SizeText(c.size)
                                    SafetyBadge(c.safety)
                                }
                                Checkbox(c.id in sel, onCheckedChange = { sel = if (it) sel + c.id else sel - c.id })
                            }
                        }
                    }
                }
            }
            item { SectionTitle("Worth a look", "these need your judgement") }
            item { WorthALook(vm) }
        }
        item { SectionTitle("Storage", if (vm.volumes.size > 1) "tap one to scan only that storage" else null) }
        vm.volumes.forEach { v -> item(key = "vol-" + v.id) { VolumeCard(vm, v) } }
    }
    CleanDialog(vm, confirm, onClose = { confirm = null })
}

@Composable
private fun Hero(title: String, text: String, button: String, enabled: Boolean, onClick: () -> Unit) {
    Panel(hero = true) {
        Column(Modifier.padding(20.dp), verticalArrangement = Arrangement.spacedBy(10.dp)) {
            Text(title, style = MaterialTheme.typography.titleLarge, fontWeight = FontWeight.SemiBold)
            Text(text, style = MaterialTheme.typography.bodyMedium)
            PrimaryButton(onClick = onClick, enabled = enabled, modifier = Modifier.padding(top = 4.dp)) {
                Icon(Icons.Outlined.Search, null, Modifier.padding(end = 8.dp))
                Text(button)
            }
        }
    }
}

@Composable
private fun ResultHero(vm: AppModel, selected: Long, found: Long, canClean: Boolean, onClean: () -> Unit) {
    val res = vm.lastClean
    Panel(hero = true) {
        Column(Modifier.padding(20.dp), verticalArrangement = Arrangement.spacedBy(6.dp)) {
            when {
                res != null -> {
                    BigSize(res.freed)
                    Text("freed. Your phone is a little lighter.", style = MaterialTheme.typography.bodyLarge)
                    if (res.trashed > 0) Text("${plural(res.trashed, "item")} went to the trash — you can put them back from More.", style = MaterialTheme.typography.bodySmall)
                }
                found == 0L -> {
                    Icon(Icons.Outlined.CheckCircle, null, tint = Tones.safe)
                    Text("No junk to clean" + (vm.selected?.let { " on ${it.name}" } ?: ""), style = MaterialTheme.typography.titleLarge, fontWeight = FontWeight.SemiBold)
                    Text("Scanned ${relTime(vm.scannedAt)}. Anything else worth a look is listed below.", style = MaterialTheme.typography.bodyMedium)
                }
                else -> {
                    BigSize(selected)
                    Text(if (selected > 0) "ready to clean safely" else "selected — tick something below", style = MaterialTheme.typography.bodyLarge)
                    Text("${found.fmtSize()} of junk found · scanned ${relTime(vm.scannedAt)}", style = MaterialTheme.typography.bodySmall)
                }
            }
            Row(Modifier.padding(top = 8.dp), horizontalArrangement = Arrangement.spacedBy(10.dp)) {
                if (res == null && found > 0) PrimaryButton(onClick = onClean, enabled = canClean && selected > 0 && vm.busy == null) {
                    Icon(Icons.Outlined.DeleteSweep, null, Modifier.padding(end = 8.dp))
                    Text(if (selected > 0) "Clean ${selected.fmtSize()}" else "Clean")
                }
                SecondaryButton(onClick = { vm.scanOverview() }, enabled = vm.busy == null) { Text("Scan again") }
            }
        }
    }
}

@Composable
private fun BigSize(bytes: Long) {
    val parts = bytes.fmtSize().split(" ")
    Row(verticalAlignment = Alignment.Bottom) {
        Text(parts[0], fontSize = 52.sp, fontWeight = FontWeight.Bold, lineHeight = 52.sp)
        Spacer(Modifier.width(6.dp))
        Text(parts.getOrElse(1) { "" }, fontSize = 22.sp, fontWeight = FontWeight.SemiBold, modifier = Modifier.padding(bottom = 6.dp))
    }
}

@Composable
private fun WorthALook(vm: AppModel) {
    val nav = LocalNav.current
    val start = rememberStart()
    val ctx = androidx.compose.ui.platform.LocalContext.current
    val apps = vm.appsHere(vm.apps.orEmpty())
    val unusedCut = System.currentTimeMillis() - TimeUnit.DAYS.toMillis(vm.settings.unusedDays.toLong())
    val unused = apps.filter { it.canUninstall && it.lastUsed in 1 until unusedCut && it.installed < unusedCut }
    val caches = apps.filter { it.cacheBytes > 50L shl 20 }
    val rows = buildList<@Composable () -> Unit> {
        vm.leftovers?.takeIf { it.isNotEmpty() }?.let { l ->
            add { Look(Icons.Outlined.FolderOff, "Leftovers of ${plural(l.size, "removed app")}", "Folders apps left behind. Review what's safe to delete.", l.sumOf { it.size }) { nav(Tab.Clean, "leftovers") } }
        }
        vm.old?.takeIf { it.isNotEmpty() }?.let { o ->
            add { Look(Icons.Outlined.Download, "${plural(o.size, "old installer")} and archives in Downloads", "Not opened for over ${vm.settings.oldDownloadDays} days.", o.sumOf { it.size }) { nav(Tab.Files, "old") } }
        }
        if (unused.isNotEmpty()) add {
            Look(Icons.Outlined.HourglassEmpty, "${plural(unused.size, "app")} you haven't opened in ${vm.settings.unusedDays} days", unused.take(3).joinToString { it.label }, unused.sumOf { it.size.coerceAtLeast(0) }) { nav(Tab.Apps, "unused") }
        }
        if (caches.isNotEmpty()) add {
            Look(Icons.Outlined.Memory, "${plural(caches.size, "app")} with big caches", "Android lets only you clear these, from each app's settings.", caches.sumOf { it.cacheBytes }) { nav(Tab.Apps, "caches") }
        }
        add { Look(Icons.Outlined.Search, "See what's taking up space" + (vm.selected?.let { " on ${it.name}" } ?: ""), "Find the biggest files.", -1) { nav(Tab.Files, "large") } }
        add { Look(Icons.Outlined.ContentCopy, "Find duplicate files", "Identical photos, videos and downloads.", -1) { nav(Tab.Files, "dups") } }
        add { Look(Icons.Outlined.Apps, "See the apps" + (vm.selected?.let { " on ${it.name}" } ?: ""), "Every app and installer on this storage.", -1) { nav(Tab.Apps, null) } }
    }
    Column(verticalArrangement = Arrangement.spacedBy(12.dp)) {
        if (!vm.hasUsage) Note(Icons.Outlined.Warning, "Allow usage access to see app sizes, big caches and apps you never open.", "Allow") { start(Device.usageIntent(ctx)) }
        Panel {
            rows.forEachIndexed { i, r ->
                if (i > 0) HorizontalDivider(color = MaterialTheme.colorScheme.outlineVariant.copy(alpha = 0.5f))
                r()
            }
        }
    }
}

@Composable
private fun Look(icon: ImageVector, title: String, desc: String, size: Long, onClick: () -> Unit) {
    RowItem(title, desc, leading = { IconBox(icon) }, onClick = onClick) {
        if (size > 0) SizeText(size, muted = true)
        Icon(Icons.AutoMirrored.Outlined.ArrowForward, null, tint = MaterialTheme.colorScheme.onSurfaceVariant)
    }
}

@Composable
fun VolumeCard(vm: AppModel, v: Volume) {
    val on = vm.volumeId == v.id
    val share = v.used.toFloat() / v.total.coerceAtLeast(1)
    // A phone needs a couple of GB free to update; a card just shouldn't be nearly full.
    val low = share > 0.95f || (v.primary && v.free < 2L * 1024 * 1024 * 1024)
    Panel(selected = on, onClick = { vm.pickVolume(if (on) "" else v.id) }) {
        Column(Modifier.padding(16.dp), verticalArrangement = Arrangement.spacedBy(10.dp)) {
            Row(verticalAlignment = Alignment.CenterVertically) {
                Icon(volumeIcon(v), null)
                Spacer(Modifier.width(10.dp))
                Text(v.name, style = MaterialTheme.typography.titleSmall, modifier = Modifier.weight(1f))
                if (low) Badge("Low on space", Tones.bad)
                else if (on) Badge("Scanning only this", MaterialTheme.colorScheme.primary)
            }
            Box(Modifier.fillMaxWidth().height(8.dp).clip(RoundedCornerShape(4.dp)).background(MaterialTheme.colorScheme.surfaceVariant)) {
                Box(Modifier.fillMaxWidth(share.coerceIn(0f, 1f)).fillMaxHeight().background(if (low) Tones.bad else MaterialTheme.colorScheme.primary))
            }
            Row {
                Text("${v.free.fmtSize()} free", style = MaterialTheme.typography.bodySmall, modifier = Modifier.weight(1f))
                Text("of ${v.total.fmtSize()}", style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
            }
            if (on) TextLinkButton(onClick = { vm.pickVolume("") }) { Text("Scan all storage instead") }
        }
    }
}

