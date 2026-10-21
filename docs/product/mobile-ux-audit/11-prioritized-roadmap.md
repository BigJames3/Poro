# Feuille de route UX/UI priorisée

Cette priorisation concerne **l’application Android et sa cohérence avec les API existantes**. Elle ne remplace pas le backlog technique `docs/audits/BACKLOG-PORO.md` (PORO-xxx).

Références produit : ADR-0004 (social d’abord, marketplace P1) ; app réelle (onglet Boutique déjà présent).

## Méthode

| Niveau | Définition utilisée ici |
|---|---|
| **P0** | Sans cela, un lancement public des parcours **déjà retenus et déjà supportés par l’API** est dangereux, illégal vis-à-vis des stores, ou trompeur. |
| **P1** | Améliore nettement l’usage ou aligne l’app sur une API déjà là, sans bloquer un premier cercle de test. |
| **P2** | Extension. Y compris tout ce dont le **backend n’existe pas**. |

La marketplace n’est pas classée P0 **par automatisme**. Elle l’est seulement **si** l’onglet Boutique reste visible au lancement (Q-01). Deux scénarios sont donc indiqués.

---

## Scénario A — Lancement social (ADR-0004 strict)

Onglet Boutique **masqué**. Checkout/vendeur hors navigation.

### P0

| Item | Justification | Dépendances | Preuve d’état |
|---|---|---|---|
| Parcours auth OTP + email + reset + sessions | Sans compte, l’app actuelle ne s’ouvre pas | SMS/SMTP réels hors scope app | Implémenté |
| Feed 3 onglets + lecture HLS | Cœur produit | Vidéos `ready` | Implémenté |
| Like, commentaire, follow, share | Contrat social P0 backend | — | Implémenté (edit comment non) |
| Publication picker | Sans UGC, feed vide | — | Implémenté, fragile process |
| Recherche | Découverte | — | Implémenté |
| Profil + username + avatar | Identité publique | — | Implémenté |
| Notifications in-app | Boucle sociale | — | Implémenté |
| Suppression de compte | Exigence stores | AUTH-008 | Implémenté |
| **UI-MOD-001 Signaler** | Stores + harcèlement ; API prête | MOD-001 | **UI absente** |
| **UI-LEG-001 CGU / confidentialité** | Stores + lois 4 pays | Textes juridiques | **Absent** |
| Alignement limites bio/nom | Évite des échecs inexpliqués | USER | Écart code |
| Messages d’erreur déjà mappés | Ne pas régresser | — | Implémenté |

Hors P0 mobile : Wave, chat, FCM, Enterprise, admin.

### P1 (scénario A)

| Item | Pourquoi |
|---|---|
| WorkManager upload | Perte d’envoi = abandon créateur |
| FCM + POST_NOTIFICATIONS + `devices` | Préférences push aujourd’hui mensongères |
| Mute + échec HLS visible | Lecture utilisable en public / 3G |
| UI-SOC-004 listes follow | API prête ; graphe social illisible |
| Édition commentaire 15 min | API prête |
| SHARE_BASE_URL + App Links `/v/{id}` | Partage stérile |
| Design tokens (error, overlay, chip non cliquable) | Charte provisoire |
| Accessibilité TabRow / contraste | Inclusion |
| Réafficher Boutique (parcours COD complets) | P1 produit ADR-0004 |

### P2 (scénario A)

Grille profil public **si** API liste publique ; recherche boutique PORO-026 ; favoris ; block ; GPS ; Wave (PORO-015) ; GeniusPay ; chat ; stats ; live ; `values-en` ; data saver ; caméra in-app.

---

## Scénario B — Lancement avec onglet Boutique (état actuel de l’APK)

Tout le P0 du scénario A, **plus** les parcours COD déjà codés, parce qu’ils sont découvrables :

| Item | Justification |
|---|---|
| Trouver boutique par handle, fiche, panier, checkout COD, commandes acheteur/vendeur, ma boutique, éditeur produit | Déjà dans `TopLevelTab` ; un onglet cassé ou « bientôt » serait pire qu’un masquage |
| Copies COD honnêtes (pas « payé ») | Risque de confiance / juridique |

Restent **P1** même en scénario B : FCM, WorkManager, deep links, report (P0 quand même), legal (P0), listes follow.

Restent **P2** : Wave, recherche catalogue, GPS.

---

## Ordre recommandé (après décision Q-01)

1. **Décision produit** : scénario A ou B ; visiteur (Q-02) ; textes légaux (Q-03).
2. **Correctifs de vérité** : chip créateur, limites 50/160, switch push (désactiver ou labelliser « bientôt » tant que pas de FCM), SHARE_BASE_URL.
3. **Trous P0** : report sheet, écran légal (contenu fourni par le métier).
4. **Durcissement P1 lecture/publication** : mute, erreur player, WorkManager.
5. **Push** : projet Firebase (équipe mobile) puis `devices`.
6. **Social P1** : followers, edit comment.
7. **Marketplace P1 ou déjà P0 selon Q-01** : ne pas commencer Wave avant `services/payment`.
8. **Charte** : tokens dans `Theme.kt` une fois la marque livrée.

Ne pas paralléliser un nouveau design system et une refonte de navigation.

## Dépendances bloquantes hors UX

| Bloquant | Owner | Impact UI |
|---|---|---|
| Compte Africa’s Talking + Sender ID | Exploitation | OTP réel |
| SMTP | Exploitation | Verify / reset réels |
| Firebase + `google-services.json` | Mobile | Push |
| URL HTTPS publique + site share | Exploitation | Deep links |
| Textes CGU / privacy | Juridique | UI-LEG-001 |
| Service payment | Backend PORO-015 | Interdit de maquetter un checkout Wave « branché » |
| Staging | PORO-010 | Tests AC hors local |

## Ce qui ne doit pas être priorisé comme P0 UX

- File modérateur dans l’app (JWT staff sur gateway ≠ produit grand public).
- Espace Enterprise.
- Chat, live, ads, reco ML.
- Deuxième navigation.
- WebView du site.

## Lien backlog technique

Les items PORO-013 (Android) et PORO-014 (CI) sont **partiellement dépassés** : l’app consomme déjà l’API et la CI assemble l’APK. Le reste de PORO-013 (Firebase, applicationId définitif — **déjà `com.poro.app`**, plus `com.example.myporo`) doit être recalé. `ANDROID-INTEGRATION.md` doit être mis à jour (PORO-032), pas recopié ici.
