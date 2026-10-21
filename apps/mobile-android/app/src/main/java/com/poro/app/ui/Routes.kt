package com.poro.app.ui

import kotlinx.serialization.Serializable

// Destinations de l'application (navigation typée). Les onglets principaux sont dans TopLevelTab.

// Connexion (utilisables sans session).
@Serializable
data object PhoneRoute

/** [phone] au format E.164 ; [expiresInSeconds] : durée de vie du code envoyé. */
@Serializable
data class OtpRoute(val phone: String, val expiresInSeconds: Int)

@Serializable
data object EmailLoginRoute

@Serializable
data object EmailRegisterRoute

/** [email] : adresse déjà saisie sur l'écran de connexion, reprise pour éviter de la retaper. */
@Serializable
data class ForgotPasswordRoute(val email: String = "")

@Serializable
data class ResetPasswordRoute(val email: String)

// Session ouverte.

/** Onglet d'accueil : le fil de vidéos. */
@Serializable
data object HomeRoute

/**
 * Vérification de l'adresse email : juste après l'inscription ([codeAlreadySent]), ou depuis le
 * profil (un code est alors demandé à l'ouverture). On peut la remettre à plus tard.
 */
@Serializable
data class EmailVerifyRoute(val codeAlreadySent: Boolean = true)

/** Onglet « Rechercher ». */
@Serializable
data object SearchRoute

@Serializable
data object NotificationsRoute

@Serializable
data object NotificationPreferencesRoute

/** Onglet « Publier ». */
@Serializable
data object UploadRoute

/** Une vidéo ouverte seule (recherche, notification, profil). */
@Serializable
data class VideoRoute(val videoId: String)

/** Profil d'un autre compte ; [username] null si l'auteur n'en a pas encore (profil réduit). */
@Serializable
data class PublicProfileRoute(val userId: String, val username: String? = null)

@Serializable
data object ProfileRoute

@Serializable
data object EditProfileRoute

@Serializable
data object SettingsRoute

@Serializable
data object DeleteAccountRoute

/** Onglet « Boutique » : panier, commandes, recherche d'une boutique par identifiant, vente. */
@Serializable
data object ShopHomeRoute

/** Vitrine d'une boutique, par son identifiant « @… ». */
@Serializable
data class ShopRoute(val handle: String)

@Serializable
data class ProductRoute(val productId: String)

@Serializable
data object CartRoute

@Serializable
data object CheckoutRoute

/** Ses achats, ou les commandes reçues par sa boutique ([asSeller]). */
@Serializable
data class OrdersRoute(val asSeller: Boolean = false)

@Serializable
data class OrderRoute(val orderId: String, val asSeller: Boolean = false)

@Serializable
data object MyShopRoute

/** Création ([productId] null) ou modification d'un produit ; [currency] : celle de la boutique. */
@Serializable
data class ProductEditorRoute(val productId: String? = null, val currency: String)
