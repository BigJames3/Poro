package com.poro.app.ui.feed

import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import com.poro.app.data.video.VideoDto
import com.poro.app.data.video.VideoRepository
import com.poro.app.ui.common.Loadable
import com.poro.app.ui.common.toLoadable
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.launch

/** Une vidéo ouverte seule (recherche, notification, profil). */
class VideoViewModel(
    private val videoId: String,
    private val videos: VideoRepository,
) : ViewModel() {

    private val _video = MutableStateFlow<Loadable<VideoDto>>(Loadable.Loading)
    val video: StateFlow<Loadable<VideoDto>> = _video.asStateFlow()

    init {
        load()
    }

    fun load() {
        _video.value = Loadable.Loading
        viewModelScope.launch { _video.value = videos.video(videoId).toLoadable() }
    }
}
