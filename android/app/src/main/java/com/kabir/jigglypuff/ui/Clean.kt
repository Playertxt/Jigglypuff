package com.kabir.jigglypuff.ui

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.outlined.Delete
import androidx.compose.material.icons.outlined.ExpandLess
import androidx.compose.material.icons.outlined.ExpandMore
import androidx.compose.material.icons.outlined.FolderOff
import androidx.compose.material.icons.outlined.Lock
import androidx.compose.material.icons.outlined.TaskAlt
import androidx.compose.material3.Button
import androidx.compose.material3.Checkbox
import androidx.compose.material3.HorizontalDivider
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedButton
import androidx.compose.material3.PrimaryTabRow
import androidx.compose.material3.Tab
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
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.unit.dp
import com.kabir.jigglypuff.AppModel
import com.kabir.jigglypuff.engine.Device
import com.kabir.jigglypuff.engine.Target
import com.kabir.jigglypuff.engine.fmtSize

@Composable
fun SubTabs(titles: List<String>, selected: Int, onSelect: (Int) -> Unit) {
    PrimaryTabRow(selectedTabIndex = selected, containerColor = MaterialTheme.colorScheme.background) {
        titles.forEachIndexed { i, t -> Tab(selected = i == selected, onClick = { onSelect(i) }, text = { Text(t, maxLines = 1) }) }
    }
}

/** Says which storage a list covers, with a way back to all of it. */
@Composable
fun ScopeLine(vm: AppModel, what: String) {
    val v = vm.selected ?: return
    Row(Modifier.padding(horizontal = 4.dp), verticalAlignment = Alignment.CenterVertically) {
        Icon(volumeIcon(v), null, Modifier.padding(end = 6.dp), tint = MaterialTheme.colorScheme.onSurfaceVariant)
        Text("$what on ${v.name} only", style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant, modifier = Modifier.weight(1f))
        androidx.compose.material3.TextButton(onClick = { vm.pickVolume("") }) { Text("Show all") }
    }
}

@Composable
fun NeedsAllFiles(vm: AppModel): Boolean {
    if (vm.hasAllFiles) return false
    val ctx = LocalContext.current
    val start = rememberStart()
    EmptyState(Icons.Outlined.Lock, "Allow All files access", "Android only lets Jigglypuff look through your storage after you allow it.", "Allow") { start(Device.allFilesIntent(ctx)) }
    return true
}

@Composable
fun CleanScreen(vm: AppModel, sub: String?) {
    var tab by rememberSaveable(sub) { mutableStateOf(if (sub == "leftovers") 1 else 0) }
    Column {
        SubTabs(listOf("Junk", "Leftovers"), tab) { tab = it }
        if (tab == 0) JunkTab(vm) else LeftoversTab(vm)
    }
}

@Composable
private fun JunkTab(vm: AppModel) {
    var sel by remember(vm.junk) { mutableStateOf(defaultJunk(vm.junk)) }
    var open by remember { mutableStateOf(setOf<String>()) }
    var confirm by remember { mutableStateOf<CleanRequest?>(null) }
    LaunchedEffect(vm.hasAllFiles, vm.volumeId) { if (vm.hasAllFiles && vm.junk == null && vm.busy == null) vm.scanJunk() }
    ScreenList {
        item { ScopeLine(vm, "Junk") }
        item {
            if (NeedsAllFiles(vm)) return@item
            when {
                vm.busy == "junk" || vm.busy == "overview" -> ScanningCard(vm, "Looking for junk…")
                vm.junk == null -> {}
                vm.junk!!.isEmpty() -> EmptyState(Icons.Outlined.TaskAlt, "No junk here", "Jigglypuff didn't find anything worth removing right now.", "Scan again") { vm.scanJunk() }
                else -> {
                    val cats = vm.junk!!
                    val total = cats.filter { it.id in sel }.sumOf { it.size }
                    Column(verticalArrangement = Arrangement.spacedBy(12.dp)) {
                        Row(horizontalArrangement = Arrangement.spacedBy(10.dp), verticalAlignment = Alignment.CenterVertically) {
                            Button(enabled = total > 0 || sel.isNotEmpty(), onClick = {
                                confirm = CleanRequest("Clean ${total.fmtSize()}?", "Cleaned junk files", junkTargets(cats, sel))
                            }) { Text(if (total > 0) "Clean ${total.fmtSize()}" else "Clean selected") }
                            OutlinedButton(onClick = { vm.scanJunk() }, enabled = vm.busy == null) { Text("Scan again") }
                        }
                        Panel {
                            cats.forEachIndexed { i, c ->
                                if (i > 0) HorizontalDivider(color = MaterialTheme.colorScheme.outlineVariant.copy(alpha = 0.5f))
                                RowItem(c.title, "${plural(c.items.size, "item")} · ${c.desc}", leading = { IconBox(junkIcon(c.id)) },
                                    onClick = { sel = if (c.id in sel) sel - c.id else sel + c.id }) {
                                    Column(horizontalAlignment = Alignment.End) {
                                        SizeText(c.size)
                                        SafetyBadge(c.safety)
                                    }
                                    IconButton(onClick = { open = if (c.id in open) open - c.id else open + c.id }) {
                                        Icon(if (c.id in open) Icons.Outlined.ExpandLess else Icons.Outlined.ExpandMore, "Show files")
                                    }
                                    Checkbox(c.id in sel, onCheckedChange = { sel = if (it) sel + c.id else sel - c.id })
                                }
                                if (c.id in open) Column(Modifier.fillMaxWidth().padding(start = 66.dp, end = 16.dp, bottom = 12.dp), verticalArrangement = Arrangement.spacedBy(3.dp)) {
                                    c.items.sortedByDescending { it.size }.take(40).forEach {
                                        Row {
                                            Text(prettyPath(vm, it.path), style = MaterialTheme.typography.bodySmall, maxLines = 1, modifier = Modifier.weight(1f))
                                            Text(if (it.isDir) "folder" else it.size.fmtSize(), style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
                                        }
                                    }
                                    if (c.items.size > 40) Text("…and ${c.items.size - 40} more", style = MaterialTheme.typography.bodySmall)
                                }
                            }
                        }
                        Text(
                            "Other apps' caches can only be cleared from Android's settings. Apps › Caches lists the biggest ones.",
                            style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant, modifier = Modifier.padding(horizontal = 4.dp),
                        )
                    }
                }
            }
        }
    }
    CleanDialog(vm, confirm, onClose = { confirm = null })
}

@Composable
private fun LeftoversTab(vm: AppModel) {
    var sel by remember(vm.leftovers) { mutableStateOf(vm.leftovers.orEmpty().filter { it.high }.map { it.path }.toSet()) }
    var confirm by remember { mutableStateOf<CleanRequest?>(null) }
    LaunchedEffect(vm.hasAllFiles, vm.volumeId) { if (vm.hasAllFiles && vm.leftovers == null && vm.busy == null) vm.scanLeftovers() }
    ScreenList {
        item { ScopeLine(vm, "Leftovers") }
        item {
            if (NeedsAllFiles(vm)) return@item
            val list = vm.leftovers
            when {
                vm.busy == "leftovers" || vm.busy == "overview" -> ScanningCard(vm, "Looking for leftovers…")
                list == null -> {}
                list.isEmpty() -> EmptyState(Icons.Outlined.TaskAlt, "No leftovers found", "Every folder Jigglypuff checked belongs to an app you still have.", "Scan again") { vm.scanLeftovers() }
                else -> Column(verticalArrangement = Arrangement.spacedBy(12.dp)) {
                    Text(
                        "Folders left by apps you removed. Ones named after an app that isn't installed are ticked; check the others before removing them.",
                        style = MaterialTheme.typography.bodyMedium, color = MaterialTheme.colorScheme.onSurfaceVariant, modifier = Modifier.padding(horizontal = 4.dp),
                    )
                    val size = list.filter { it.path in sel }.sumOf { it.size }
                    Row(horizontalArrangement = Arrangement.spacedBy(10.dp)) {
                        Button(enabled = sel.isNotEmpty(), onClick = {
                            confirm = CleanRequest("Remove ${plural(sel.size, "folder")}?", "Removed leftovers",
                                list.filter { it.path in sel }.map { Target(it.path, it.size, trash = true, isDir = true) })
                        }) {
                            Icon(Icons.Outlined.Delete, null, Modifier.padding(end = 8.dp))
                            Text(if (sel.isEmpty()) "Remove selected" else "Remove ${size.fmtSize()}")
                        }
                        OutlinedButton(onClick = { vm.scanLeftovers() }, enabled = vm.busy == null) { Text("Scan again") }
                    }
                    Panel {
                        list.forEachIndexed { i, l ->
                            if (i > 0) HorizontalDivider(color = MaterialTheme.colorScheme.outlineVariant.copy(alpha = 0.5f))
                            RowItem(l.name, "${prettyPath(vm, l.path)}\n${l.reason} · ${plural(l.files, "file")} · changed ${relTime(l.modified)}",
                                leading = { IconBox(Icons.Outlined.FolderOff) }, onClick = { sel = if (l.path in sel) sel - l.path else sel + l.path }) {
                                Column(horizontalAlignment = Alignment.End) {
                                    SizeText(l.size)
                                    if (l.high) Badge("Likely leftover", Tones.safe) else Badge("Check first", Tones.caution)
                                }
                                Checkbox(l.path in sel, onCheckedChange = { sel = if (it) sel + l.path else sel - l.path })
                            }
                        }
                    }
                }
            }
        }
    }
    CleanDialog(vm, confirm, onClose = { confirm = null })
}
