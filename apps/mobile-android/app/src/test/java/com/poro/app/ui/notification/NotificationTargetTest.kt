package com.poro.app.ui.notification

import com.poro.app.data.notification.ActorDto
import com.poro.app.data.notification.NotificationDto
import org.junit.Assert.assertEquals
import org.junit.Test

class NotificationTargetTest {

    private fun notification(type: String, data: Map<String, String> = emptyMap(), actor: ActorDto? = null) =
        NotificationDto(id = "n1", type = type, data = data, actor = actor, entityType = "video", entityId = "v9")

    @Test
    fun `video notifications open the video`() {
        assertEquals(NotificationTarget.Video("v1"), notification("like", mapOf("video_id" to "v1")).target())
        assertEquals(NotificationTarget.Video("v1"), notification("reply", mapOf("video_id" to "v1", "comment_id" to "c1")).target())
        assertEquals("à défaut, l'entité", NotificationTarget.Video("v9"), notification("video_ready").target())
    }

    @Test
    fun `a follow opens the follower's profile`() {
        val actor = ActorDto(id = "u2", username = "kofi")
        assertEquals(NotificationTarget.Profile("u2", "kofi"), notification("follow", actor = actor).target())
    }

    @Test
    fun `orders open the buyer or the seller view`() {
        assertEquals(NotificationTarget.Order("o1", asSeller = true), notification("order_new", mapOf("order_id" to "o1", "role" to "seller")).target())
        assertEquals(NotificationTarget.Order("o1", asSeller = false), notification("order_shipped", mapOf("order_id" to "o1", "role" to "buyer")).target())
    }

    @Test
    fun `unknown types go nowhere`() {
        assertEquals(NotificationTarget.None, notification("promo").target())
    }
}
