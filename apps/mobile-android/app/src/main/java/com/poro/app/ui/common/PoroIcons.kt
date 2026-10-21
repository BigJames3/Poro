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

    /** Boutique (tracé de Material « Store ») : l'onglet n'est pas le panier. */
    val Store: ImageVector by lazy {
        materialIcon(name = "Poro.Store") {
            materialPath {
                moveTo(20.0f, 4.0f)
                horizontalLineTo(4.0f)
                verticalLineToRelative(2.0f)
                horizontalLineToRelative(16.0f)
                verticalLineTo(4.0f)
                close()
                moveTo(21.0f, 14.0f)
                verticalLineToRelative(-2.0f)
                lineToRelative(-1.0f, -5.0f)
                horizontalLineTo(4.0f)
                lineToRelative(-1.0f, 5.0f)
                verticalLineToRelative(2.0f)
                horizontalLineToRelative(1.0f)
                verticalLineToRelative(6.0f)
                horizontalLineToRelative(10.0f)
                verticalLineToRelative(-6.0f)
                horizontalLineToRelative(4.0f)
                verticalLineToRelative(6.0f)
                horizontalLineToRelative(2.0f)
                verticalLineToRelative(-6.0f)
                horizontalLineToRelative(1.0f)
                close()
                moveTo(12.0f, 18.0f)
                horizontalLineTo(6.0f)
                verticalLineToRelative(-4.0f)
                horizontalLineToRelative(6.0f)
                verticalLineToRelative(4.0f)
                close()
            }
        }
    }

    /** Volume (Material « VolumeUp ») : son du fil. */
    val VolumeOn: ImageVector by lazy {
        materialIcon(name = "Poro.VolumeOn") {
            materialPath {
                moveTo(3.0f, 9.0f)
                verticalLineToRelative(6.0f)
                horizontalLineToRelative(4.0f)
                lineToRelative(5.0f, 5.0f)
                verticalLineTo(4.0f)
                lineTo(7.0f, 9.0f)
                horizontalLineTo(3.0f)
                close()
                moveTo(16.5f, 12.0f)
                curveToRelative(0.0f, -1.77f, -1.02f, -3.29f, -2.5f, -4.03f)
                verticalLineToRelative(8.05f)
                curveToRelative(1.48f, -0.73f, 2.5f, -2.25f, 2.5f, -4.02f)
                close()
                moveTo(14.0f, 3.23f)
                verticalLineToRelative(2.06f)
                curveToRelative(2.89f, 0.86f, 5.0f, 3.54f, 5.0f, 6.71f)
                reflectiveCurveToRelative(-2.11f, 5.85f, -5.0f, 6.71f)
                verticalLineToRelative(2.06f)
                curveToRelative(4.01f, -0.91f, 7.0f, -4.49f, 7.0f, -8.77f)
                reflectiveCurveToRelative(-2.99f, -7.86f, -7.0f, -8.77f)
                close()
            }
        }
    }

    /** Volume coupé (Material « VolumeOff »). */
    val VolumeOff: ImageVector by lazy {
        materialIcon(name = "Poro.VolumeOff") {
            materialPath {
                moveTo(16.5f, 12.0f)
                curveToRelative(0.0f, -1.77f, -1.02f, -3.29f, -2.5f, -4.03f)
                verticalLineToRelative(2.21f)
                lineToRelative(2.45f, 2.45f)
                curveToRelative(0.03f, -0.2f, 0.05f, -0.41f, 0.05f, -0.63f)
                close()
                moveTo(19.0f, 12.0f)
                curveToRelative(0.0f, 0.94f, -0.2f, 1.82f, -0.54f, 2.64f)
                lineToRelative(1.51f, 1.51f)
                curveTo(20.63f, 14.91f, 21.0f, 13.5f, 21.0f, 12.0f)
                curveToRelative(0.0f, -4.28f, -2.99f, -7.86f, -7.0f, -8.77f)
                verticalLineToRelative(2.06f)
                curveToRelative(2.89f, 0.86f, 5.0f, 3.54f, 5.0f, 6.71f)
                close()
                moveTo(4.27f, 3.0f)
                lineTo(3.0f, 4.27f)
                lineTo(7.73f, 9.0f)
                horizontalLineTo(3.0f)
                verticalLineToRelative(6.0f)
                horizontalLineToRelative(4.0f)
                lineToRelative(5.0f, 5.0f)
                verticalLineToRelative(-6.73f)
                lineToRelative(4.25f, 4.25f)
                curveToRelative(-0.67f, 0.52f, -1.42f, 0.93f, -2.25f, 1.18f)
                verticalLineToRelative(2.06f)
                curveToRelative(1.38f, -0.31f, 2.63f, -0.95f, 3.69f, -1.81f)
                lineTo(19.73f, 21.0f)
                lineTo(21.0f, 19.73f)
                lineToRelative(-9.0f, -9.0f)
                lineTo(4.27f, 3.0f)
                close()
                moveTo(12.0f, 4.0f)
                lineTo(9.91f, 6.09f)
                lineTo(12.0f, 8.18f)
                verticalLineTo(4.0f)
                close()
            }
        }
    }

    /** Trois points verticaux (tracé de Material « MoreVert »). */
    val MoreVert: ImageVector by lazy {
        materialIcon(name = "Poro.MoreVert") {
            materialPath {
                moveTo(12.0f, 8.0f)
                curveToRelative(1.1f, 0.0f, 2.0f, -0.9f, 2.0f, -2.0f)
                reflectiveCurveToRelative(-0.9f, -2.0f, -2.0f, -2.0f)
                reflectiveCurveToRelative(-2.0f, 0.9f, -2.0f, 2.0f)
                reflectiveCurveToRelative(0.9f, 2.0f, 2.0f, 2.0f)
                close()
                moveTo(12.0f, 10.0f)
                curveToRelative(-1.1f, 0.0f, -2.0f, 0.9f, -2.0f, 2.0f)
                reflectiveCurveToRelative(0.9f, 2.0f, 2.0f, 2.0f)
                reflectiveCurveToRelative(2.0f, -0.9f, 2.0f, -2.0f)
                reflectiveCurveToRelative(-0.9f, -2.0f, -2.0f, -2.0f)
                close()
                moveTo(12.0f, 16.0f)
                curveToRelative(-1.1f, 0.0f, -2.0f, 0.9f, -2.0f, 2.0f)
                reflectiveCurveToRelative(0.9f, 2.0f, 2.0f, 2.0f)
                reflectiveCurveToRelative(2.0f, -0.9f, 2.0f, -2.0f)
                reflectiveCurveToRelative(-0.9f, -2.0f, -2.0f, -2.0f)
                close()
            }
        }
    }
}
