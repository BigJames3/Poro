package com.poro.app.ui.legal

import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import com.poro.app.core.legal.LegalDocument
import com.poro.app.core.legal.LegalDocumentId
import com.poro.app.core.legal.LegalParser
import com.poro.app.core.legal.LegalTextSource
import kotlinx.coroutines.CoroutineDispatcher
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.launch
import kotlinx.coroutines.withContext

/** Un document : en cours de lecture, absent (pas encore de texte), ou disponible. */
sealed interface LegalDocumentState {
    data object Loading : LegalDocumentState
    data object Unavailable : LegalDocumentState
    data class Ready(val document: LegalDocument) : LegalDocumentState
}

/** Ligne de la liste : titre du document s'il est lisible, et son statut. */
data class LegalEntry(val id: LegalDocumentId, val document: LegalDocument?)

private fun LegalTextSource.load(id: LegalDocumentId): LegalDocument? = read(id)?.let { LegalParser.parse(id, it) }

class LegalIndexViewModel(
    private val source: LegalTextSource,
    io: CoroutineDispatcher = Dispatchers.IO,
) : ViewModel() {
    private val _entries = MutableStateFlow<List<LegalEntry>?>(null)
    val entries: StateFlow<List<LegalEntry>?> = _entries.asStateFlow()

    init {
        viewModelScope.launch {
            _entries.value = withContext(io) { LegalDocumentId.entries.map { LegalEntry(it, source.load(it)) } }
        }
    }
}

class LegalDocumentViewModel(
    id: LegalDocumentId,
    private val source: LegalTextSource,
    io: CoroutineDispatcher = Dispatchers.IO,
) : ViewModel() {
    private val _state = MutableStateFlow<LegalDocumentState>(LegalDocumentState.Loading)
    val state: StateFlow<LegalDocumentState> = _state.asStateFlow()

    init {
        viewModelScope.launch {
            val document = withContext(io) { source.load(id) }
            _state.value = document?.let { LegalDocumentState.Ready(it) } ?: LegalDocumentState.Unavailable
        }
    }
}
