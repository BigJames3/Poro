package com.poro.app.ui.shop

import androidx.activity.compose.BackHandler
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import androidx.lifecycle.createSavedStateHandle
import androidx.navigation.NavGraphBuilder
import androidx.navigation.compose.composable
import androidx.navigation.toRoute
import com.poro.app.R
import com.poro.app.ui.AddressesRoute
import com.poro.app.ui.AppNav
import com.poro.app.ui.CartRoute
import com.poro.app.ui.CheckoutRoute
import com.poro.app.ui.CollectEvents
import com.poro.app.ui.ConsumeResult
import com.poro.app.ui.MyShopRoute
import com.poro.app.ui.OrderRoute
import com.poro.app.ui.OrdersRoute
import com.poro.app.ui.ProductEditorRoute
import com.poro.app.ui.ProductRoute
import com.poro.app.ui.RESULT_ADDRESSES_CHANGED
import com.poro.app.ui.SearchRoute
import com.poro.app.ui.ShopHomeRoute
import com.poro.app.ui.ShopRoute
import com.poro.app.ui.poroViewModel
import com.poro.app.ui.search.SearchTab

/** Un écran enfant a changé le catalogue du vendeur (produit créé, modifié ou supprimé). */
const val RESULT_PRODUCTS_CHANGED = "products_changed"

/** Boutiques, produits, panier, commande et suivi, côté acheteur et côté vendeur. */
fun NavGraphBuilder.shopGraph(nav: AppNav) {
    val container = nav.container

    composable<ShopHomeRoute> {
        val vm = poroViewModel { ShopHomeViewModel(container.shopRepository, container.orderRepository) }
        val state by vm.state.collectAsStateWithLifecycle()
        // À chaque affichage de l'onglet : le panier a pu changer entre-temps.
        LaunchedEffect(Unit) { vm.refresh() }
        CollectEvents(vm.events) { event ->
            when (event) {
                is ShopHomeEvent.OpenShop -> nav.controller.navigate(ShopRoute(event.handle))
            }
        }
        ShopHomeScreen(
            state = state,
            onHandleChange = vm::onHandleChange,
            onFind = vm::find,
            onCart = { nav.controller.navigate(CartRoute) },
            onOrders = { nav.controller.navigate(OrdersRoute()) },
            onMyShop = { nav.controller.navigate(MyShopRoute) },
            onSearchCatalog = { nav.controller.navigate(SearchRoute(tab = SearchTab.Products.name)) },
        )
    }
    composable<ShopRoute> { entry ->
        val route = entry.toRoute<ShopRoute>()
        val vm = poroViewModel { ShopViewModel(route.handle, container.shopRepository) }
        val state by vm.state.collectAsStateWithLifecycle()
        ShopScreen(
            state = state,
            onBack = nav::back,
            onRetry = vm::load,
            onLoadMore = vm::loadMore,
            onOpenProduct = { nav.controller.navigate(ProductRoute(it.productId)) },
        )
    }
    composable<ProductRoute> { entry ->
        val route = entry.toRoute<ProductRoute>()
        val vm = poroViewModel { ProductViewModel(route.productId, container.shopRepository, container.orderRepository) }
        val state by vm.state.collectAsStateWithLifecycle()
        CollectEvents(vm.events) { event ->
            when (event) {
                is ProductEvent.AddedToCart ->
                    nav.showMessageWithAction(R.string.product_added, R.string.product_view_cart, { nav.controller.navigate(CartRoute) }, event.quantityInCart)
            }
        }
        ProductScreen(
            state = state,
            onBack = nav::back,
            onRetry = vm::load,
            onOpenShop = { nav.controller.navigate(ShopRoute(it)) },
            onSelectVariant = vm::selectVariant,
            onQuantityChange = vm::setQuantity,
            onAddToCart = vm::addToCart,
        )
    }
    composable<CartRoute> {
        val vm = poroViewModel { CartViewModel(container.orderRepository) }
        val state by vm.state.collectAsStateWithLifecycle()
        // À l'affichage et au retour de la commande (panier vidé, prix changés).
        LaunchedEffect(Unit) { vm.load() }
        CartScreen(
            state = state,
            onBack = nav::back,
            onRetry = vm::load,
            onOpenProduct = { nav.controller.navigate(ProductRoute(it)) },
            onQuantityChange = vm::setQuantity,
            onRemove = vm::remove,
            onCheckout = { nav.controller.navigate(CheckoutRoute) },
            onClear = vm::clear,
        )
    }
    composable<AddressesRoute> {
        val vm = poroViewModel { AddressesViewModel(container.orderRepository) }
        val state by vm.state.collectAsStateWithLifecycle()
        CollectEvents(vm.events) { event ->
            when (event) {
                AddressesEvent.Saved -> nav.showMessage(R.string.addresses_saved)
                AddressesEvent.Deleted -> nav.showMessage(R.string.addresses_deleted)
            }
        }
        val back = { if (state.changed) nav.backWithResult(RESULT_ADDRESSES_CHANGED) else nav.back() }
        // Le geste retour système passe aussi par là, sinon le checkout garderait l'ancienne liste.
        BackHandler(enabled = state.changed) { back() }
        AddressesScreen(
            state = state,
            onBack = back,
            onRetry = vm::load,
            onAdd = vm::startCreate,
            onEdit = vm::startEdit,
            onFormChange = vm::onFormChange,
            onLabelChange = vm::onLabelChange,
            onSave = vm::save,
            onCancel = vm::cancelForm,
            onSetDefault = vm::setDefault,
            onDelete = vm::delete,
        )
    }
    composable<CheckoutRoute> { entry ->
        val vm = poroViewModel { CheckoutViewModel(container.orderRepository, createSavedStateHandle()) }
        val state by vm.state.collectAsStateWithLifecycle()
        ConsumeResult(entry, RESULT_ADDRESSES_CHANGED) { vm.reloadAddresses() }
        CollectEvents(vm.events) { event ->
            when (event) {
                is CheckoutEvent.Placed -> {
                    nav.showMessage(R.string.checkout_placed)
                    // Commandes passées : on les montre, sans pouvoir revenir au formulaire.
                    nav.controller.navigate(OrdersRoute()) { popUpTo<ShopHomeRoute>() }
                }
            }
        }
        CheckoutScreen(
            state = state,
            onBack = nav::back,
            onFormChange = vm::onFormChange,
            onUseAddress = vm::useAddress,
            onSaveAddressChange = vm::onSaveAddressChange,
            onReview = vm::review,
            onEdit = vm::edit,
            onConfirm = vm::confirm,
            onRetryLoad = vm::retryLoad,
            onManageAddresses = { nav.controller.navigate(AddressesRoute) },
        )
    }
    composable<OrdersRoute> { entry ->
        val route = entry.toRoute<OrdersRoute>()
        val vm = poroViewModel { OrdersViewModel(container.orderRepository, route.asSeller) }
        val state by vm.state.collectAsStateWithLifecycle()
        ConsumeResult(entry, RESULT_ORDER_CHANGED, vm::refresh)
        OrdersScreen(
            asSeller = route.asSeller,
            state = state,
            onBack = nav::back,
            onLoadMore = vm::loadMore,
            onOpen = { nav.controller.navigate(OrderRoute(it.orderId, route.asSeller)) },
        )
    }
    composable<OrderRoute> { entry ->
        val route = entry.toRoute<OrderRoute>()
        val vm = poroViewModel { OrderViewModel(route.orderId, container.orderRepository, route.asSeller) }
        val state by vm.state.collectAsStateWithLifecycle()
        OrderScreen(
            asSeller = route.asSeller,
            state = state,
            // La liste peut afficher un statut périmé : elle se recharge au retour.
            onBack = { nav.backWithResult(RESULT_ORDER_CHANGED) },
            onRetry = vm::load,
            onCancel = vm::cancel,
            onConfirmReceipt = vm::confirmReceipt,
            onTrackingChange = vm::onTrackingChange,
            onShip = vm::ship,
        )
    }
    composable<MyShopRoute> { entry ->
        val vm = poroViewModel { MyShopViewModel(container.shopRepository, container.mediaReader) }
        val state by vm.state.collectAsStateWithLifecycle()
        LaunchedEffect(Unit) { if (state.loading) vm.load() }
        ConsumeResult(entry, RESULT_PRODUCTS_CHANGED) { vm.loadProducts(reset = true) }
        CollectEvents(vm.events) { event ->
            when (event) {
                MyShopEvent.Saved -> nav.showMessage(R.string.shop_saved)
            }
        }
        MyShopScreen(
            state = state,
            onBack = nav::back,
            actions = MyShopActions(
                onOpenFormChange = vm::onOpenFormChange,
                onOpenShop = vm::openShop,
                onEditName = vm::onEditName,
                onEditDescription = vm::onEditDescription,
                onSaveDetails = vm::saveDetails,
                onSetOpen = vm::setOpen,
                onLogoPicked = vm::onLogoPicked,
                onRemoveLogo = vm::removeLogo,
                onLoadMoreProducts = { vm.loadProducts() },
                onNewProduct = { state.shop?.let { nav.controller.navigate(ProductEditorRoute(currency = it.currency)) } },
                onEditProduct = { product -> nav.controller.navigate(ProductEditorRoute(product.productId, product.currency)) },
                onSellerOrders = { nav.controller.navigate(OrdersRoute(asSeller = true)) },
            ),
        )
    }
    composable<ProductEditorRoute> { entry ->
        val route = entry.toRoute<ProductEditorRoute>()
        val vm = poroViewModel { ProductEditorViewModel(route.productId, route.currency, container.shopRepository, container.mediaReader) }
        val state by vm.state.collectAsStateWithLifecycle()
        CollectEvents(vm.events) { event ->
            when (event) {
                ProductEditorEvent.Deleted -> {
                    nav.showMessage(R.string.product_deleted)
                    nav.backWithResult(RESULT_PRODUCTS_CHANGED)
                }
            }
        }
        ProductEditorScreen(
            state = state,
            currency = route.currency,
            onBack = { nav.backWithResult(RESULT_PRODUCTS_CHANGED) },
            actions = ProductEditorActions(
                onTitleChange = vm::onTitleChange,
                onDescriptionChange = vm::onDescriptionChange,
                onVariantChange = vm::onVariantChange,
                onAddVariant = vm::addVariant,
                onRemoveVariant = vm::removeVariant,
                onSave = vm::save,
                onSetStatus = vm::setStatus,
                onImagePicked = vm::onImagePicked,
                onRemoveImage = vm::removeImage,
                onDelete = vm::delete,
            ),
        )
    }
}

/** Le statut d'une commande a pu changer (annulée, expédiée, reçue). */
const val RESULT_ORDER_CHANGED = "order_changed"
