package com.poro.app.ui.profile

import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import androidx.lifecycle.createSavedStateHandle
import androidx.navigation.NavGraphBuilder
import androidx.navigation.compose.composable
import com.poro.app.R
import com.poro.app.core.format.Formats
import com.poro.app.ui.AddressesRoute
import com.poro.app.ui.AppNav
import com.poro.app.ui.CollectEvents
import com.poro.app.ui.ConsumeResult
import com.poro.app.ui.DeleteAccountRoute
import com.poro.app.ui.EditProfileRoute
import com.poro.app.ui.EmailVerifyRoute
import com.poro.app.ui.NotificationPreferencesRoute
import com.poro.app.ui.ProfileRoute
import com.poro.app.ui.RESULT_PROFILE_CHANGED
import com.poro.app.ui.LegalDocumentRoute
import com.poro.app.ui.SettingsRoute
import com.poro.app.ui.poroViewModel

/**
 * Profil, modification, paramètres et suppression du compte. [profileExtra] : contenu d'autres
 * fonctionnalités affiché sur le profil (compteurs, vidéos publiées).
 */
fun NavGraphBuilder.profileGraph(nav: AppNav, profileExtra: @Composable (MyAccount) -> Unit = {}) {
    val container = nav.container

    composable<ProfileRoute> { entry ->
        val vm = poroViewModel { ProfileViewModel(container.profileRepository, container.accountRepository) }
        val state by vm.state.collectAsStateWithLifecycle()
        // Retour de la modification ou de la vérification : le profil affiché doit suivre.
        ConsumeResult(entry, RESULT_PROFILE_CHANGED, vm::refresh)
        ProfileScreen(
            state = state,
            onRetry = vm::refresh,
            onEdit = { nav.controller.navigate(EditProfileRoute) },
            onSettings = { nav.controller.navigate(SettingsRoute) },
            onBecomeCreator = vm::becomeCreator,
            onVerifyEmail = { nav.controller.navigate(EmailVerifyRoute(codeAlreadySent = false)) },
            extra = profileExtra,
        )
    }
    composable<EditProfileRoute> {
        val vm = poroViewModel { EditProfileViewModel(container.profileRepository, container.mediaReader, createSavedStateHandle()) }
        val state by vm.state.collectAsStateWithLifecycle()
        CollectEvents(vm.events) { event ->
            when (event) {
                EditProfileEvent.Saved -> {
                    nav.backWithResult(RESULT_PROFILE_CHANGED)
                    nav.showMessage(R.string.profile_saved)
                }
            }
        }
        EditProfileScreen(
            state = state,
            // L'avatar est enregistré dès son envoi : le profil doit le montrer même sans « Enregistrer ».
            onBack = { nav.backWithResult(RESULT_PROFILE_CHANGED) },
            onUsernameChange = vm::onUsernameChange,
            onDisplayNameChange = vm::onDisplayNameChange,
            onBioChange = vm::onBioChange,
            onAvatarPicked = vm::onAvatarPicked,
            onRemoveAvatar = vm::removeAvatar,
            onSave = vm::save,
        )
    }
    composable<SettingsRoute> {
        val vm = poroViewModel { SettingsViewModel(container.accountRepository, container.authRepository) }
        val state by vm.state.collectAsStateWithLifecycle()
        SettingsScreen(
            state = state,
            onBack = nav::back,
            onRetrySessions = vm::loadSessions,
            onRevoke = vm::revoke,
            onRevokeOthers = vm::revokeOthers,
            onNotifications = { nav.controller.navigate(NotificationPreferencesRoute) },
            onAddresses = { nav.controller.navigate(AddressesRoute) },
            onLegal = { id -> nav.controller.navigate(LegalDocumentRoute(id)) },
            onDeleteAccount = { nav.controller.navigate(DeleteAccountRoute) },
            onLogout = vm::logout,
        )
    }
    composable<DeleteAccountRoute> {
        val vm = poroViewModel {
            DeleteAccountViewModel(container.accountRepository, container.authRepository, createSavedStateHandle(), container.clock)
        }
        val state by vm.state.collectAsStateWithLifecycle()
        CollectEvents(vm.events) { event ->
            when (event) {
                // La session locale est déjà effacée : la navigation suit (retour à la connexion).
                is DeleteAccountEvent.Scheduled -> nav.showMessage(R.string.delete_scheduled, Formats.date(event.deletionScheduledAt))
            }
        }
        DeleteAccountScreen(
            state = state,
            onBack = nav::back,
            onRetryLoad = vm::loadAccount,
            onPasswordChange = vm::onPasswordChange,
            onCodeChange = vm::onCodeChange,
            onSendCode = vm::sendCode,
            onAcknowledgedChange = vm::onAcknowledgedChange,
            onDelete = vm::delete,
        )
    }
}
