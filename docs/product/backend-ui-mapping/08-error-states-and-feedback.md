# États et feedback

Règle unique : `error.code` → string FR (`AuthErrorMessages.kt`). Jamais `error.message` anglais. Composants : `LoadableContent`, `SubmitButton`, `ErrorMessageText`, `EmptyState`, snackbar.

Famille visuelle : [mobile-design/07](../mobile-design/07-states-and-feedback.md).

## 1. Matrice par type d’écran

| Écran / action | Loading | Vide | Page 2 | Réseau | 401 | 403/404 | 422 | 429 | Double tap | Succès |
|---|---|---|---|---|---|---|---|---|---|---|
| Auth OTP/email | CTA spinner | — | — | Réessayer | — | blocked | codes otp/email | countdown | working | Home / returnTo |
| Fil | Spinner blanc | EmptyState | pager | Réessayer | login / guest | — | invalid_cursor reset | message | — | play |
| Vidéo isolée | Spinner | — | — | Réessayer | idem | removed | — | — | — | player |
| Player HLS | miniature | — | — | P1 retry | — | — | — | — | une decode | loop |
| Comments GET | spinner sheet | comments_empty | load more | Réessayer | — | video_not_found | — | — | — | liste |
| Like / follow | optimistic | — | — | revert snackbar | prompt | 404 | — | message | busy | compteur |
| Report | working | — | — | Réessayer | prompt | self 422 | motifs | 11e/h | working | 201/200 |
| Upload | % | — | — | Failed retry | — | — | format/size/durée | throttle | isBusy | ready/failed |
| Search | debounce | 0 résultats | cursor | Réessayer | — | — | — | Retry-After | — | hits |
| Notif inbox | Loadable | empty | cursor | Réessayer | prompt | — | — | — | — | liste |
| Shop handle | finding | hint | — | Réessayer | — | not_found | handle <3 | — | finding | vitrine |
| PDP / add cart | adding | — | — | Réessayer | prompt | — | own_shop, cart_full | — | adding | snackbar |
| Checkout | working | — | — | Vérif + GET orders | — | stale preview | delivery | — | working | confirmed, pas paid |
| Orders | Loadable | orders_empty | cursor | Réessayer | — | — | status_conflict | — | working | statut serveur |
| Éditeur photo | working/pending | 0 photo | — | retry pending | — | — | format/taille/10 | 30/min | working | AsyncImage |
| Légal | — | corps indisponible | — | — | — | — | — | — | — | placeholder |
| Sessions | liste | — | — | Réessayer | expire | — | — | — | — | révoquée |

Hors-ligne = même famille que réseau. Pas d’écran offline dédié (pas de cache métier).

## 2. Session et back

| Cas | Feedback |
|---|---|
| Splash Unknown | Spinner `loading` |
| Expire refresh | `session_expired` + login ; cible guest si abandon |
| Logout | Pile auth ; cible Tendances |
| Back checkout récap | `edit()`, pas pop confirm |
| Back prompt guest | Contenu intact |
| Contenu disparu (404) | Copy gone, pas login |

## 3. Interdits

Annoncer un succès sans `data`. Forcer `paid`. Retry immédiat en boucle. 401 visiteur = uniquement prompt, pas un crash. Staff 403 ≠ « réessayer ».
