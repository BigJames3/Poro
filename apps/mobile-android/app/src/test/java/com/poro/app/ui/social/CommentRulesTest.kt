package com.poro.app.ui.social

import com.poro.app.data.social.CommentDto
import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Test

class CommentRulesTest {

    private val now = 1_700_000_000_000L

    @Test
    fun `own comment is editable only within 15 minutes`() {
        val mine = CommentDto(id = "c1", userId = "u1", content = "ok", createdAt = "2023-11-14T22:12:50Z")
        val created = com.poro.app.core.format.Formats.parseIsoMillis(mine.createdAt)!!
        assertTrue(CommentRules.canEdit(mine, "u1", created + 60_000))
        assertFalse(CommentRules.canEdit(mine, "u1", created + CommentRules.EDIT_WINDOW_MS + 1))
        assertFalse(CommentRules.canEdit(mine, "u2", created + 1_000))
        assertFalse(CommentRules.canEdit(mine.copy(deleted = true), "u1", created + 1_000))
        assertFalse(CommentRules.canEdit(mine.copy(createdAt = null), "u1", now))
    }
}
