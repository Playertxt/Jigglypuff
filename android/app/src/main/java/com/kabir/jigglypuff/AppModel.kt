package com.kabir.jigglypuff

import android.app.Application
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.setValue
import androidx.lifecycle.AndroidViewModel
import androidx.lifecycle.viewModelScope
import com.kabir.jigglypuff.engine.ApkFile
import com.kabir.jigglypuff.engine.ApkStatus
import com.kabir.jigglypuff.engine.AppInfo
import com.kabir.jigglypuff.engine.CleanResult
import com.kabir.jigglypuff.engine.Cleaner
import com.kabir.jigglypuff.engine.Device
import com.kabir.jigglypuff.engine.DupGroup
import com.kabir.jigglypuff.engine.Guard
import com.kabir.jigglypuff.engine.History
import com.kabir.jigglypuff.engine.HistoryEntry
import com.kabir.jigglypuff.engine.Item
import com.kabir.jigglypuff.engine.JunkCategory
import com.kabir.jigglypuff.engine.Leftover
import com.kabir.jigglypuff.engine.Progress
import com.kabir.jigglypuff.engine.Scanner
import com.kabir.jigglypuff.engine.Settings
import com.kabir.jigglypuff.engine.Target
import com.kabir.jigglypuff.engine.Tokens
import com.kabir.jigglypuff.engine.Volume
import kotlinx.coroutines.CancellationException
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.Job
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.launch
import kotlinx.coroutines.withContext
import java.io.File

/** Everything the screens show, and the jobs that fill it in. */
class AppModel(app: Application) : AndroidViewModel(app) {
    private val ctx get() = getApplication<Application>()
    private val history = History(app)

    var settings by mutableStateOf(Settings.load(app))
        private set
    var volumes by mutableStateOf(listOf<Volume>())
        private set
    /** The storage every scan looks at; "" means all of them. */
    var volumeId by mutableStateOf("")
        private set
    var hasAllFiles by mutableStateOf(false)
        private set
    var hasUsage by mutableStateOf(false)
        private set
    var historyList by mutableStateOf(listOf<HistoryEntry>())
        private set

    /** The name of the running job, if any. */
    var busy by mutableStateOf<String?>(null)
        private set
    val progress = MutableStateFlow(Progress())
    var error by mutableStateOf<String?>(null)
    private var job: Job? = null

    var junk by mutableStateOf<List<JunkCategory>?>(null)
    var leftovers by mutableStateOf<List<Leftover>?>(null)
    var old by mutableStateOf<List<Item>?>(null)
    var large by mutableStateOf<List<Item>?>(null)
    var dups by mutableStateOf<List<DupGroup>?>(null)
    var apks by mutableStateOf<List<ApkFile>?>(null)
    var apps by mutableStateOf<List<AppInfo>?>(null)
    var scannedAt by mutableStateOf(0L)
    var lastClean by mutableStateOf<CleanResult?>(null)

    val selected: Volume? get() = volumes.firstOrNull { it.id == volumeId }
    val scope: List<Volume> get() = selected?.let { listOf(it) } ?: volumes

    init {
        refresh()
        // Old trash goes for good after the number of days in settings.
        viewModelScope.launch(Dispatchers.IO) {
            cleaner().emptyTrash(Device.volumes(ctx), all = false)
            withContext(Dispatchers.Main) { historyList = history.all() }
        }
    }

    /** Re-reads storage, permissions and history (called whenever the app comes back to the front). */
    fun refresh() {
        volumes = Device.volumes(ctx)
        if (volumeId.isNotEmpty() && volumes.none { it.id == volumeId }) volumeId = ""
        hasAllFiles = Device.hasAllFiles()
        hasUsage = Device.hasUsage(ctx)
        historyList = history.all()
    }

    fun pickVolume(id: String) {
        if (id == volumeId) return
        volumeId = id
        junk = null; leftovers = null; old = null; large = null; dups = null; apks = null; scannedAt = 0; lastClean = null
    }

    fun updateSettings(s: Settings) {
        settings = s
        s.save(ctx)
    }

    fun cancel() {
        job?.cancel()
    }

    private fun run(name: String, block: suspend () -> Unit) {
        if (busy != null) return
        busy = name
        error = null
        progress.value = Progress()
        job = viewModelScope.launch {
            try {
                block()
            } catch (_: CancellationException) {
            } catch (e: Exception) {
                error = e.message ?: e.toString()
            } finally {
                busy = null
            }
        }
    }

    private fun post(p: Progress) {
        progress.value = p
    }

    private fun scanner(installed: Map<String, Long>) = Scanner(settings, installed) { item ->
        Device.readApk(ctx, item, installed).status.let { it == ApkStatus.INSTALLED || it == ApkStatus.OLDER_THAN_INSTALLED }
    }

    private fun appTokens(installed: Map<String, Long>, list: List<AppInfo>): Set<String> =
        (installed.keys.flatMap { Tokens.of(it) } + list.flatMap { Tokens.of(it.label) }).toSet()

    private fun guard(installed: Set<String>) = Guard(volumes, installed, settings)

    private fun cleaner(installed: Set<String> = Device.installedPackages(ctx).keys) =
        Cleaner(guard(installed), history, settings).also { it.setVolumes(volumes) }

    // Scans ------------------------------------------------------------------------------

    /** The overview scan: junk, leftovers, old downloads and apps on the chosen storage. */
    fun scanOverview() = run("overview") {
        val where = scope
        val r = withContext(Dispatchers.IO) {
            val installed = Device.installedPackages(ctx)
            val s = scanner(installed)
            post(Progress("Reading your apps"))
            val list = Device.apps(ctx)
            val j = s.junk(where, ::post)
            post(Progress("Looking for leftovers"))
            val l = s.leftovers(where, appTokens(installed, list), ::post)
            post(Progress("Checking Downloads"))
            val o = s.oldDownloads(where, settings.oldDownloadDays)
            Quad(list, j, l, o)
        }
        apps = r.a; junk = r.b; leftovers = r.c; old = r.d
        scannedAt = System.currentTimeMillis()
        lastClean = null
    }

    fun scanJunk() = run("junk") {
        val where = scope
        junk = withContext(Dispatchers.IO) { scanner(Device.installedPackages(ctx)).junk(where, ::post) }
    }

    fun scanLeftovers() = run("leftovers") {
        val where = scope
        leftovers = withContext(Dispatchers.IO) {
            val installed = Device.installedPackages(ctx)
            scanner(installed).leftovers(where, appTokens(installed, Device.apps(ctx)), ::post)
        }
    }

    fun scanLarge() = run("large") {
        val where = scope
        large = withContext(Dispatchers.IO) { scanner(emptyMap()).large(where, settings.largeMinMB * 1024L * 1024L, ::post) }
    }

    fun scanOld() = run("old") {
        val where = scope
        old = withContext(Dispatchers.IO) { scanner(emptyMap()).oldDownloads(where, settings.oldDownloadDays) }
    }

    fun scanDups() = run("dups") {
        val where = scope
        dups = withContext(Dispatchers.IO) { scanner(emptyMap()).duplicates(where, settings.dupMinKB * 1024L, ::post) }
    }

    fun scanApks() = run("apks") {
        val where = scope
        apks = withContext(Dispatchers.IO) {
            val installed = Device.installedPackages(ctx)
            scanner(installed).apks(where, ::post).map { Device.readApk(ctx, it, installed) }
        }
    }

    fun loadApps() = run("apps") {
        apps = withContext(Dispatchers.IO) { Device.apps(ctx) }
    }

    /** Apps on the chosen storage. Apps can only live on internal storage or an SD card set up as internal. */
    fun appsHere(list: List<AppInfo>): List<AppInfo> {
        val v = selected ?: return list
        return list.filter { it.storageUuid != null && it.storageUuid == v.uuid }
    }

    // Cleaning ---------------------------------------------------------------------------

    /** Checks every target with the guard first; refused ones are dropped and reported. */
    suspend fun vet(targets: List<Target>): Pair<List<Target>, List<String>> = withContext(Dispatchers.IO) {
        val g = guard(Device.installedPackages(ctx).keys)
        val ok = mutableListOf<Target>()
        val refused = mutableListOf<String>()
        for (t in targets) {
            val why = g.check(t.path)
            if (why == null) ok += t else refused += "${File(t.path).name}: $why"
        }
        ok to refused
    }

    fun clean(title: String, targets: List<Target>, done: (CleanResult) -> Unit = {}) = run("clean") {
        val res = withContext(Dispatchers.IO) { cleaner().execute(title, targets) }
        val gone = targets.map { it.path }.filter { !File(it).exists() }.toSet()
        junk = junk?.map { c -> c.copy(items = c.items.filter { it.path !in gone }) }?.filter { it.items.isNotEmpty() }
        leftovers = leftovers?.filter { it.path !in gone }
        old = old?.filter { it.path !in gone }
        large = large?.filter { it.path !in gone }
        dups = dups?.map { g -> g.copy(files = g.files.filter { it.path !in gone }) }?.filter { it.files.size > 1 }
        apks = apks?.filter { it.path !in gone }
        lastClean = res
        refresh()
        done(res)
    }

    fun restore(e: HistoryEntry, done: (Int) -> Unit) = run("restore") {
        val n = withContext(Dispatchers.IO) { cleaner().restore(e) }
        refresh()
        done(n)
    }

    fun emptyTrash() = run("trash") {
        withContext(Dispatchers.IO) { cleaner().emptyTrash(volumes, all = true) }
        refresh()
    }

    private data class Quad<A, B, C, D>(val a: A, val b: B, val c: C, val d: D)
}
