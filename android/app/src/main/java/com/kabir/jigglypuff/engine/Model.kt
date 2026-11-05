package com.kabir.jigglypuff.engine

import java.io.File
import java.util.UUID

/** A storage volume: internal storage, an SD card or a USB drive. */
data class Volume(
    val id: String,
    val name: String,
    val root: File,
    val primary: Boolean,
    val removable: Boolean,
    val total: Long,
    val free: Long,
    /** The volume's id for app-storage queries, null when apps can't live on it. */
    val uuid: UUID?,
) {
    val used get() = (total - free).coerceAtLeast(0)
}

enum class Safety { SAFE, CAUTION, REVIEW }

/** One file or folder found by a scan. */
data class Item(val path: String, val size: Long, val modified: Long = 0, val isDir: Boolean = false) {
    val name: String get() = path.substringAfterLast('/')
    val parent: String get() = path.substringBeforeLast('/', "")
}

/** A junk category on the clean screen. */
data class JunkCategory(
    val id: String,
    val title: String,
    val desc: String,
    val safety: Safety,
    val items: List<Item>,
    /** User files go to Jigglypuff's trash, caches are deleted straight away. */
    val trash: Boolean,
) {
    val size: Long = items.sumOf { it.size }
}

/** A folder left behind by an app that is no longer installed. */
data class Leftover(
    val path: String,
    val size: Long,
    val files: Int,
    val modified: Long,
    val reason: String,
    /** High confidence: named after a package that isn't installed. */
    val high: Boolean,
) {
    val name: String get() = path.substringAfterLast('/')
}

data class DupGroup(val id: String, val size: Long, val files: List<Item>) {
    val wasted: Long get() = size * (files.size - 1)
}

data class AppInfo(
    val pkg: String,
    val label: String,
    val version: String,
    val versionCode: Long,
    val installed: Long,
    val updated: Long,
    val system: Boolean,
    val storageUuid: UUID?,
    val appBytes: Long,
    val dataBytes: Long,
    val cacheBytes: Long,
    /** 0 when Android has no record of it being opened. */
    val lastUsed: Long,
) {
    /** App plus its data (data already includes the cache); -1 if unknown. */
    val size: Long get() = if (appBytes < 0) -1 else appBytes + dataBytes
    val canUninstall: Boolean get() = !system
}

enum class ApkStatus { INSTALLED, OLDER_THAN_INSTALLED, UPDATE, NOT_INSTALLED, BROKEN }

/** An APK installer file lying on a storage volume. */
data class ApkFile(
    val path: String,
    val size: Long,
    val modified: Long,
    val label: String,
    val pkg: String,
    val versionName: String,
    val status: ApkStatus,
) {
    val name: String get() = path.substringAfterLast('/')
}

data class Progress(val message: String = "", val count: Long = 0, val done: Int = 0, val total: Int = 0)

data class CleanResult(
    val freed: Long,
    val removed: Int,
    val trashed: Int,
    val failed: Int,
    val skipped: Int,
    val errors: List<String>,
)

/** What a confirmation asks about and then removes, exactly. */
data class Target(val path: String, val size: Long, val trash: Boolean, val isDir: Boolean = false)

fun Long.fmtSize(): String {
    if (this < 0) return "—"
    if (this < 1024) return "$this B"
    val units = arrayOf("KB", "MB", "GB", "TB")
    var v = this / 1024.0
    var i = 0
    while (v >= 1024 && i < units.size - 1) {
        v /= 1024
        i++
    }
    return if (v >= 100 || i == 0) "${v.toLong()} ${units[i]}" else String.format(java.util.Locale.US, "%.1f %s", v, units[i])
}
