package com.poro.app.ui.shop

import androidx.activity.compose.rememberLauncherForActivityResult
import androidx.activity.result.PickVisualMediaRequest
import androidx.activity.result.contract.ActivityResultContracts
import androidx.compose.foundation.horizontalScroll
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.text.KeyboardOptions
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.AlertDialog
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.HorizontalDivider
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
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.layout.ContentScale
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.semantics.heading
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.text.input.KeyboardType
import androidx.compose.ui.unit.dp
import coil3.compose.AsyncImage
import com.poro.app.R
import com.poro.app.data.shop.ProductDto
import com.poro.app.ui.auth.ErrorMessageText
import com.poro.app.ui.auth.SubmitButton
import com.poro.app.ui.common.BackTopBar

class ProductEditorActions(
    val onTitleChange: (String) -> Unit,
    val onDescriptionChange: (String) -> Unit,
    val onVariantChange: (Int, VariantDraft) -> Unit,
    val onAddVariant: () -> Unit,
    val onRemoveVariant: (Int) -> Unit,
    val onSave: () -> Unit,
    val onSetStatus: (String) -> Unit,
    val onImagePicked: (String) -> Unit,
    val onRemoveImage: (String) -> Unit,
    val onDelete: () -> Unit,
)

@Composable
fun ProductEditorScreen(state: ProductEditorUiState, currency: String, onBack: () -> Unit, actions: ProductEditorActions) {
    var confirmDelete by rememberSaveable { mutableStateOf(false) }
    val picker = rememberLauncherForActivityResult(ActivityResultContracts.PickVisualMedia()) { uri ->
        uri?.let { actions.onImagePicked(it.toString()) }
    }
    val title = stringResource(if (state.productId == null) R.string.product_new else R.string.product_edit_title)
    Scaffold(topBar = { BackTopBar(title, onBack) }) { padding ->
        if (state.loading) {
            Box(Modifier.fillMaxSize().padding(padding), contentAlignment = Alignment.Center) { CircularProgressIndicator() }
            return@Scaffold
        }
        Column(
            Modifier.fillMaxSize().padding(padding).verticalScroll(rememberScrollState()).padding(24.dp),
            verticalArrangement = Arrangement.spacedBy(12.dp),
        ) {
            state.status?.let { Text(stringResource(productStatusLabel(it)), style = MaterialTheme.typography.titleSmall) }
            OutlinedTextField(
                value = state.title,
                onValueChange = actions.onTitleChange,
                label = { Text(stringResource(R.string.product_title_label)) },
                singleLine = true,
                modifier = Modifier.fillMaxWidth(),
            )
            OutlinedTextField(
                value = state.description,
                onValueChange = actions.onDescriptionChange,
                label = { Text(stringResource(R.string.product_description_label)) },
                supportingText = { Text(stringResource(R.string.optional)) },
                modifier = Modifier.fillMaxWidth(),
            )
            Text(stringResource(R.string.product_variants), style = MaterialTheme.typography.titleMedium, modifier = Modifier.semantics { heading() })
            state.variants.forEachIndexed { index, variant ->
                VariantFields(index, variant, currency, canRemove = state.variants.size > 1, actions)
                HorizontalDivider()
            }
            TextButton(onClick = actions.onAddVariant, enabled = state.variants.size < 50) { Text(stringResource(R.string.product_add_variant)) }
            state.error?.let { ErrorMessageText(it) }
            SubmitButton(stringResource(R.string.action_save), enabled = state.canSave(currency), loading = state.working, onClick = actions.onSave)

            // Photos, publication et suppression : une fois le produit créé.
            if (state.productId == null) return@Column
            Text(stringResource(R.string.product_photos), style = MaterialTheme.typography.titleMedium, modifier = Modifier.semantics { heading() })
            Row(Modifier.horizontalScroll(rememberScrollState()), horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                state.images.forEachIndexed { i, image ->
                    Column(horizontalAlignment = Alignment.CenterHorizontally) {
                        AsyncImage(
                            image.url,
                            contentDescription = stringResource(R.string.product_photo, i + 1, state.images.size),
                            contentScale = ContentScale.Crop,
                            modifier = Modifier.size(96.dp),
                        )
                        TextButton(onClick = { actions.onRemoveImage(image.imageId) }, enabled = !state.working) { Text(stringResource(R.string.action_delete)) }
                    }
                }
            }
            OutlinedButton(
                onClick = { picker.launch(PickVisualMediaRequest(ActivityResultContracts.PickVisualMedia.ImageOnly)) },
                enabled = !state.working && state.images.size < 8,
                modifier = Modifier.fillMaxWidth(),
            ) { Text(stringResource(R.string.product_add_photo)) }
            when (state.status) {
                ProductDto.ACTIVE -> OutlinedButton(onClick = { actions.onSetStatus(ProductDto.ARCHIVED) }, enabled = !state.working, modifier = Modifier.fillMaxWidth()) {
                    Text(stringResource(R.string.product_archive))
                }
                else -> SubmitButton(stringResource(R.string.product_publish), enabled = !state.working, loading = false, onClick = { actions.onSetStatus(ProductDto.ACTIVE) })
            }
            TextButton(onClick = { confirmDelete = true }, enabled = !state.working) {
                Text(stringResource(R.string.product_delete), color = MaterialTheme.colorScheme.error)
            }
        }
    }
    if (confirmDelete) {
        AlertDialog(
            onDismissRequest = { confirmDelete = false },
            title = { Text(stringResource(R.string.product_delete_confirm)) },
            confirmButton = {
                TextButton(onClick = {
                    confirmDelete = false
                    actions.onDelete()
                }) { Text(stringResource(R.string.action_delete)) }
            },
            dismissButton = { TextButton(onClick = { confirmDelete = false }) { Text(stringResource(R.string.action_cancel)) } },
        )
    }
}

@Composable
private fun VariantFields(index: Int, variant: VariantDraft, currency: String, canRemove: Boolean, actions: ProductEditorActions) {
    Column(verticalArrangement = Arrangement.spacedBy(8.dp)) {
        OutlinedTextField(
            value = variant.title,
            onValueChange = { actions.onVariantChange(index, variant.copy(title = it.take(60))) },
            label = { Text(stringResource(R.string.variant_title_label)) },
            supportingText = { Text(stringResource(R.string.variant_title_hint)) },
            singleLine = true,
            modifier = Modifier.fillMaxWidth(),
        )
        Row(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
            OutlinedTextField(
                value = variant.price,
                onValueChange = { actions.onVariantChange(index, variant.copy(price = it.take(15))) },
                label = { Text(stringResource(R.string.variant_price_label, currency)) },
                singleLine = true,
                isError = variant.price.isNotBlank() && variant.priceIn(currency) == null,
                keyboardOptions = KeyboardOptions(keyboardType = KeyboardType.Decimal),
                modifier = Modifier.weight(1f),
            )
            OutlinedTextField(
                value = variant.stock,
                onValueChange = { actions.onVariantChange(index, variant.copy(stock = it.filter(Char::isDigit).take(7))) },
                label = { Text(stringResource(R.string.variant_stock_label)) },
                supportingText = if (variant.reserved > 0) {
                    { Text(stringResource(R.string.variant_reserved, variant.reserved)) }
                } else {
                    null
                },
                singleLine = true,
                keyboardOptions = KeyboardOptions(keyboardType = KeyboardType.Number),
                modifier = Modifier.weight(1f),
            )
        }
        if (canRemove) TextButton(onClick = { actions.onRemoveVariant(index) }) { Text(stringResource(R.string.variant_remove)) }
    }
}
