package com.poro.app.ui.common

import androidx.compose.material.icons.materialIcon
import androidx.compose.material.icons.materialPath
import androidx.compose.ui.graphics.vector.ImageVector

/** Icônes absentes de material-icons-core (le jeu étendu pèse plusieurs Mo). */
object PoroIcons {
    /** Bulle de discussion (tracé de Material « ChatBubble »). */
    val Comment: ImageVector by lazy {
        materialIcon(name = "Poro.Comment") {
            materialPath {
                moveTo(20f, 2f)
                horizontalLineTo(4f)
                curveToRelative(-1.1f, 0f, -2f, 0.9f, -2f, 2f)
                verticalLineToRelative(18f)
                lineToRelative(4f, -4f)
                horizontalLineToRelative(14f)
                curveToRelative(1.1f, 0f, 2f, -0.9f, 2f, -2f)
                verticalLineTo(4f)
                curveToRelative(0f, -1.1f, -0.9f, -2f, -2f, -2f)
                close()
            }
        }
    }
}
