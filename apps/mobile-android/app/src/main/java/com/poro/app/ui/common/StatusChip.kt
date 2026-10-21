package com.poro.app.ui.common

import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Modifier
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.unit.dp
import com.poro.app.R
import com.poro.app.data.order.OrderDto
import com.poro.app.ui.theme.PoroSize
import com.poro.app.ui.theme.PoroSpace

/** Statut lisible : le texte reste obligatoire, la couleur n'est qu'un appoint. */
@Composable
fun StatusChip(status: String, modifier: Modifier = Modifier) {
    val scheme = MaterialTheme.colorScheme
    val (label, container, content) = when (status) {
        OrderDto.CONFIRMED, OrderDto.PAID, OrderDto.COMPLETED ->
            Triple(orderStatusLabel(status), scheme.secondaryContainer, scheme.onSecondaryContainer)
        OrderDto.CANCELLED ->
            Triple(orderStatusLabel(status), scheme.errorContainer, scheme.onErrorContainer)
        OrderDto.SHIPPED ->
            Triple(orderStatusLabel(status), scheme.tertiaryContainer, scheme.onTertiaryContainer)
        else ->
            Triple(orderStatusLabel(status), scheme.surfaceVariant, scheme.onSurfaceVariant)
    }
    Surface(
        modifier = modifier,
        color = container,
        contentColor = content,
        shape = RoundedCornerShape(PoroSize.radiusImage),
    ) {
        Text(
            stringResource(label),
            style = MaterialTheme.typography.labelLarge,
            modifier = Modifier.padding(horizontal = PoroSpace.sm, vertical = PoroSpace.xxs),
        )
    }
}

private fun orderStatusLabel(status: String): Int = when (status) {
    OrderDto.PENDING -> R.string.order_status_pending
    OrderDto.AWAITING_PAYMENT -> R.string.order_status_awaiting_payment
    OrderDto.CONFIRMED -> R.string.order_status_confirmed
    OrderDto.PAID -> R.string.order_status_paid
    OrderDto.SHIPPED -> R.string.order_status_shipped
    OrderDto.COMPLETED -> R.string.order_status_completed
    OrderDto.CANCELLED -> R.string.order_status_cancelled
    else -> R.string.order_status_unknown
}
