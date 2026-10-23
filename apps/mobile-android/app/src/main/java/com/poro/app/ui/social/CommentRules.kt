package com.poro.app.ui.social

import com.poro.app.core.format.Formats
import com.poro.app.data.social.CommentDto

/** Fenêtre d'édition : 15 minutes après la publication (docs/api/social.openapi.yaml). */
object CommentRules {
    const val EDIT_WINDOW_MS = 15 * 60 * 1000L

    fun canEdit(comment: CommentDto, myUserId: String?, nowMs: Long): Boolean {
        if (comment.deleted || comment.userId == null || comment.userId != myUserId) return false
        val created = Formats.parseIsoMillis(comment.createdAt) ?: return false
        val age = nowMs - created
        return age in 0..EDIT_WINDOW_MS
    }
}
