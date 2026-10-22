package com.poro.app.core.legal

import android.content.Context
import com.poro.app.R

/** Documents embarqués dans `res/raw` : disponibles hors ligne, mis à jour avec l'application. */
class RawLegalTextSource(private val context: Context) : LegalTextSource {
    override fun read(id: LegalDocumentId): String? {
        val res = when (id) {
            LegalDocumentId.Terms -> R.raw.legal_terms
            LegalDocumentId.Privacy -> R.raw.legal_privacy
            LegalDocumentId.Community -> R.raw.legal_community
            LegalDocumentId.Marketplace -> R.raw.legal_marketplace
        }
        return runCatching { context.resources.openRawResource(res).bufferedReader().use { it.readText() } }.getOrNull()
    }
}
