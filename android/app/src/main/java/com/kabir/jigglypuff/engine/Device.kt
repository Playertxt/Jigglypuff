package com.kabir.jigglypuff.engine

import android.app.AppOpsManager
import android.app.usage.StorageStatsManager
import android.app.usage.UsageStatsManager
import android.content.Context
import android.content.Intent
import android.content.pm.ApplicationInfo
import android.content.pm.PackageInfo
import android.net.Uri
import android.os.Environment
import android.os.Process
import android.os.StatFs
import android.os.storage.StorageManager
import android.provider.Settings

/** Storage volumes, permissions and installed apps. */
object Device {

    fun volumes(ctx: Context): List<Volume> {
        val sm = ctx.getSystemService(StorageManager::class.java)
        val ssm = ctx.getSystemService(StorageStatsManager::class.java)
        return sm.storageVolumes.mapNotNull { v ->
            if (v.state != Environment.MEDIA_MOUNTED && v.state != Environment.MEDIA_MOUNTED_READ_ONLY) return@mapNotNull null
            val dir = v.directory ?: return@mapNotNull null
            var total = 0L
            var free = 0L
            if (v.isPrimary) {
                runCatching {
                    total = ssm.getTotalBytes(StorageManager.UUID_DEFAULT)
                    free = ssm.getFreeBytes(StorageManager.UUID_DEFAULT)
                }
            }
            if (total == 0L) runCatching {
                val st = StatFs(dir.path)
                total = st.totalBytes
                free = st.availableBytes
            }
            val uuid = if (v.isPrimary) StorageManager.UUID_DEFAULT else runCatching { sm.getUuidForPath(dir) }.getOrNull()
            Volume(
                id = if (v.isPrimary) "primary" else (v.uuid ?: dir.name),
                name = if (v.isPrimary) "Internal storage" else (v.getDescription(ctx) ?: dir.name),
                root = dir, primary = v.isPrimary, removable = v.isRemovable,
                total = total, free = free, uuid = uuid,
            )
        }.sortedBy { !it.primary }
    }

    // Permissions -------------------------------------------------------------------

    /** "All files access": needed to look through shared storage. */
    fun hasAllFiles(): Boolean = Environment.isExternalStorageManager()

    fun allFilesIntent(ctx: Context): Intent =
        Intent(Settings.ACTION_MANAGE_APP_ALL_FILES_ACCESS_PERMISSION, Uri.parse("package:" + ctx.packageName))

    /** "Usage access": needed for app sizes and when each app was last opened. */
    fun hasUsage(ctx: Context): Boolean {
        val ops = ctx.getSystemService(AppOpsManager::class.java)
        return ops.unsafeCheckOpNoThrow(AppOpsManager.OPSTR_GET_USAGE_STATS, Process.myUid(), ctx.packageName) == AppOpsManager.MODE_ALLOWED
    }

    fun usageIntent(ctx: Context): Intent =
        Intent(Settings.ACTION_USAGE_ACCESS_SETTINGS, Uri.parse("package:" + ctx.packageName))

    fun appSettingsIntent(pkg: String): Intent =
        Intent(Settings.ACTION_APPLICATION_DETAILS_SETTINGS, Uri.fromParts("package", pkg, null))

    /** Android shows its own "Uninstall this app?" prompt; nothing is removed silently. */
    fun uninstallIntent(pkg: String): Intent = Intent(Intent.ACTION_DELETE, Uri.fromParts("package", pkg, null))

    // Apps --------------------------------------------------------------------------

    /** Apps you installed (and updated system apps), with sizes and last use when usage access is on. */
    @Suppress("DEPRECATION")
    fun apps(ctx: Context): List<AppInfo> {
        val pm = ctx.packageManager
        val usage = hasUsage(ctx)
        val ssm = ctx.getSystemService(StorageStatsManager::class.java)
        val lastUsed: Map<String, Long> = if (usage) {
            runCatching {
                val usm = ctx.getSystemService(UsageStatsManager::class.java)
                usm.queryAndAggregateUsageStats(0, System.currentTimeMillis()).mapValues { it.value.lastTimeUsed }
            }.getOrDefault(emptyMap())
        } else emptyMap()
        val user = Process.myUserHandle()
        return pm.getInstalledApplications(0).mapNotNull { ai ->
            val system = ai.flags and ApplicationInfo.FLAG_SYSTEM != 0
            val updated = ai.flags and ApplicationInfo.FLAG_UPDATED_SYSTEM_APP != 0
            if (system && !updated) return@mapNotNull null
            if (ai.packageName == ctx.packageName) return@mapNotNull null
            val pi: PackageInfo = runCatching { pm.getPackageInfo(ai.packageName, 0) }.getOrNull() ?: return@mapNotNull null
            var appB = -1L
            var dataB = -1L
            var cacheB = -1L
            if (usage) runCatching {
                val st = ssm.queryStatsForPackage(ai.storageUuid, ai.packageName, user)
                appB = st.appBytes
                dataB = st.dataBytes
                cacheB = st.cacheBytes
            }
            AppInfo(
                pkg = ai.packageName, label = ai.loadLabel(pm).toString(), version = pi.versionName ?: "",
                versionCode = pi.longVersionCode, installed = pi.firstInstallTime, updated = pi.lastUpdateTime,
                system = system, storageUuid = ai.storageUuid, appBytes = appB, dataBytes = dataB, cacheBytes = cacheB,
                lastUsed = lastUsed[ai.packageName] ?: 0,
            )
        }.sortedBy { it.label.lowercase() }
    }

    /** Every package name on the device, including system apps. */
    @Suppress("DEPRECATION")
    fun installedPackages(ctx: Context): Map<String, Long> =
        ctx.packageManager.getInstalledPackages(0).associate { it.packageName to it.longVersionCode }

    /** Reads an APK file's name and version without installing it. */
    @Suppress("DEPRECATION")
    fun readApk(ctx: Context, item: Item, installed: Map<String, Long>): ApkFile {
        val pm = ctx.packageManager
        val info = runCatching { pm.getPackageArchiveInfo(item.path, 0) }.getOrNull()
        val ai = info?.applicationInfo
        if (info == null || ai == null) {
            return ApkFile(item.path, item.size, item.modified, item.name.removeSuffix(".apk"), "", "", ApkStatus.BROKEN)
        }
        ai.sourceDir = item.path
        ai.publicSourceDir = item.path
        val label = runCatching { ai.loadLabel(pm).toString() }.getOrDefault(info.packageName)
        val have = installed[info.packageName]
        val status = when {
            have == null -> ApkStatus.NOT_INSTALLED
            have == info.longVersionCode -> ApkStatus.INSTALLED
            have > info.longVersionCode -> ApkStatus.OLDER_THAN_INSTALLED
            else -> ApkStatus.UPDATE
        }
        return ApkFile(item.path, item.size, item.modified, label, info.packageName, info.versionName ?: "", status)
    }
}
