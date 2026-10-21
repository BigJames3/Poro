package com.poro.app.ui.legal

import androidx.navigation.NavGraphBuilder
import androidx.navigation.compose.composable
import androidx.navigation.toRoute
import com.poro.app.ui.AppNav
import com.poro.app.ui.LegalDocumentRoute
import com.poro.app.ui.LegalRoute

/** Documents légaux : consultables sans session (pied des écrans de connexion et paramètres). */
fun NavGraphBuilder.legalGraph(nav: AppNav) {
    composable<LegalRoute> {
        LegalScreen(
            onBack = nav::back,
            onOpen = { document -> nav.controller.navigate(LegalDocumentRoute(document.id)) },
        )
    }
    composable<LegalDocumentRoute> { entry ->
        val route = entry.toRoute<LegalDocumentRoute>()
        LegalDocumentScreen(id = route.id, onBack = nav::back)
    }
}
