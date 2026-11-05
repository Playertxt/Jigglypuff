package com.kabir.jigglypuff.ui

import android.content.Intent
import android.net.Uri
import android.provider.DocumentsContract
import androidx.activity.compose.rememberLauncherForActivityResult
import androidx.activity.result.contract.ActivityResultContracts
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.padding
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.outlined.Add
import androidx.compose.material.icons.outlined.CheckCircle
import androidx.compose.material.icons.outlined.Close
import androidx.compose.material.icons.outlined.Code
import androidx.compose.material.icons.outlined.DeleteForever
import androidx.compose.material.icons.outlined.Folder
import androidx.compose.material.icons.outlined.History
import androidx.compose.material.icons.outlined.Lock
import androidx.compose.material.icons.outlined.QueryStats
import androidx.compose.material.icons.outlined.Restore
import androidx.compose.material3.AlertDialog
import androidx.compose.material3.Button
import androidx.compose.material3.DropdownMenu
import androidx.compose.material3.DropdownMenuItem
import androidx.compose.material3.FilledTonalButton
import androidx.compose.material3.HorizontalDivider
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedButton
import androidx.compose.material3.SegmentedButton
import androidx.compose.material3.SegmentedButtonDefaults
import androidx.compose.material3.SingleChoiceSegmentedButtonRow
import androidx.compose.material3.Switch
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberCoroutineScope
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.unit.dp
import com.kabir.jigglypuff.AppModel
import com.kabir.jigglypuff.BuildConfig
import com.kabir.jigglypuff.engine.Device
import com.kabir.jigglypuff.engine.fmtSize
import kotlinx.coroutines.launch

@Composable
fun MoreScreen(vm: AppModel) {
    val ctx = LocalContext.current
    val start = rememberStart()
    val snack = LocalSnack.current
    val scope = rememberCoroutineScope()
    var askEmpty by remember { mutableStateOf(false) }
    val s = vm.settings
    val pickFolder = rememberLauncherForActivityResult(ActivityResultContracts.OpenDocumentTree()) { uri ->
        val path = uri?.let { treeToPath(vm, it) }
        if (path != null && path !in s.protected) vm.updateSettings(s.copy(protected = s.protected + path))
    }
    val trash = vm.historyList.filter { it.restorable }

    ScreenList {
        // Trash and history -----------------------------------------------------------------
        item { SectionTitle("Trash and history") }
        item {
            Panel {
                RowItem("Jigglypuff's trash", if (trash.isEmpty()) "Empty" else "${trash.sumOf { it.trashBytes }.fmtSize()} from ${plural(trash.size, "clean-up")} · emptied after ${s.trashDays} days",
                    leading = { IconBox(Icons.Outlined.DeleteForever) }) {
                    if (trash.isNotEmpty()) OutlinedButton(onClick = { askEmpty = true }, enabled = vm.busy == null) { Text("Empty") }
                }
                if (vm.historyList.isEmpty()) {
                    HorizontalDivider(color = MaterialTheme.colorScheme.outlineVariant.copy(alpha = 0.5f))
                    RowItem("No clean-ups yet", "Everything you remove is listed here.", leading = { IconBox(Icons.Outlined.History) })
                }
                vm.historyList.take(50).forEach { e ->
                    HorizontalDivider(color = MaterialTheme.colorScheme.outlineVariant.copy(alpha = 0.5f))
                    val detail = buildList {
                        add(relTime(e.time))
                        if (e.freed > 0) add("freed ${e.freed.fmtSize()}")
                        if (e.trashed > 0) add("${e.trashed} to trash")
                        if (e.removed > 0) add("${e.removed} deleted")
                        if (e.failed > 0) add("${e.failed} failed")
                        when (e.state) { "restored" -> add("put back"); "emptied" -> add("trash emptied") }
                    }.joinToString(" · ")
                    RowItem(e.title, detail, leading = { IconBox(Icons.Outlined.History) }) {
                        if (e.restorable) FilledTonalButton(onClick = {
                            vm.restore(e) { n -> scope.launch { snack.showSnackbar("Put back ${plural(n, "item")}") } }
                        }, enabled = vm.busy == null) {
                            Icon(Icons.Outlined.Restore, null, Modifier.padding(end = 6.dp))
                            Text("Put back")
                        }
                    }
                }
            }
        }

        // Permissions -----------------------------------------------------------------------
        item { SectionTitle("Permissions") }
        item {
            Panel {
                PermRow("All files access", "Lets Jigglypuff look through your storage for junk, large files and duplicates.", vm.hasAllFiles, Icons.Outlined.Lock) { start(Device.allFilesIntent(ctx)) }
                HorizontalDivider(color = MaterialTheme.colorScheme.outlineVariant.copy(alpha = 0.5f))
                PermRow("Usage access", "Lets Jigglypuff see app sizes, caches and when you last opened each app.", vm.hasUsage, Icons.Outlined.QueryStats) { start(Device.usageIntent(ctx)) }
            }
        }

        // Settings --------------------------------------------------------------------------
        item { SectionTitle("Settings") }
        item {
            Panel {
                Column(Modifier.padding(16.dp), verticalArrangement = Arrangement.spacedBy(6.dp)) {
                    Text("Theme", style = MaterialTheme.typography.bodyLarge)
                    val themes = listOf("system" to "System", "light" to "Light", "dark" to "Dark")
                    SingleChoiceSegmentedButtonRow {
                        themes.forEachIndexed { i, (k, l) ->
                            SegmentedButton(selected = s.theme == k, onClick = { vm.updateSettings(s.copy(theme = k)) }, shape = SegmentedButtonDefaults.itemShape(i, themes.size)) { Text(l) }
                        }
                    }
                }
                HorizontalDivider(color = MaterialTheme.colorScheme.outlineVariant.copy(alpha = 0.5f))
                RowItem("Use the trash", "Removed files, leftovers and APKs go to a hidden trash folder first, so you can put them back.") {
                    Switch(s.useTrash, { vm.updateSettings(s.copy(useTrash = it)) })
                }
                HorizontalDivider(color = MaterialTheme.colorScheme.outlineVariant.copy(alpha = 0.5f))
                SettingChoice("Empty the trash after", "${s.trashDays} days", listOf(3, 7, 14, 30).map { it to "$it days" }) { vm.updateSettings(s.copy(trashDays = it)) }
                HorizontalDivider(color = MaterialTheme.colorScheme.outlineVariant.copy(alpha = 0.5f))
                SettingChoice("Skip temporary files newer than", "${s.tempMinAgeHours} hours", listOf(1, 24, 72, 168).map { it to "$it hours" }) { vm.updateSettings(s.copy(tempMinAgeHours = it)) }
                HorizontalDivider(color = MaterialTheme.colorScheme.outlineVariant.copy(alpha = 0.5f))
                SettingChoice("An app is unused after", "${s.unusedDays} days", listOf(30, 60, 90, 180, 365).map { it to "$it days" }) { vm.updateSettings(s.copy(unusedDays = it)) }
            }
        }
        item {
            Panel {
                RowItem("Protected folders", "Jigglypuff never removes anything inside these.", leading = { IconBox(Icons.Outlined.Lock) }) {
                    IconButton(onClick = { pickFolder.launch(null) }) { Icon(Icons.Outlined.Add, "Add a folder") }
                }
                s.protected.forEach { p ->
                    HorizontalDivider(color = MaterialTheme.colorScheme.outlineVariant.copy(alpha = 0.5f))
                    RowItem(p.substringAfterLast('/'), prettyPath(vm, p), leading = { IconBox(Icons.Outlined.Folder) }) {
                        IconButton(onClick = { vm.updateSettings(s.copy(protected = s.protected - p)) }) { Icon(Icons.Outlined.Close, "Remove") }
                    }
                }
            }
        }

        // About -----------------------------------------------------------------------------
        item { SectionTitle("About") }
        item {
            Panel {
                RowItem("Jigglypuff ${BuildConfig.VERSION_NAME}", "A careful, minimal phone cleaner. Nothing is removed without showing you first.", leading = { IconBox(Icons.Outlined.CheckCircle) })
                HorizontalDivider(color = MaterialTheme.colorScheme.outlineVariant.copy(alpha = 0.5f))
                RowItem("Source code", "github.com/Playertxt/Jigglypuff", leading = { IconBox(Icons.Outlined.Code) },
                    onClick = { start(Intent(Intent.ACTION_VIEW, Uri.parse("https://github.com/Playertxt/Jigglypuff"))) })
            }
        }
    }

    if (askEmpty) AlertDialog(
        onDismissRequest = { askEmpty = false },
        title = { Text("Empty the trash?") },
        text = { Text("${trash.sumOf { it.trashBytes }.fmtSize()} will be deleted for good. This can't be undone.") },
        confirmButton = { Button(onClick = { askEmpty = false; vm.emptyTrash() }) { Text("Empty trash") } },
        dismissButton = { TextButton(onClick = { askEmpty = false }) { Text("Cancel") } },
    )
}

@Composable
private fun PermRow(title: String, desc: String, granted: Boolean, icon: androidx.compose.ui.graphics.vector.ImageVector, onAllow: () -> Unit) {
    RowItem(title, desc, leading = { IconBox(icon) }) {
        if (granted) Badge("Allowed", Tones.safe) else Button(onClick = onAllow) { Text("Allow") }
    }
}

@Composable
private fun <T> SettingChoice(title: String, value: String, options: List<Pair<T, String>>, onPick: (T) -> Unit) {
    var open by remember { mutableStateOf(false) }
    RowItem(title, null, onClick = { open = true }) {
        Box {
            Text(value, color = MaterialTheme.colorScheme.primary)
            DropdownMenu(open, { open = false }) {
                options.forEach { (v, l) -> DropdownMenuItem(text = { Text(l) }, onClick = { onPick(v); open = false }) }
            }
        }
    }
}

/** Turns a folder picked in Android's file picker into a real path, e.g. primary:DCIM/Camera → /storage/emulated/0/DCIM/Camera. */
private fun treeToPath(vm: AppModel, uri: Uri): String? {
    val id = runCatching { DocumentsContract.getTreeDocumentId(uri) }.getOrNull() ?: return null
    val vol = id.substringBefore(':')
    val rel = id.substringAfter(':', "")
    val root = if (vol == "primary") vm.volumes.firstOrNull { it.primary } else vm.volumes.firstOrNull { it.root.name == vol }
    return root?.let { if (rel.isEmpty()) it.root.path else "${it.root.path}/$rel" }
}

