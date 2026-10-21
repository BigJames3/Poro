package com.poro.app.ui.shop

import androidx.annotation.StringRes
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.AlertDialog
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.HorizontalDivider
import androidx.compose.material3.ListItem
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedButton
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.Scaffold
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.saveable.rememberSaveable
import androidx.compose.runtime.setValue
import androidx.compose.foundation.clickable
import androidx.compose.ui.Modifier
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.semantics.Role
import androidx.compose.ui.unit.dp
import com.poro.app.R
import com.poro.app.core.format.Formats
import com.poro.app.data.order.OrderDto
import com.poro.app.ui.auth.ErrorMessageText
import com.poro.app.ui.auth.SubmitButton
import com.poro.app.ui.common.BackTopBar
import com.poro.app.ui.common.LoadableContent

@StringRes
fun orderStatusLabel(status: String): Int = when (status) {
    OrderDto.PENDING -> R.string.order_status_pending
    OrderDto.AWAITING_PAYMENT -> R.string.order_status_awaiting_payment
    OrderDto.CONFIRMED -> R.string.order_status_confirmed
    OrderDto.PAID -> R.string.order_status_paid
    OrderDto.SHIPPED -> R.string.order_status_shipped
    OrderDto.COMPLETED -> R.string.order_status_completed
    OrderDto.CANCELLED -> R.string.order_status_cancelled
    else -> R.string.order_status_unknown
}

/** Montant à afficher : le total définitif, sinon le sous-total vu au moment de commander. */
fun OrderDto.displayTotal(): String = Formats.money(total ?: subtotalSeen, currency)

@Composable
fun OrdersScreen(asSeller: Boolean, state: OrdersUiState, onBack: () -> Unit, onLoadMore: () -> Unit, onOpen: (OrderDto) -> Unit) {
    val title = stringResource(if (asSeller) R.string.seller_orders_title else R.string.orders_title)
    Scaffold(topBar = { BackTopBar(title, onBack) }) { padding ->
        LazyColumn(Modifier.fillMaxSize().padding(padding)) {
            items(state.orders, key = { it.orderId }) { order ->
                ListItem(
                    headlineContent = { Text(order.shop.name.ifBlank { stringResource(R.string.order_title) }) },
                    supportingContent = {
                        Text(
                            listOfNotNull(
                                stringResource(orderStatusLabel(order.status)),
                                (order.placedAt ?: order.createdAt)?.let { Formats.date(it) },
                                order.displayTotal(),
                            ).joinToString(" · "),
                        )
                    },
                    modifier = Modifier.clickable(role = Role.Button) { onOpen(order) },
                )
                HorizontalDivider()
            }
            if (state.isEmpty) item { Text(stringResource(if (asSeller) R.string.seller_orders_empty else R.string.orders_empty), Modifier.padding(24.dp)) }
            state.error?.let { item { ErrorMessageText(it, Modifier.padding(24.dp)) } }
            if (state.loading) item { CircularProgressIndicator(Modifier.padding(24.dp)) }
            if (state.canLoadMore) item { LaunchedEffect(state.orders.size) { onLoadMore() } }
        }
    }
}

@Composable
fun OrderScreen(
    asSeller: Boolean,
    state: OrderUiState,
    onBack: () -> Unit,
    onRetry: () -> Unit,
    onCancel: () -> Unit,
    onConfirmReceipt: () -> Unit,
    onTrackingChange: (String) -> Unit,
    onShip: () -> Unit,
) {
    var confirmCancel by rememberSaveable { mutableStateOf(false) }
    Scaffold(topBar = { BackTopBar(stringResource(R.string.order_title), onBack) }) { padding ->
        LoadableContent(state.order, onRetry, Modifier.padding(padding)) { order ->
            Column(
                Modifier.fillMaxSize().padding(padding).verticalScroll(rememberScrollState()).padding(24.dp),
                verticalArrangement = Arrangement.spacedBy(12.dp),
            ) {
                Text(order.shop.name, style = MaterialTheme.typography.titleLarge)
                Text(stringResource(orderStatusLabel(order.status)), style = MaterialTheme.typography.titleMedium)
                order.cancelReason?.let { Text(stringResource(R.string.order_cancel_reason, it)) }
                order.items.forEach { item ->
                    Row(Modifier.fillMaxWidth(), horizontalArrangement = Arrangement.SpaceBetween) {
                        Text(stringResource(R.string.order_line, item.quantity, item.title), Modifier.weight(1f))
                        Text(Formats.money((if (item.unitPrice > 0) item.unitPrice else item.unitPriceSeen) * item.quantity, order.currency))
                    }
                }
                HorizontalDivider()
                Text(stringResource(R.string.checkout_total, order.displayTotal()), style = MaterialTheme.typography.titleMedium)
                if (order.paymentMethodLabel.isNotBlank()) Text(order.paymentMethodLabel)
                order.delivery?.let { d ->
                    Text(stringResource(R.string.checkout_delivery_to), style = MaterialTheme.typography.titleSmall)
                    Text(listOfNotNull(d.fullName, d.phone, d.city, d.address, d.landmark).filter { it.isNotBlank() }.joinToString("\n"))
                }
                order.tracking?.let { Text(stringResource(R.string.order_tracking, it)) }
                state.error?.let { ErrorMessageText(it) }

                if (asSeller && order.status in setOf(OrderDto.CONFIRMED, OrderDto.PAID)) {
                    OutlinedTextField(
                        value = state.tracking,
                        onValueChange = onTrackingChange,
                        label = { Text(stringResource(R.string.order_tracking_label)) },
                        supportingText = { Text(stringResource(R.string.optional)) },
                        singleLine = true,
                        modifier = Modifier.fillMaxWidth(),
                    )
                    SubmitButton(stringResource(R.string.order_ship), enabled = true, loading = state.working, onClick = onShip)
                }
                if (!asSeller && order.status == OrderDto.SHIPPED) {
                    SubmitButton(stringResource(R.string.order_confirm_receipt), enabled = true, loading = state.working, onClick = onConfirmReceipt)
                }
                if (order.cancellable) {
                    OutlinedButton(onClick = { confirmCancel = true }, enabled = !state.working, modifier = Modifier.fillMaxWidth()) {
                        Text(stringResource(R.string.order_cancel))
                    }
                }
            }
        }
    }
    if (confirmCancel) {
        AlertDialog(
            onDismissRequest = { confirmCancel = false },
            title = { Text(stringResource(R.string.order_cancel_confirm_title)) },
            text = { Text(stringResource(R.string.order_cancel_confirm_text)) },
            confirmButton = {
                TextButton(onClick = {
                    confirmCancel = false
                    onCancel()
                }) { Text(stringResource(R.string.order_cancel)) }
            },
            dismissButton = { TextButton(onClick = { confirmCancel = false }) { Text(stringResource(R.string.action_keep)) } },
        )
    }
}
