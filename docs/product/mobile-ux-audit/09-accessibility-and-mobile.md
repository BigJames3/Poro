# Accessibilité, performance mobile, connectivité, localisation

## 1. Plateforme

| Paramètre | Valeur |
|---|---|
| minSdk | 24 (Android 7.0) |
| targetSdk / compileSdk | 37 |
| Orientation | Non verrouillée dans le manifeste (défaut). Checkout utilise `SavedStateHandle`. **À confirmer** le fil en paysage (crop ZOOM). |
| Barres système | `enableEdgeToEdge()` ; `windowSoftInputMode=adjustResize` |
| RTL | `supportsRtl=true` ; icône retour `AutoMirrored` |
| Sauvegarde | `allowBackup=false` + exclusion extraction (jetons) |

Pas de permission CAMERA, RECORD_AUDIO, READ_MEDIA_*, ACCESS_FINE_LOCATION, POST_NOTIFICATIONS.

## 2. Accessibilité (constat + gaps)

**Présent**

- Titres `semantics { heading() }`.
- Erreurs `LiveRegionMode.Polite`.
- Spinner `contentDescription = Chargement…`.
- Retour nommé.
- OTP, pays, quantité, photos produit « x sur y », cloche pluriel, like/comment/share avec compteurs.
- Overlay vidéo : description titre + auteur ; `onClickLabel` play/pause.
- Compteurs follow fusionnés TalkBack.
- Autofill email / mot de passe / SMS.
- Cibles 48 dp fréquentes.

**Gaps à traiter (recommandations P1)**

| Gap | Risque | Correction visée |
|---|---|---|
| Chip Créateur cliquable no-op | Focus inutile | Non cliquable |
| Images décoratives `contentDescription=null` | OK si vraiment décoratif ; les miniatures de « mes vidéos » ont un label | Garder |
| Onglets feed = `TextButton` pas `TabRow` | Rôle tab imparfait | TabRow ou `Role.Tab` |
| Pas de `MuteButton` | Vidéo sonore forcée | Contrôle + état annoncé |
| Contraste overlay sur vidéo claire | Texte illisible | Scrim plus fort / revue appareil |
| `primaryContainer` avatar non tokenisé | Contraste initiale | Tokeniser |
| Mot de passe show/hide | État non annoncé comme IconToggle | `Role` + « masqué/affiché » |
| Pas de `prefers-reduced-motion` app | Pager OK | Éviter animations ajoutées |
| TalkBack sur pager vertical | Gestes conflictuels | Vérifier swipe vs défilement accessibilité **À confirmer sur appareil** |

Ordre de focus : haut → contenu → nav bar. Les sheets doivent piéger le focus le temps de leur ouverture (comportement M3).

## 3. Lecture vidéo (contraintes)

| Sujet | Comportement actuel | Règle à conserver / améliorer |
|---|---|---|
| Plein écran | Crop `RESIZE_MODE_ZOOM` sous la nav bar | Ne pas recouvrir les actions |
| Contrôles | Tap unique | Ajouter mute P1 ; pas de double-tap like non demandé |
| Pause | Tap + `ON_PAUSE` | Reprise sauf pause user |
| Progression | Absente | Optionnelle P2 ; ne pas imposer un chrome YouTube |
| Préchargement | Page settled seulement | Conserver (données / batterie) |
| Son | Volume système, pas de mute app | Mute P1 |
| Échec HLS | Pas d’état UI dédié | Message + retry P1 |
| Overlay | Dégradé bas | Contraste |
| Accidents | Nav bar toujours là | Ne pas placer like sous le pouce de navigation système sans padding |

Boucle `REPEAT_MODE_ONE` : prévenir TalkBack que la vidéo rejoue (**gap**).

## 4. Marketplace lisibilité

- Prix : `Formats.money` — XOF/XAF « 12 500 FCFA » (espaces insécables), NGN naira + kobo.
- Rupture : `product_out_of_stock`, articles panier `unavailable[]`.
- Vendeur : `product_sold_by`.
- Total checkout distinct du sous-panier ; **pas de frais de livraison dans le total** (notice livreur).
- Succès = commande **placée**, libellé `checkout_placed`, pas « payé ».
- Statuts commande : strings dédiées ; `awaiting_payment` / `paid` préparés mais non produits par le COD.

Erreur métier jamais remplacée par une animation de succès.

## 5. Réseau, données, batterie

| Contexte | Stratégie actuelle | Recommandation |
|---|---|---|
| Réseau lent | Timeouts OkHttp (valeurs **À confirmer** dans le client) | Retry manuel ; 429 respecté |
| Hors ligne | `error_network` | Pas de file hors-ligne sauf WorkManager upload (P1) |
| HLS | Direct SeaweedFS, pas de CDN (PORO-029) | Échec lecture fréquent en 3G **À confirmer** |
| Images | Coil 3 | Miniatures feed d’abord, player ensuite |
| Données | Une vidéo à la fois | Ne pas précharger tout le For You (200 ids serveur, pages 10) |
| Upload 256 Mo | Multipart 8 Mio, retry partie | Avertir Wi‑Fi **recommandation P1** (pas de code) |
| Batterie | Pause background | Conserver ; pas de location GPS |

Économie de données : aucun toggle « data saver » dans l’app.

## 6. Localisation et inclusion

| Sujet | Fait |
|---|---|
| Langue UI | Français uniquement |
| SMS | FR pour +225/+221/+237, EN pour +234 (serveur) |
| Dates | `Locale.FRANCE` `dd/MM/yyyy` ; relatif FR |
| Compteurs | virgule FR « 1,2 k » |
| Devise | FCFA / ₦ selon boutique, pas selon l’acheteur |
| Clavier | IME OTP, téléphone, email |
| Pays | 4 pays ; autres numéros refusés |
| Genre | « Abonné·e » dans `action_unfollow` |

Ne pas afficher l’anglais de `error.message`. Ne pas inventer une langue wolof/haoussa/pidgin : **non supportée**.

Formats NGN : 2 décimales max à la saisie vendeur. XOF : pas de décimales.

## 7. Permissions système — UX

| Permission | Quand | UI si refus |
|---|---|---|
| INTERNET | Install | Sans elle l’app est inutilisable (normale) |
| POST_NOTIFICATIONS | P1 après Firebase | Préférences in-app restent ; pas de harcèlement de dialogue |
| PHOTO_PICKER | Android 13+ sans permission | Picker système ; pas de fallback galerie full |
| CAMERA | Non demandée | Picker seulement |
| LOCATION | Non demandée | Checkout adresse texte ; ne pas bloquer |

Refus ≠ masquer l’action métier serveur.

## 8. États globaux à prévoir sur chaque écran chargé

Voir aussi [10-acceptance-criteria.md](10-acceptance-criteria.md). Minimum :

Initial → Chargement → Succès | Vide | Erreur métier | Hors ligne | Session expirée (redirection) | Rate limit | Contenu disparu (404).

Paiement (quand Wave existera) : en attente / confirmé **par le serveur** / refusé / annulé / inconnu (re-GET commande). **Aujourd’hui** : seulement « commande passée » COD.
