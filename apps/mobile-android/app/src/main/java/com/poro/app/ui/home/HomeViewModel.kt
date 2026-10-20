package com.poro.app.ui.home

import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import com.poro.app.data.auth.AuthRepository
import java.io.IOException
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.launch
import kotlinx.coroutines.withContext
import okhttp3.HttpUrl
import okhttp3.OkHttpClient
import okhttp3.Request

/** État de la connexion à l'API, affiché sur l'écran d'accueil provisoire. */
sealed interface ApiStatus {
    data object Checking : ApiStatus
    data object Reachable : ApiStatus
    data class Unreachable(val reason: String) : ApiStatus
}

class HomeViewModel(
    private val client: OkHttpClient,
    private val healthUrl: HttpUrl,
    private val auth: AuthRepository,
) : ViewModel() {

    private val _status = MutableStateFlow<ApiStatus>(ApiStatus.Checking)
    val status: StateFlow<ApiStatus> = _status.asStateFlow()

    private val _signingOut = MutableStateFlow(false)
    val signingOut: StateFlow<Boolean> = _signingOut.asStateFlow()

    init {
        check()
    }

    fun check() {
        _status.value = ApiStatus.Checking
        viewModelScope.launch {
            _status.value = withContext(Dispatchers.IO) {
                try {
                    client.newCall(Request.Builder().url(healthUrl).get().build()).execute().use { response ->
                        if (response.isSuccessful) ApiStatus.Reachable else ApiStatus.Unreachable("HTTP ${response.code}")
                    }
                } catch (e: IOException) {
                    ApiStatus.Unreachable(e.message ?: e.javaClass.simpleName)
                }
            }
        }
    }

    /** La navigation revient à la connexion dès que la session passe à SignedOut. */
    fun logout() {
        if (_signingOut.value) return
        _signingOut.value = true
        viewModelScope.launch { auth.logout() }
    }
}
