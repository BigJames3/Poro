package com.poro.app

import android.os.Bundle
import androidx.activity.ComponentActivity
import androidx.activity.compose.setContent
import androidx.activity.enableEdgeToEdge
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.padding
import androidx.compose.material3.Scaffold
import androidx.compose.ui.Modifier
import com.poro.app.ui.PoroNavHost
import com.poro.app.ui.theme.PoroTheme

class MainActivity : ComponentActivity() {
    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        enableEdgeToEdge()
        val container = (application as PoroApp).container
        setContent {
            PoroTheme {
                Scaffold(modifier = Modifier.fillMaxSize()) { padding ->
                    PoroNavHost(container = container, modifier = Modifier.padding(padding))
                }
            }
        }
    }
}
