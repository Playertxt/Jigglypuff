package com.kabir.jigglypuff.ui

import android.content.ActivityNotFoundException
import android.content.Context
import android.content.Intent
import android.webkit.MimeTypeMap
import androidx.compose.foundation.Image
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.layout.widthIn
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.outlined.Apps
import androidx.compose.material.icons.outlined.CleaningServices
import androidx.compose.material.icons.outlined.ExpandMore
import androidx.compose.material.icons.outlined.Folder
import androidx.compose.material.icons.outlined.Home
import androidx.compose.material.icons.outlined.MoreHoriz
import androidx.compose.material.icons.outlined.SdCard
import androidx.compose.material.icons.outlined.Smartphone
import androidx.compose.material.icons.outlined.Storage
import androidx.compose.material.icons.outlined.Usb
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.DropdownMenu
import androidx.compose.material3.DropdownMenuItem
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.Icon
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Scaffold
import androidx.compose.material3.SnackbarHost
import androidx.compose.material3.SnackbarHostState
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.material3.TopAppBar
import androidx.compose.material3.TopAppBarDefaults
import androidx.compose.runtime.Composable
import androidx.compose.runtime.CompositionLocalProvider
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.produceState
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberCoroutineScope
import androidx.compose.runtime.saveable.rememberSaveable
import androidx.compose.runtime.setValue
import androidx.compose.runtime.staticCompositionLocalOf
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.vector.ImageVector
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.res.painterResource
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.core.content.FileProvider
import androidx.lifecycle.Lifecycle
import androidx.lifecycle.compose.LifecycleEventEffect
import com.kabir.jigglypuff.AppModel
import com.kabir.jigglypuff.R
import com.kabir.jigglypuff.engine.CleanResult
import com.kabir.jigglypuff.engine.Target
import com.kabir.jigglypuff.engine.Volume
import com.kabir.jigglypuff.engine.fmtSize
import kotlinx.coroutines.launch
import java.io.File

enum class Tab(val label: String, val icon: ImageVector) {
    Overview("Overview", Icons.Outlined.Home),
    Clean("Clean", Icons.Outlined.CleaningServices),
    Apps("Apps", Icons.Outlined.Apps),
    Files("Files", Icons.Outlined.Folder),
    More("More", Icons.Outlined.MoreHoriz),
}

/** Opens a tab, optionally on one of its sub-tabs ("leftovers", "old", ...). */
val LocalNav = staticCompositionLocalOf<(Tab, String?) -> Unit> { { _, _ -> } }
val LocalSnack = staticCompositionLocalOf { SnackbarHostState() }

@Composable
fun JigglypuffApp(vm: AppModel) {
    JigglypuffTheme(vm.settings.theme, vm.look) {
        var tab by rememberSaveable { mutableStateOf(Tab.Overview) }
        var sub by remember { mutableStateOf<String?>(null) }
        val snack = remember { SnackbarHostState() }
        LifecycleEventEffect(Lifecycle.Event.ON_RESUME) {
            vm.refresh()
            if (vm.apps != null) vm.loadApps() // an app may have been uninstalled meanwhile
        }
        Scaffold(
            topBar = { TopBar(vm) },
            bottomBar = { LookNavBar(Tab.entries, tab) { tab = it; sub = null } },
            snackbarHost = { SnackbarHost(snack) },
            containerColor = Color.Transparent,
            contentColor = MaterialTheme.colorScheme.onBackground,
            modifier = Modifier.lookBackground(),
        ) { pad ->
            CompositionLocalProvider(LocalSnack provides snack, LocalNav provides { t, s -> tab = t; sub = s }) {
                Box(Modifier.padding(pad).fillMaxSize()) {
                    when (tab) {
                        Tab.Overview -> OverviewScreen(vm)
                        Tab.Clean -> CleanScreen(vm, sub)
                        Tab.Apps -> AppsScreen(vm, sub)
                        Tab.Files -> FilesScreen(vm, sub)
                        Tab.More -> MoreScreen(vm)
                    }
                }
            }
        }
        vm.error?.let { msg ->
            LookDialog(
                onDismissRequest = { vm.error = null }, confirmButton = { TextLinkButton(onClick = { vm.error = null }) { Text("OK") } },
                title = { Text("Something went wrong") }, text = { Text(msg) },
            )
        }
    }
}

@OptIn(ExperimentalMaterial3Api::class)
@Composable
private fun TopBar(vm: AppModel) {
    TopAppBar(
        title = {
            Row(verticalAlignment = Alignment.CenterVertically) {
                Image(painterResource(R.drawable.ic_logo), null, Modifier.size(30.dp))
                Spacer(Modifier.width(10.dp))
                Text("Jigglypuff", fontWeight = FontWeight.SemiBold)
            }
        },
        actions = { StoragePicker(vm) },
        colors = TopAppBarDefaults.topAppBarColors(containerColor = Color.Transparent),
    )
}

fun volumeIcon(v: Volume?): ImageVector = when {
    v == null -> Icons.Outlined.Storage
    v.primary -> Icons.Outlined.Smartphone
    v.name.contains("usb", ignoreCase = true) -> Icons.Outlined.Usb
    else -> Icons.Outlined.SdCard
}

/** Chooses which storage every scan looks at. */
@Composable
fun StoragePicker(vm: AppModel) {
    var open by remember { mutableStateOf(false) }
    val sel = vm.selected
    Box(Modifier.padding(end = 8.dp)) {
        Surface(onClick = { open = true }, shape = lookShape("chip"), color = MaterialTheme.colorScheme.secondaryContainer) {
            Row(Modifier.padding(start = 12.dp, end = 8.dp, top = 7.dp, bottom = 7.dp), verticalAlignment = Alignment.CenterVertically) {
                Icon(volumeIcon(sel ?: vm.volumes.singleOrNull()), null, Modifier.size(18.dp))
                Spacer(Modifier.width(6.dp))
                Text(sel?.name ?: if (vm.volumes.size > 1) "All storage" else (vm.volumes.firstOrNull()?.name ?: "Storage"),
                    style = MaterialTheme.typography.labelLarge, maxLines = 1, overflow = TextOverflow.Ellipsis, modifier = Modifier.widthIn(max = 140.dp))
                Icon(Icons.Outlined.ExpandMore, "Choose storage", Modifier.size(18.dp))
            }
        }
        DropdownMenu(expanded = open, onDismissRequest = { open = false }) {
            Text("Storage to scan", style = MaterialTheme.typography.labelMedium, color = MaterialTheme.colorScheme.onSurfaceVariant, modifier = Modifier.padding(horizontal = 16.dp, vertical = 6.dp))
            if (vm.volumes.size > 1) DropdownMenuItem(
                text = { Text("All storage") }, leadingIcon = { Icon(Icons.Outlined.Storage, null) },
                onClick = { vm.pickVolume(""); open = false },
            )
            vm.volumes.forEach { v ->
                DropdownMenuItem(
                    text = { Column { Text(v.name); Text("${v.free.fmtSize()} free of ${v.total.fmtSize()}", style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant) } },
                    leadingIcon = { Icon(volumeIcon(v), null) },
                    onClick = { vm.pickVolume(v.id); open = false },
                )
            }
            if (vm.volumes.size < 2) Text(
                "Plug in an SD card or USB drive to scan it.", style = MaterialTheme.typography.bodySmall,
                color = MaterialTheme.colorScheme.onSurfaceVariant, modifier = Modifier.padding(horizontal = 16.dp, vertical = 8.dp).widthIn(max = 240.dp),
            )
        }
    }
}

/** What a confirmation shows; the history gets [historyTitle]. */
data class CleanRequest(val dialogTitle: String, val historyTitle: String, val targets: List<Target>, val intro: String? = null)

/**
 * The only way anything gets removed: lists exactly what goes, leaves out anything the guard refuses,
 * and says what goes to the trash and what is deleted for good.
 */
@Composable
fun CleanDialog(vm: AppModel, req: CleanRequest?, onClose: () -> Unit, onDone: (CleanResult) -> Unit = {}) {
    if (req == null) return
    val snack = LocalSnack.current
    val scope = rememberCoroutineScope()
    val vetted by produceState<Pair<List<Target>, List<String>>?>(null, req) { value = vm.vet(req.targets) }
    val v = vetted
    LookDialog(
        onDismissRequest = onClose,
        title = { Text(req.dialogTitle) },
        text = {
            if (v == null) {
                Box(Modifier.fillMaxWidth().padding(24.dp), contentAlignment = Alignment.Center) { CircularProgressIndicator() }
            } else {
                val (ok, refused) = v
                val trash = if (vm.settings.useTrash) ok.count { it.trash } else 0
                val gone = ok.size - trash
                Column(verticalArrangement = Arrangement.spacedBy(8.dp)) {
                    Text(req.intro ?: "Here's exactly what Jigglypuff will remove.", style = MaterialTheme.typography.bodyMedium)
                    Text("${plural(ok.size, "item")} · ${ok.sumOf { it.size }.fmtSize()}", fontWeight = FontWeight.SemiBold)
                    if (trash > 0) Text("$trash go to Jigglypuff's trash. You can put them back from More for ${vm.settings.trashDays} days.", style = MaterialTheme.typography.bodySmall)
                    if (gone > 0) Text(
                        if (vm.settings.useTrash) "$gone caches and empty folders are deleted for good." else "Everything is deleted for good: the trash is turned off in settings.",
                        style = MaterialTheme.typography.bodySmall,
                    )
                    Surface(shape = lookShape("input"), color = MaterialTheme.colorScheme.surfaceContainerHighest) {
                        LazyColumn(Modifier.heightIn(max = 220.dp).padding(10.dp), verticalArrangement = Arrangement.spacedBy(4.dp)) {
                            items(ok.take(300)) { t -> Text(prettyPath(vm, t.path), style = MaterialTheme.typography.bodySmall, maxLines = 1, overflow = TextOverflow.Ellipsis) }
                            if (ok.size > 300) item { Text("…and ${ok.size - 300} more", style = MaterialTheme.typography.bodySmall) }
                        }
                    }
                    if (refused.isNotEmpty()) Text(
                        "${plural(refused.size, "protected item")} left out: ${refused.take(2).joinToString("; ")}",
                        style = MaterialTheme.typography.bodySmall, color = Tones.caution,
                    )
                }
            }
        },
        confirmButton = {
            DangerButton(
                enabled = v != null && v.first.isNotEmpty(),
                onClick = {
                    val ok = v!!.first
                    onClose()
                    vm.clean(req.historyTitle, ok) { r ->
                        scope.launch { snack.showSnackbar(resultText(r)) }
                        onDone(r)
                    }
                },
            ) { Text("Remove") }
        },
        dismissButton = { TextLinkButton(onClick = onClose) { Text("Cancel") } },
    )
}

fun resultText(r: CleanResult): String = buildList {
    add(if (r.freed > 0) "Freed ${r.freed.fmtSize()}" else "Done")
    if (r.trashed > 0) add("${r.trashed} in the trash")
    if (r.failed > 0) add("${r.failed} couldn't be removed")
}.joinToString(" · ")

/** Opens a file in whatever app handles it. */
fun openFile(ctx: Context, path: String) {
    val f = File(path)
    val uri = FileProvider.getUriForFile(ctx, ctx.packageName + ".files", f)
    val type = MimeTypeMap.getSingleton().getMimeTypeFromExtension(f.extension.lowercase()) ?: "*/*"
    try {
        ctx.startActivity(Intent(Intent.ACTION_VIEW).setDataAndType(uri, type).addFlags(Intent.FLAG_GRANT_READ_URI_PERMISSION or Intent.FLAG_ACTIVITY_NEW_TASK))
    } catch (_: ActivityNotFoundException) {
        android.widget.Toast.makeText(ctx, "No app can open this file", android.widget.Toast.LENGTH_SHORT).show()
    }
}

@Composable
fun rememberStart(): (Intent) -> Unit {
    val ctx = LocalContext.current
    return remember(ctx) {
        { i: Intent ->
            try {
                ctx.startActivity(i)
            } catch (_: ActivityNotFoundException) {
                // Some phones don't support the per-app settings shortcut: fall back to the general screen.
                runCatching { ctx.startActivity(Intent(i.action)) }
            }
        }
    }
}
