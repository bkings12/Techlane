package com.techlane.core

import android.app.Activity
import android.content.Context
import android.content.ContextWrapper
import android.content.Intent
import android.os.Handler
import android.os.Looper
import android.print.PrintAttributes
import android.print.PrintManager
import android.util.Log
import android.view.View
import android.view.ViewGroup
import android.webkit.WebView
import android.webkit.WebViewClient
import android.widget.Toast
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.withContext
import okhttp3.OkHttpClient
import okhttp3.Request

/**
 * Fetch authenticated HTML documents and print or share them.
 * Does not touch theme colors.
 */
object PrintSupport {
    private const val TAG = "PrintSupport"
    private val http = OkHttpClient()
    private val mainHandler = Handler(Looper.getMainLooper())

    /**
     * Strong ref — the print spooler reads the WebView asynchronously.
     * A WeakReference lets GC collect it mid-print and kills the process.
     */
    private var activePrintWebView: WebView? = null
    private var cleanupRunnable: Runnable? = null

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

    /**
     * Opens the system print dialog for [html].
     * Never throws to the caller’s thread after scheduling — WebView / PrintManager
     * failures are caught on the main looper and fall back to share.
     */
    fun printHtml(context: Context, html: String, jobName: String = "TechLane document") {
        val activity = context.findActivity()
        if (activity == null) {
            toast(context, "Print needs an open screen — try again")
            return
        }
        if (html.isBlank()) {
            toast(activity, "Nothing to print")
            return
        }
        mainHandler.post {
            try {
                startWebViewPrint(activity, html, jobName)
            } catch (t: Throwable) {
                Log.e(TAG, "printHtml failed to start", t)
                toast(activity, "Print failed — sharing instead")
                runCatching { shareHtml(activity, html, jobName) }
            }
        }
    }

    private fun startWebViewPrint(activity: Activity, html: String, jobName: String) {
        val container = activity.findViewById<ViewGroup>(android.R.id.content)
            ?: error("No content root")

        releaseActiveWebView()

        val webView = WebView(activity).apply {
            layoutParams = ViewGroup.LayoutParams(1, 1)
            visibility = View.INVISIBLE
            setInitialScale(100)
            settings.apply {
                javaScriptEnabled = false
                loadWithOverviewMode = true
                useWideViewPort = true
                allowFileAccess = false
                builtInZoomControls = false
                displayZoomControls = false
            }
        }
        // Attached WebView is required — detached ones crash createPrintDocumentAdapter
        // on many OEM builds (including POS terminals).
        container.addView(webView)
        activePrintWebView = webView

        webView.webViewClient = object : WebViewClient() {
            override fun onPageFinished(view: WebView, url: String?) {
                mainHandler.post {
                    try {
                        if (activity.isFinishing || activity.isDestroyed) {
                            releaseActiveWebView()
                            return@post
                        }
                        val pm = activity.getSystemService(Context.PRINT_SERVICE) as? PrintManager
                        if (pm == null) {
                            toast(activity, "Printing not available on this device")
                            runCatching { shareHtml(activity, html, jobName) }
                            releaseActiveWebView()
                            return@post
                        }
                        val adapter = view.createPrintDocumentAdapter(jobName)
                        pm.print(
                            jobName,
                            adapter,
                            PrintAttributes.Builder()
                                .setMediaSize(PrintAttributes.MediaSize.ISO_A4)
                                .setColorMode(PrintAttributes.COLOR_MODE_MONOCHROME)
                                .setMinMargins(PrintAttributes.Margins.NO_MARGINS)
                                .build(),
                        )
                        // Spooler pulls pages async — keep the WebView alive, then tear down.
                        scheduleCleanup(90_000L)
                    } catch (t: Throwable) {
                        Log.e(TAG, "print job failed", t)
                        toast(activity, "Print failed — sharing instead")
                        runCatching { shareHtml(activity, html, jobName) }
                        releaseActiveWebView()
                    }
                }
            }

            @Deprecated("Deprecated in Java")
            override fun onReceivedError(
                view: WebView?,
                errorCode: Int,
                description: String?,
                failingUrl: String?,
            ) {
                Log.e(TAG, "WebView error $errorCode: $description")
                toast(activity, "Could not render receipt")
                releaseActiveWebView()
            }
        }

        webView.loadDataWithBaseURL(
            "https://techlane.local/",
            html,
            "text/html",
            "UTF-8",
            null,
        )
    }

    private fun scheduleCleanup(delayMs: Long) {
        cleanupRunnable?.let { mainHandler.removeCallbacks(it) }
        val runnable = Runnable { releaseActiveWebView() }
        cleanupRunnable = runnable
        mainHandler.postDelayed(runnable, delayMs)
    }

    private fun releaseActiveWebView() {
        cleanupRunnable?.let { mainHandler.removeCallbacks(it) }
        cleanupRunnable = null
        val view = activePrintWebView ?: return
        activePrintWebView = null
        runCatching {
            (view.parent as? ViewGroup)?.removeView(view)
            view.stopLoading()
            view.webViewClient = WebViewClient()
            view.destroy()
        }
    }

    fun shareText(context: Context, text: String, title: String = "Share") {
        val intent = Intent(Intent.ACTION_SEND).apply {
            type = "text/plain"
            putExtra(Intent.EXTRA_TEXT, text)
        }
        startChooser(context, intent, title)
    }

    fun shareHtml(context: Context, html: String, title: String = "Share receipt") {
        val intent = Intent(Intent.ACTION_SEND).apply {
            type = "text/html"
            putExtra(Intent.EXTRA_TEXT, html)
            putExtra(Intent.EXTRA_SUBJECT, title)
        }
        startChooser(context, intent, title)
    }

    private fun startChooser(context: Context, intent: Intent, title: String) {
        val chooser = Intent.createChooser(intent, title)
        if (context.findActivity() == null) {
            chooser.addFlags(Intent.FLAG_ACTIVITY_NEW_TASK)
        }
        context.startActivity(chooser)
    }

    private fun toast(context: Context, message: String) {
        mainHandler.post {
            Toast.makeText(context.applicationContext, message, Toast.LENGTH_LONG).show()
        }
    }
}

private tailrec fun Context.findActivity(): Activity? = when (this) {
    is Activity -> this
    is ContextWrapper -> baseContext.findActivity()
    else -> null
}
