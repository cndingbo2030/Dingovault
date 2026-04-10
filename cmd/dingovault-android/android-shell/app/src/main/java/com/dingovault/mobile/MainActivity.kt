package com.dingovault.mobile

import android.Manifest
import android.annotation.SuppressLint
import android.content.pm.PackageManager
import android.os.Build
import android.os.Bundle
import android.view.View
import android.webkit.WebChromeClient
import android.webkit.WebResourceRequest
import android.webkit.WebView
import android.webkit.WebViewClient
import androidx.activity.OnBackPressedCallback
import androidx.appcompat.app.AlertDialog
import androidx.appcompat.app.AppCompatActivity
import androidx.core.app.ActivityCompat
import androidx.core.content.ContextCompat
import mobile.EventSink
import mobile.Mobile
import org.json.JSONObject
import java.util.concurrent.Executors

class MainActivity : AppCompatActivity() {

    private lateinit var webView: WebView
    private lateinit var splash: View
    private val executor = Executors.newSingleThreadExecutor()

    @SuppressLint("SetJavaScriptEnabled")
    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        setContentView(R.layout.activity_main)
        webView = findViewById(R.id.webview)
        splash = findViewById(R.id.splash)

        WebView.setWebContentsDebuggingEnabled(BuildConfig.DEBUG)

        webView.settings.javaScriptEnabled = true
        webView.settings.domStorageEnabled = true
        webView.settings.allowFileAccess = true
        webView.settings.allowFileAccessFromFileURLs = true
        webView.settings.allowUniversalAccessFromFileURLs = true
        webView.setBackgroundColor(0xFF121216.toInt())
        webView.webChromeClient = WebChromeClient()

        onBackPressedDispatcher.addCallback(
            this,
            object : OnBackPressedCallback(true) {
                override fun handleOnBackPressed() {
                    webView.evaluateJavascript(
                        "(function(){try{if(window.__dingoConsumeAndroidBack)return !!window.__dingoConsumeAndroidBack();}catch(e){}return false;})()"
                    ) { res ->
                        val consumed = res == "true" || res == "\"true\""
                        if (!consumed) {
                            isEnabled = false
                            onBackPressedDispatcher.onBackPressed()
                            isEnabled = true
                        }
                    }
                }
            }
        )

        maybeAskStorageThenBoot()
    }

    private fun maybeAskStorageThenBoot() {
        val prefs = getSharedPreferences("dingovault", MODE_PRIVATE)
        if (prefs.getBoolean("storage_rationale_shown", false)) {
            startGoBackend()
            return
        }
        AlertDialog.Builder(this)
            .setTitle(R.string.storage_dialog_title)
            .setMessage(R.string.storage_dialog_message)
            .setCancelable(false)
            .setPositiveButton(R.string.storage_dialog_accept) { _, _ ->
                prefs.edit().putBoolean("storage_rationale_shown", true).apply()
                requestOptionalExternalRead()
                startGoBackend()
            }
            .setNegativeButton(R.string.storage_dialog_skip) { _, _ ->
                prefs.edit().putBoolean("storage_rationale_shown", true).apply()
                startGoBackend()
            }
            .show()
    }

    private fun requestOptionalExternalRead() {
        if (Build.VERSION.SDK_INT < 23) return
        if (Build.VERSION.SDK_INT > 32) return
        val perm = Manifest.permission.READ_EXTERNAL_STORAGE
        if (ContextCompat.checkSelfPermission(this, perm) == PackageManager.PERMISSION_GRANTED) return
        ActivityCompat.requestPermissions(this, arrayOf(perm), REQ_STORAGE)
    }

    private fun startGoBackend() {
        val files = filesDir.absolutePath
        val ext = getExternalFilesDir(null)?.absolutePath ?: files
        executor.execute {
            val initErr = try {
                Mobile.init(files, ext)
                null
            } catch (e: Exception) {
                e.message ?: e.toString()
            }
            runOnUiThread {
                if (initErr != null) {
                    AlertDialog.Builder(this@MainActivity)
                        .setTitle(R.string.init_failed_title)
                        .setMessage(initErr)
                        .setPositiveButton(android.R.string.ok, null)
                        .show()
                    splash.visibility = View.GONE
                    return@runOnUiThread
                }
                Mobile.setEventSink(WebEventSink(webView))
                webView.addJavascriptInterface(JsBridge(), "AndroidBridge")
                webView.webViewClient = object : WebViewClient() {
                    override fun shouldOverrideUrlLoading(view: WebView, request: WebResourceRequest): Boolean {
                        return false
                    }

                    override fun onPageFinished(view: WebView, url: String) {
                        splash.visibility = View.GONE
                        webView.visibility = View.VISIBLE
                    }
                }
                webView.loadUrl("file:///android_asset/dist/index.html")
            }
        }
    }

    private inner class JsBridge {
        @JavascriptInterface
        fun call(method: String, argsJson: String, promiseId: String) {
            executor.execute {
                val out: String = try {
                    Mobile.invoke(method, argsJson)
                } catch (e: Exception) {
                    "{\"ok\":false,\"error\":" + JSONObject.quote(e.message ?: "error") + "}"
                }
                val pid = JSONObject.quote(promiseId)
                val payload = JSONObject.quote(out)
                runOnUiThread {
                    webView.evaluateJavascript("window.__dingoResolve($pid, $payload);", null)
                }
            }
        }
    }

    private class WebEventSink(private val wv: WebView) : EventSink {
        override fun emit(name: String?, payloadJSON: String?) {
            val n = JSONObject.quote(name ?: "")
            val raw = payloadJSON?.trim()?.takeIf { it.isNotEmpty() } ?: "{}"
            wv.post {
                val js =
                    "(function(){var n=$n;var p=$raw;var evs=(window.__dvEvs&&window.__dvEvs[n])||[];" +
                        "evs.forEach(function(f){try{f(p);}catch(e){}});})();"
                wv.evaluateJavascript(js, null)
            }
        }
    }

    override fun onDestroy() {
        Mobile.setEventSink(null)
        try {
            executor.submit {
                try {
                    Mobile.shutdown()
                } catch (_: Exception) {
                }
            }.get()
        } catch (_: Exception) {
        }
        executor.shutdownNow()
        if (this::webView.isInitialized) {
            webView.removeAllViews()
            webView.destroy()
        }
        super.onDestroy()
    }

    companion object {
        private const val REQ_STORAGE = 1001
    }
}
