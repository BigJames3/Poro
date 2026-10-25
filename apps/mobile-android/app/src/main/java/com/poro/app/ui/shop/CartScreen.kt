package com.poro.app.ui.shop

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.outlined.ShoppingCart
import androidx.compose.material3.AlertDialog
import androidx.compose.material3.HorizontalDivider
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Scaffold
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.saveable.rememberSaveable
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.layout.ContentScale
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.semantics.heading
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.unit.dp
import coil3.compose.AsyncImage
import com.poro.app.R
import com.poro.app.core.format.Formats
import com.poro.app.data.order.CartLine
import com.poro.app.ui.auth.ErrorMessageText
import com.poro.app.ui.auth.SubmitButton
import com.poro.app.ui.common.BackTopBar
import com.poro.app.ui.common.EmptyState
import com.poro.app.ui.common.LoadableContent
import com.poro.app.ui.theme.PoroSpacing

@Composable
fun CartScreen(
    state: CartUiState,
    onBack: () -> Unit,
    onRetry: () -> Unit,
    onOpenProduct: (String) -> Unit,
    onQuantityChange: (variantId: String, quantity: Int) -> Unit,
    onRemove: (variantId: String) -> Unit,
    onCheckout: () -> Unit,
    onClear: () -> Unit,
) {
    // Vider retire toutes les lignes d'un coup : on demande confirmation.
    var confirmClear by rememberSaveable { mutableStateOf(false) }
    if (confirmClear) {
        AlertDialog(
            onDismissRequest = { confirmClear = false },
            title = { Text(stringResource(R.string.cart_clear_confirm_title)) },
            text = { Text(stringResource(R.string.cart_clear_confirm_text)) },
            confirmButton = {
                TextButton(onClick = {
                    confirmClear = false
                    onClear()
                }) { Text(stringResource(R.string.cart_clear)) }
            },
            dismissButton = { TextButton(onClick = { confirmClear = false }) { Text(stringResource(R.string.action_cancel)) } },
        )
    }
    Scaffold(topBar = { BackTopBar(stringResource(R.string.cart_title), onBack) }) { padding ->
        LoadableContent(state.cart, onRetry, Modifier.padding(padding)) { cart ->
            if (cart.shops.isEmpty() && cart.unavailable.isEmpty()) {
                EmptyState(stringResource(R.string.cart_empty), Modifier.padding(padding), icon = Icons.Outlined.ShoppingCart)
                return@LoadableContent
            }
            LazyColumn(Modifier.fillMaxSize().padding(padding), verticalArrangement = Arrangement.spacedBy(PoroSpacing.S)) {
                item(key = "clear") {
                    TextButton(
                        onClick = { confirmClear = true },
                        enabled = "cart" !in state.busy,
                        modifier = Modifier.padding(horizontal = PoroSpacing.S),
                    ) { Text(stringResource(R.string.cart_clear)) }
                }
                cart.shops.forEach { group ->
                    item(key = "shop-${group.shop.shopId}") {
                        Text(group.shop.name, style = MaterialTheme.typography.titleMedium, modifier = Modifier.padding(PoroSpacing.L).semantics { heading() })
                    }
                    items(group.items, key = { it.variantId }) { line ->
                        CartLineRow(line, group.shop.currency, line.variantId in state.busy, onOpenProduct, onQuantityChange, onRemove)
                    }
                    item(key = "subtotal-${group.shop.shopId}") {
                        Text(
                            stringResource(R.string.cart_subtotal, Formats.money(group.subtotal, group.shop.currency)),
                            style = MaterialTheme.typography.titleSmall,
                            modifier = Modifier.padding(horizontal = PoroSpacing.L),
                        )
                        HorizontalDivider(Modifier.padding(top = PoroSpacing.S))
                    }
                }
                if (cart.unavailable.isNotEmpty()) {
                    item(key = "unavailable") {
                        Text(
                            stringResource(R.string.cart_unavailable),
                            color = MaterialTheme.colorScheme.error,
                            modifier = Modifier.padding(PoroSpacing.L).semantics { heading() },
                        )
                    }
                    items(cart.unavailable, key = { "u-" + it.variantId }) { line ->
                        CartLineRow(line, null, line.variantId in state.busy, onOpenProduct, onQuantityChange, onRemove)
                    }
                }
                item(key = "checkout") {
                    Column(Modifier.padding(PoroSpacing.L), verticalArrangement = Arrangement.spacedBy(PoroSpacing.S)) {
                        state.error?.let { ErrorMessageText(it) }
                        Text(stringResource(R.string.cart_payment_notice), style = MaterialTheme.typography.bodySmall)
                        SubmitButton(stringResource(R.string.cart_checkout), enabled = state.canCheckout, loading = false, onClick = onCheckout)
                    }
                }
            }
        }
    }
}

@Composable
private fun CartLineRow(
    line: CartLine,
    currency: String?,
    busy: Boolean,
    onOpenProduct: (String) -> Unit,
    onQuantityChange: (String, Int) -> Unit,
    onRemove: (String) -> Unit,
) {
    Row(Modifier.fillMaxWidth().padding(horizontal = PoroSpacing.L), horizontalArrangement = Arrangement.spacedBy(PoroSpacing.M)) {
        AsyncImage(line.imageUrl, contentDescription = null, contentScale = ContentScale.Crop, modifier = Modifier.size(72.dp))
        Column(Modifier.weight(1f), verticalArrangement = Arrangement.spacedBy(PoroSpacing.XS)) {
            val title = line.title ?: stringResource(R.string.cart_item_unknown)
            line.productId?.let { id -> TextButton(onClick = { onOpenProduct(id) }) { Text(title) } } ?: Text(title)
            line.variantTitle?.let { Text(it, style = MaterialTheme.typography.bodySmall) }
            if (currency != null && line.lineTotal != null) Text(Formats.money(line.lineTotal, currency), style = MaterialTheme.typography.titleSmall)
            if (line.available && !line.inStock) Text(stringResource(R.string.product_out_of_stock), color = MaterialTheme.colorScheme.error)
            Row(verticalAlignment = Alignment.CenterVertically) {
                if (line.available) QuantityStepper(line.quantity, { onQuantityChange(line.variantId, it) }, enabled = !busy)
                TextButton(onClick = { onRemove(line.variantId) }, enabled = !busy) { Text(stringResource(R.string.cart_remove)) }
            }
        }
    }
}
