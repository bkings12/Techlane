package com.techlane.pos.core.print

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

/**
 * Hands receipt HTML to the Android print framework.
 *
 * The awkward parts here are load-bearing and were learned the hard way on real
 * counter hardware, so they are carried over from the ops app rather than
 * rewritten:
 *
 *  - the WebView must be **attached** to the window; a detached one crashes
 *    createPrintDocumentAdapter on several OEM builds, POS terminals included;
 *  - it must be held by a **strong** reference, because the print spooler reads
 *    it asynchronously and a collected WebView takes the process down with it;
 *  - failures fall back to a share sheet, so a receipt is never simply lost.
 */
object ReceiptPrinter {

    private const val TAG = "ReceiptPrinter"
    private val mainHandler = Handler(Looper.getMainLooper())

    private var activeWebView: WebView? = null
    private var cleanup: Runnable? = null

    fun print(context: Context, html: String, jobName: String = "TechLane receipt") {
        val activity = context.findActivity()
        if (activity == null) {
            toast(context, "Printing needs an open screen — try again")
            return
        }
        if (html.isBlank()) {
            toast(activity, "Nothing to print")
            return
        }
        mainHandler.post {
            try {
                startPrint(activity, html, jobName)
            } catch (t: Throwable) {
                Log.e(TAG, "print failed to start", t)
                toast(activity, "Print unavailable — sharing instead")
                runCatching { share(activity, html, jobName) }
            }
        }
    }

    fun share(context: Context, html: String, title: String = "Receipt") {
        val intent = Intent(Intent.ACTION_SEND).apply {
            type = "text/html"
            putExtra(Intent.EXTRA_TEXT, html)
            putExtra(Intent.EXTRA_SUBJECT, title)
        }
        val chooser = Intent.createChooser(intent, title)
        if (context.findActivity() == null) chooser.addFlags(Intent.FLAG_ACTIVITY_NEW_TASK)
        context.startActivity(chooser)
    }

    private fun startPrint(activity: Activity, html: String, jobName: String) {
        val container = activity.findViewById<ViewGroup>(android.R.id.content) ?: error("No content root")
        release()

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
        container.addView(webView)
        activeWebView = webView

        webView.webViewClient = object : WebViewClient() {
            override fun onPageFinished(view: WebView, url: String?) {
                mainHandler.post {
                    try {
                        if (activity.isFinishing || activity.isDestroyed) {
                            release()
                            return@post
                        }
                        val printManager = activity.getSystemService(Context.PRINT_SERVICE) as? PrintManager
                        if (printManager == null) {
                            toast(activity, "Printing not available on this device")
                            runCatching { share(activity, html, jobName) }
                            release()
                            return@post
                        }
                        printManager.print(
                            jobName,
                            view.createPrintDocumentAdapter(jobName),
                            PrintAttributes.Builder()
                                .setMediaSize(PrintAttributes.MediaSize.ISO_A4)
                                .setColorMode(PrintAttributes.COLOR_MODE_MONOCHROME)
                                .setMinMargins(PrintAttributes.Margins.NO_MARGINS)
                                .build(),
                        )
                        // The spooler pulls pages long after print() returns.
                        scheduleRelease(90_000L)
                    } catch (t: Throwable) {
                        Log.e(TAG, "print job failed", t)
                        toast(activity, "Print failed — sharing instead")
                        runCatching { share(activity, html, jobName) }
                        release()
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
                toast(activity, "Could not render the receipt")
                release()
            }
        }

        webView.loadDataWithBaseURL("https://techlane.local/", html, "text/html", "UTF-8", null)
    }

    private fun scheduleRelease(delayMs: Long) {
        cleanup?.let { mainHandler.removeCallbacks(it) }
        val runnable = Runnable { release() }
        cleanup = runnable
        mainHandler.postDelayed(runnable, delayMs)
    }

    private fun release() {
        cleanup?.let { mainHandler.removeCallbacks(it) }
        cleanup = null
        val view = activeWebView ?: return
        activeWebView = null
        runCatching {
            (view.parent as? ViewGroup)?.removeView(view)
            view.stopLoading()
            view.webViewClient = WebViewClient()
            view.destroy()
        }
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
