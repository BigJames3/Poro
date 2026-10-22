package com.poro.app.ui.legal

import com.poro.app.core.legal.LegalDocumentId
import com.poro.app.core.legal.LegalTextSource
import com.poro.app.testing.MainDispatcherRule
import kotlinx.coroutines.ExperimentalCoroutinesApi
import kotlinx.coroutines.test.UnconfinedTestDispatcher
import kotlinx.coroutines.test.runCurrent
import kotlinx.coroutines.test.runTest
import org.junit.Assert.assertEquals
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Rule
import org.junit.Test

@OptIn(ExperimentalCoroutinesApi::class)
class LegalViewModelsTest {

    @get:Rule
    val main = MainDispatcherRule()

    private val source = LegalTextSource { id ->
        if (id == LegalDocumentId.Terms) "---\nversion: 0.1\nstatus: draft\n---\n# CGU\n\n[À valider]" else null
    }

    @Test
    fun `a missing document is unavailable and a draft is flagged`() = runTest {
        val io = UnconfinedTestDispatcher(testScheduler)

        val missing = LegalDocumentViewModel(LegalDocumentId.Privacy, source, io)
        val draft = LegalDocumentViewModel(LegalDocumentId.Terms, source, io)
        val index = LegalIndexViewModel(source, io)
        runCurrent()

        assertEquals(LegalDocumentState.Unavailable, missing.state.value)
        val ready = draft.state.value as LegalDocumentState.Ready
        assertEquals("CGU", ready.document.title)
        assertTrue(!ready.document.validated)
        val entries = index.entries.value!!
        assertEquals(LegalDocumentId.entries.toList(), entries.map { it.id })
        assertNull(entries.first { it.id == LegalDocumentId.Privacy }.document)
    }
}
