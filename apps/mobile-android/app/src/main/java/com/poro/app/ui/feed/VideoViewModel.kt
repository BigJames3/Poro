package com.poro.app.ui.feed

import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import com.poro.app.core.network.ApiResult
import com.poro.app.data.user.ProfileDto
import com.poro.app.data.user.ProfileRepository
import com.poro.app.data.video.VideoDto
import com.poro.app.data.video.VideoRepository
import com.poro.app.ui.common.Loadable
import com.poro.app.ui.common.toLoadable
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.launch

/**
 * Une vidéo ouverte seule (recherche, notification, profil). Son auteur (nom, avatar) est lu
 * ensuite : la vidéo s'affiche sans l'attendre, et un échec laisse le libellé par défaut.
 */
class VideoViewModel(
    private val videoId: String,
    private val videos: VideoRepository,
    private val profiles: ProfileRepository,
) : ViewModel() {

    private val _video = MutableStateFlow<Loadable<VideoDto>>(Loadable.Loading)
    val video: StateFlow<Loadable<VideoDto>> = _video.asStateFlow()

    private val _author = MutableStateFlow<ProfileDto?>(null)
    val author: StateFlow<ProfileDto?> = _author.asStateFlow()

    init {
        load()
    }

    fun load() {
        _video.value = Loadable.Loading
        viewModelScope.launch {
            val result = videos.video(videoId)
            _video.value = result.toLoadable()
            val ownerId = (result as? ApiResult.Success)?.data?.userId?.takeIf { it.isNotEmpty() } ?: return@launch
            if (_author.value?.userId == ownerId) return@launch
            _author.value = (profiles.profiles(listOf(ownerId)) as? ApiResult.Success)?.data?.firstOrNull()
        }
    }
}
