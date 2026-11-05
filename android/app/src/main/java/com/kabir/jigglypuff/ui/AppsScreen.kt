package com.kabir.jigglypuff.ui

import android.os.storage.StorageManager
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.text.KeyboardOptions
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.outlined.Apps
import androidx.compose.material.icons.outlined.Delete
import androidx.compose.material.icons.outlined.Inventory2
import androidx.compose.material.icons.outlined.Search
import androidx.compose.material.icons.outlined.SdCard
import androidx.compose.material.icons.outlined.TaskAlt
import androidx.compose.material.icons.outlined.Warning
import androidx.compose.material3.Button
import androidx.compose.material3.Checkbox
import androidx.compose.material3.DropdownMenu
import androidx.compose.material3.DropdownMenuItem
import androidx.compose.material3.FilledTonalButton
import androidx.compose.material3.HorizontalDivider
import androidx.compose.material3.Icon
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedButton
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.saveable.rememberSaveable
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.text.input.ImeAction
import androidx.compose.ui.unit.dp
import com.kabir.jigglypuff.AppModel
import com.kabir.jigglypuff.engine.ApkStatus
import com.kabir.jigglypuff.engine.AppInfo
import com.kabir.jigglypuff.engine.Device
import com.kabir.jigglypuff.engine.Target
import com.kabir.jigglypuff.engine.fmtSize
import java.util.concurrent.TimeUnit

@Composable
fun AppsScreen(vm: AppModel, sub: String?) {
    val tabs = listOf("Apps", "Unused", "Caches", "APK files")
    var tab by rememberSaveable(sub) { mutableStateOf(when (sub) { "unused" -> 1; "caches" -> 2; "apks" -> 3; else -> 0 }) }
    LaunchedEffect(Unit) { if (vm.apps == null) vm.loadApps() }
    Column {
        SubTabs(tabs, tab) { tab = it }
        when (tab) {
            0 -> AppList(vm)
            1 -> UnusedApps(vm)
            2 -> CacheList(vm)
            else -> ApkList(vm)
        }
    }
}

@Composable
private fun UsageNote(vm: AppModel) {
    if (vm.hasUsage) return
    val ctx = LocalContext.current
    val start = rememberStart()
    Note(Icons.Outlined.Warning, "Allow usage access so Jigglypuff can show app sizes and when you last opened each app.", "Allow") { start(Device.usageIntent(ctx)) }
}

private fun meta(a: AppInfo): String = listOfNotNull(
    a.version.takeIf { it.isNotEmpty() }?.let { "v$it" },
    if (a.lastUsed > 0) "used ${relTime(a.lastUsed)}" else null,
    "installed ${relTime(a.installed)}",
).joinToString(" · ")

@Composable
private fun AppRow(vm: AppModel, a: AppInfo, right: @Composable () -> Unit = { UninstallButton(a) }) {
    val start = rememberStart()
    val onSd = a.storageUuid != null && a.storageUuid != StorageManager.UUID_DEFAULT
    RowItem(a.label, meta(a), leading = { AppIcon(a.pkg, a.label) }, onClick = { start(Device.appSettingsIntent(a.pkg)) }) {
        Column(horizontalAlignment = Alignment.End) {
            SizeText(a.size, muted = a.size < 0)
            if (onSd && vm.selected == null) Badge("SD card", MaterialTheme.colorScheme.primary)
            if (a.system) Badge("System", MaterialTheme.colorScheme.outline)
        }
        right()
    }
}

@Composable
private fun UninstallButton(a: AppInfo) {
    val start = rememberStart()
    if (a.canUninstall) OutlinedButton(onClick = { start(Device.uninstallIntent(a.pkg)) }, contentPadding = androidx.compose.foundation.layout.PaddingValues(horizontal = 12.dp)) { Text("Uninstall") }
}

@Composable
private fun AppList(vm: AppModel) {
    var query by rememberSaveable { mutableStateOf("") }
    var sort by rememberSaveable { mutableStateOf("name") }
    var sortOpen by remember { mutableStateOf(false) }
    val sel = vm.selected
    ScreenList {
        item { UsageNote(vm) }
        if (sel != null && sel.uuid == null) item {
            Note(Icons.Outlined.SdCard, "Android can't run apps from ${sel.name} unless it's set up as internal storage. The APK files tab shows the installers saved on it.")
        }
        item {
            Row(verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                OutlinedTextField(query, { query = it }, Modifier.weight(1f), placeholder = { Text("Search apps") }, singleLine = true,
                    leadingIcon = { Icon(Icons.Outlined.Search, null) }, keyboardOptions = KeyboardOptions(imeAction = ImeAction.Search))
                androidx.compose.foundation.layout.Box {
                    TextButton(onClick = { sortOpen = true }) { Text(when (sort) { "size" -> "By size"; "used" -> "By last use"; else -> "By name" }) }
                    DropdownMenu(sortOpen, { sortOpen = false }) {
                        listOf("name" to "By name", "size" to "By size", "used" to "By last use").forEach { (k, l) ->
                            DropdownMenuItem(text = { Text(l) }, onClick = { sort = k; sortOpen = false })
                        }
                    }
                }
            }
        }
        val all = vm.apps
        if (all == null) {
            item { ScanningCard(vm, "Reading your apps…") }
            return@ScreenList
        }
        val q = query.trim().lowercase()
        val shown = vm.appsHere(all).filter { q.isEmpty() || it.label.lowercase().contains(q) || it.pkg.contains(q) }.let { l ->
            when (sort) {
                "size" -> l.sortedByDescending { it.size }
                "used" -> l.sortedBy { if (it.lastUsed == 0L) Long.MAX_VALUE else it.lastUsed }
                else -> l
            }
        }
        val total = shown.sumOf { it.size.coerceAtLeast(0) }
        item {
            Text(
                "${plural(shown.size, "app")}${if (total > 0) " · ${total.fmtSize()}" else ""}" + (sel?.let { " on ${it.name}" } ?: ""),
                style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant, modifier = Modifier.padding(horizontal = 4.dp),
            )
        }
        if (shown.isEmpty()) item { EmptyState(Icons.Outlined.Apps, "No apps here", if (q.isNotEmpty()) "Nothing matches “$query”." else "No apps are installed on this storage.") }
        else item {
            Panel {
                shown.forEachIndexed { i, a ->
                    if (i > 0) HorizontalDivider(color = MaterialTheme.colorScheme.outlineVariant.copy(alpha = 0.5f))
                    AppRow(vm, a)
                }
            }
        }
    }
}

@Composable
private fun UnusedApps(vm: AppModel) {
    val cut = System.currentTimeMillis() - TimeUnit.DAYS.toMillis(vm.settings.unusedDays.toLong())
    ScreenList {
        item { UsageNote(vm) }
        val all = vm.apps
        if (all == null) {
            item { ScanningCard(vm, "Reading your apps…") }
            return@ScreenList
        }
        if (!vm.hasUsage) return@ScreenList
        val list = vm.appsHere(all).filter { it.canUninstall && it.installed < cut && it.lastUsed < cut }.sortedByDescending { it.size }
        item {
            Text("Apps you haven't opened in ${vm.settings.unusedDays} days. Uninstall the ones you don't need; Android asks you to confirm each one.",
                style = MaterialTheme.typography.bodyMedium, color = MaterialTheme.colorScheme.onSurfaceVariant, modifier = Modifier.padding(horizontal = 4.dp))
        }
        if (list.isEmpty()) item { EmptyState(Icons.Outlined.TaskAlt, "You use all your apps", "Every app here was opened in the last ${vm.settings.unusedDays} days.") }
        else item {
            Panel {
                list.forEachIndexed { i, a ->
                    if (i > 0) HorizontalDivider(color = MaterialTheme.colorScheme.outlineVariant.copy(alpha = 0.5f))
                    AppRow(vm, a)
                }
            }
        }
    }
}

@Composable
private fun CacheList(vm: AppModel) {
    val start = rememberStart()
    ScreenList {
        item { UsageNote(vm) }
        val all = vm.apps
        if (all == null) {
            item { ScanningCard(vm, "Reading your apps…") }
            return@ScreenList
        }
        if (!vm.hasUsage) return@ScreenList
        val list = vm.appsHere(all).filter { it.cacheBytes > 1L shl 20 }.sortedByDescending { it.cacheBytes }
        item {
            Text("Since Android 6, only you can clear another app's cache. Tap Clear to open that app's storage page, then tap “Clear cache”. The app rebuilds what it needs.",
                style = MaterialTheme.typography.bodyMedium, color = MaterialTheme.colorScheme.onSurfaceVariant, modifier = Modifier.padding(horizontal = 4.dp))
        }
        item {
            Text("${list.sumOf { it.cacheBytes }.fmtSize()} of caches in ${plural(list.size, "app")}",
                style = MaterialTheme.typography.titleSmall, modifier = Modifier.padding(horizontal = 4.dp))
        }
        if (list.isEmpty()) item { EmptyState(Icons.Outlined.TaskAlt, "No big caches", "No app keeps more than 1 MB of cache.") }
        else item {
            Panel {
                list.forEachIndexed { i, a ->
                    if (i > 0) HorizontalDivider(color = MaterialTheme.colorScheme.outlineVariant.copy(alpha = 0.5f))
                    RowItem(a.label, "Cache ${a.cacheBytes.fmtSize()} · app and data ${a.size.fmtSize()}", leading = { AppIcon(a.pkg, a.label) }) {
                        FilledTonalButton(onClick = { start(Device.appSettingsIntent(a.pkg)) }) { Text("Clear") }
                    }
                }
            }
        }
    }
}

@Composable
private fun ApkList(vm: AppModel) {
    var sel by remember(vm.apks) {
        mutableStateOf(vm.apks.orEmpty().filter { it.status == ApkStatus.INSTALLED || it.status == ApkStatus.OLDER_THAN_INSTALLED }.map { it.path }.toSet())
    }
    var confirm by remember { mutableStateOf<CleanRequest?>(null) }
    LaunchedEffect(vm.hasAllFiles, vm.volumeId) { if (vm.hasAllFiles && vm.apks == null && vm.busy == null) vm.scanApks() }
    ScreenList {
        item { ScopeLine(vm, "APK files") }
        item {
            if (NeedsAllFiles(vm)) return@item
            val list = vm.apks
            when {
                vm.busy == "apks" -> ScanningCard(vm, "Looking for APK files…")
                list == null -> {}
                list.isEmpty() -> EmptyState(Icons.Outlined.Inventory2, "No APK files", "There are no app installers saved on this storage.", "Scan again") { vm.scanApks() }
                else -> Column(verticalArrangement = Arrangement.spacedBy(12.dp)) {
                    Text("App installers saved on your storage. Ones for apps you already have are ticked.",
                        style = MaterialTheme.typography.bodyMedium, color = MaterialTheme.colorScheme.onSurfaceVariant, modifier = Modifier.padding(horizontal = 4.dp))
                    Row(horizontalArrangement = Arrangement.spacedBy(10.dp)) {
                        val size = list.filter { it.path in sel }.sumOf { it.size }
                        Button(enabled = sel.isNotEmpty(), onClick = {
                            confirm = CleanRequest("Remove ${plural(sel.size, "installer")}?", "Removed APK files",
                                list.filter { it.path in sel }.map { Target(it.path, it.size, trash = true) })
                        }) {
                            Icon(Icons.Outlined.Delete, null, Modifier.padding(end = 8.dp))
                            Text(if (sel.isEmpty()) "Remove selected" else "Remove ${size.fmtSize()}")
                        }
                        OutlinedButton(onClick = { vm.scanApks() }, enabled = vm.busy == null) { Text("Scan again") }
                    }
                    Panel {
                        list.forEachIndexed { i, a ->
                            if (i > 0) HorizontalDivider(color = MaterialTheme.colorScheme.outlineVariant.copy(alpha = 0.5f))
                            RowItem(
                                a.label + (a.versionName.takeIf { it.isNotEmpty() }?.let { " $it" } ?: ""),
                                "${prettyPath(vm, a.path)} · ${relTime(a.modified)}",
                                leading = { ApkIcon(a.path, a.label) },
                                onClick = { sel = if (a.path in sel) sel - a.path else sel + a.path },
                            ) {
                                Column(horizontalAlignment = Alignment.End) {
                                    SizeText(a.size)
                                    when (a.status) {
                                        ApkStatus.INSTALLED -> Badge("Installed", Tones.safe)
                                        ApkStatus.OLDER_THAN_INSTALLED -> Badge("Older than installed", Tones.safe)
                                        ApkStatus.UPDATE -> Badge("Newer version", MaterialTheme.colorScheme.primary)
                                        ApkStatus.NOT_INSTALLED -> Badge("Not installed", MaterialTheme.colorScheme.outline)
                                        ApkStatus.BROKEN -> Badge("Damaged", Tones.bad)
                                    }
                                }
                                Checkbox(a.path in sel, onCheckedChange = { sel = if (it) sel + a.path else sel - a.path })
                            }
                        }
                    }
                }
            }
        }
    }
    CleanDialog(vm, confirm, onClose = { confirm = null })
}

