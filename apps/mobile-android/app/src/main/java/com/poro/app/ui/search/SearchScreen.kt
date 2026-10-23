package com.poro.app.ui.search

import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.LazyListScope
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.text.KeyboardActions
import androidx.compose.foundation.text.KeyboardOptions
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.Clear
import androidx.compose.material.icons.filled.Search
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.ListItem
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.ScrollableTabRow
import androidx.compose.material3.Tab
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.ui.Modifier
import androidx.compose.ui.layout.ContentScale
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.semantics.Role
import androidx.compose.ui.text.input.ImeAction
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import coil3.compose.AsyncImage
import com.poro.app.R
import com.poro.app.core.format.Formats
import com.poro.app.data.search.HashtagHit
import com.poro.app.data.search.ProductHit
import com.poro.app.data.search.ShopHit
import com.poro.app.data.search.UserHit
import com.poro.app.data.search.VideoHit
import com.poro.app.ui.auth.ErrorMessageText
import com.poro.app.ui.common.Avatar
import com.poro.app.ui.common.displayNameOf

@Composable
fun SearchScreen(
    state: SearchUiState,
    onQueryChange: (String) -> Unit,
    onSubmit: (String) -> Unit,
    onSelectTab: (SearchTab) -> Unit,
    onLoadMore: () -> Unit,
    onOpenVideo: (VideoHit) -> Unit,
    onOpenUser: (UserHit) -> Unit,
    onOpenProduct: (ProductHit) -> Unit,
    onOpenShop: (ShopHit) -> Unit,
) {
    Column(Modifier.fillMaxSize()) {
        OutlinedTextField(
            value = state.query,
            onValueChange = onQueryChange,
            placeholder = { Text(stringResource(R.string.search_placeholder)) },
            leadingIcon = { Icon(Icons.Filled.Search, contentDescription = null) },
            trailingIcon = {
                if (state.query.isNotEmpty()) {
                    IconButton(onClick = { onQueryChange("") }) {
                        Icon(Icons.Filled.Clear, contentDescription = stringResource(R.string.search_clear))
                    }
                }
            },
            singleLine = true,
            keyboardOptions = KeyboardOptions(imeAction = ImeAction.Search),
            keyboardActions = KeyboardActions(onSearch = { onSubmit(state.query) }),
            modifier = Modifier.fillMaxWidth().padding(16.dp),
        )
        val suggestions = state.suggestions
        if (suggestions.users.isNotEmpty() || suggestions.hashtags.isNotEmpty()) {
            // Suggestions pendant la frappe, à la place des résultats.
            LazyColumn {
                items(suggestions.users, key = { "u" + it.userId }) { user -> UserRow(user) { onOpenUser(user) } }
                items(suggestions.hashtags, key = { "h" + it.tag }) { tag -> HashtagRow(tag) { onSubmit("#" + tag.tag) } }
            }
            return@Column
        }
        if (state.submitted == null) {
            Text(stringResource(R.string.search_hint), modifier = Modifier.padding(16.dp), style = MaterialTheme.typography.bodyMedium)
            return@Column
        }
        ScrollableTabRow(selectedTabIndex = state.tab.ordinal, edgePadding = 16.dp) {
            SearchTab.entries.forEach { tab ->
                Tab(
                    selected = tab == state.tab,
                    onClick = { onSelectTab(tab) },
                    text = { Text(stringResource(tab.label)) },
                )
            }
        }
        LazyColumn(Modifier.fillMaxSize()) {
            when (state.tab) {
                SearchTab.Videos -> results(state.videos, onLoadMore) { video -> VideoRow(video) { onOpenVideo(video) } }
                SearchTab.Users -> results(state.users, onLoadMore) { user -> UserRow(user) { onOpenUser(user) } }
                SearchTab.Hashtags -> results(state.hashtags, onLoadMore) { tag -> HashtagRow(tag) { onSubmit("#" + tag.tag) } }
                SearchTab.Products -> results(state.products, onLoadMore) { product -> ProductRow(product) { onOpenProduct(product) } }
                SearchTab.Shops -> results(state.shops, onLoadMore) { shop -> ShopRow(shop) { onOpenShop(shop) } }
            }
        }
    }
}

private val SearchTab.label: Int
    get() = when (this) {
        SearchTab.Videos -> R.string.search_tab_videos
        SearchTab.Users -> R.string.search_tab_users
        SearchTab.Hashtags -> R.string.search_tab_hashtags
        SearchTab.Products -> R.string.search_tab_products
        SearchTab.Shops -> R.string.search_tab_shops
    }

/** Liste paginée : la page suivante se charge quand on atteint la fin. */
private fun <T> LazyListScope.results(results: Results<T>, onLoadMore: () -> Unit, row: @Composable (T) -> Unit) {
    items(results.items) { row(it) }
    if (results.isEmpty) item { Text(stringResource(R.string.search_no_results), modifier = Modifier.padding(16.dp)) }
    results.error?.let { item { ErrorMessageText(it, Modifier.padding(16.dp)) } }
    if (results.loading) item { CircularProgressIndicator(Modifier.padding(16.dp)) }
    if (results.canLoadMore) item { LaunchedEffect(results.items.size) { onLoadMore() } }
}

@Composable
private fun VideoRow(video: VideoHit, onClick: () -> Unit) {
    Row(
        Modifier.fillMaxWidth().clickable(role = Role.Button, onClick = onClick).padding(horizontal = 16.dp, vertical = 8.dp),
        horizontalArrangement = Arrangement.spacedBy(12.dp),
    ) {
        Box(Modifier.size(width = 72.dp, height = 96.dp)) {
            AsyncImage(video.thumbnailUrl, contentDescription = null, contentScale = ContentScale.Crop, modifier = Modifier.fillMaxSize())
        }
        Column(verticalArrangement = Arrangement.spacedBy(4.dp)) {
            Text(video.title, style = MaterialTheme.typography.titleSmall, maxLines = 2, overflow = TextOverflow.Ellipsis)
            Text(displayNameOf(video.author.displayName, video.author.username), style = MaterialTheme.typography.bodySmall)
            Text(
                listOfNotNull(
                    stringResource(R.string.search_likes, Formats.count(video.stats.likes)),
                    video.durationMs?.let { Formats.duration(it) },
                ).joinToString(" · "),
                style = MaterialTheme.typography.bodySmall,
                color = MaterialTheme.colorScheme.onSurfaceVariant,
            )
        }
    }
}

@Composable
private fun UserRow(user: UserHit, onClick: () -> Unit) {
    ListItem(
        leadingContent = { Avatar(user.avatarUrl, user.displayName ?: user.username, size = 40.dp) },
        headlineContent = { Text(displayNameOf(user.displayName, user.username)) },
        supportingContent = { Text("@${user.username} · " + stringResource(R.string.profile_followers, Formats.count(user.followers))) },
        modifier = Modifier.clickable(role = Role.Button, onClick = onClick),
    )
}

@Composable
private fun HashtagRow(tag: HashtagHit, onClick: () -> Unit) {
    ListItem(
        headlineContent = { Text("#${tag.tag}") },
        supportingContent = { Text(stringResource(R.string.search_hashtag_videos, Formats.count(tag.videosCount))) },
        modifier = Modifier.clickable(role = Role.Button, onClick = onClick),
    )
}

@Composable
private fun ProductRow(product: ProductHit, onClick: () -> Unit) {
    Row(
        Modifier.fillMaxWidth().clickable(role = Role.Button, onClick = onClick).padding(horizontal = 16.dp, vertical = 8.dp),
        horizontalArrangement = Arrangement.spacedBy(12.dp),
    ) {
        Box(Modifier.size(72.dp)) {
            AsyncImage(product.imageUrl, contentDescription = null, contentScale = ContentScale.Crop, modifier = Modifier.fillMaxSize())
        }
        Column(verticalArrangement = Arrangement.spacedBy(4.dp)) {
            Text(product.title, style = MaterialTheme.typography.titleSmall, maxLines = 2, overflow = TextOverflow.Ellipsis)
            product.minPrice?.let {
                Text(stringResource(R.string.search_price_from, Formats.money(it, product.currency)), style = MaterialTheme.typography.bodyMedium)
            }
            Text(
                listOfNotNull(
                    product.shop.name,
                    if (product.inStock) null else stringResource(R.string.search_out_of_stock),
                ).joinToString(" · "),
                style = MaterialTheme.typography.bodySmall,
                color = MaterialTheme.colorScheme.onSurfaceVariant,
            )
        }
    }
}

@Composable
private fun ShopRow(shop: ShopHit, onClick: () -> Unit) {
    ListItem(
        leadingContent = { Avatar(shop.logoUrl, shop.name, size = 40.dp) },
        headlineContent = { Text(shop.name) },
        supportingContent = { Text("@${shop.handle}") },
        modifier = Modifier.clickable(role = Role.Button, onClick = onClick),
    )
}

