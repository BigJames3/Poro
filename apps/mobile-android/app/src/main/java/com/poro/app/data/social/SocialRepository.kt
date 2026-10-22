package com.poro.app.data.social

import com.poro.app.core.network.ApiResult
import com.poro.app.core.network.PageDto
import com.poro.app.core.network.apiCall

enum class ShareChannel(val wire: String) { WhatsApp("whatsapp"), CopyLink("copy_link"), Other("other") }

interface SocialRepository {
    suspend fun videoStats(videoId: String): ApiResult<VideoStatsDto>

    /** Compteurs de plusieurs vidéos (50 au plus par requête, découpées ici). */
    suspend fun videosStats(videoIds: List<String>): ApiResult<List<VideoStatsDto>>

    /** Pose ou retire le j'aime ; renvoie les compteurs à jour. */
    suspend fun setLiked(videoId: String, liked: Boolean): ApiResult<VideoStatsDto>

    suspend fun comments(videoId: String, cursor: String?): ApiResult<PageDto<CommentDto>>

    suspend fun replies(commentId: String, cursor: String?): ApiResult<PageDto<CommentDto>>

    /** Commentaire, ou réponse à un commentaire de premier niveau ([parentId]). */
    suspend fun postComment(videoId: String, content: String, parentId: String?): ApiResult<CommentDto>

    suspend fun editComment(commentId: String, content: String): ApiResult<CommentDto>

    suspend fun deleteComment(commentId: String): ApiResult<Unit>

    suspend fun setCommentLiked(commentId: String, liked: Boolean): ApiResult<CommentLikeDto>

    suspend fun userStats(userId: String): ApiResult<UserStatsDto>

    suspend fun setFollowing(userId: String, following: Boolean): ApiResult<UserStatsDto>

    suspend fun share(videoId: String, channel: ShareChannel): ApiResult<ShareResultDto>
}

class DefaultSocialRepository(private val api: SocialApi) : SocialRepository {
    override suspend fun videoStats(videoId: String) = apiCall { api.videoStats(videoId) }

    override suspend fun videosStats(videoIds: List<String>): ApiResult<List<VideoStatsDto>> {
        val all = ArrayList<VideoStatsDto>(videoIds.size)
        for (chunk in videoIds.distinct().chunked(STATS_BATCH)) {
            when (val result = apiCall { api.videosStats(chunk.joinToString(",")) }) {
                is ApiResult.Success -> all += result.data.items
                is ApiResult.Failure -> return result
                is ApiResult.NetworkError -> return result
            }
        }
        return ApiResult.Success(all)
    }

    override suspend fun setLiked(videoId: String, liked: Boolean) =
        apiCall { if (liked) api.like(videoId) else api.unlike(videoId) }

    override suspend fun comments(videoId: String, cursor: String?) = apiCall { api.comments(videoId, cursor, PAGE_SIZE) }

    override suspend fun replies(commentId: String, cursor: String?) = apiCall { api.replies(commentId, cursor, PAGE_SIZE) }

    override suspend fun postComment(videoId: String, content: String, parentId: String?) =
        apiCall { api.postComment(videoId, NewComment(content.trim(), parentId)) }

    override suspend fun editComment(commentId: String, content: String) =
        apiCall { api.editComment(commentId, CommentText(content.trim())) }

    override suspend fun deleteComment(commentId: String) = apiCall(ifEmpty = {}) { api.deleteComment(commentId) }

    override suspend fun setCommentLiked(commentId: String, liked: Boolean) =
        apiCall { if (liked) api.likeComment(commentId) else api.unlikeComment(commentId) }

    override suspend fun userStats(userId: String) = apiCall { api.userStats(userId) }

    override suspend fun setFollowing(userId: String, following: Boolean) =
        apiCall { if (following) api.follow(userId) else api.unfollow(userId) }

    override suspend fun share(videoId: String, channel: ShareChannel) = apiCall { api.share(videoId, ShareRequest(channel.wire)) }

    private companion object {
        const val PAGE_SIZE = 20
        const val STATS_BATCH = 50
    }
}
