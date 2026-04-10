package com.dingovault.mobile

import android.os.Bundle
import android.widget.LinearLayout
import android.widget.TextView
import androidx.appcompat.app.AppCompatActivity
import mobile.Mobile

class MainActivity : AppCompatActivity() {
    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        val root = LinearLayout(this).apply { orientation = LinearLayout.VERTICAL }
        val ext = getExternalFilesDir(null)?.absolutePath ?: filesDir.absolutePath
        val body = TextView(this).apply {
            text = "Dingovault ${Mobile.version()}\n\nVault:\n${Mobile.vaultPath(ext)}"
            textSize = 14f
            setPadding(32, 48, 32, 32)
        }
        root.addView(body)
        setContentView(root)
    }
}
