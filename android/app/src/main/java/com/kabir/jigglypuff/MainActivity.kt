package com.kabir.jigglypuff

import android.os.Bundle
import androidx.activity.ComponentActivity
import androidx.activity.compose.setContent
import androidx.activity.enableEdgeToEdge
import androidx.activity.viewModels
import com.kabir.jigglypuff.ui.JigglypuffApp

class MainActivity : ComponentActivity() {
    private val vm: AppModel by viewModels()

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        enableEdgeToEdge()
        setContent { JigglypuffApp(vm) }
    }
}
