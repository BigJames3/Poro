package com.poro.app.ui.shop

import androidx.activity.compose.rememberLauncherForActivityResult
import androidx.activity.result.PickVisualMediaRequest
import androidx.activity.result.contract.ActivityResultContracts
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.FlowRow
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.text.KeyboardOptions
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.FilterChip
import androidx.compose.material3.HorizontalDivider
import androidx.compose.material3.ListItem
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedButton
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.Scaffold
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.layout.ContentScale
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.semantics.Role
import androidx.compose.ui.semantics.heading
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.text.input.KeyboardType
import androidx.compose.ui.unit.dp
import coil3.compose.AsyncImage
import com.poro.app.R
import com.poro.app.core.format.Formats
import com.poro.app.core.phone.LaunchCountry
import com.poro.app.data.shop.ProductDto
import com.poro.app.data.shop.ShopDto
import com.poro.app.ui.auth.ErrorMessageText
import com.poro.app.ui.auth.SubmitButton
import com.poro.app.ui.auth.phone.label
import com.poro.app.ui.common.Avatar
import com.poro.app.ui.common.BackTopBar
import com.poro.app.ui.common.EmptyState
import com.poro.app.ui.common.TextLimitHint
import com.poro.app.ui.theme.PoroSpacing

class MyShopActions(
    val onOpenFormChange: (OpenShopForm) -> Unit,
    val onOpenShop: () -> Unit,
    val onEditName: (String) -> Unit,
    val onEditDescription: (String) -> Unit,
    val onSaveDetails: () -> Unit,
    val onSetOpen: (Boolean) -> Unit,
    val onLogoPicked: (String) -> Unit,
    val onRemoveLogo: () -> Unit,
    val onLoadMoreProducts: () -> Unit,
    val onNewProduct: () -> Unit,
    val onEditProduct: (ProductDto) -> Unit,
    val onSellerOrders: () -> Unit,
)

@Composable
fun MyShopScreen(state: MyShopUiState, onBack: () -> Unit, actions: MyShopActions) {
    val shop = state.shop
    Scaffold(topBar = { BackTopBar(stringResource(if (shop == null) R.string.my_shop_open else R.string.my_shop_title), onBack) }) { padding ->
        if (state.loading) {
            Box(Modifier.fillMaxSize().padding(padding), contentAlignment = Alignment.Center) { CircularProgressIndicator() }
            return@Scaffold
        }
        Column(
            Modifier.fillMaxSize().padding(padding).verticalScroll(rememberScrollState()).padding(PoroSpacing.XL),
            verticalArrangement = Arrangement.spacedBy(PoroSpacing.M),
        ) {
            if (shop == null) OpenShopFields(state, actions) else Dashboard(shop, state, actions)
        }
    }
}

@Composable
private fun OpenShopFields(state: MyShopUiState, actions: MyShopActions) {
    val form = state.openForm
    Text(stringResource(R.string.my_shop_open_intro))
    OutlinedTextField(
        value = form.name,
        onValueChange = { actions.onOpenFormChange(form.copy(name = it.take(60))) },
        label = { Text(stringResource(R.string.shop_name_label)) },
        singleLine = true,
        modifier = Modifier.fillMaxWidth(),
    )
    OutlinedTextField(
        value = form.handle,
        onValueChange = { actions.onOpenFormChange(form.copy(handle = it.trim().take(31))) },
        label = { Text(stringResource(R.string.shop_handle_label)) },
        prefix = { Text("@") },
        supportingText = { Text(stringResource(R.string.shop_handle_hint)) },
        singleLine = true,
        keyboardOptions = KeyboardOptions(keyboardType = KeyboardType.Ascii),
        modifier = Modifier.fillMaxWidth(),
    )
    Text(stringResource(R.string.shop_country_label), style = MaterialTheme.typography.titleSmall)
    FlowRow(horizontalArrangement = Arrangement.spacedBy(PoroSpacing.S)) {
        LaunchCountry.entries.forEach { country ->
            FilterChip(
                selected = form.country == country,
                onClick = { actions.onOpenFormChange(form.copy(country = country)) },
                label = { Text("${country.flag} ${stringResource(country.label())}") },
            )
        }
    }
    Text(stringResource(R.string.shop_country_hint), style = MaterialTheme.typography.bodySmall)
    OutlinedTextField(
        value = form.description,
        onValueChange = { actions.onOpenFormChange(form.copy(description = it.take(2 * OpenShopForm.DESCRIPTION_MAX))) },
        label = { Text(stringResource(R.string.shop_description_label)) },
        supportingText = { TextLimitHint(form.descriptionFits, OpenShopForm.DESCRIPTION_MAX, OpenShopForm.DESCRIPTION_MAX_LINES) },
        isError = !form.descriptionFits,
        modifier = Modifier.fillMaxWidth(),
    )
    state.error?.let { ErrorMessageText(it) }
    SubmitButton(stringResource(R.string.my_shop_open_action), enabled = form.isValid, loading = state.working, onClick = actions.onOpenShop)
}

@Composable
private fun Dashboard(shop: ShopDto, state: MyShopUiState, actions: MyShopActions) {
    val picker = rememberLauncherForActivityResult(ActivityResultContracts.PickVisualMedia()) { uri ->
        uri?.let { actions.onLogoPicked(it.toString()) }
    }
    Row(verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(PoroSpacing.L)) {
        Avatar(shop.logoUrl, shop.name, size = 72.dp)
        Column {
            Text("@${shop.handle}", style = MaterialTheme.typography.titleMedium)
            Text(stringResource(shopStatusLabel(shop.status)), style = MaterialTheme.typography.bodySmall)
            TextButton(onClick = { picker.launch(PickVisualMediaRequest(ActivityResultContracts.PickVisualMedia.ImageOnly)) }, enabled = !state.working) {
                Text(stringResource(R.string.shop_change_logo))
            }
            if (shop.logoUrl != null) {
                TextButton(onClick = actions.onRemoveLogo, enabled = !state.working) {
                    Text(stringResource(R.string.shop_remove_logo))
                }
            }
        }
    }
    OutlinedButton(onClick = actions.onSellerOrders, modifier = Modifier.fillMaxWidth()) { Text(stringResource(R.string.seller_orders_title)) }
    OutlinedTextField(
        value = state.editName,
        onValueChange = actions.onEditName,
        label = { Text(stringResource(R.string.shop_name_label)) },
        singleLine = true,
        modifier = Modifier.fillMaxWidth(),
    )
    OutlinedTextField(
        value = state.editDescription,
        onValueChange = actions.onEditDescription,
        label = { Text(stringResource(R.string.shop_description_label)) },
        supportingText = { TextLimitHint(state.editDescriptionFits, OpenShopForm.DESCRIPTION_MAX, OpenShopForm.DESCRIPTION_MAX_LINES) },
        isError = !state.editDescriptionFits,
        modifier = Modifier.fillMaxWidth(),
    )
    state.error?.let { ErrorMessageText(it) }
    SubmitButton(stringResource(R.string.action_save), enabled = state.canSaveDetails, loading = state.working, onClick = actions.onSaveDetails)
    when (shop.status) {
        ShopDto.ACTIVE -> OutlinedButton(onClick = { actions.onSetOpen(false) }, enabled = !state.working, modifier = Modifier.fillMaxWidth()) {
            Text(stringResource(R.string.shop_close))
        }
        ShopDto.CLOSED -> OutlinedButton(onClick = { actions.onSetOpen(true) }, enabled = !state.working, modifier = Modifier.fillMaxWidth()) {
            Text(stringResource(R.string.shop_reopen))
        }
    }
    HorizontalDivider()
    Row(Modifier.fillMaxWidth(), verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.SpaceBetween) {
        Text(stringResource(R.string.my_products_title), style = MaterialTheme.typography.titleMedium, modifier = Modifier.semantics { heading() })
        TextButton(onClick = actions.onNewProduct) { Text(stringResource(R.string.product_new)) }
    }
    if (state.products.isEmpty()) EmptyState(stringResource(R.string.my_products_empty))
    state.products.forEach { product ->
        ListItem(
            leadingContent = {
                AsyncImage(product.images.firstOrNull()?.url, contentDescription = null, contentScale = ContentScale.Crop, modifier = Modifier.size(56.dp))
            },
            headlineContent = { Text(product.title) },
            supportingContent = {
                val price = product.variants.minOfOrNull { it.price }?.let { Formats.money(it, product.currency) }
                Text(listOfNotNull(stringResource(productStatusLabel(product.status)), price).joinToString(" · "))
            },
            modifier = Modifier.clickable(role = Role.Button) { actions.onEditProduct(product) },
        )
    }
    if (state.productsCursor != null) {
        TextButton(onClick = actions.onLoadMoreProducts) { Text(stringResource(R.string.action_load_more)) }
    }
}

fun shopStatusLabel(status: String?): Int = when (status) {
    ShopDto.SUSPENDED -> R.string.shop_status_suspended
    ShopDto.CLOSED -> R.string.shop_status_closed
    else -> R.string.shop_status_active
}

fun productStatusLabel(status: String?): Int = when (status) {
    ProductDto.ACTIVE -> R.string.product_status_active
    ProductDto.ARCHIVED -> R.string.product_status_archived
    else -> R.string.product_status_draft
}
