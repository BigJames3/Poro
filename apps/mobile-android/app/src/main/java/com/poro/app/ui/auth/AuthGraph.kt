package com.poro.app.ui.auth

import androidx.compose.runtime.getValue
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import androidx.lifecycle.createSavedStateHandle
import androidx.navigation.NavGraphBuilder
import androidx.navigation.compose.composable
import androidx.navigation.toRoute
import com.poro.app.R
import com.poro.app.ui.AppNav
import com.poro.app.ui.CollectEvents
import com.poro.app.ui.EmailLoginRoute
import com.poro.app.ui.EmailRegisterRoute
import com.poro.app.ui.EmailVerifyRoute
import com.poro.app.ui.ForgotPasswordRoute
import com.poro.app.ui.OtpRoute
import com.poro.app.ui.PhoneRoute
import com.poro.app.ui.RESULT_PROFILE_CHANGED
import com.poro.app.ui.ResetPasswordRoute
import com.poro.app.ui.auth.email.EmailLoginEvent
import com.poro.app.ui.auth.email.EmailLoginScreen
import com.poro.app.ui.auth.email.EmailLoginViewModel
import com.poro.app.ui.auth.email.EmailRegisterEvent
import com.poro.app.ui.auth.email.EmailRegisterScreen
import com.poro.app.ui.auth.email.EmailRegisterViewModel
import com.poro.app.ui.auth.email.EmailVerifyEvent
import com.poro.app.ui.auth.email.EmailVerifyScreen
import com.poro.app.ui.auth.email.EmailVerifyViewModel
import com.poro.app.ui.auth.email.ForgotPasswordEvent
import com.poro.app.ui.auth.email.ForgotPasswordScreen
import com.poro.app.ui.auth.email.ForgotPasswordViewModel
import com.poro.app.ui.auth.email.ResetPasswordEvent
import com.poro.app.ui.auth.email.ResetPasswordScreen
import com.poro.app.ui.auth.email.ResetPasswordViewModel
import com.poro.app.ui.auth.otp.OtpEvent
import com.poro.app.ui.auth.otp.OtpVerifyScreen
import com.poro.app.ui.auth.otp.OtpVerifyViewModel
import com.poro.app.ui.auth.phone.PhoneEntryEvent
import com.poro.app.ui.auth.phone.PhoneEntryScreen
import com.poro.app.ui.auth.phone.PhoneEntryViewModel
import com.poro.app.ui.poroViewModel

/** Connexion par téléphone et par email, puis vérification de l'adresse après l'inscription. */
fun NavGraphBuilder.authGraph(nav: AppNav) {
    val container = nav.container

    composable<PhoneRoute> {
        val vm = poroViewModel { PhoneEntryViewModel(container.authRepository, createSavedStateHandle(), container.clock) }
        val state by vm.state.collectAsStateWithLifecycle()
        CollectEvents(vm.events) { event ->
            when (event) {
                is PhoneEntryEvent.CodeSent -> nav.controller.navigate(OtpRoute(event.phone, event.expiresInSeconds))
            }
        }
        PhoneEntryScreen(
            state = state,
            onCountryChange = vm::onCountryChange,
            onInputChange = vm::onInputChange,
            onSubmit = vm::submit,
            onUseEmail = { nav.controller.navigate(EmailLoginRoute) },
        )
    }
    composable<OtpRoute> { entry ->
        val route = entry.toRoute<OtpRoute>()
        val vm = poroViewModel {
            OtpVerifyViewModel(route.phone, route.expiresInSeconds, container.authRepository, createSavedStateHandle(), container.clock)
        }
        val state by vm.state.collectAsStateWithLifecycle()
        CollectEvents(vm.events) { event ->
            when (event) {
                is OtpEvent.SignedIn -> {
                    nav.goHome()
                    if (event.deletionCancelled) nav.showMessage(R.string.home_deletion_cancelled)
                }
            }
        }
        OtpVerifyScreen(
            state = state,
            onCodeChange = vm::onCodeChange,
            onVerify = vm::verify,
            onResend = vm::resend,
            onChangeNumber = nav::back,
        )
    }
    composable<EmailLoginRoute> {
        val vm = poroViewModel { EmailLoginViewModel(container.authRepository, createSavedStateHandle(), container.clock) }
        val state by vm.state.collectAsStateWithLifecycle()
        CollectEvents(vm.events) { event ->
            when (event) {
                is EmailLoginEvent.SignedIn -> {
                    nav.goHome()
                    if (event.deletionCancelled) nav.showMessage(R.string.home_deletion_cancelled)
                }
            }
        }
        EmailLoginScreen(
            state = state,
            onEmailChange = vm::onEmailChange,
            onPasswordChange = vm::onPasswordChange,
            onSubmit = vm::submit,
            onForgotPassword = { nav.controller.navigate(ForgotPasswordRoute(state.email)) },
            onCreateAccount = { nav.controller.navigate(EmailRegisterRoute) },
            onUsePhone = {
                if (!nav.controller.popBackStack<PhoneRoute>(inclusive = false)) nav.controller.navigate(PhoneRoute)
            },
        )
    }
    composable<EmailRegisterRoute> {
        val vm = poroViewModel { EmailRegisterViewModel(container.authRepository, createSavedStateHandle(), container.clock) }
        val state by vm.state.collectAsStateWithLifecycle()
        CollectEvents(vm.events) { event ->
            when (event) {
                // Session ouverte : les écrans de connexion quittent la pile.
                EmailRegisterEvent.Registered ->
                    nav.controller.navigate(EmailVerifyRoute(codeAlreadySent = true)) {
                        popUpTo(nav.controller.graph.id) { inclusive = true }
                    }
            }
        }
        EmailRegisterScreen(
            state = state,
            onFullNameChange = vm::onFullNameChange,
            onEmailChange = vm::onEmailChange,
            onPasswordChange = vm::onPasswordChange,
            onSubmit = vm::submit,
            onHaveAccount = {
                if (!nav.controller.popBackStack<EmailLoginRoute>(inclusive = false)) nav.controller.navigate(EmailLoginRoute)
            },
        )
    }
    composable<EmailVerifyRoute> { entry ->
        val route = entry.toRoute<EmailVerifyRoute>()
        val vm = poroViewModel {
            EmailVerifyViewModel(route.codeAlreadySent, container.authRepository, createSavedStateHandle(), container.clock)
        }
        val state by vm.state.collectAsStateWithLifecycle()
        // Après l'inscription, l'écran est seul dans la pile : la suite est l'accueil. Depuis le profil, on y revient.
        val leave = { if (route.codeAlreadySent) nav.goHome() else nav.backWithResult(RESULT_PROFILE_CHANGED) }
        CollectEvents(vm.events) { event ->
            when (event) {
                EmailVerifyEvent.Verified -> {
                    leave()
                    nav.showMessage(R.string.email_verified)
                }
            }
        }
        EmailVerifyScreen(
            state = state,
            onCodeChange = vm::onCodeChange,
            onVerify = vm::verify,
            onResend = vm::resend,
            onLater = leave,
        )
    }
    composable<ForgotPasswordRoute> { entry ->
        val route = entry.toRoute<ForgotPasswordRoute>()
        val vm = poroViewModel {
            ForgotPasswordViewModel(route.email, container.authRepository, createSavedStateHandle(), container.clock)
        }
        val state by vm.state.collectAsStateWithLifecycle()
        CollectEvents(vm.events) { event ->
            when (event) {
                is ForgotPasswordEvent.CodeSent -> nav.controller.navigate(ResetPasswordRoute(event.email))
            }
        }
        ForgotPasswordScreen(state = state, onEmailChange = vm::onEmailChange, onSubmit = vm::submit)
    }
    composable<ResetPasswordRoute> { entry ->
        val route = entry.toRoute<ResetPasswordRoute>()
        val vm = poroViewModel {
            ResetPasswordViewModel(route.email, container.authRepository, createSavedStateHandle(), container.clock)
        }
        val state by vm.state.collectAsStateWithLifecycle()
        CollectEvents(vm.events) { event ->
            when (event) {
                // Retour à la connexion par email (adresse conservée) avec le nouveau mot de passe.
                ResetPasswordEvent.PasswordChanged -> {
                    if (!nav.controller.popBackStack<EmailLoginRoute>(inclusive = false)) {
                        nav.controller.navigate(EmailLoginRoute) { popUpTo(nav.controller.graph.id) { inclusive = true } }
                    }
                    nav.showMessage(R.string.password_changed)
                }
            }
        }
        ResetPasswordScreen(
            state = state,
            onCodeChange = vm::onCodeChange,
            onPasswordChange = vm::onPasswordChange,
            onSubmit = vm::submit,
            onResend = vm::resend,
        )
    }
}
