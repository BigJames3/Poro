package com.poro.app.ui.shop

import androidx.activity.compose.BackHandler
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.FlowRow
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.text.KeyboardOptions
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.AssistChip
import androidx.compose.material3.Checkbox
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.HorizontalDivider
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedButton
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.Scaffold
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.semantics.heading
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.text.input.ImeAction
import androidx.compose.ui.text.input.KeyboardCapitalization
import androidx.compose.ui.text.input.KeyboardType
import androidx.compose.ui.unit.dp
import com.poro.app.R
import com.poro.app.core.format.Formats
import com.poro.app.data.order.AddressDto
import com.poro.app.data.order.PreviewDto
import com.poro.app.ui.auth.ErrorMessageText
import com.poro.app.ui.auth.SubmitButton
import com.poro.app.ui.common.BackTopBar

@Composable
fun CheckoutScreen(
    state: CheckoutUiState,
    onBack: () -> Unit,
    onFormChange: (DeliveryForm) -> Unit,
    onUseAddress: (AddressDto) -> Unit,
    onSaveAddressChange: (Boolean) -> Unit,
    onReview: () -> Unit,
    onEdit: () -> Unit,
    onConfirm: () -> Unit,
    onRetryLoad: () -> Unit,
    onManageAddresses: () -> Unit,
) {
    val preview = state.preview
    // Sur le récapitulatif, « retour » revient à la livraison plutôt que de quitter la commande.
    BackHandler(enabled = preview != null && !state.working, onBack = onEdit)
    val title = stringResource(if (preview == null) R.string.checkout_delivery_title else R.string.checkout_review_title)
    Scaffold(topBar = { BackTopBar(title, if (preview == null) onBack else onEdit) }) { padding ->
        if (state.loading) {
            Box(Modifier.fillMaxSize().padding(padding), contentAlignment = Alignment.Center) { CircularProgressIndicator() }
            return@Scaffold
        }
        Column(
            Modifier.fillMaxSize().padding(padding).verticalScroll(rememberScrollState()).padding(24.dp),
            verticalArrangement = Arrangement.spacedBy(12.dp),
        ) {
            if (preview == null) {
                DeliveryFields(state, onFormChange, onUseAddress, onSaveAddressChange, onManageAddresses)
                state.error?.let { ErrorMessageText(it) }
                if (state.error != null) {
                    TextButton(onClick = onRetryLoad, modifier = Modifier.fillMaxWidth()) {
                        Text(stringResource(R.string.action_retry))
                    }
                }
                SubmitButton(stringResource(R.string.checkout_continue), enabled = state.form.isValid, loading = state.working, onClick = onReview)
            } else {
                Review(preview)
                state.error?.let { ErrorMessageText(it) }
                SubmitButton(stringResource(R.string.checkout_confirm), enabled = true, loading = state.working, onClick = onConfirm)
                OutlinedButton(onClick = onEdit, enabled = !state.working, modifier = Modifier.fillMaxWidth()) {
                    Text(stringResource(R.string.checkout_edit_delivery))
                }
            }
        }
    }
}

@Composable
private fun DeliveryFields(
    state: CheckoutUiState,
    onFormChange: (DeliveryForm) -> Unit,
    onUseAddress: (AddressDto) -> Unit,
    onSaveAddressChange: (Boolean) -> Unit,
    onManageAddresses: () -> Unit,
) {
    val form = state.form
    if (state.addresses.isNotEmpty()) {
        Text(stringResource(R.string.checkout_saved_addresses), style = MaterialTheme.typography.titleSmall)
        FlowRow(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
            state.addresses.forEach { address ->
                AssistChip(onClick = { onUseAddress(address) }, label = { Text(address.label ?: "${address.fullName}, ${address.city}") })
            }
        }
    }
    TextButton(onClick = onManageAddresses, enabled = !state.working) { Text(stringResource(R.string.checkout_manage_addresses)) }
    DeliveryField(form.fullName, R.string.delivery_full_name, KeyboardType.Text, KeyboardCapitalization.Words) { onFormChange(form.copy(fullName = it.take(80))) }
    DeliveryField(form.phone, R.string.delivery_phone, KeyboardType.Phone, supporting = R.string.delivery_phone_hint) {
        onFormChange(form.copy(phone = it.filter { c -> c.isDigit() || c == '+' }.take(16)))
    }
    DeliveryField(form.city, R.string.delivery_city, KeyboardType.Text, KeyboardCapitalization.Words) { onFormChange(form.copy(city = it.take(80))) }
    DeliveryField(
        form.address, R.string.delivery_address, KeyboardType.Text, KeyboardCapitalization.Sentences, singleLine = false,
        supporting = when {
            form.addressTooShort -> R.string.delivery_too_short
            !form.addressFits -> R.string.delivery_text_limit
            else -> null
        },
        isError = form.addressTooShort || !form.addressFits,
    ) {
        onFormChange(form.copy(address = it.take(300)))
    }
    DeliveryField(
        form.landmark, R.string.delivery_landmark, KeyboardType.Text, KeyboardCapitalization.Sentences,
        supporting = when {
            form.landmarkTooShort -> R.string.delivery_too_short
            !form.landmarkFits -> R.string.delivery_text_limit
            else -> R.string.optional
        },
        isError = form.landmarkTooShort || !form.landmarkFits,
    ) {
        onFormChange(form.copy(landmark = it.take(300)))
    }
    Row(verticalAlignment = Alignment.CenterVertically) {
        Checkbox(checked = state.saveAddress, onCheckedChange = onSaveAddressChange)
        Text(stringResource(R.string.checkout_save_address))
    }
}

@Composable
internal fun DeliveryField(
    value: String,
    label: Int,
    keyboardType: KeyboardType,
    capitalization: KeyboardCapitalization = KeyboardCapitalization.None,
    supporting: Int? = null,
    singleLine: Boolean = true,
    isError: Boolean = false,
    onChange: (String) -> Unit,
) {
    OutlinedTextField(
        value = value,
        onValueChange = onChange,
        label = { Text(stringResource(label)) },
        supportingText = supporting?.let { { Text(stringResource(it)) } },
        singleLine = singleLine,
        isError = isError,
        keyboardOptions = KeyboardOptions(capitalization = capitalization, keyboardType = keyboardType, imeAction = ImeAction.Next),
        modifier = Modifier.fillMaxWidth(),
    )
}

/** Récapitulatif à relire : articles par boutique, totaux par devise, livraison et paiement. */
@Composable
private fun Review(preview: PreviewDto) {
    preview.shops.forEach { group ->
        Text(group.shop.name, style = MaterialTheme.typography.titleMedium, modifier = Modifier.semantics { heading() })
        group.items.forEach { item ->
            Row(Modifier.fillMaxWidth(), horizontalArrangement = Arrangement.SpaceBetween) {
                Text(stringResource(R.string.checkout_line, item.quantity, item.title, item.variantTitle), Modifier.weight(1f))
                Text(Formats.money(item.lineTotal, group.currency))
            }
        }
        Text(stringResource(R.string.cart_subtotal, Formats.money(group.subtotal, group.currency)), style = MaterialTheme.typography.titleSmall)
        HorizontalDivider()
    }
    preview.totals.forEach { total ->
        Text(stringResource(R.string.checkout_total, Formats.money(total.amount, total.currency)), style = MaterialTheme.typography.titleLarge)
    }
    if (preview.deliveryFeeNotice.isNotBlank()) Text(preview.deliveryFeeNotice, style = MaterialTheme.typography.bodySmall)
    Text(stringResource(R.string.checkout_delivery_to), style = MaterialTheme.typography.titleSmall)
    with(preview.delivery) {
        Text(listOfNotNull(fullName, phone, city, address, landmark).filter { it.isNotBlank() }.joinToString("\n"))
    }
    Text(stringResource(R.string.checkout_payment), style = MaterialTheme.typography.titleSmall)
    Text(preview.paymentMethodLabel.ifBlank { stringResource(R.string.payment_cash_on_delivery) })
    Text(stringResource(R.string.cart_payment_notice), style = MaterialTheme.typography.bodySmall)
}
