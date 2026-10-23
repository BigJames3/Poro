package com.poro.app.ui.social

import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.ListItem
import androidx.compose.material3.Scaffold
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.semantics.Role
import androidx.compose.ui.unit.dp
import com.poro.app.R
import com.poro.app.core.format.Formats
import com.poro.app.data.social.FollowEntryDto
import com.poro.app.ui.auth.ErrorMessageText
import com.poro.app.ui.common.Avatar
import com.poro.app.ui.common.BackTopBar
import com.poro.app.ui.common.EmptyState

@Composable
fun FollowListScreen(
    kind: FollowListKind,
    state: FollowListUiState,
    onBack: () -> Unit,
    onRetry: () -> Unit,
    onOpen: (FollowEntryDto) -> Unit,
) {
    val title = stringResource(
        if (kind == FollowListKind.Followers) R.string.follow_list_followers else R.string.follow_list_following,
    )
    Scaffold(topBar = { BackTopBar(title, onBack) }) { padding ->
        LazyColumn(Modifier.fillMaxSize().padding(padding)) {
            if (state.endReached && state.items.isEmpty() && state.error == null) {
                item {
                    EmptyState(
                        stringResource(
                            if (kind == FollowListKind.Followers) {
                                R.string.follow_list_empty_followers
                            } else {
                                R.string.follow_list_empty_following
                            },
                        ),
                    )
                }
            }
            items(state.items, key = { it.userId }) { entry ->
                val name = stringResource(R.string.member_fallback)
                ListItem(
                    headlineContent = { Text(name) },
                    supportingContent = {
                        val whenFollowed = Formats.relative(entry.followedAt, System.currentTimeMillis())
                        if (whenFollowed.isNotEmpty()) Text(whenFollowed)
                    },
                    leadingContent = { Avatar(url = null, name = name, size = 40.dp) },
                    modifier = Modifier.fillMaxWidth().clickable(role = Role.Button, onClickLabel = stringResource(R.string.follow_list_open)) {
                        onOpen(entry)
                    },
                )
            }
            state.error?.let { item { ErrorMessageText(it, Modifier.padding(16.dp)) } }
            if (!state.endReached) {
                item {
                    LaunchedEffect(state.items.size) { onRetry() }
                    Box(Modifier.fillMaxWidth().padding(16.dp), contentAlignment = Alignment.Center) {
                        CircularProgressIndicator()
                    }
                }
            }
        }
    }
}
