package com.poro.app.testing

import com.poro.app.core.network.ApiResult
import com.poro.app.core.network.PageDto
import com.poro.app.data.social.CommentDto
import com.poro.app.data.social.CommentLikeDto
import com.poro.app.data.social.ShareChannel
import com.poro.app.data.social.ShareResultDto
import com.poro.app.data.social.SocialRepository
import com.poro.app.data.social.UserStatsDto
import com.poro.app.data.social.VideoStatsDto

/** Réponses programmées ; sans réponse programmée, un succès plausible. Enregistre les appels. */
class FakeSocialRepository : SocialRepository {
    val calls = mutableListOf<String>()
    val likeResults = ArrayDeque<ApiResult<VideoStatsDto>>()
    val commentPages = ArrayDeque<ApiResult<PageDto<CommentDto>>>()
    val replyPages = ArrayDeque<ApiResult<PageDto<CommentDto>>>()
    val postResults = ArrayDeque<ApiResult<CommentDto>>()
    val followResults = ArrayDeque<ApiResult<UserStatsDto>>()
    var stats = VideoStatsDto(videoId = "v1", likes = 10, comments = 2, shares = 1, likedByMe = false)
    var userStats = UserStatsDto(userId = "u2", followers = 5, following = 3, followedByMe = false)

    override suspend fun videoStats(videoId: String): ApiResult<VideoStatsDto> {
        calls += "stats:$videoId"
        return ApiResult.Success(stats)
    }

    /** null : échec réseau de la lecture groupée. */
    var batchStats: List<VideoStatsDto>? = emptyList()

    override suspend fun videosStats(videoIds: List<String>): ApiResult<List<VideoStatsDto>> {
        calls += "batch:" + videoIds.joinToString(",")
        return batchStats?.let { ApiResult.Success(it) } ?: ApiResult.NetworkError(java.io.IOException("offline"))
    }

    override suspend fun setLiked(videoId: String, liked: Boolean): ApiResult<VideoStatsDto> {
        calls += "like:$videoId:$liked"
        return likeResults.removeFirst()
    }

    override suspend fun comments(videoId: String, cursor: String?) = commentPages.removeFirst().also { calls += "comments:$cursor" }

    override suspend fun replies(commentId: String, cursor: String?) = replyPages.removeFirst().also { calls += "replies:$commentId:$cursor" }

    override suspend fun postComment(videoId: String, content: String, parentId: String?): ApiResult<CommentDto> {
        calls += "post:$content:$parentId"
        return postResults.removeFirst()
    }

    override suspend fun editComment(commentId: String, content: String): ApiResult<CommentDto> =
        ApiResult.Success(CommentDto(id = commentId, content = content, edited = true))

    override suspend fun deleteComment(commentId: String): ApiResult<Unit> {
        calls += "delete:$commentId"
        return ApiResult.Success(Unit)
    }

    override suspend fun setCommentLiked(commentId: String, liked: Boolean): ApiResult<CommentLikeDto> {
        calls += "likeComment:$commentId:$liked"
        return ApiResult.Success(CommentLikeDto(commentId, likesCount = if (liked) 1 else 0, likedByMe = liked))
    }

    override suspend fun userStats(userId: String): ApiResult<UserStatsDto> = ApiResult.Success(userStats)

    override suspend fun setFollowing(userId: String, following: Boolean): ApiResult<UserStatsDto> {
        calls += "follow:$userId:$following"
        return followResults.removeFirst()
    }

    override suspend fun share(videoId: String, channel: ShareChannel): ApiResult<ShareResultDto> {
        calls += "share:$videoId:${channel.wire}"
        return ApiResult.Success(ShareResultDto("s1", sharesCount = 2))
    }
}
