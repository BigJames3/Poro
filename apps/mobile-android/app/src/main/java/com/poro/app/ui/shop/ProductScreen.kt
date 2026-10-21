package com.poro.app.ui.shop

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.FlowRow
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.aspectRatio
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.pager.HorizontalPager
import androidx.compose.foundation.pager.rememberPagerState
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.FilterChip
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Scaffold
import androidx.compose.material3.OutlinedButton
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.layout.ContentScale
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.unit.dp
import coil3.compose.AsyncImage
import com.poro.app.R
import com.poro.app.ui.auth.ErrorMessageText
import com.poro.app.ui.auth.SubmitButton
import com.poro.app.ui.common.BackTopBar
import com.poro.app.ui.common.LoadableContent
import com.poro.app.ui.common.PriceText
import com.poro.app.ui.theme.PoroSize
import com.poro.app.ui.theme.PoroSpace

@Composable
fun ProductScreen(
    state: ProductUiState,
    onBack: () -> Unit,
    onRetry: () -> Unit,
    onOpenShop: (String) -> Unit,
    onSelectVariant: (String) -> Unit,
    onQuantityChange: (Int) -> Unit,
    onAddToCart: () -> Unit,
) {
    Scaffold(topBar = { BackTopBar(state.product.valueOrNull?.title.orEmpty(), onBack) }) { padding ->
        LoadableContent(state.product, onRetry, Modifier.padding(padding)) { product ->
            val selected = product.variants.firstOrNull { it.variantId == state.variantId }
            Column(
                Modifier.fillMaxSize().verticalScroll(rememberScrollState()),
                verticalArrangement = Arrangement.spacedBy(PoroSpace.sm),
            ) {
                if (product.images.isNotEmpty()) {
                    val pager = rememberPagerState(pageCount = { product.images.size })
                    HorizontalPager(pager, modifier = Modifier.fillMaxWidth().aspectRatio(1f)) { page ->
                        AsyncImage(
                            product.images[page].url,
                            contentDescription = stringResource(R.string.product_photo, page + 1, product.images.size),
                            contentScale = ContentScale.Crop,
                            modifier = Modifier.fillMaxSize(),
                        )
                    }
                }
                Column(Modifier.padding(horizontal = PoroSpace.md), verticalArrangement = Arrangement.spacedBy(PoroSpace.sm)) {
                    selected?.let { PriceText(it.price, product.currency, style = MaterialTheme.typography.titleLarge) }
                    Text(product.title, style = MaterialTheme.typography.headlineSmall)
                    TextButton(onClick = { onOpenShop(product.shop.handle) }) {
                        Text(stringResource(R.string.product_sold_by, product.shop.name))
                    }
                    if (product.variants.size > 1) {
                        FlowRow(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                            product.variants.forEach { variant ->
                                FilterChip(
                                    selected = variant.variantId == state.variantId,
                                    onClick = { onSelectVariant(variant.variantId) },
                                    label = { Text(variant.title) },
                                    enabled = variant.inStock,
                                )
                            }
                        }
                    }
                    if (selected != null && !selected.inStock) Text(stringResource(R.string.product_out_of_stock), color = MaterialTheme.colorScheme.error)
                    QuantityStepper(state.quantity, onQuantityChange)
                    product.description?.let { Text(it) }
                    state.error?.let { ErrorMessageText(it) }
                    SubmitButton(
                        text = stringResource(R.string.product_add_to_cart),
                        enabled = state.canAdd && selected?.inStock != false,
                        loading = state.adding,
                        onClick = onAddToCart,
                    )
                }
            }
        }
    }
}

/** − quantité + , de 1 à 99 ; TalkBack lit « Quantité : 2 ». */
@Composable
fun QuantityStepper(quantity: Int, onChange: (Int) -> Unit, modifier: Modifier = Modifier, enabled: Boolean = true) {
    Row(modifier, verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(4.dp)) {
        OutlinedButton(onClick = { onChange(quantity - 1) }, enabled = enabled && quantity > 1, modifier = Modifier.heightIn(min = PoroSize.touch)) {
            Text("−", modifier = Modifier.padding(horizontal = 4.dp))
        }
        Text(stringResource(R.string.quantity, quantity), modifier = Modifier.padding(horizontal = 8.dp))
        OutlinedButton(onClick = { onChange(quantity + 1) }, enabled = enabled && quantity < 99, modifier = Modifier.heightIn(min = PoroSize.touch)) {
            Text("+", modifier = Modifier.padding(horizontal = 4.dp))
        }
    }
}
