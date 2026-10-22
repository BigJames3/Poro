package com.poro.app.data.moderation

import com.poro.app.core.network.ApiErrorCodes
import com.poro.app.core.network.ApiResult
import com.poro.app.core.network.PoroJson
import kotlinx.coroutines.test.runTest
import okhttp3.MediaType.Companion.toMediaType
import okhttp3.OkHttpClient
import okhttp3.mockwebserver.MockResponse
import okhttp3.mockwebserver.MockWebServer
import org.junit.After
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Before
import org.junit.Test
import retrofit2.Retrofit
import retrofit2.converter.kotlinx.serialization.asConverterFactory

class ModerationRepositoryTest {

    private lateinit var server: MockWebServer
    private lateinit var repo: DefaultModerationRepository

    @Before
    fun setUp() {
        server = MockWebServer()
        val api = Retrofit.Builder()
            .baseUrl(server.url("/"))
            .client(OkHttpClient())
            .addConverterFactory(PoroJson.asConverterFactory("application/json".toMediaType()))
            .build()
            .create(ModerationApi::class.java)
        repo = DefaultModerationRepository(api)
    }

    @After
    fun tearDown() {
        server.shutdown()
    }

    @Test
    fun `a report sends the contract values and a blank detail is left out`() = runTest {
        server.enqueue(json(201, """{"data":{"report_id":"r1","case_id":"c1","created":true},"error":null,"meta":{}}"""))
        server.enqueue(json(200, """{"data":{"report_id":"r1","case_id":"c1","created":false},"error":null,"meta":{}}"""))

        repo.report(ReportTargetType.Comment, "id-1", ReportReason.Harassment, "  ")
        val again = (repo.report(ReportTargetType.Video, "id-2", ReportReason.Other, " Lien frauduleux ") as ApiResult.Success).data

        val first = server.takeRequest()
        assertEquals("/api/v1/moderation/reports", first.path)
        assertEquals("""{"target_type":"comment","target_id":"id-1","reason":"harassment"}""", first.body.readUtf8())
        assertEquals(
            """{"target_type":"video","target_id":"id-2","reason":"other","comment":"Lien frauduleux"}""",
            server.takeRequest().body.readUtf8(),
        )
        assertFalse(again.created)
    }

    @Test
    fun `reporting oneself is a failure with its code`() = runTest {
        server.enqueue(json(422, """{"data":null,"error":{"code":"cannot_report_self","message":"no"},"meta":{}}"""))

        val result = repo.report(ReportTargetType.User, "me", ReportReason.Spam, null)

        assertEquals(ApiErrorCodes.CANNOT_REPORT_SELF, (result as ApiResult.Failure).code)
    }

    private fun json(code: Int, body: String) =
        MockResponse().setResponseCode(code).setHeader("Content-Type", "application/json").setBody(body)
}
