package com.poro.app.core.network

import java.io.IOException
import kotlin.coroutines.resume
import kotlinx.coroutines.suspendCancellableCoroutine
import okhttp3.Call
import okhttp3.Callback
import okhttp3.MediaType.Companion.toMediaType
import okhttp3.MultipartBody
import okhttp3.OkHttpClient
import okhttp3.Request
import okhttp3.RequestBody.Companion.toRequestBody
import okhttp3.Response

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

    /**
     * Une image d'une publication photo : `PUT` présigné avec les en-têtes signés. Le type vient
     * de `Content-Type` ; la longueur est celle du corps (OkHttp l'écrit, et le stockage refuse
     * une autre longueur que celle signée).
     */
    suspend fun putSigned(url: String, headers: Map<String, String>, bytes: ByteArray): UploadResult {
        val contentType = headers.entries.firstOrNull { it.key.equals("Content-Type", ignoreCase = true) }?.value
        val builder = Request.Builder().url(url).put(bytes.toRequestBody(contentType?.toMediaType()))
        headers.forEach { (name, value) ->
            if (!name.equals("Content-Type", ignoreCase = true) && !name.equals("Content-Length", ignoreCase = true)) {
                builder.header(name, value)
            }
        }
        return execute(builder.build())
    }

    /** Une partie d'un envoi multipart S3 (`PUT` présigné) ; l'ETag rendu sert à l'assemblage. */
    suspend fun putPart(url: String, bytes: ByteArray): UploadResult =
        execute(Request.Builder().url(url).put(bytes.toRequestBody()).build())

    /**
     * Envoi asynchrone, annulé avec la coroutine : « Annuler » coupe tout de suite la partie en
     * cours (jusqu'à 8 Mio) au lieu d'attendre la fin de son envoi.
     */
    private suspend fun execute(request: Request): UploadResult = suspendCancellableCoroutine { continuation ->
        val call = client.newCall(request)
        continuation.invokeOnCancellation { call.cancel() }
        call.enqueue(object : Callback {
            override fun onFailure(call: Call, e: IOException) {
                if (continuation.isActive) continuation.resume(UploadResult.NetworkError(e))
            }

            override fun onResponse(call: Call, response: Response) {
                val result = response.use {
                    if (it.isSuccessful) UploadResult.Success(it.header("ETag")) else UploadResult.Rejected(it.code)
                }
                if (continuation.isActive) continuation.resume(result)
            }
        })
    }
}
