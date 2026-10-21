# Poro — Spécification du fil vidéo

Le fil est le produit. Le redesign doit le rendre **lisible sur tout contenu** et **actionnable au pouce**, sans ajouter de chrome commercial.

Fichiers actuels : `FeedScreen.kt`, `VideoActions.kt`, `CommentsSheet.kt`, `ShareSheet.kt`, `VideoPlayer.kt`.

## 1. Objectif

Permettre à un membre connecté de regarder Pour toi, Abonnements et Tendances, d’interagir (like, commenter, partager, s’abonner via le profil), et plus tard de signaler.

Invité : **non activable** — `GET /feed/*` Bearer obligatoire.

## 2. Structure visuelle (une page)

```
┌─────────────────────────────────────┐
│ ▓ scrim 40 %                        │
│   Pour toi   Abonnements  Tendances │  🔔
│                                     │
│                                     │
│              ▶  (si pause)          │
│                                     │
│                          ♥  12,4 k  │
│                          💬  830    │
│                          ↗  120     │
│                          ⋮          │
│ ▓▓▓ scrim 0 → 72 %                  │
│ [avatar] Nom                        │
│ Titre deux lignes maximum           │
│ #tag #tag                           │
└─────────────────────────────────────┘
        Accueil  Search  +  Shop  Profil
```

### Zones

| Zone | Contenu | Z |
|---|---|---|
| Vidéo | Miniature Coil + `VideoSurface` crop ZOOM | 0 |
| Scrim haut | Dégradé noir 40 % sur ~88 dp | 1 |
| Tabs + cloche | Blanc ; selected bold + underline 2 dp orange **Δ** | 2 |
| Play | Triangle 72 dp, blanc, **Δ** ombre/contraste (plus 0.8 seul) | 2 |
| Scrim bas | Transparent → noir 72 % sur ~200 dp | 1 |
| Méta | Avatar 48, nom, titre, hashtags | 2 |
| Actions | Colonne 48 dp, icônes 32, compteur `labelMedium` | 2 |
| Nav | M3, toujours visible sur Home | 3 |

## 3. Contrôles de lecture

| Contrôle | Aujourd’hui | Cible |
|---|---|---|
| Play / pause | Tap page | Conserver ; icône play seulement si pause |
| Mute | Absent | **P1** : bouton sous ⋮ ou au-dessus like, état annoncé |
| Seek | Absent | P2 optionnel, pas un chrome YouTube |
| Volume | Système | Conserver |
| Boucle | `REPEAT_MODE_ONE` | Conserver ; TalkBack : ne pas spammer |
| Préchargement | Page settled | Conserver (données) |
| Échec HLS | Pas d’UI | **P1** : texte + Réessayer sur la page, miniature reste |

Pas de double-tap like. Pas de long-press vitesse.

Le player reste noir même en thème clair.

## 4. Informations créateur

| Élément | Règle |
|---|---|
| Avatar | 48 dp, photo ou initiale `primaryContainer` Poro |
| Nom | `displayNameOf` → display / @username / « Membre Poro » |
| Zone tactile | Toute la rangée avatar+nom, min 48 dp |
| Tap | `PublicProfileRoute(userId, username)` |
| Badge créateur | **Pas** sur l’overlay (charge visuelle). Sur le profil seulement |

Ne pas afficher le pays, l’email, le téléphone (absents du DTO feed).

## 5. Actions

Ordre haut → bas (pouce droit) :

1. Like — `#FF3B5C` si `likedByMe`, sinon blanc.
2. Commentaires — `PoroIcons.Comment`.
3. Partage — `Icons.Filled.Share`.
4. **Δ** Menu ⋮ — Signaler (et seulement Signaler pour v1).

Compteurs : `Formats.count` (ex. `1,2 k`). 0 s’affiche « 0 » (déjà). TalkBack : description inclut le compteur, texte visuel `clearAndSetSemantics`.

Follow **n’est pas** sur l’overlay (pas d’API follow-from-feed distincte nécessaire ; le profil suffit). Ne pas ajouter un « + » sur l’avatar sans le contrat actuel (follow existe, mais l’overlay est déjà dense).

### Like

- Optimistic.
- `PUT /videos/{id}/like` / `DELETE`.
- Erreur : revert + snackbar.
- 429 : message, pas de countdown dédié aujourd’hui — **P1** aligner sur `SubmitButton`.

### Commentaires

Sheet 85 %. Composer bas. Réponses 1 niveau. Delete auteur ou owner vidéo. **Δ** ⋮ Signaler sur la ligne (sauf soi).

API : voir inventaire. Auteurs : tant que le DTO n’a que `user_id`, garder « Membre Poro » / « Vous ». Ne pas inventer un username.

### Partage

Sheet WhatsApp / Autres. `POST /shares`. URL seulement si `SHARE_BASE_URL` configuré.

### Signalement

`target_type=video`, `target_id=videoId`. Motifs enum. Spec : [phase-2/04](../mobile-ux-phase-2/04-reporting-ux-spec.md).

Masquer ⋮ sur **sa** vidéo n’est pas une autorisation : le serveur renvoie `cannot_report_self`. On peut quand même cacher l’entrée sur l’overlay de ses items feed (jamais ses vidéos en For You — déjà le contrat feed).

## 6. Onglets de fil

| Kind | Label | Empty | API |
|---|---|---|---|
| ForYou | Pour toi | `feed_empty` | `GET /feed/for-you` |
| Following | Abonnements | `feed_following_empty` | `GET /feed/following` |
| Trending | Tendances | `feed_empty` | `GET /feed/trending` |

**Δ** : `Role.Tab` / `TabRow` overlay, pas seulement `TextButton`. Selected = blanc + underline orange 2 dp × 16 dp. Unselected = blanc 70 %.

Re-tap kind courant = refresh. Changement de kind = reload ce fil. Pager position **par kind** (ViewModel).

`invalid_cursor` → recommencer sans curseur (déjà).

## 7. Cloche

Coin haut droit, blanc, badge unread. Tap → `NotificationsRoute`.

Badge : `secondary` / chiffre. 0 = pas de badge. Pluriel TalkBack déjà.

API : `GET /notifications/unread-count`.

## 8. États du fil

| État | UI |
|---|---|
| Premier chargement | Spinner blanc centre, tabs visibles |
| Vide Pour toi / Tendances | `EmptyState` texte blanc, pas d’illustration P0 |
| Vide Abonnements | Copy + **Δ** CTA « Rechercher des créateurs » → onglet Search |
| Erreur réseau | `ErrorMessageText` clair + `OutlinedButton` Réessayer (déjà, **Δ** bouton plus contrasté) |
| Page en cours | Miniature jusqu’au HLS |
| Pause | Play centre |
| HLS fail (P1) | Message + retry, tabs restent |
| Session 401 | Redirect Phone |

## 9. Vidéo isolée (`VideoRoute`)

Même overlay. Back blanc haut gauche dans le scrim. Entrées : search, notif, upload, mes vidéos.

| `status` | UI |
|---|---|
| ready + approved | Player |
| processing | Texte attente, pas de player |
| failed | Échec traitement |
| removed / 404 | Retirée / introuvable |

Pas d’actions like si médias absents.

## 10. Profil public lié au fil

Pas de grille (API absente). Header + follow. **Δ** ⋮ Signaler. Compteurs tappable → `FollowListRoute` en P1 seulement.

## 11. Accessibilité lecture

- Page : `contentDescription` titre + auteur (déjà).
- `onClickLabel` play/pause (déjà).
- Actions nommées avec compteurs (déjà).
- ⋮ : « Plus d’actions » / « Signaler cette vidéo ».
- Mute P1 : « Son activé / coupé ».
- Contraste texte overlay : viser 4,5:1 grâce au scrim 72 % — **vérifier sur appareil** avec vidéo blanche et vidéo noire.
- Touch : colonne actions ≥ 48 ; avatar row ≥ 48.

## 12. Performance

- Une vidéo décodée à la fois (déjà).
- Pas de Lottie like.
- Pas de préchargement des 200 ids For You.
- Scrim = `Brush` déjà ; un second Brush haut : coût négligeable vs un blur.

## 13. Petit écran / densité

| Largeur | Adaptation |
|---|---|
| < 360 dp | Tabs : ellipsis ou scroll horizontal ; actions inchangées |
| Police XXL | Titre 2 lignes ; tabs wrap → scroll |
| Paysage | Crop ZOOM déjà ; **À confirmer** si lock portrait souhaité — hors scope visuel |
| Gesture nav | Padding bas overlay = nav bar + 8 dp (déjà via Scaffold activité) |

## 14. API réelles

| Action | Endpoint | Auth |
|---|---|---|
| Charger | `GET /api/v1/feed/{for-you,following,trending}?cursor&limit` | Bearer |
| Stats | `GET /api/v1/videos/{id}/stats` | Opt |
| Like | `PUT/DELETE /api/v1/videos/{id}/like` | Bearer |
| Comments | `GET/POST /api/v1/videos/{id}/comments` | GET opt / POST Bearer |
| Share | `POST /api/v1/videos/{id}/shares` | Bearer |
| Report | `POST /api/v1/moderation/reports` | Bearer |
| Unread | `GET /api/v1/notifications/unread-count` | Bearer |

Page size actuelle : 10. Ne pas monter au-delà de 30 (contrat).

## 15. Critères d’acceptation fil

1. Les trois kinds restent et rechargent indépendamment.
2. Texte et cloche lisibles sur fond blanc **et** noir (revue appareil).
3. Like / comment / share inchangés fonctionnellement.
4. ⋮ ouvre le report, pas une page marketing.
5. Nav bar ne recouvre pas la dernière action.
6. Aucun overlay « acheter » ou produit.
7. Invité non implémenté tant que trending 401.
8. Aucune régression HLS (une page settled = un play).
