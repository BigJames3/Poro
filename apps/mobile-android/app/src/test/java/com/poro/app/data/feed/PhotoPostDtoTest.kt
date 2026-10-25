package com.poro.app.data.feed

import com.poro.app.core.network.PoroJson
import com.poro.app.data.search.VideoHit
import com.poro.app.data.video.VideoDto
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Test

/** Réponses du contrat (feed, video et search OpenAPI) pour une publication photo et une vidéo. */
class PhotoPostDtoTest {

    @Test
    fun `a feed photo post has its images and no HLS, a video stays a video`() {
        val photo = PoroJson.decodeFromString<FeedItem>(
            """{"video_id":"p1","media_type":"photo","images":[
            {"url":"https://cdn/p/image-1.jpg","thumbnail_url":"https://cdn/p/thumb-1.jpg","width":1536,"height":2048},
            {"url":"https://cdn/p/image-2.jpg","thumbnail_url":"https://cdn/p/thumb-2.jpg","width":2048,"height":1536}],
            "title":"","hashtags":["abidjan"],"duration_ms":0,"published_at":"2026-10-25T10:00:00Z",
            "thumbnail_url":"https://cdn/p/thumb-1.jpg","hls_url":"","stats":{"likes":1,"comments":0,"shares":0},
            "author":{"id":"u1","username":"awa","display_name":"Awa","avatar_url":null}}""",
        )
        assertTrue(photo.isPhoto)
        assertEquals(listOf(1536, 2048), photo.images.map { it.width })

        val legacy = PoroJson.decodeFromString<FeedItem>("""{"video_id":"v1","title":"Danse","hls_url":"https://cdn/v/master.m3u8"}""")
        assertFalse("an item without media_type is a video", legacy.isPhoto)
    }

    @Test
    fun `a ready photo post from the video api carries its ordered images`() {
        val post = PoroJson.decodeFromString<VideoDto>(
            """{"id":"p1","user_id":"u1","media_type":"photo","title":"","description":"Marché","status":"ready",
            "moderation_status":"approved","images":[{"position":1,"url":"u1","thumbnail_url":"t1","width":32,"height":64}],
            "created_at":"2026-10-25T10:00:00Z"}""",
        )
        assertTrue(post.isPhoto && post.isReady)
        assertEquals("t1", post.images.single().thumbnailUrl)
    }

    @Test
    fun `a search hit says whether it is a photo post`() {
        assertTrue(PoroJson.decodeFromString<VideoHit>("""{"video_id":"p1","media_type":"photo"}""").isPhoto)
        assertFalse(PoroJson.decodeFromString<VideoHit>("""{"video_id":"v1"}""").isPhoto)
    }
}
