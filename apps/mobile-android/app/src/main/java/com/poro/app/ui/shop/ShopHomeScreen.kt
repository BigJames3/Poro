package com.poro.app.ui.shop

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.BoxWithConstraints
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.text.KeyboardActions
import androidx.compose.foundation.text.KeyboardOptions
import androidx.compose.foundation.verticalScroll
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.ShoppingCart
import androidx.compose.material3.Badge
import androidx.compose.material3.BadgedBox
import androidx.compose.material3.Card
import androidx.compose.material3.CardDefaults
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedButton
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.semantics.heading
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.text.input.ImeAction
import androidx.compose.ui.text.input.KeyboardType
import androidx.compose.ui.unit.dp
import com.poro.app.R
import com.poro.app.ui.auth.ErrorMessageText
import com.poro.app.ui.auth.SubmitButton
import com.poro.app.ui.theme.PoroSize
import com.poro.app.ui.theme.PoroSpace

@Composable
fun ShopHomeScreen(
    state: ShopHomeUiState,
    onHandleChange: (String) -> Unit,
    onFind: () -> Unit,
    onCart: () -> Unit,
    onOrders: () -> Unit,
    onMyShop: () -> Unit,
) {
    Column(
        Modifier.fillMaxSize().verticalScroll(rememberScrollState()).padding(PoroSpace.lg),
        verticalArrangement = Arrangement.spacedBy(PoroSpace.md),
    ) {
        Row(Modifier.fillMaxWidth(), verticalAlignment = Alignment.CenterVertically) {
            Text(
                stringResource(R.string.shop_home_title),
                style = MaterialTheme.typography.headlineMedium,
                modifier = Modifier.weight(1f).semantics { heading() },
            )
            IconButton(onClick = onCart) {
                BadgedBox(badge = { if (state.cartCount > 0) Badge { Text(state.cartCount.toString()) } }) {
                    Icon(Icons.Filled.ShoppingCart, contentDescription = stringResource(R.string.cart_title_count, state.cartCount))
                }
            }
        }
        Card(
            shape = RoundedCornerShape(PoroSize.radiusCard),
            colors = CardDefaults.cardColors(containerColor = MaterialTheme.colorScheme.surface),
            elevation = CardDefaults.cardElevation(defaultElevation = 1.dp),
        ) {
            Column(Modifier.padding(PoroSpace.md), verticalArrangement = Arrangement.spacedBy(PoroSpace.sm)) {
                Text(stringResource(R.string.shop_find_title), style = MaterialTheme.typography.titleMedium, modifier = Modifier.semantics { heading() })
                Text(stringResource(R.string.shop_find_hint), style = MaterialTheme.typography.bodyMedium)
                OutlinedTextField(
                    value = state.handle,
                    onValueChange = onHandleChange,
                    label = { Text(stringResource(R.string.shop_handle_label)) },
                    prefix = { Text("@") },
                    singleLine = true,
                    keyboardOptions = KeyboardOptions(keyboardType = KeyboardType.Ascii, imeAction = ImeAction.Search),
                    keyboardActions = KeyboardActions(onSearch = { onFind() }),
                    modifier = Modifier.fillMaxWidth(),
                )
                state.error?.let { ErrorMessageText(it) }
                SubmitButton(
                    text = stringResource(R.string.shop_find_action),
                    enabled = state.handle.removePrefix("@").length >= 3 && !state.finding,
                    loading = state.finding,
                    onClick = onFind,
                )
            }
        }
        BoxWithConstraints(Modifier.fillMaxWidth()) {
            val stacked = maxWidth < 360.dp
            val sellLabel = stringResource(if (state.hasShop == false) R.string.my_shop_open else R.string.my_shop_title)
            if (stacked) {
                Column(verticalArrangement = Arrangement.spacedBy(PoroSpace.sm)) {
                    HubTile(stringResource(R.string.orders_title), onOrders)
                    HubTile(sellLabel, onMyShop)
                }
            } else {
                Row(horizontalArrangement = Arrangement.spacedBy(PoroSpace.sm), modifier = Modifier.fillMaxWidth()) {
                    HubTile(stringResource(R.string.orders_title), onOrders, Modifier.weight(1f))
                    HubTile(sellLabel, onMyShop, Modifier.weight(1f))
                }
            }
        }
    }
}

@Composable
private fun HubTile(label: String, onClick: () -> Unit, modifier: Modifier = Modifier) {
    OutlinedButton(onClick = onClick, modifier = modifier.fillMaxWidth().heightIn(min = PoroSize.touch)) {
        Text(label)
    }
}
