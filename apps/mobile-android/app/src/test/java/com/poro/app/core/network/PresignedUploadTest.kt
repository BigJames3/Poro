package com.poro.app.core.network

import java.net.InetAddress
import java.util.concurrent.TimeUnit
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.cancelAndJoin
import kotlinx.coroutines.launch
import kotlinx.coroutines.runBlocking
import kotlinx.coroutines.test.runTest
import okhttp3.OkHttpClient
import okhttp3.mockwebserver.MockResponse
import okhttp3.mockwebserver.MockWebServer
import okhttp3.mockwebserver.SocketPolicy
import org.junit.After
import org.junit.Assert.assertEquals
import org.junit.Assert.assertTrue
import org.junit.Before
import org.junit.Test

/**
 * Chemin d'envoi des images (écart E-01) : formulaire présigné tel que le stockage l'attend,
 * URL `localhost` du backend local redirigée vers l'hôte de l'API comme sur émulateur.
 */
class PresignedUploadTest {

    private lateinit var storage: MockWebServer

    @Before
    fun setUp() {
        storage = MockWebServer()
        // Même adresse que la cible de la réécriture (localhost peut aussi désigner ::1).
        storage.start(InetAddress.getByName("127.0.0.1"), 0)
    }

    @After
    fun tearDown() {
        storage.shutdown()
    }

    /** Client des médias en debug : `localhost` est envoyé à 127.0.0.1, comme vers 10.0.2.2 sur émulateur. */
    private val debugMediaClient = OkHttpClient.Builder().addInterceptor(LoopbackRewriteInterceptor("127.0.0.1")).build()

    private val fields = mapOf("key" to "avatars/uploads/u1/k1", "Content-Type" to "image/jpeg", "Policy" to "eyJ9", "X-Amz-Signature" to "abc")

    @Test
    fun `the form sends every field before the file, to the signed host`() = runTest {
        storage.enqueue(MockResponse().setResponseCode(204).setHeader("ETag", "\"e1\""))
        val url = "http://localhost:${storage.port}/poro-uploads"

        val result = PresignedUploader(debugMediaClient).postForm(url, fields, byteArrayOf(1, 2, 3), "image/jpeg")

        assertEquals(UploadResult.Success("\"e1\""), result)
        val request = storage.takeRequest()
        assertEquals("POST", request.method)
        assertEquals("/poro-uploads", request.path)
        // Le stockage vérifie l'hôte signé : il doit rester celui de l'URL, pas 127.0.0.1.
        assertEquals("localhost:${storage.port}", request.getHeader("Host"))
        val body = request.body.readUtf8()
        val fileAt = body.indexOf("name=\"file\"")
        assertTrue("fichier présent", fileAt > 0)
        fields.keys.forEach { name ->
            val at = body.indexOf("name=\"$name\"")
            assertTrue("champ $name avant le fichier", at in 0 until fileAt)
        }
        assertTrue("type du fichier", body.substring(fileAt).contains("Content-Type: image/jpeg"))
    }

    @Test
    fun `a refusal of the storage is reported with its status`() = runTest {
        storage.enqueue(MockResponse().setResponseCode(403))

        val result = PresignedUploader(OkHttpClient()).postForm(storage.url("/b").toString(), fields, byteArrayOf(1), "image/jpeg")

        assertEquals(UploadResult.Rejected(403), result)
    }

    @Test
    fun `an unreachable storage is a network error, not a refusal`() = runTest {
        // Port 9000 sans tunnel adb ni SeaweedFS : connexion refusée.
        val url = storage.url("/b").toString()
        storage.shutdown()

        val result = PresignedUploader(OkHttpClient()).postForm(url, fields, byteArrayOf(1), "image/jpeg")

        assertTrue(result.toString(), result is UploadResult.NetworkError)
    }

    @Test
    fun `a multipart part is sent with PUT and returns its ETag`() = runTest {
        storage.enqueue(MockResponse().setHeader("ETag", "\"p1\""))

        val result = PresignedUploader(debugMediaClient).putPart("http://localhost:${storage.port}/v/part?partNumber=1", byteArrayOf(9, 9))

        assertEquals(UploadResult.Success("\"p1\""), result)
        val request = storage.takeRequest()
        assertEquals("PUT", request.method)
        assertEquals(2L, request.bodySize)
    }

    @Test
    fun `cancelling stops a part being sent at once`() = runBlocking {
        // Le stockage ne répond jamais : sans annulation, l'envoi attendrait le délai du client.
        storage.enqueue(MockResponse().setSocketPolicy(SocketPolicy.NO_RESPONSE))
        val url = "http://127.0.0.1:${storage.port}/part"
        val upload = launch(Dispatchers.IO) { PresignedUploader(OkHttpClient()).putPart(url, ByteArray(1024)) }
        storage.takeRequest(5, TimeUnit.SECONDS)
        val started = System.nanoTime()
        upload.cancelAndJoin()
        val elapsedMs = (System.nanoTime() - started) / 1_000_000
        assertTrue("annulé en $elapsedMs ms", elapsedMs < 2_000)
        assertTrue(upload.isCancelled)
    }
}
