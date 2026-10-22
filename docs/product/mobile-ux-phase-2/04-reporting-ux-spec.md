# Axe C1 — Spécification du signalement

Contrat : `POST /api/v1/moderation/reports` (`docs/api/moderation.openapi.yaml`), vérifié dans
`services/moderation/src/moderation/api/schemas.py`, `routes.py` et `cases.py`.
L'application n'a aujourd'hui **aucun** parcours de signalement.

## 1. Contrat réel

| Élément | Valeur |
|---|---|
| Authentification | Jeton obligatoire |
| Corps | `target_type` ∈ {`video`, `comment`, `user`} (obligatoire), `target_id` UUID (obligatoire), `reason` (obligatoire), `comment` texte ≤ 500, facultatif, `null` ou vide → non enregistré |
| Motifs (`reason`) | `spam`, `nudity`, `violence`, `harassment`, `hate`, `fraud`, `other` — aucun autre |
| `other` | Le détail **n'est pas** exigé par le serveur |
| 201 | Nouveau signalement `{report_id, case_id, created: true}` |
| 200 | Même cible déjà signalée par ce compte : premier signalement renvoyé, `created: false` |
| 422 `cannot_report_self` | Son propre contenu ou son propre compte |
| 400 `invalid_request` | Corps invalide |
| 429 `rate_limited` | 10 signalements par heure et par compte (`REPORTS_PER_HOUR`) ; 120 requêtes/min |
| 503 | Service indisponible |
| Cible inexistante | **Acceptée** (aucune vérification d'existence) : l'app ne doit proposer le signalement que sur un contenu affiché |
| Non pris en charge | Produits, boutiques, messages : **exclus** (D6) |

## 2. Points d'entrée

| Contenu | Où | `target_type` / `target_id` | Masqué quand |
|---|---|---|---|
| Vidéo | Menu « ⋯ » de `VideoActions` (fil, vidéo seule) | `video` / `video_id` | Vidéo de l'utilisateur ; visiteur → feuille de connexion |
| Commentaire | Appui long ou menu de la ligne dans `CommentsSheet` | `comment` / `comment.id` | Son commentaire ; commentaire supprimé |
| Compte | Menu de la barre de `PublicProfileScreen` | `user` / `user_id` | Son propre profil |

## 3. Parcours

```
[⋯ Signaler] ──► Feuille « Signaler cette vidéo »
                  ○ Spam ou publicité trompeuse      (spam)
                  ○ Nudité ou contenu sexuel         (nudity)
                  ○ Violence                          (violence)
                  ○ Harcèlement                       (harassment)
                  ○ Discours haineux                  (hate)
                  ○ Arnaque ou fraude                 (fraud)
                  ○ Autre                             (other)
                  [ Précisez (facultatif, 500 caractères) ]
                  [Envoyer]  (actif quand un motif est choisi)
                      │
          ┌───────────┼───────────────┬─────────────────┬──────────────┐
        201          200            422 self          429             réseau/503
   « Merci, notre  « Vous avez déjà  « Vous ne pouvez « Trop de       Erreur dans la feuille,
   équipe va       signalé ce        pas signaler     signalements.   saisie conservée,
   l'examiner. »   contenu. »        votre contenu »  Réessayez dans  [Réessayer]
   feuille fermée  feuille fermée    feuille fermée   N min » (Retry-After)
```

Les libellés français des motifs sont à valider par le produit ; les **valeurs envoyées** sont
exactement celles du contrat.

## 4. Règles

| Règle | Détail |
|---|---|
| Validation | Motif obligatoire ; détail ≤ 500 caractères (compteur visible à partir de 400) ; espaces seuls = absent |
| Double soumission | Bouton désactivé et indicateur pendant l'envoi ; le serveur est de toute façon idempotent (200) |
| Conservation | Motif et détail conservés en cas d'erreur et après rotation (`SavedStateHandle`) |
| Après succès | Le contenu reste visible (aucune décision n'est prise par le signalement) ; ne pas promettre de retrait |
| Confidentialité | Aucune information sur le signaleur affichée à quiconque ; pas de journalisation du détail |
| Accessibilité | Boutons radio avec `Role.RadioButton`, groupe `selectableGroup`, titre de la feuille en `heading`, cibles ≥ 48 dp, message de résultat annoncé (`liveRegion`) |

## 5. Implémentation prévue (Lot 2, non réalisée)

| Couche | Élément |
|---|---|
| Données | `data/moderation/ModerationApi.kt` (`POST api/v1/moderation/reports`), `ReportRequest`, `ReportDto` ; `ModerationRepository` |
| Erreurs | `ApiErrorCodes.CANNOT_REPORT_SELF` + message |
| UI | `ui/moderation/ReportSheet.kt`, `ReportViewModel.kt` (clé par cible) ; entrée dans `VideoActions`, `CommentsSheet`, `PublicProfileScreen` |
| Tests | Dépôt (MockWebServer : 201, 200, 422, 429) ; ViewModel (motif requis, conservation, idempotence) |

## 6. Critères d'acceptation

1. Les 7 motifs, et seulement eux, sont proposés ; la valeur envoyée correspond au contrat.
2. Signaler deux fois la même vidéo affiche « Vous avez déjà signalé ce contenu » (réponse 200).
3. Le menu « Signaler » est absent sur ses propres vidéos, commentaires et profil.
4. Une coupure réseau garde le motif et le détail ; « Réessayer » renvoie la même requête.
5. Au 11e signalement dans l'heure, le message de limite s'affiche avec le délai.
6. Avec TalkBack, la feuille est annoncée, chaque motif est lisible et sélectionnable.
