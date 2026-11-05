package com.kabir.jigglypuff.ui

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.padding
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.automirrored.outlined.InsertDriveFile
import androidx.compose.material.icons.outlined.Archive
import androidx.compose.material.icons.outlined.AudioFile
import androidx.compose.material.icons.outlined.ContentCopy
import androidx.compose.material.icons.outlined.Delete
import androidx.compose.material.icons.outlined.Download
import androidx.compose.material.icons.outlined.Image
import androidx.compose.material.icons.outlined.Inventory2
import androidx.compose.material.icons.outlined.OpenInNew
import androidx.compose.material.icons.outlined.Search
import androidx.compose.material.icons.outlined.TaskAlt
import androidx.compose.material.icons.outlined.VideoFile
import androidx.compose.material3.Button
import androidx.compose.material3.Checkbox
import androidx.compose.material3.DropdownMenu
import androidx.compose.material3.DropdownMenuItem
import androidx.compose.material3.HorizontalDivider
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedButton
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.saveable.rememberSaveable
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.vector.ImageVector
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.unit.dp
import com.kabir.jigglypuff.AppModel
import com.kabir.jigglypuff.engine.DupGroup
import com.kabir.jigglypuff.engine.Item
import com.kabir.jigglypuff.engine.Target
import com.kabir.jigglypuff.engine.fmtSize

fun fileIcon(name: String): ImageVector = when (name.substringAfterLast('.', "").lowercase()) {
    "jpg", "jpeg", "png", "gif", "webp", "heic", "heif", "bmp" -> Icons.Outlined.Image
    "mp4", "mkv", "mov", "avi", "webm", "3gp", "m4v" -> Icons.Outlined.VideoFile
    "mp3", "m4a", "wav", "flac", "ogg", "opus", "aac" -> Icons.Outlined.AudioFile
    "zip", "rar", "7z", "tar", "gz", "xz", "bz2", "iso", "img" -> Icons.Outlined.Archive
    "apk", "apks", "xapk" -> Icons.Outlined.Inventory2
    else -> Icons.AutoMirrored.Outlined.InsertDriveFile
}

@Composable
fun FilesScreen(vm: AppModel, sub: String?) {
    var tab by rememberSaveable(sub) { mutableStateOf(when (sub) { "old" -> 1; "dups" -> 2; else -> 0 }) }
    Column {
        SubTabs(listOf("Large files", "Old downloads", "Duplicates"), tab) { tab = it }
        when (tab) {
            0 -> LargeTab(vm)
            1 -> OldTab(vm)
            else -> DupsTab(vm)
        }
    }
}

@Composable
private fun <T> Choice(label: String, options: List<Pair<T, String>>, onPick: (T) -> Unit) {
    var open by remember { mutableStateOf(false) }
    Box {
        OutlinedButton(onClick = { open = true }) { Text(label) }
        DropdownMenu(open, { open = false }) {
            options.forEach { (v, l) -> DropdownMenuItem(text = { Text(l) }, onClick = { onPick(v); open = false }) }
        }
    }
}

/** A selectable list of files with an Open button on each. */
@Composable
private fun FileList(vm: AppModel, files: List<Item>, sel: Set<String>, onSel: (Set<String>) -> Unit) {
    val ctx = LocalContext.current
    Panel {
        files.forEachIndexed { i, f ->
            if (i > 0) HorizontalDivider(color = MaterialTheme.colorScheme.outlineVariant.copy(alpha = 0.5f))
            RowItem(f.name, "${prettyPath(vm, f.parent)} · ${relTime(f.modified)}", leading = { IconBox(fileIcon(f.name)) },
                onClick = { onSel(if (f.path in sel) sel - f.path else sel + f.path) }) {
                SizeText(f.size)
                IconButton(onClick = { openFile(ctx, f.path) }) { Icon(Icons.Outlined.OpenInNew, "Open", tint = MaterialTheme.colorScheme.onSurfaceVariant) }
                Checkbox(f.path in sel, onCheckedChange = { onSel(if (it) sel + f.path else sel - f.path) })
            }
        }
    }
}

@Composable
private fun RemoveBar(sel: Set<String>, files: List<Item>, onRemove: () -> Unit, extra: @Composable () -> Unit = {}) {
    Row(horizontalArrangement = Arrangement.spacedBy(10.dp), verticalAlignment = Alignment.CenterVertically) {
        val size = files.filter { it.path in sel }.sumOf { it.size }
        Button(enabled = sel.isNotEmpty(), onClick = onRemove) {
            Icon(Icons.Outlined.Delete, null, Modifier.padding(end = 8.dp))
            Text(if (sel.isEmpty()) "Remove selected" else "Remove ${size.fmtSize()}")
        }
        extra()
    }
}

@Composable
private fun LargeTab(vm: AppModel) {
    var sel by remember(vm.large) { mutableStateOf(setOf<String>()) }
    var confirm by remember { mutableStateOf<CleanRequest?>(null) }
    ScreenList {
        item { ScopeLine(vm, "Large files") }
        item {
            if (NeedsAllFiles(vm)) return@item
            Column(verticalArrangement = Arrangement.spacedBy(12.dp)) {
                Row(horizontalArrangement = Arrangement.spacedBy(10.dp), verticalAlignment = Alignment.CenterVertically) {
                    val mb = vm.settings.largeMinMB
                    Choice("Bigger than ${if (mb >= 1024) "${mb / 1024} GB" else "$mb MB"}", listOf(50, 100, 250, 500, 1024, 4096).map { it to (if (it >= 1024) "Bigger than ${it / 1024} GB" else "Bigger than $it MB") }) {
                        vm.updateSettings(vm.settings.copy(largeMinMB = it))
                    }
                    Button(onClick = { vm.scanLarge() }, enabled = vm.busy == null) {
                        Icon(Icons.Outlined.Search, null, Modifier.padding(end = 8.dp))
                        Text(if (vm.large == null) "Scan" else "Scan again")
                    }
                }
                val list = vm.large
                when {
                    vm.busy == "large" -> ScanningCard(vm, "Looking for large files…")
                    list == null -> EmptyState(Icons.Outlined.Search, "Find what's taking up space", "Jigglypuff lists the biggest files on " + (vm.selected?.name ?: "all your storage") + ". Nothing is removed unless you pick it.")
                    list.isEmpty() -> EmptyState(Icons.Outlined.TaskAlt, "No large files", "Nothing is bigger than ${vm.settings.largeMinMB} MB.")
                    else -> {
                        Text("${plural(list.size, "file")} · ${list.sumOf { it.size }.fmtSize()}", style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant, modifier = Modifier.padding(horizontal = 4.dp))
                        RemoveBar(sel, list, { confirm = CleanRequest("Remove ${plural(sel.size, "file")}?", "Removed large files", list.filter { it.path in sel }.map { Target(it.path, it.size, trash = true) }) })
                        FileList(vm, list, sel) { sel = it }
                    }
                }
            }
        }
    }
    CleanDialog(vm, confirm, onClose = { confirm = null })
}

@Composable
private fun OldTab(vm: AppModel) {
    var sel by remember(vm.old) { mutableStateOf(setOf<String>()) }
    var confirm by remember { mutableStateOf<CleanRequest?>(null) }
    LaunchedEffect(vm.hasAllFiles, vm.volumeId) { if (vm.hasAllFiles && vm.old == null && vm.busy == null) vm.scanOld() }
    ScreenList {
        item { ScopeLine(vm, "Old downloads") }
        item {
            if (NeedsAllFiles(vm)) return@item
            Column(verticalArrangement = Arrangement.spacedBy(12.dp)) {
                Text("Installers, archives and disk images in Downloads that you haven't touched in a while.",
                    style = MaterialTheme.typography.bodyMedium, color = MaterialTheme.colorScheme.onSurfaceVariant, modifier = Modifier.padding(horizontal = 4.dp))
                Choice("Not opened for ${vm.settings.oldDownloadDays} days", listOf(30, 60, 90, 180, 365).map { it to "Not opened for $it days" }) {
                    vm.updateSettings(vm.settings.copy(oldDownloadDays = it))
                    vm.scanOld()
                }
                val list = vm.old
                when {
                    vm.busy == "old" || vm.busy == "overview" -> ScanningCard(vm, "Checking Downloads…")
                    list == null -> {}
                    list.isEmpty() -> EmptyState(Icons.Outlined.Download, "Downloads is tidy", "No old installers or archives found.")
                    else -> {
                        RemoveBar(sel, list, { confirm = CleanRequest("Remove ${plural(sel.size, "download")}?", "Removed old downloads", list.filter { it.path in sel }.map { Target(it.path, it.size, trash = true) }) }) {
                            androidx.compose.material3.TextButton(onClick = { sel = if (sel.size == list.size) emptySet() else list.map { it.path }.toSet() }) {
                                Text(if (sel.size == list.size) "Select none" else "Select all")
                            }
                        }
                        FileList(vm, list, sel) { sel = it }
                    }
                }
            }
        }
    }
    CleanDialog(vm, confirm, onClose = { confirm = null })
}

/** Ticks every copy except the oldest one in each group. */
private fun autoSelect(groups: List<DupGroup>) = groups.flatMap { g -> g.files.sortedBy { it.modified }.drop(1).map { it.path } }.toSet()

@Composable
private fun DupsTab(vm: AppModel) {
    var sel by remember(vm.dups) { mutableStateOf(autoSelect(vm.dups.orEmpty())) }
    var confirm by remember { mutableStateOf<CleanRequest?>(null) }
    val ctx = LocalContext.current
    ScreenList {
        item { ScopeLine(vm, "Duplicates") }
        item {
            if (NeedsAllFiles(vm)) return@item
            Column(verticalArrangement = Arrangement.spacedBy(12.dp)) {
                Row(horizontalArrangement = Arrangement.spacedBy(10.dp), verticalAlignment = Alignment.CenterVertically) {
                    val kb = vm.settings.dupMinKB
                    Choice("Bigger than ${if (kb >= 1024) "${kb / 1024} MB" else "$kb KB"}", listOf(100 to "Bigger than 100 KB", 1024 to "Bigger than 1 MB", 10240 to "Bigger than 10 MB", 102400 to "Bigger than 100 MB")) {
                        vm.updateSettings(vm.settings.copy(dupMinKB = it))
                    }
                    Button(onClick = { vm.scanDups() }, enabled = vm.busy == null) {
                        Icon(Icons.Outlined.ContentCopy, null, Modifier.padding(end = 8.dp))
                        Text(if (vm.dups == null) "Find duplicates" else "Scan again")
                    }
                }
                val groups = vm.dups
                when {
                    vm.busy == "dups" -> ScanningCard(vm, "Comparing files…")
                    groups == null -> EmptyState(Icons.Outlined.ContentCopy, "Find identical files", "Files are compared by their content (SHA-256), not their names. Jigglypuff always keeps at least one copy.")
                    groups.isEmpty() -> EmptyState(Icons.Outlined.TaskAlt, "No duplicates", "Every file is one of a kind.")
                    else -> {
                        val all = groups.flatMap { it.files }
                        // Never every copy of a file.
                        val keepsOne = groups.all { g -> g.files.any { it.path !in sel } }
                        Text("${plural(groups.size, "group")} · ${groups.sumOf { it.wasted }.fmtSize()} wasted. The oldest copy of each is kept.",
                            style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant, modifier = Modifier.padding(horizontal = 4.dp))
                        RemoveBar(if (keepsOne) sel else emptySet(), all, {
                            confirm = CleanRequest("Remove ${plural(sel.size, "copy")}?", "Removed duplicates", all.filter { it.path in sel }.map { Target(it.path, it.size, trash = true) },
                                "One copy of every file stays where it is.")
                        })
                        if (!keepsOne) Text("Keep at least one copy of each file.", color = Tones.caution, style = MaterialTheme.typography.bodySmall)
                        groups.take(200).forEach { g ->
                            Panel {
                                Text("${g.files.size} copies · ${g.size.fmtSize()} each", style = MaterialTheme.typography.titleSmall, modifier = Modifier.padding(start = 16.dp, top = 12.dp))
                                g.files.forEach { f ->
                                    RowItem(f.name, "${prettyPath(vm, f.parent)} · ${relTime(f.modified)}", leading = { IconBox(fileIcon(f.name)) },
                                        onClick = { sel = if (f.path in sel) sel - f.path else sel + f.path }) {
                                        IconButton(onClick = { openFile(ctx, f.path) }) { Icon(Icons.Outlined.OpenInNew, "Open", tint = MaterialTheme.colorScheme.onSurfaceVariant) }
                                        Checkbox(f.path in sel, onCheckedChange = { sel = if (it) sel + f.path else sel - f.path })
                                    }
                                }
                            }
                        }
                    }
                }
            }
        }
    }
    CleanDialog(vm, confirm, onClose = { confirm = null })
}
