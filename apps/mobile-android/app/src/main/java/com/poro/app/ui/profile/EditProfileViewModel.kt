package com.poro.app.ui.profile

import androidx.annotation.StringRes
import androidx.lifecycle.SavedStateHandle
import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import com.poro.app.R
import com.poro.app.core.account.AccountRules
import com.poro.app.core.text.Runes
import com.poro.app.core.media.LocalMediaReader
import com.poro.app.core.network.ApiErrorCodes
import com.poro.app.core.network.ApiResult
import com.poro.app.data.user.ProfileDto
import com.poro.app.data.user.ProfileRepository
import com.poro.app.ui.auth.ErrorMessage
import com.poro.app.ui.auth.toErrorMessage
import java.io.IOException
import kotlinx.coroutines.Job
import kotlinx.coroutines.channels.Channel
import kotlinx.coroutines.delay
import kotlinx.coroutines.flow.Flow
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.flow.receiveAsFlow
import kotlinx.coroutines.flow.update
import kotlinx.coroutines.launch

/** État du pseudo saisi, vérifié auprès du serveur après une courte pause de frappe. */
enum class UsernameStatus(@StringRes val hint: Int?) {
    /** Pseudo actuel (ou aucun) : rien à vérifier. */
    Unchanged(null),
    Checking(R.string.username_checking),
    Available(R.string.username_available),
    Invalid(R.string.username_invalid),
    Reserved(R.string.username_reserved),
    Taken(R.string.username_taken),

    /** Vérification impossible (réseau) : l'enregistrement tranchera. */
    Unknown(null),
    ;

    val allowsSaving: Boolean get() = this == Unchanged || this == Available || this == Unknown
}

data class EditProfileUiState(
    val loading: Boolean = true,
    val username: String = "",
    val displayName: String = "",
    val bio: String = "",
    val avatarUrl: String? = null,
    val usernameStatus: UsernameStatus = UsernameStatus.Unchanged,
    val saving: Boolean = false,
    val uploadingAvatar: Boolean = false,
    val error: ErrorMessage? = null,
) {
    val canSave: Boolean
        get() = !loading && !saving && !uploadingAvatar && usernameStatus.allowsSaving &&
            AccountRules.displayNameLength(displayName) <= AccountRules.DISPLAY_NAME_MAX && AccountRules.isValidBio(bio)
}

sealed interface EditProfileEvent {
    data object Saved : EditProfileEvent
}

/** Les champs saisis survivent à la mort du processus ; l'avatar est envoyé dès qu'il est choisi. */
class EditProfileViewModel(
    private val profiles: ProfileRepository,
    private val media: LocalMediaReader,
    private val savedState: SavedStateHandle,
) : ViewModel() {

    private val _state = MutableStateFlow(EditProfileUiState())
    val state: StateFlow<EditProfileUiState> = _state.asStateFlow()

    private val _events = Channel<EditProfileEvent>(Channel.BUFFERED)
    val events: Flow<EditProfileEvent> = _events.receiveAsFlow()

    /** Pseudo enregistré côté serveur : le retaper ne déclenche aucune vérification. */
    private var savedUsername: String? = null
    private var availabilityCheck: Job? = null

    init {
        load()
    }

    fun load() {
        _state.update { it.copy(loading = true, error = null) }
        viewModelScope.launch {
            when (val result = profiles.me()) {
                is ApiResult.Success -> {
                    val profile = result.data
                    savedUsername = profile.username
                    _state.update {
                        it.copy(
                            loading = false,
                            username = savedState[KEY_USERNAME] ?: profile.username.orEmpty(),
                            displayName = savedState[KEY_DISPLAY_NAME] ?: profile.displayName.orEmpty(),
                            bio = savedState[KEY_BIO] ?: profile.bio.orEmpty(),
                            avatarUrl = profile.avatarUrl,
                        )
                    }
                    onUsernameChange(_state.value.username)
                }
                else -> _state.update { it.copy(loading = false, error = result.toErrorMessage()) }
            }
        }
    }

    fun onUsernameChange(raw: String) {
        val username = AccountRules.normalizeUsername(raw).take(AccountRules.USERNAME_MAX)
        savedState[KEY_USERNAME] = username
        availabilityCheck?.cancel()
        val status = when {
            username.isEmpty() && savedUsername == null -> UsernameStatus.Unchanged
            username == savedUsername -> UsernameStatus.Unchanged
            !AccountRules.isValidUsername(username) -> UsernameStatus.Invalid
            else -> UsernameStatus.Checking
        }
        _state.update { it.copy(username = username, usernameStatus = status, error = null) }
        if (status == UsernameStatus.Checking) {
            availabilityCheck = viewModelScope.launch {
                delay(CHECK_DELAY_MS)
                val result = profiles.usernameAvailability(username)
                val checked = when {
                    result !is ApiResult.Success -> UsernameStatus.Unknown
                    result.data.available -> UsernameStatus.Available
                    result.data.reason == "reserved" -> UsernameStatus.Reserved
                    result.data.reason == "invalid" -> UsernameStatus.Invalid
                    else -> UsernameStatus.Taken
                }
                _state.update { if (it.username == username) it.copy(usernameStatus = checked) else it }
            }
        }
    }

    fun onDisplayNameChange(value: String) {
        // Marge pour les espaces regroupés par le serveur ; canSave vérifie la longueur réelle.
        val name = Runes.take(value, AccountRules.DISPLAY_NAME_MAX * 2)
        savedState[KEY_DISPLAY_NAME] = name
        _state.update { it.copy(displayName = name, error = null) }
    }

    fun onBioChange(value: String) {
        val bio = Runes.take(value, AccountRules.BIO_MAX * 2)
        savedState[KEY_BIO] = bio
        _state.update { it.copy(bio = bio, error = null) }
    }

    fun save() {
        val current = _state.value
        if (!current.canSave) return
        _state.update { it.copy(saving = true, error = null) }
        viewModelScope.launch {
            // Un pseudo vide ne s'efface pas : on n'envoie le champ que s'il est rempli.
            val result = profiles.update(current.username.ifEmpty { null }, current.displayName, current.bio)
            when (result) {
                is ApiResult.Success -> {
                    applyProfile(result.data)
                    _state.update { it.copy(saving = false) }
                    _events.send(EditProfileEvent.Saved)
                }
                else -> {
                    val code = (result as? ApiResult.Failure)?.code
                    _state.update {
                        it.copy(
                            saving = false,
                            error = result.toErrorMessage(),
                            usernameStatus = when (code) {
                                ApiErrorCodes.USERNAME_TAKEN -> UsernameStatus.Taken
                                ApiErrorCodes.USERNAME_RESERVED -> UsernameStatus.Reserved
                                ApiErrorCodes.USERNAME_INVALID -> UsernameStatus.Invalid
                                else -> it.usernameStatus
                            },
                        )
                    }
                }
            }
        }
    }

    /** Photo choisie dans le sélecteur : contrôlée ici (type, 5 Mio) pour éviter un envoi voué à l'échec. */
    fun onAvatarPicked(uri: String) {
        if (_state.value.uploadingAvatar) return
        _state.update { it.copy(uploadingAvatar = true, error = null) }
        viewModelScope.launch {
            val local = media.describe(uri)
            val error = when {
                local == null || local.mimeType !in AVATAR_TYPES -> ErrorMessage(R.string.error_avatar_invalid)
                local.sizeBytes > AVATAR_MAX_BYTES -> ErrorMessage(R.string.error_avatar_too_large)
                else -> null
            }
            if (error != null || local == null) {
                _state.update { it.copy(uploadingAvatar = false, error = error) }
                return@launch
            }
            val bytes = try {
                media.read(uri, 0, local.sizeBytes.toInt())
            } catch (e: IOException) {
                _state.update { it.copy(uploadingAvatar = false, error = ErrorMessage(R.string.error_avatar_invalid)) }
                return@launch
            }
            val result = profiles.uploadAvatar(bytes, local.mimeType)
            if (result is ApiResult.Success) applyProfile(result.data)
            _state.update { it.copy(uploadingAvatar = false, error = result.toErrorMessage()) }
        }
    }

    fun removeAvatar() {
        if (_state.value.uploadingAvatar || _state.value.avatarUrl == null) return
        _state.update { it.copy(uploadingAvatar = true, error = null) }
        viewModelScope.launch {
            val result = profiles.removeAvatar()
            if (result is ApiResult.Success) applyProfile(result.data)
            _state.update { it.copy(uploadingAvatar = false, error = result.toErrorMessage()) }
        }
    }

    private fun applyProfile(profile: ProfileDto) {
        savedUsername = profile.username
        _state.update { it.copy(avatarUrl = profile.avatarUrl) }
        if (_state.value.username == profile.username) {
            _state.update { it.copy(usernameStatus = UsernameStatus.Unchanged) }
        }
    }

    private companion object {
        const val KEY_USERNAME = "username"
        const val KEY_DISPLAY_NAME = "display_name"
        const val KEY_BIO = "bio"
        const val CHECK_DELAY_MS = 400L
        const val AVATAR_MAX_BYTES = 5L * 1024 * 1024
        val AVATAR_TYPES = setOf("image/jpeg", "image/png", "image/webp")
    }
}
