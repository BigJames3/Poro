package com.poro.app.ui.social

import android.content.ActivityNotFoundException
import android.content.Context
import android.content.Intent
import com.poro.app.data.social.ShareChannel

/**
 * Ouvre l'application de partage choisie avec [text]. Renvoie false si elle n'est pas installée
 * (WhatsApp absent) : l'appelant propose alors le partage générique.
 */
fun launchShare(context: Context, text: String, channel: ShareChannel): Boolean {
    val send = Intent(Intent.ACTION_SEND).setType("text/plain").putExtra(Intent.EXTRA_TEXT, text)
    return try {
        when (channel) {
            ShareChannel.WhatsApp -> context.startActivity(send.setPackage(WHATSAPP_PACKAGE))
            else -> context.startActivity(Intent.createChooser(send, null))
        }
        true
    } catch (e: ActivityNotFoundException) {
        false
    }
}

/**
 * Texte partagé. Le projet n'a pas encore de site public ni de liens d'application : sans
 * [shareBaseUrl] (BuildConfig.SHARE_BASE_URL, vide par défaut), le texte ne contient pas de lien.
 */
fun shareText(title: String, videoId: String, shareBaseUrl: String): String {
    val link = shareBaseUrl.trimEnd('/').takeIf { it.isNotEmpty() }?.let { "$it/v/$videoId" }
    return listOfNotNull("« $title » sur Poro", link).joinToString("\n")
}

private const val WHATSAPP_PACKAGE = "com.whatsapp"
