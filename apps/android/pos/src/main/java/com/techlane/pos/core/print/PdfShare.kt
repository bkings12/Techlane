package com.techlane.pos.core.print

import android.content.Context
import android.content.Intent
import androidx.core.content.FileProvider
import java.io.File

/**
 * Shares a receipt PDF through Android's native share sheet. Not sales- or
 * repair-specific — both receipt types render server-side to the same
 * [internal/receipts.Document] shape and both already expose a receipt.pdf
 * endpoint, so this is the one place that turns "PDF bytes" into "a share
 * intent" for either.
 */
object PdfShare {
    /**
     * Writes [bytes] to the app's cache (see file_paths.xml's receipts_cache)
     * and opens the system share sheet for it as a PDF.
     */
    fun share(context: Context, bytes: ByteArray, fileName: String, subject: String = "Receipt") {
        val dir = File(context.cacheDir, "receipts").apply { mkdirs() }
        val file = File(dir, fileName)
        file.writeBytes(bytes)
        val uri = FileProvider.getUriForFile(context, "${context.packageName}.fileprovider", file)
        val intent = Intent(Intent.ACTION_SEND).apply {
            type = "application/pdf"
            putExtra(Intent.EXTRA_STREAM, uri)
            putExtra(Intent.EXTRA_SUBJECT, subject)
            addFlags(Intent.FLAG_GRANT_READ_URI_PERMISSION)
        }
        val chooser = Intent.createChooser(intent, subject)
        if (context !is android.app.Activity) chooser.addFlags(Intent.FLAG_ACTIVITY_NEW_TASK)
        context.startActivity(chooser)
    }
}
