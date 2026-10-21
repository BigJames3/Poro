package com.poro.app.ui.shop

import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.aspectRatio
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.lazy.grid.GridCells
import androidx.compose.foundation.lazy.grid.GridItemSpan
import androidx.compose.foundation.lazy.grid.LazyVerticalGrid
import androidx.compose.foundation.lazy.grid.items
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Scaffold
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.ui.draw.clip
import androidx.compose.ui.layout.ContentScale
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.semantics.Role
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import coil3.compose.AsyncImage
import com.poro.app.R
import com.poro.app.data.shop.ProductDto
import com.poro.app.ui.auth.ErrorMessageText
import com.poro.app.ui.common.Avatar
import com.poro.app.ui.common.BackTopBar
import com.poro.app.ui.common.EmptyState
import com.poro.app.ui.common.LoadableContent
import com.poro.app.ui.common.PriceText
import com.poro.app.ui.theme.PoroSize
import com.poro.app.ui.theme.PoroSpace

@Composable
fun ShopScreen(state: ShopUiState, onBack: () -> Unit, onRetry: () -> Unit, onLoadMore: () -> Unit, onOpenProduct: (ProductDto) -> Unit) {
    val title = state.shop.valueOrNull?.name ?: stringResource(R.string.shop_title)
    Scaffold(topBar = { BackTopBar(title, onBack) }) { padding ->
        LoadableContent(state.shop, onRetry, Modifier.padding(padding)) { shop ->
            LazyVerticalGrid(columns = GridCells.Fixed(2), modifier = Modifier.fillMaxSize()) {
                item(span = { GridItemSpan(maxLineSpan) }) {
                    Row(Modifier.padding(PoroSpace.md), horizontalArrangement = Arrangement.spacedBy(PoroSpace.sm), verticalAlignment = Alignment.CenterVertically) {
                        Avatar(shop.logoUrl, shop.name, size = 64.dp)
                        Column {
                            Text("@${shop.handle}", style = MaterialTheme.typography.bodyMedium)
                            shop.description?.let { Text(it, style = MaterialTheme.typography.bodySmall) }
                        }
                    }
                }
                items(state.products, key = { it.productId }) { product -> ProductCard(product) { onOpenProduct(product) } }
                if (state.started && !state.loading && state.products.isEmpty()) {
                    item(span = { GridItemSpan(maxLineSpan) }) { EmptyState(stringResource(R.string.shop_no_products)) }
                }
                state.error?.let { item(span = { GridItemSpan(maxLineSpan) }) { ErrorMessageText(it, Modifier.padding(16.dp)) } }
                if (state.loading) item(span = { GridItemSpan(maxLineSpan) }) { CircularProgressIndicator(Modifier.padding(16.dp)) }
                if (state.canLoadMore) item(span = { GridItemSpan(maxLineSpan) }) { LaunchedEffect(state.products.size) { onLoadMore() } }
            }
        }
    }
}

/** Vignette d'un produit : photo, titre et prix le plus bas de ses déclinaisons. */
@Composable
fun ProductCard(product: ProductDto, onClick: () -> Unit) {
    val minPrice = product.variants.minOfOrNull { it.price }
    Column(Modifier.padding(8.dp).clickable(role = Role.Button, onClick = onClick)) {
        AsyncImage(
            product.images.firstOrNull()?.url,
            contentDescription = null,
            contentScale = ContentScale.Crop,
            modifier = Modifier.fillMaxWidth().aspectRatio(1f).clip(RoundedCornerShape(PoroSize.radiusImage)),
        )
        Text(product.title, maxLines = 2, overflow = TextOverflow.Ellipsis, style = MaterialTheme.typography.bodyMedium)
        minPrice?.let {
            PriceText(
                amount = it,
                currency = product.currency,
                from = product.variants.map { v -> v.price }.distinct().size > 1,
                style = MaterialTheme.typography.titleMedium,
            )
        }
    }
}
