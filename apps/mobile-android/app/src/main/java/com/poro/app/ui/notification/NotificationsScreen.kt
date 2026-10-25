package com.poro.app.ui.notification

import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.Notifications
import androidx.compose.material.icons.outlined.Notifications
import androidx.compose.material3.Badge
import androidx.compose.material3.BadgedBox
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.ListItem
import androidx.compose.material3.ListItemDefaults
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Scaffold
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.material3.pulltorefresh.PullToRefreshBox
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.semantics.Role
import androidx.compose.ui.unit.dp
import com.poro.app.R
import com.poro.app.core.format.Formats
import com.poro.app.data.notification.NotificationDto
import com.poro.app.ui.common.Avatar
import com.poro.app.ui.common.BackTopBar
import com.poro.app.ui.common.EmptyState
import com.poro.app.ui.common.ErrorRetry
import com.poro.app.ui.quantityString
import com.poro.app.ui.theme.PoroSpacing

@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun NotificationsScreen(
    state: NotificationsUiState,
    onBack: () -> Unit,
    onOpen: (NotificationDto) -> Unit,
    onReadAll: () -> Unit,
    onLoadMore: () -> Unit,
    onRetry: () -> Unit,
    onRefresh: () -> Unit,
) {
    Scaffold(
        topBar = {
            BackTopBar(stringResource(R.string.notifications_title), onBack) {
                if (state.hasUnread) TextButton(onClick = onReadAll) { Text(stringResource(R.string.notifications_read_all)) }
            }
        },
    ) { padding ->
        PullToRefreshBox(isRefreshing = state.refreshing, onRefresh = onRefresh, modifier = Modifier.fillMaxSize().padding(padding)) {
            LazyColumn(Modifier.fillMaxSize()) {
                items(state.items, key = { it.id }) { notification -> NotificationRow(notification) { onOpen(notification) } }
                if (state.isEmpty) item { EmptyState(stringResource(R.string.notifications_empty), icon = Icons.Outlined.Notifications) }
                state.error?.let { item { ErrorRetry(it, onRetry, Modifier.fillMaxWidth().padding(PoroSpacing.L)) } }
                if (state.loading) item { CircularProgressIndicator(Modifier.padding(PoroSpacing.L)) }
                if (state.canLoadMore) item { LaunchedEffect(state.items.size) { onLoadMore() } }
            }
        }
    }
}

/** Titre et texte viennent du serveur, déjà en français ; une notification non lue est surlignée. */
@Composable
private fun NotificationRow(notification: NotificationDto, onClick: () -> Unit) {
    val actor = notification.actor
    ListItem(
        leadingContent = { Avatar(actor?.avatarUrl, actor?.displayName ?: actor?.username ?: "Poro", size = 40.dp) },
        headlineContent = { Text(notification.title) },
        supportingContent = {
            Text(notification.body + " · " + Formats.relative(notification.lastActivityAt ?: notification.createdAt, System.currentTimeMillis()))
        },
        colors = ListItemDefaults.colors(
            containerColor = if (notification.read) Color.Transparent else MaterialTheme.colorScheme.secondaryContainer,
        ),
        modifier = Modifier.clickable(role = Role.Button, onClick = onClick),
    )
}

/** Cloche avec le nombre de notifications non lues (relu à chaque affichage). */
@Composable
fun NotificationBell(count: Int, onClick: () -> Unit, tint: Color = Color.White) {
    val description = if (count > 0) {
        quantityString(R.plurals.notifications_unread, count)
    } else {
        stringResource(R.string.notifications_title)
    }
    IconButton(onClick = onClick) {
        BadgedBox(badge = { if (count > 0) Badge { Text(if (count > 99) "99+" else count.toString()) } }) {
            Icon(Icons.Filled.Notifications, contentDescription = description, tint = tint)
        }
    }
}
