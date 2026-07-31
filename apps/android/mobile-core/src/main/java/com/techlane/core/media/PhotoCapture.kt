package com.techlane.core.media

import android.content.ContentValues
import android.content.Context
import android.content.Intent
import android.content.pm.PackageManager
import android.graphics.Bitmap
import android.graphics.BitmapFactory
import android.net.Uri
import android.os.Build
import android.os.Environment
import android.provider.MediaStore
import android.provider.OpenableColumns
import androidx.core.content.FileProvider
import java.io.ByteArrayOutputStream
import java.io.File
import kotlin.math.max
import kotlin.math.roundToInt

/**
 * Camera / gallery helpers for shop floor photos (IMEI stickers, device condition).
 * Phone cameras routinely exceed the API's 5 MB limit — we downscale + recompress.
 */
object PhotoCapture {
    const val MaxUploadBytes = 5 * 1024 * 1024
    private const val TargetMaxBytes = 1_500_000
    private const val MaxEdgePx = 1600

    /**
     * Destination URI for [androidx.activity.result.contract.ActivityResultContracts.TakePicture].
     *
     * Prefer MediaStore on Android 10+ — many OEM cameras (Tecno/Infinix/Itel, some Samsung)
     * refuse FileProvider cache URIs with "Couldn't access SD card".
     * Older APIs use app-specific external files via FileProvider (no storage permission).
     */
    fun createCameraOutputUri(context: Context, prefix: String = "capture"): Uri {
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.Q) {
            val values = ContentValues().apply {
                put(MediaStore.Images.Media.DISPLAY_NAME, "$prefix-${System.currentTimeMillis()}.jpg")
                put(MediaStore.Images.Media.MIME_TYPE, "image/jpeg")
                put(MediaStore.Images.Media.RELATIVE_PATH, Environment.DIRECTORY_PICTURES + "/TechLane")
            }
            val uri = context.contentResolver.insert(MediaStore.Images.Media.EXTERNAL_CONTENT_URI, values)
            if (uri != null) {
                grantUriToCameraApps(context, uri)
                return uri
            }
        }
        return createFileProviderUri(context, prefix)
    }

    private fun createFileProviderUri(context: Context, prefix: String): Uri {
        // Root of <external-files-path> is getExternalFilesDir(null).
        val base = context.getExternalFilesDir(null) ?: context.filesDir
        val dir = File(base, "camera").apply { mkdirs() }
        val file = File(dir, "$prefix-${System.currentTimeMillis()}.jpg").apply {
            if (!exists()) createNewFile()
        }
        val uri = FileProvider.getUriForFile(
            context,
            "${context.packageName}.fileprovider",
            file,
        )
        grantUriToCameraApps(context, uri)
        return uri
    }

    /** Explicit grants help OEM camera packages that ignore intent flags alone. */
    fun grantUriToCameraApps(context: Context, uri: Uri) {
        val intent = Intent(MediaStore.ACTION_IMAGE_CAPTURE)
        val flags = Intent.FLAG_GRANT_WRITE_URI_PERMISSION or Intent.FLAG_GRANT_READ_URI_PERMISSION
        val cameras = context.packageManager.queryIntentActivities(intent, PackageManager.MATCH_DEFAULT_ONLY)
        for (info in cameras) {
            runCatching {
                context.grantUriPermission(info.activityInfo.packageName, uri, flags)
            }
        }
    }

    /**
     * Reads an image URI and returns JPEG bytes under [MaxUploadBytes].
     * Accepts camera files even when [ActivityResultContracts.TakePicture] returns false
     * (common OEM quirk) as long as the file has content.
     */
    fun readImageBytes(
        context: Context,
        uri: Uri,
        fallbackName: String = "photo.jpg",
    ): Pair<ByteArray, String> {
        val raw = context.contentResolver.openInputStream(uri)?.use { it.readBytes() }
            ?: error("Could not read photo")
        if (raw.isEmpty()) error("Photo was empty — try again")
        val name = displayName(context, uri) ?: fallbackName
        val jpeg = compressToJpeg(raw)
        if (jpeg.size > MaxUploadBytes) {
            error("Photo is still too large after compression — try again from farther away")
        }
        val outName = if (name.contains('.')) {
            name.substringBeforeLast('.') + ".jpg"
        } else {
            "$name.jpg"
        }
        return jpeg to outName
    }

    /** True when the camera wrote bytes even if the activity result was cancelled. */
    fun hasContent(context: Context, uri: Uri): Boolean {
        return runCatching {
            context.contentResolver.openInputStream(uri)?.use { stream ->
                val buf = ByteArray(16)
                stream.read(buf) > 0
            } == true
        }.getOrDefault(false)
    }

    private fun displayName(context: Context, uri: Uri): String? {
        return context.contentResolver.query(
            uri,
            arrayOf(OpenableColumns.DISPLAY_NAME),
            null,
            null,
            null,
        )?.use { cursor ->
            if (cursor.moveToFirst()) cursor.getString(0) else null
        }
    }

    fun compressToJpeg(raw: ByteArray): ByteArray {
        if (raw.size <= TargetMaxBytes && looksLikeJpeg(raw)) {
            // Already small enough — keep original when possible.
            return raw
        }
        val bounds = BitmapFactory.Options().apply { inJustDecodeBounds = true }
        BitmapFactory.decodeByteArray(raw, 0, raw.size, bounds)
        if (bounds.outWidth <= 0 || bounds.outHeight <= 0) {
            // Not a decodeable bitmap — return as-is and let the API reject if needed.
            return raw
        }
        var sample = 1
        val longEdge = max(bounds.outWidth, bounds.outHeight)
        while (longEdge / sample > MaxEdgePx * 2) {
            sample *= 2
        }
        val opts = BitmapFactory.Options().apply { inSampleSize = sample }
        val bitmap = BitmapFactory.decodeByteArray(raw, 0, raw.size, opts)
            ?: return raw
        val scaled = scaleDown(bitmap, MaxEdgePx)
        if (scaled !== bitmap) bitmap.recycle()
        try {
            var quality = 88
            var out = encode(scaled, quality)
            while (out.size > TargetMaxBytes && quality > 50) {
                quality -= 12
                out = encode(scaled, quality)
            }
            if (out.size > MaxUploadBytes && quality > 40) {
                quality = 40
                out = encode(scaled, quality)
            }
            return out
        } finally {
            scaled.recycle()
        }
    }

    private fun scaleDown(src: Bitmap, maxEdge: Int): Bitmap {
        val longEdge = max(src.width, src.height)
        if (longEdge <= maxEdge) return src
        val scale = maxEdge.toFloat() / longEdge.toFloat()
        val w = max(1, (src.width * scale).roundToInt())
        val h = max(1, (src.height * scale).roundToInt())
        return Bitmap.createScaledBitmap(src, w, h, true)
    }

    private fun encode(bitmap: Bitmap, quality: Int): ByteArray {
        val stream = ByteArrayOutputStream()
        bitmap.compress(Bitmap.CompressFormat.JPEG, quality, stream)
        return stream.toByteArray()
    }

    private fun looksLikeJpeg(bytes: ByteArray): Boolean {
        return bytes.size >= 3 &&
            bytes[0] == 0xFF.toByte() &&
            bytes[1] == 0xD8.toByte() &&
            bytes[2] == 0xFF.toByte()
    }
}
