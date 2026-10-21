package com.poro.app.ui.social

import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import com.poro.app.core.network.ApiResult
import com.poro.app.data.social.SocialRepository
import com.poro.app.data.social.UserStatsDto
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.launch

/** Compteurs d'abonnés et d'abonnements, affichés sur son propre profil. */
class UserStatsViewModel(private val userId: String, private val social: SocialRepository) : ViewModel() {

    private val _stats = MutableStateFlow<UserStatsDto?>(null)
    val stats: StateFlow<UserStatsDto?> = _stats.asStateFlow()

    init {
        viewModelScope.launch { _stats.value = (social.userStats(userId) as? ApiResult.Success)?.data }
    }
}
