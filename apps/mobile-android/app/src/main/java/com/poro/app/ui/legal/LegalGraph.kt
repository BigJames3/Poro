package com.poro.app.ui.legal

import androidx.compose.runtime.getValue
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import androidx.navigation.NavGraphBuilder
import androidx.navigation.compose.composable
import androidx.navigation.toRoute
import com.poro.app.core.legal.LegalDocumentId
import com.poro.app.ui.AppNav
import com.poro.app.ui.LegalDocumentRoute
import com.poro.app.ui.LegalIndexRoute
import com.poro.app.ui.poroViewModel

/** Informations légales, consultables avec ou sans session (liens des écrans de connexion). */
fun NavGraphBuilder.legalGraph(nav: AppNav) {
    val container = nav.container

    composable<LegalIndexRoute> {
        val vm = poroViewModel { LegalIndexViewModel(container.legalTexts) }
        val entries by vm.entries.collectAsStateWithLifecycle()
        LegalIndexScreen(entries, onBack = nav::back, onOpen = { nav.controller.navigate(LegalDocumentRoute(it.slug)) })
    }
    composable<LegalDocumentRoute> { entry ->
        val id = LegalDocumentId.fromSlug(entry.toRoute<LegalDocumentRoute>().slug) ?: LegalDocumentId.Terms
        val vm = poroViewModel { LegalDocumentViewModel(id, container.legalTexts) }
        val state by vm.state.collectAsStateWithLifecycle()
        LegalDocumentScreen(id, state, onBack = nav::back)
    }
}
