package com.poro.app.core.network

import java.io.IOException
import kotlin.coroutines.cancellation.CancellationException
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.withContext
import okhttp3.MediaType.Companion.toMediaType
import okhttp3.MultipartBody
import okhttp3.OkHttpClient
import okhttp3.Request
import okhttp3.RequestBody.Companion.toRequestBody

/** Résultat d'un envoi direct vers le stockage : succès, refus du stockage (HTTP) ou réseau coupé. */
sealed interface UploadResult {
    /** [etag] : en-tête `ETag` de la réponse, exigé pour assembler un envoi en plusieurs parties. */
    data class Success(val etag: String?) : UploadResult
    data class Rejected(val httpStatus: Int) : UploadResult
    data class NetworkError(val cause: IOException) : UploadResult
}

/**
 * Envois directs vers le stockage objet, sans passer par l'API (URL présignées) ; toujours
 * avec le client des médias, qui n'ajoute pas de jeton.
 */
class PresignedUploader(private val client: OkHttpClient) {

    /** Formulaire POST présigné (avatar, logo, photo de produit) : les champs, puis `file`. */
    suspend fun postForm(url: String, fields: Map<String, String>, bytes: ByteArray, contentType: String): UploadResult {
        val body = MultipartBody.Builder().setType(MultipartBody.FORM).apply {
            fields.forEach { (name, value) -> addFormDataPart(name, value) }
            addFormDataPart("file", "upload", bytes.toRequestBody(contentType.toMediaType()))
        }.build()
        return execute(Request.Builder().url(url).post(body).build())
    }

    /** Une partie d'un envoi multipart S3 (`PUT` présigné) ; l'ETag rendu sert à l'assemblage. */
    suspend fun putPart(url: String, bytes: ByteArray): UploadResult =
        execute(Request.Builder().url(url).put(bytes.toRequestBody()).build())

    private suspend fun execute(request: Request): UploadResult = withContext(Dispatchers.IO) {
        try {
            client.newCall(request).execute().use { response ->
                if (response.isSuccessful) UploadResult.Success(response.header("ETag")) else UploadResult.Rejected(response.code)
            }
        } catch (e: CancellationException) {
            throw e
        } catch (e: IOException) {
            UploadResult.NetworkError(e)
        }
    }
}
