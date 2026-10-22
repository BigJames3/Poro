package com.poro.app.ui.notification

import com.poro.app.data.notification.NotificationDto

/** Où mène une notification touchée. */
sealed interface NotificationTarget {
    data class Video(val videoId: String) : NotificationTarget
    data class Profile(val userId: String, val username: String?) : NotificationTarget

    /** Commande vue par l'acheteur (`GET /orders/{id}`) ou par le vendeur (`GET /seller/orders/{id}`). */
    data class Order(val orderId: String, val asSeller: Boolean) : NotificationTarget
    data object None : NotificationTarget
}

/** Cible d'après `data` (contrat : video_id, user_id, order_id et role), à défaut l'entité concernée. */
fun NotificationDto.target(): NotificationTarget {
    data["order_id"]?.let { return NotificationTarget.Order(it, asSeller = data["role"] == "seller") }
    return when (type) {
        "follow" -> (data["user_id"] ?: actor?.id)?.let { NotificationTarget.Profile(it, actor?.username) }
        "like", "comment", "reply", "video_ready" -> (data["video_id"] ?: entityId.takeIf { entityType == "video" })
            ?.let(NotificationTarget::Video)
        else -> null
    } ?: NotificationTarget.None
}
