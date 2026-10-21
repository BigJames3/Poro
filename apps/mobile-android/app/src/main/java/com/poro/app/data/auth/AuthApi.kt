package com.poro.app.data.auth

import com.poro.app.core.network.ApiEnvelope
import kotlinx.serialization.SerialName
import kotlinx.serialization.Serializable
import retrofit2.Call
import retrofit2.Response
import retrofit2.http.Body
import retrofit2.http.GET
import retrofit2.http.POST

/** Routes du service Auth (docs/api/auth.openapi.yaml). */
interface AuthApi {
    @POST("api/v1/auth/otp/request")
    suspend fun requestOtp(@Body body: OtpRequest): Response<ApiEnvelope<CodeSent>>

    @POST("api/v1/auth/otp/verify")
    suspend fun verifyOtp(@Body body: OtpVerify): Response<ApiEnvelope<SignedIn>>

    @POST("api/v1/auth/logout")
    suspend fun logout(): Response<ApiEnvelope<Unit>>

    @GET("api/v1/auth/me")
    suspend fun me(): Response<ApiEnvelope<UserDto>>

    @POST("api/v1/auth/email/register")
    suspend fun emailRegister(@Body body: EmailRegister): Response<ApiEnvelope<SignedIn>>

    @POST("api/v1/auth/email/login")
    suspend fun emailLogin(@Body body: EmailLogin): Response<ApiEnvelope<SignedIn>>

    /** Bearer : code de vérification envoyé à l'adresse du compte connecté. */
    @POST("api/v1/auth/email/verification/request")
    suspend fun requestEmailVerification(): Response<ApiEnvelope<CodeSent>>

    @POST("api/v1/auth/email/verification/confirm")
    suspend fun confirmEmailVerification(@Body body: CodeConfirm): Response<ApiEnvelope<UserDto>>

    /** Toujours 202, que l'adresse ait un compte ou non. */
    @POST("api/v1/auth/email/password/forgot")
    suspend fun forgotPassword(@Body body: PasswordForgot): Response<ApiEnvelope<CodeSent>>

    /** 204 sans corps ; déconnecte tous les appareils du compte. */
    @POST("api/v1/auth/email/password/reset")
    suspend fun resetPassword(@Body body: PasswordReset): Response<ApiEnvelope<Unit>>
}

/**
 * Rafraîchissement, appelé de façon synchrone par [com.poro.app.core.network.TokenAuthenticator]
 * avec un client HTTP sans authenticator (pas de récursion).
 */
interface RefreshApi {
    @POST("api/v1/auth/refresh")
    fun refresh(@Body body: RefreshRequest): Call<ApiEnvelope<TokensDto>>
}

@Serializable
data class OtpRequest(val phone: String)

@Serializable
data class CodeSent(@SerialName("expires_in") val expiresIn: Int = 0)

@Serializable
data class OtpVerify(
    val phone: String,
    val code: String,
    /** Affiché dans la liste des sessions (60 caractères au plus). */
    @SerialName("device_name") val deviceName: String? = null,
)

@Serializable
data class EmailRegister(
    val email: String,
    val password: String,
    @SerialName("full_name") val fullName: String,
    @SerialName("device_name") val deviceName: String? = null,
)

@Serializable
data class EmailLogin(
    val email: String,
    val password: String,
    @SerialName("device_name") val deviceName: String? = null,
)

@Serializable
data class CodeConfirm(val code: String)

@Serializable
data class PasswordForgot(val email: String)

@Serializable
data class PasswordReset(
    val email: String,
    val code: String,
    @SerialName("new_password") val newPassword: String,
)

@Serializable
data class RefreshRequest(@SerialName("refresh_token") val refreshToken: String)

@Serializable
data class TokensDto(
    @SerialName("access_token") val accessToken: String,
    @SerialName("refresh_token") val refreshToken: String,
    @SerialName("expires_in") val expiresIn: Int = 0,
    @SerialName("token_type") val tokenType: String = "Bearer",
)

@Serializable
data class SignedIn(
    @SerialName("access_token") val accessToken: String,
    @SerialName("refresh_token") val refreshToken: String,
    @SerialName("expires_in") val expiresIn: Int = 0,
    val user: UserDto? = null,
    /** Vrai quand cette connexion a annulé une suppression de compte programmée : à afficher. */
    @SerialName("deletion_cancelled") val deletionCancelled: Boolean = false,
)

@Serializable
data class UserDto(
    val id: String,
    val phone: String? = null,
    val email: String? = null,
    val roles: List<String> = emptyList(),
    val status: String? = null,
    @SerialName("country_code") val countryCode: String? = null,
    val language: String? = null,
    @SerialName("email_verified") val emailVerified: Boolean = false,
    @SerialName("deletion_scheduled_at") val deletionScheduledAt: String? = null,
)
