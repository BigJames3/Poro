package com.poro.app.ui.search

import androidx.compose.runtime.getValue
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import androidx.lifecycle.createSavedStateHandle
import androidx.navigation.NavGraphBuilder
import androidx.navigation.compose.composable
import com.poro.app.ui.AppNav
import com.poro.app.ui.ProductRoute
import com.poro.app.ui.PublicProfileRoute
import com.poro.app.ui.SearchRoute
import com.poro.app.ui.ShopRoute
import com.poro.app.ui.VideoRoute
import com.poro.app.ui.poroViewModel

/** Onglet « Rechercher ». */
fun NavGraphBuilder.searchGraph(nav: AppNav) {
    composable<SearchRoute> {
        val vm = poroViewModel { SearchViewModel(nav.container.searchRepository, createSavedStateHandle()) }
        val state by vm.state.collectAsStateWithLifecycle()
        SearchScreen(
            state = state,
            onQueryChange = vm::onQueryChange,
            onSubmit = vm::submit,
            onSelectTab = vm::selectTab,
            onLoadMore = { vm.loadMore() },
            onOpenVideo = { nav.controller.navigate(VideoRoute(it.videoId)) },
            onOpenUser = { nav.controller.navigate(PublicProfileRoute(it.userId, it.username)) },
            onOpenProduct = { nav.controller.navigate(ProductRoute(it.productId)) },
            onOpenShop = { nav.controller.navigate(ShopRoute(it.handle)) },
        )
    }
}
