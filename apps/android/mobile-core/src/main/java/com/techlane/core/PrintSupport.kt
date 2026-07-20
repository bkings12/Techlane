package com.techlane.core

import android.content.Context
import android.content.Intent
import android.print.PrintAttributes
import android.print.PrintManager
import android.webkit.WebView
import android.webkit.WebViewClient
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.withContext
import okhttp3.OkHttpClient
import okhttp3.Request

/**
 * Fetch authenticated HTML/PDF documents and print or share them.
 * Does not touch theme colors.
 */
object PrintSupport {
    private val http = OkHttpClient()

    suspend fun fetchBytes(url: String, bearerToken: String?): ByteArray = withContext(Dispatchers.IO) {
        val req = Request.Builder().url(url).apply {
            if (!bearerToken.isNullOrBlank()) header("Authorization", "Bearer $bearerToken")
        }.build()
        http.newCall(req).execute().use { res ->
            if (!res.isSuccessful) error("Download failed: HTTP ${res.code}")
            res.body?.bytes() ?: error("Empty body")
        }
    }

    suspend fun fetchText(url: String, bearerToken: String?): String =
        String(fetchBytes(url, bearerToken), Charsets.UTF_8)

    fun printHtml(context: Context, html: String, jobName: String = "TechLane document") {
        val webView = WebView(context.applicationContext)
        webView.webViewClient = object : WebViewClient() {
            override fun onPageFinished(view: WebView, url: String?) {
                val pm = context.getSystemService(Context.PRINT_SERVICE) as PrintManager
                val adapter = view.createPrintDocumentAdapter(jobName)
                pm.print(jobName, adapter, PrintAttributes.Builder().build())
            }
        }
        webView.loadDataWithBaseURL(null, html, "text/html", "UTF-8", null)
    }

    fun shareText(context: Context, text: String, title: String = "Share") {
        val intent = Intent(Intent.ACTION_SEND).apply {
            type = "text/plain"
            putExtra(Intent.EXTRA_TEXT, text)
        }
        context.startActivity(Intent.createChooser(intent, title))
    }
}
