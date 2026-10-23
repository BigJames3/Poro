package com.poro.app.ui.shop

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.imePadding
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.text.KeyboardOptions
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.AlertDialog
import androidx.compose.material3.ListItem
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedButton
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.Scaffold
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.saveable.rememberSaveable
import androidx.compose.runtime.setValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.text.input.KeyboardCapitalization
import androidx.compose.ui.text.input.KeyboardType
import androidx.compose.ui.unit.dp
import com.poro.app.R
import com.poro.app.data.order.AddressDto
import com.poro.app.ui.auth.ErrorMessageText
import com.poro.app.ui.auth.SubmitButton
import com.poro.app.ui.common.BackTopBar
import com.poro.app.ui.common.LoadableContent

@Composable
fun AddressesScreen(
    state: AddressesUiState,
    onBack: () -> Unit,
    onRetry: () -> Unit,
    onAdd: () -> Unit,
    onEdit: (AddressDto) -> Unit,
    onFormChange: (DeliveryForm) -> Unit,
    onLabelChange: (String) -> Unit,
    onSave: () -> Unit,
    onCancel: () -> Unit,
    onSetDefault: (String) -> Unit,
    onDelete: (String) -> Unit,
) {
    // Supprimer est définitif : on demande confirmation.
    var pendingDelete by rememberSaveable { mutableStateOf<String?>(null) }
    pendingDelete?.let { id ->
        AlertDialog(
            onDismissRequest = { pendingDelete = null },
            title = { Text(stringResource(R.string.addresses_delete_confirm)) },
            confirmButton = {
                TextButton(onClick = {
                    pendingDelete = null
                    onDelete(id)
                }) { Text(stringResource(R.string.action_delete)) }
            },
            dismissButton = { TextButton(onClick = { pendingDelete = null }) { Text(stringResource(R.string.action_cancel)) } },
        )
    }
    Scaffold(topBar = { BackTopBar(stringResource(R.string.addresses_title), onBack) }) { padding ->
        if (state.showForm) {
            AddressForm(state, onFormChange, onLabelChange, onSave, onCancel, Modifier.padding(padding))
            return@Scaffold
        }
        LoadableContent(state.list, onRetry, Modifier.padding(padding)) { addresses ->
            LazyColumn(Modifier.fillMaxSize(), verticalArrangement = Arrangement.spacedBy(4.dp)) {
                if (addresses.isEmpty()) {
                    item { Text(stringResource(R.string.addresses_empty), Modifier.padding(24.dp)) }
                }
                items(addresses, key = { it.addressId }) { address ->
                    AddressRow(
                        address,
                        enabled = !state.working,
                        onEdit = { onEdit(address) },
                        onSetDefault = { onSetDefault(address.addressId) },
                        onDelete = { pendingDelete = address.addressId },
                    )
                }
                item {
                    Column(Modifier.padding(16.dp), verticalArrangement = Arrangement.spacedBy(8.dp)) {
                        state.error?.let { ErrorMessageText(it) }
                        OutlinedButton(onClick = onAdd, enabled = !state.working, modifier = Modifier.fillMaxWidth()) {
                            Text(stringResource(R.string.addresses_add))
                        }
                    }
                }
            }
        }
    }
}

@Composable
private fun AddressRow(
    address: AddressDto,
    enabled: Boolean,
    onEdit: () -> Unit,
    onSetDefault: () -> Unit,
    onDelete: () -> Unit,
) {
    ListItem(
        headlineContent = { Text(address.label?.takeIf { it.isNotBlank() } ?: address.fullName) },
        supportingContent = {
            Text(listOfNotNull(address.fullName, address.phone, address.city, address.address).joinToString(" · "))
        },
        trailingContent = {
            Column {
                if (address.isDefault) {
                    Text(stringResource(R.string.addresses_default), style = MaterialTheme.typography.labelSmall)
                } else {
                    TextButton(onClick = onSetDefault, enabled = enabled) { Text(stringResource(R.string.addresses_set_default)) }
                }
                TextButton(onClick = onEdit, enabled = enabled) { Text(stringResource(R.string.action_edit)) }
                TextButton(onClick = onDelete, enabled = enabled) { Text(stringResource(R.string.action_delete)) }
            }
        },
    )
}

@Composable
private fun AddressForm(
    state: AddressesUiState,
    onFormChange: (DeliveryForm) -> Unit,
    onLabelChange: (String) -> Unit,
    onSave: () -> Unit,
    onCancel: () -> Unit,
    modifier: Modifier = Modifier,
) {
    val form = state.form
    // Défile au-dessus du clavier : le bouton Enregistrer reste atteignable sur petit écran.
    Column(
        modifier.fillMaxSize().imePadding().verticalScroll(rememberScrollState()).padding(24.dp),
        verticalArrangement = Arrangement.spacedBy(12.dp),
    ) {
        Text(
            stringResource(if (state.editingId == null) R.string.addresses_add else R.string.addresses_edit),
            style = MaterialTheme.typography.titleMedium,
        )
        OutlinedTextField(
            value = state.label,
            onValueChange = onLabelChange,
            label = { Text(stringResource(R.string.addresses_label)) },
            supportingText = { Text(stringResource(R.string.optional)) },
            singleLine = true,
            keyboardOptions = KeyboardOptions(capitalization = KeyboardCapitalization.Words),
            modifier = Modifier.fillMaxWidth(),
        )
        DeliveryField(form.fullName, R.string.delivery_full_name, KeyboardType.Text, KeyboardCapitalization.Words) {
            onFormChange(form.copy(fullName = it.take(80)))
        }
        DeliveryField(form.phone, R.string.delivery_phone, KeyboardType.Phone, supporting = R.string.delivery_phone_hint) {
            onFormChange(form.copy(phone = it.filter { c -> c.isDigit() || c == '+' }.take(16)))
        }
        DeliveryField(form.city, R.string.delivery_city, KeyboardType.Text, KeyboardCapitalization.Words) {
            onFormChange(form.copy(city = it.take(80)))
        }
        DeliveryField(form.address, R.string.delivery_address, KeyboardType.Text, KeyboardCapitalization.Sentences, singleLine = false) {
            onFormChange(form.copy(address = it.take(300)))
        }
        DeliveryField(form.landmark, R.string.delivery_landmark, KeyboardType.Text, KeyboardCapitalization.Sentences, supporting = R.string.optional) {
            onFormChange(form.copy(landmark = it.take(200)))
        }
        state.error?.let { ErrorMessageText(it) }
        SubmitButton(
            stringResource(R.string.action_save),
            enabled = state.canSave,
            loading = state.working,
            onClick = onSave,
        )
        OutlinedButton(onClick = onCancel, enabled = !state.working, modifier = Modifier.fillMaxWidth()) {
            Text(stringResource(R.string.action_cancel))
        }
    }
}
