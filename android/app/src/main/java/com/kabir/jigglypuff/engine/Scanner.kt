package com.kabir.jigglypuff.engine

import kotlinx.coroutines.currentCoroutineContext
import kotlinx.coroutines.ensureActive
import java.io.File
import java.io.RandomAccessFile
import java.nio.file.Files
import java.security.MessageDigest
import java.util.concurrent.TimeUnit

/** Folders every storage volume has; Jigglypuff never deletes these themselves. */
val STANDARD_DIRS = setOf(
    "android", "dcim", "download", "downloads", "pictures", "music", "movies", "documents", "alarms",
    "notifications", "podcasts", "ringtones", "audiobooks", "recordings",
    // System folders on SD cards and USB drives.
    "lost.dir", "system volume information", "\$recycle.bin",
)

const val TRASH_DIR = ".jigglypuff-trash"

/** Looks through storage volumes. Scanning never changes anything. */
class Scanner(
    private val settings: Settings,
    private val installed: Map<String, Long>,
    /** Whether an APK file's app is already installed at that version or newer. */
    private val apkInstalled: (Item) -> Boolean = { false },
) {

    private val day = TimeUnit.DAYS.toMillis(1)

    /** Android's private app folders, Jigglypuff's trash and protected folders are never walked. */
    private fun skip(root: File, dir: File): Boolean {
        val rel = dir.path.removePrefix(root.path).trimStart('/').lowercase()
        if (rel == "android/data" || rel == "android/obb" || rel == TRASH_DIR) return true
        return settings.protected.any { dir.path == it || dir.path.startsWith("$it/") }
    }

    private fun isLink(f: File) = runCatching { Files.isSymbolicLink(f.toPath()) }.getOrDefault(true)

    /** Visits every file below dir (depth first). onDir can return false to skip a folder. */
    private suspend fun walk(root: File, dir: File, onFile: (File) -> Unit, onDir: (File) -> Boolean = { true }) {
        val stack = ArrayDeque<File>()
        stack.add(dir)
        var n = 0
        while (stack.isNotEmpty()) {
            val d = stack.removeLast()
            val kids = d.listFiles() ?: continue
            for (f in kids) {
                if (++n % 512 == 0) currentCoroutineContext().ensureActive()
                if (isLink(f)) continue
                if (f.isDirectory) {
                    if (!skip(root, f) && onDir(f)) stack.add(f)
                } else {
                    onFile(f)
                }
            }
        }
    }

    // Junk ---------------------------------------------------------------------------

    suspend fun junk(volumes: List<Volume>, progress: (Progress) -> Unit): List<JunkCategory> {
        val thumbs = mutableListOf<Item>()
        val temp = mutableListOf<Item>()
        val empty = mutableListOf<Item>()
        val apkSame = mutableListOf<Item>()
        val apkOther = mutableListOf<Item>()
        val now = System.currentTimeMillis()
        val minAge = settings.tempMinAgeHours * TimeUnit.HOURS.toMillis(1)
        var count = 0L
        for (v in volumes) {
            val root = v.root
            progress(Progress("Looking through ${v.name}"))
            walk(root, root, onFile = { f ->
                count++
                if (count % 500 == 0L) progress(Progress("${v.name}: ${f.parent?.removePrefix(root.path) ?: ""}", count))
                val name = f.name.lowercase()
                val parentName = f.parentFile?.name?.lowercase() ?: ""
                val item = Item(f.path, f.length(), f.lastModified())
                when {
                    parentName == ".thumbnails" || f.path.contains("/.thumbnails/", ignoreCase = true) -> thumbs += item
                    isTempName(name) && now - f.lastModified() >= minAge -> temp += item
                    name.endsWith(".apk") -> apkOther += item
                }
            })
            progress(Progress("${v.name}: empty folders", count))
            findEmpty(root, root, empty)
        }
        // An installer for an app that's already installed (same version or newer) is just taking space.
        if (apkOther.isNotEmpty()) {
            val same = apkOther.filter { a -> apkInstalled(a) }
            apkSame += same
            apkOther.removeAll(same.toSet())
        }
        return listOf(
            JunkCategory("thumbnails", "Thumbnail caches", "Small preview images your gallery makes again when it needs them.", Safety.SAFE, thumbs, trash = false),
            JunkCategory("temp", "Temporary and log files", "Files ending in .tmp, .temp or .log, older than ${settings.tempMinAgeHours} hours.", Safety.SAFE, temp, trash = false),
            JunkCategory("empty", "Empty folders", "Folders with nothing in them. Android's own folders are kept.", Safety.SAFE, empty, trash = false),
            JunkCategory("apk-installed", "Installers of apps you already have", "APK files for apps that are already installed at this version or newer.", Safety.SAFE, apkSame, trash = true),
            JunkCategory("apk-other", "Other APK installers", "Installers for apps that aren't installed. Keep them if you plan to install them.", Safety.CAUTION, apkOther, trash = true),
        ).filter { it.items.isNotEmpty() }
    }

    private fun isTempName(n: String) =
        n.endsWith(".tmp") || n.endsWith(".temp") || n.endsWith(".log") || n.startsWith("~$") || n == "thumbs.db" || n == ".ds_store"

    /** Adds every folder with no files anywhere below it. Returns whether dir is empty. */
    private suspend fun findEmpty(root: File, dir: File, out: MutableList<Item>): Boolean {
        currentCoroutineContext().ensureActive()
        val kids = dir.listFiles() ?: return false
        var empty = true
        val emptyKids = mutableListOf<Item>()
        for (f in kids) {
            if (isLink(f)) {
                empty = false
                continue
            }
            if (f.isDirectory) {
                if (skip(root, f) || (f.parentFile == root && f.name.lowercase() == "android")) {
                    empty = false
                    continue
                }
                if (findEmpty(root, f, emptyKids)) emptyKids += Item(f.path, 0, f.lastModified(), isDir = true) else empty = false
            } else {
                empty = false
            }
        }
        val protectedDir = dir == root || (dir.parentFile == root && dir.name.lowercase() in STANDARD_DIRS)
        if (empty && !protectedDir) return true // the caller lists dir itself; its empty children go with it
        out += emptyKids.filterNot { it.path.substringBeforeLast('/') == root.path && it.name.lowercase() in STANDARD_DIRS }
        return false
    }

    // Leftovers ----------------------------------------------------------------------

    /**
     * Folders of apps that are no longer installed: Android/media/<package>, Android/data/<package> and
     * Android/obb/<package> when Android lets us see them, and top-level folders no installed app
     * seems to use (not changed for 30 days).
     */
    suspend fun leftovers(volumes: List<Volume>, appTokens: Set<String>, progress: (Progress) -> Unit): List<Leftover> {
        val out = mutableListOf<Leftover>()
        val now = System.currentTimeMillis()
        for (v in volumes) {
            progress(Progress("Checking ${v.name}"))
            for (sub in listOf("media", "data", "obb")) {
                val base = File(v.root, "Android/$sub")
                for (d in base.listFiles().orEmpty()) {
                    if (!d.isDirectory || isLink(d) || !d.name.contains('.')) continue
                    if (installed.containsKey(d.name)) continue
                    val (size, files, newest) = stats(d)
                    out += Leftover(d.path, size, files, newest, "Named after ${d.name}, which isn't installed", high = true)
                }
            }
            for (d in v.root.listFiles().orEmpty()) {
                currentCoroutineContext().ensureActive()
                if (!d.isDirectory || isLink(d)) continue
                val name = d.name
                val low = name.lowercase()
                if (low in STANDARD_DIRS || low == TRASH_DIR || low.startsWith(".") || skip(v.root, d)) continue
                if (Tokens.of(name).any { it in appTokens }) continue
                val (size, files, newest) = stats(d)
                if (files == 0 || now - newest < 30 * day) continue
                out += Leftover(d.path, size, files, newest, "No installed app seems to use it", high = false)
            }
        }
        return out.sortedByDescending { it.size }
    }

    private suspend fun stats(dir: File): Triple<Long, Int, Long> {
        var size = 0L
        var files = 0
        var newest = dir.lastModified()
        val root = dir.parentFile ?: dir
        walk(root, dir, onFile = {
            size += it.length()
            files++
            if (it.lastModified() > newest) newest = it.lastModified()
        })
        return Triple(size, files, newest)
    }

    // Files --------------------------------------------------------------------------

    suspend fun large(volumes: List<Volume>, minBytes: Long, progress: (Progress) -> Unit): List<Item> {
        val out = mutableListOf<Item>()
        var count = 0L
        for (v in volumes) walk(v.root, v.root, onFile = { f ->
            count++
            if (count % 500 == 0L) progress(Progress(f.parent?.removePrefix(v.root.path) ?: "", count))
            val len = f.length()
            if (len >= minBytes) out += Item(f.path, len, f.lastModified())
        })
        return out.sortedByDescending { it.size }.take(300)
    }

    private val oldTypes = setOf("apk", "apks", "xapk", "zip", "rar", "7z", "tar", "gz", "xz", "bz2", "iso", "img", "exe", "msi", "dmg", "torrent")

    suspend fun oldDownloads(volumes: List<Volume>, days: Int): List<Item> {
        val cutoff = System.currentTimeMillis() - days * day
        val out = mutableListOf<Item>()
        for (v in volumes) for (dirName in listOf("Download", "Downloads")) {
            val dir = File(v.root, dirName)
            if (!dir.isDirectory) continue
            walk(v.root, dir, onFile = { f ->
                if (f.extension.lowercase() in oldTypes && f.lastModified() < cutoff) out += Item(f.path, f.length(), f.lastModified())
            })
        }
        return out.sortedByDescending { it.size }
    }

    suspend fun apks(volumes: List<Volume>, progress: (Progress) -> Unit): List<Item> {
        val out = mutableListOf<Item>()
        var count = 0L
        for (v in volumes) walk(v.root, v.root, onFile = { f ->
            count++
            if (count % 500 == 0L) progress(Progress(f.parent?.removePrefix(v.root.path) ?: "", count))
            if (f.name.endsWith(".apk", ignoreCase = true)) out += Item(f.path, f.length(), f.lastModified())
        })
        return out.sortedByDescending { it.size }
    }

    /** Groups identical files: same size, then same first and last 64 KB, then the same SHA-256. */
    suspend fun duplicates(volumes: List<Volume>, minBytes: Long, progress: (Progress) -> Unit): List<DupGroup> {
        val bySize = HashMap<Long, MutableList<File>>()
        var count = 0L
        for (v in volumes) walk(v.root, v.root, onFile = { f ->
            count++
            if (count % 500 == 0L) progress(Progress("Listing · ${f.parent?.removePrefix(v.root.path) ?: ""}", count))
            val len = f.length()
            if (len >= minBytes) bySize.getOrPut(len) { mutableListOf() } += f
        }, onDir = { !it.name.equals(".thumbnails", ignoreCase = true) })
        val candidates = bySize.filterValues { it.size > 1 }
        val total = candidates.values.sumOf { it.size }
        var done = 0
        val groups = mutableListOf<DupGroup>()
        for ((size, files) in candidates) {
            val byQuick = files.groupBy { done++; progress(Progress("Comparing · ${it.name}", done = done, total = total)); hash(it, quick = true) }
            for (same in byQuick.values.filter { it.size > 1 }) {
                currentCoroutineContext().ensureActive()
                val byFull = same.groupBy { hash(it, quick = false) }
                for ((h, g) in byFull) if (g.size > 1 && h.isNotEmpty()) {
                    groups += DupGroup(h, size, g.map { Item(it.path, it.length(), it.lastModified()) }.sortedBy { it.modified })
                }
            }
        }
        return groups.sortedByDescending { it.wasted }
    }

    private fun hash(f: File, quick: Boolean): String = runCatching {
        val md = MessageDigest.getInstance("SHA-256")
        val buf = ByteArray(64 * 1024)
        RandomAccessFile(f, "r").use { raf ->
            if (quick) {
                val n = raf.read(buf)
                if (n > 0) md.update(buf, 0, n)
                if (raf.length() > buf.size * 2) {
                    raf.seek(raf.length() - buf.size)
                    val m = raf.read(buf)
                    if (m > 0) md.update(buf, 0, m)
                }
            } else {
                while (true) {
                    val n = raf.read(buf)
                    if (n <= 0) break
                    md.update(buf, 0, n)
                }
            }
        }
        md.digest().joinToString("") { "%02x".format(it) }
    }.getOrDefault("")
}

/** Splits app and folder names into lower-case words for matching ("WhatsApp Business" → whatsapp, business). */
object Tokens {
    private val generic = setOf("com", "org", "net", "android", "app", "apps", "google", "mobile", "lite", "pro", "the", "and", "for", "io", "co")

    fun of(s: String): Set<String> {
        val words = s.replace(Regex("([a-z])([A-Z])"), "$1 $2").lowercase().split(Regex("[^a-z0-9]+"))
        val out = words.filter { it.length >= 3 && it !in generic }.toMutableSet()
        val joined = s.lowercase().replace(Regex("[^a-z0-9]"), "")
        if (joined.length >= 3) out += joined
        return out
    }
}
