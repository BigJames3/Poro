package com.poro.app.ui.legal

import androidx.annotation.StringRes
import com.poro.app.R

/** Documents exigés par les stores. Aucun corps opposable n'est encore validé. */
enum class LegalDocument(val id: String, @StringRes val title: Int) {
    Terms("tos", R.string.legal_tos),
    Privacy("privacy", R.string.legal_privacy),
    Mentions("mentions", R.string.legal_mentions),
    ;

    companion object {
        fun fromId(id: String): LegalDocument? = entries.find { it.id == id }
    }
}
