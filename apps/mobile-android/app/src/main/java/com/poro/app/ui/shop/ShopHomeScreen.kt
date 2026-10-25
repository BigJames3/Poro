package com.poro.app.ui.shop

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.text.KeyboardActions
import androidx.compose.foundation.text.KeyboardOptions
import androidx.compose.foundation.verticalScroll
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.Search
import androidx.compose.material.icons.filled.ShoppingCart
import androidx.compose.material3.Badge
import androidx.compose.material3.BadgedBox
import androidx.compose.material3.Icon
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedButton
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
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
import com.poro.app.ui.theme.PoroSpacing

@Composable
fun ShopHomeScreen(
    state: ShopHomeUiState,
    onHandleChange: (String) -> Unit,
    onFind: () -> Unit,
    onCart: () -> Unit,
    onOrders: () -> Unit,
    onMyShop: () -> Unit,
    onSearchCatalog: () -> Unit,
) {
    Column(
        Modifier.fillMaxSize().verticalScroll(rememberScrollState()).padding(PoroSpacing.XL),
        verticalArrangement = Arrangement.spacedBy(PoroSpacing.L),
    ) {
        Text(stringResource(R.string.shop_home_title), style = MaterialTheme.typography.headlineMedium, modifier = Modifier.semantics { heading() })
        OutlinedButton(onClick = onCart, modifier = Modifier.fillMaxWidth().heightIn(min = 48.dp)) {
            BadgedBox(badge = { if (state.cartCount > 0) Badge { Text(state.cartCount.toString()) } }) {
                Icon(Icons.Filled.ShoppingCart, contentDescription = null)
            }
            Text(stringResource(R.string.cart_title_count, state.cartCount), modifier = Modifier.padding(start = PoroSpacing.M))
        }
        OutlinedButton(onClick = onOrders, modifier = Modifier.fillMaxWidth().heightIn(min = 48.dp)) {
            Text(stringResource(R.string.orders_title))
        }
        OutlinedButton(onClick = onSearchCatalog, modifier = Modifier.fillMaxWidth().heightIn(min = 48.dp)) {
            Icon(Icons.Filled.Search, contentDescription = null)
            Text(stringResource(R.string.shop_search_catalog), modifier = Modifier.padding(start = PoroSpacing.M))
        }
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
        Text(stringResource(R.string.shop_sell_title), style = MaterialTheme.typography.titleMedium, modifier = Modifier.semantics { heading() })
        OutlinedButton(onClick = onMyShop, modifier = Modifier.fillMaxWidth().heightIn(min = 48.dp)) {
            Text(stringResource(if (state.hasShop == false) R.string.my_shop_open else R.string.my_shop_title))
        }
    }
}
