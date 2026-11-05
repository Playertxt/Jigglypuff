package com.kabir.jigglypuff.engine

import android.content.Context
import org.json.JSONArray
import org.json.JSONObject
import java.io.File
import java.nio.file.Files
import java.util.UUID
import java.util.concurrent.TimeUnit

/** User settings, kept in SharedPreferences. */
data class Settings(
    val theme: String = "system",
    val useTrash: Boolean = true,
    val trashDays: Int = 7,
    val tempMinAgeHours: Int = 24,
    val largeMinMB: Int = 100,
    val oldDownloadDays: Int = 60,
    val dupMinKB: Int = 1024,
    val unusedDays: Int = 90,
    val protected: List<String> = emptyList(),
) {
    fun save(ctx: Context) {
        ctx.getSharedPreferences("settings", Context.MODE_PRIVATE).edit()
            .putString("theme", theme).putBoolean("useTrash", useTrash).putInt("trashDays", trashDays)
            .putInt("tempMinAgeHours", tempMinAgeHours).putInt("largeMinMB", largeMinMB)
            .putInt("oldDownloadDays", oldDownloadDays).putInt("dupMinKB", dupMinKB).putInt("unusedDays", unusedDays)
            .putString("protected", JSONArray(protected).toString()).apply()
    }

    companion object {
        fun load(ctx: Context): Settings {
            val p = ctx.getSharedPreferences("settings", Context.MODE_PRIVATE)
            val d = Settings()
            val prot = runCatching {
                val a = JSONArray(p.getString("protected", "[]"))
                List(a.length()) { a.getString(it) }
            }.getOrDefault(emptyList())
            return Settings(
                theme = p.getString("theme", d.theme) ?: d.theme, useTrash = p.getBoolean("useTrash", d.useTrash),
                trashDays = p.getInt("trashDays", d.trashDays), tempMinAgeHours = p.getInt("tempMinAgeHours", d.tempMinAgeHours),
                largeMinMB = p.getInt("largeMinMB", d.largeMinMB), oldDownloadDays = p.getInt("oldDownloadDays", d.oldDownloadDays),
                dupMinKB = p.getInt("dupMinKB", d.dupMinKB), unusedDays = p.getInt("unusedDays", d.unusedDays), protected = prot,
            )
        }
    }
}

/**
 * Refuses anything Jigglypuff must never delete, whatever the screen asks for. Every target is checked
 * when the confirmation is built and again right before it is removed.
 */
class Guard(private val volumes: List<Volume>, private val installed: Set<String>, private val settings: Settings) {

    fun check(path: String): String? {
        val f = File(path)
        if (!f.isAbsolute) return "not a full path"
        if (runCatching { Files.isSymbolicLink(f.toPath()) }.getOrDefault(true)) return "links are never followed"
        val canon = runCatching { f.canonicalPath }.getOrNull() ?: return "can't read the path"
        val vol = volumes.firstOrNull { canon.startsWith(it.root.canonicalPath + "/") }
            ?: return "not on a storage Jigglypuff looks at"
        val rel = canon.removePrefix(vol.root.canonicalPath + "/")
        val parts = rel.split('/')
        if (parts.size == 1 && parts[0].lowercase() in STANDARD_DIRS) return "Android's own folders are protected"
        if (parts[0].lowercase() == TRASH_DIR) return "that's Jigglypuff's trash"
        if (parts[0].lowercase() == "android") {
            // Only a whole leftover folder of an app that isn't installed: Android/media/<package> and the like.
            val ok = parts.size >= 3 && parts[1].lowercase() in setOf("media", "data", "obb") && parts[2] !in installed
            if (!ok) return "Android's app folders are protected"
        }
        if (settings.protected.any { canon == it || canon.startsWith("$it/") || it.startsWith("$canon/") }) return "it's in a protected folder"
        return null
    }

    fun volumeOf(path: String): Volume? = volumes.firstOrNull { path.startsWith(it.root.path + "/") }
}

/** One clean-up in the history. Items that went to the trash can be put back. */
data class HistoryEntry(
    val id: String,
    val time: Long,
    val title: String,
    val freed: Long,
    val removed: Int,
    val trashed: Int,
    val failed: Int,
    /** original path → path in the trash */
    val trash: List<Pair<String, String>>,
    val trashBytes: Long,
    val state: String, // "" | restored | emptied
) {
    val restorable: Boolean get() = trash.isNotEmpty() && state.isEmpty()

    fun json(): JSONObject = JSONObject().put("id", id).put("time", time).put("title", title).put("freed", freed)
        .put("removed", removed).put("trashed", trashed).put("failed", failed).put("trashBytes", trashBytes).put("state", state)
        .put("trash", JSONArray(trash.map { JSONArray(listOf(it.first, it.second)) }))

    companion object {
        fun from(o: JSONObject): HistoryEntry {
            val t = o.optJSONArray("trash") ?: JSONArray()
            return HistoryEntry(
                o.getString("id"), o.getLong("time"), o.getString("title"), o.optLong("freed"), o.optInt("removed"),
                o.optInt("trashed"), o.optInt("failed"),
                List(t.length()) { t.getJSONArray(it).let { p -> p.getString(0) to p.getString(1) } },
                o.optLong("trashBytes"), o.optString("state"),
            )
        }
    }
}

class History(ctx: Context) {
    private val file = File(ctx.filesDir, "history.json")

    @Synchronized
    fun all(): List<HistoryEntry> = runCatching {
        val a = JSONArray(file.readText())
        List(a.length()) { HistoryEntry.from(a.getJSONObject(it)) }
    }.getOrDefault(emptyList())

    @Synchronized
    fun save(list: List<HistoryEntry>) {
        file.writeText(JSONArray(list.take(200).map { it.json() }).toString())
    }

    fun add(e: HistoryEntry) = save(listOf(e) + all())

    fun update(e: HistoryEntry) = save(all().map { if (it.id == e.id) e else it })
}

/** Removes exactly the confirmed targets. User files go to a trash folder on the same storage. */
class Cleaner(private val guard: Guard, private val history: History, private val settings: Settings) {

    fun execute(title: String, targets: List<Target>): CleanResult {
        val id = UUID.randomUUID().toString().take(8)
        var freed = 0L
        var removed = 0
        var trashed = 0
        var failed = 0
        var skipped = 0
        var trashBytes = 0L
        val errors = mutableListOf<String>()
        val moved = mutableListOf<Pair<String, String>>()
        // Deepest paths first, so empty folders inside empty folders go cleanly.
        for (t in targets.sortedByDescending { it.path.count { c -> c == '/' } }) {
            val f = File(t.path)
            if (!f.exists()) {
                skipped++
                continue
            }
            val refused = guard.check(t.path)
            if (refused != null) {
                failed++
                errors += "${f.name}: $refused"
                continue
            }
            if (t.isDir && !t.trash) {
                // Only folders that still hold no files at all (empty folders inside are fine).
                val stillEmpty = f.isDirectory && f.walkTopDown().none { it.isFile || runCatching { Files.isSymbolicLink(it.toPath()) }.getOrDefault(true) }
                if (stillEmpty && f.deleteRecursively()) removed++ else skipped++
                continue
            }
            val size = if (f.isDirectory) f.walkTopDown().filter { it.isFile }.sumOf { it.length() } else f.length()
            if (t.trash && settings.useTrash) {
                val vol = guard.volumeOf(f.path)
                val dest = vol?.let { File(it.root, "$TRASH_DIR/$id/" + f.path.removePrefix(it.root.path + "/")) }
                if (dest != null && prepareTrash(vol.root) && dest.parentFile?.let { it.isDirectory || it.mkdirs() } == true && f.renameTo(dest)) {
                    trashed++
                    freed += size
                    trashBytes += size
                    moved += f.path to dest.path
                } else {
                    failed++
                    errors += "${f.name}: couldn't move it to the trash"
                }
                continue
            }
            val ok = if (f.isDirectory) f.deleteRecursively() else f.delete()
            if (ok) {
                removed++
                freed += size
            } else {
                failed++
                errors += "${f.name}: couldn't delete it"
            }
        }
        if (removed + trashed + failed > 0) {
            history.add(HistoryEntry(id, System.currentTimeMillis(), title, freed, removed, trashed, failed, moved, trashBytes, ""))
        }
        return CleanResult(freed, removed, trashed, failed, skipped, errors)
    }

    /** Puts a trashed clean-up back where it was. Returns how many items came back. */
    fun restore(e: HistoryEntry): Int {
        var n = 0
        for ((orig, inTrash) in e.trash) {
            val src = File(inTrash)
            val dst = File(orig)
            if (src.exists() && !dst.exists() && dst.parentFile?.let { it.isDirectory || it.mkdirs() } == true && src.renameTo(dst)) n++
        }
        history.update(e.copy(state = "restored"))
        cleanTrashDirs()
        return n
    }

    /** Deletes trashed items for good: everything, or only clean-ups older than the setting. */
    fun emptyTrash(volumes: List<Volume>, all: Boolean) {
        val cutoff = System.currentTimeMillis() - TimeUnit.DAYS.toMillis(settings.trashDays.toLong())
        for (e in history.all()) {
            if (!e.restorable || (!all && e.time > cutoff)) continue
            for (v in volumes) File(v.root, "$TRASH_DIR/${e.id}").takeIf { it.exists() }?.deleteRecursively()
            history.update(e.copy(state = "emptied"))
        }
        cleanTrashDirs(volumes)
    }

    private var knownVolumes: List<Volume> = emptyList()

    private fun cleanTrashDirs(volumes: List<Volume> = knownVolumes) {
        for (v in volumes) {
            val t = File(v.root, TRASH_DIR)
            t.listFiles()?.filter { it.isDirectory && it.walkTopDown().none { f -> f.isFile } }?.forEach { it.deleteRecursively() }
        }
    }

    fun setVolumes(v: List<Volume>) {
        knownVolumes = v
    }

    private fun prepareTrash(root: File): Boolean {
        val t = File(root, TRASH_DIR)
        if (!t.isDirectory && !t.mkdirs()) return false
        val nomedia = File(t, ".nomedia") // keeps trashed photos out of the gallery
        if (!nomedia.exists()) runCatching { nomedia.createNewFile() }
        return true
    }
}
