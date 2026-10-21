# Bloquants et décisions requises

Les questions tranchées en phase 2 ne sont plus listées comme ouvertes (Q-01 boutique visible ; Q-02 souhaité mais **techniquement bloqué**).

## 1. Bloquants d’implémentation

| ID | Bloque | Détail | Owner |
|---|---|---|---|
| B-AND | Lots 1–5 Android | Toute écriture dans `apps/mobile-android` exige une **autorisation explicite** ultérieure | Utilisateur |
| B-FEED | Lot 4 invité | `GET /feed/*` Bearer obligatoire ; trending n’a pas besoin du `sub` mais le middleware l’exige | Backend + sécu |
| B-LEGAL | Contenu opposable | Pas de TOS/privacy dans le dépôt ; écrans = indisponible jusqu’à validation | Juridique (Q-03) |
| B-SMS | Tests réels OTP | Africa’s Talking / Sender ID non confirmés (ADR-0004) | Exploitation |
| B-FCM | Lot 5 push | Pas de projet Firebase | Mobile + ops (Q-21) |

Sans B-AND, cette phase s’arrête à la documentation — **c’est l’état actuel**.

## 2. Décisions encore ouvertes (impact phase 2+)

| ID | Question | Impact | Reco spec |
|---|---|---|---|
| Q-03 | Textes légaux 4 pays | Lot 3 contenu | Livrer URL ou markdown versionné ; sinon rester « indisponible » |
| Q-02b | Autoriser **trending public** + quota IP ? | Débloque lot 4 | Oui, For You/Following restent privés |
| Q-04 | Sens de « Devenir créateur » | Copie profil | Badge seulement ; ne pas laisser croire à un droit de publication |
| Q-10 | Charte finale | Tokens | Garder `#FF6A00` |
| Q-11 | UI EN Nigeria | Légal + SMS | FR app v1 |
| Q-15 | Copie report (délai) | Lot 2 | Sans SLA |
| Q-16 | App staff | Report | Interdit dans l’app grand public |
| Q-17 | COD Nigeria | Marketplace | Garder COD 4 pays tant que seul moyen |
| Q-21 | Firebase | Lot 5 | Cacher switch push OS ou le labelliser inopérant |
| Q-23 | Limites OpenAPI vs code | Hors lots 1–3 sauf si on touche au profil | — |

## 3. Informations manquantes

- Textes et dates d’effet légaux.
- URL publique `SHARE_BASE_URL`.
- Timeouts OkHttp exacts (**À confirmer** à l’implémentation).
- Canal « Contactez le support » (`account_blocked`).
- Validation juridique conservation commandes (ADR-0018).

## 4. Ce qui n’est pas bloquant pour lot 1

Wave, chat, search catalogue, GPS, mute, WorkManager, mode invité, file modération.

## 5. Ordre des décisions humaines

1. Autoriser le lot 1 Android **ou** rester en spec.
2. Fournir / refuser les textes légaux (lot 3).
3. Accepter ou refuser trending public (lot 4).
4. Firebase (lot 5).

## 6. Contrôle de cette phase

| Contrôle | Résultat |
|---|---|
| Fichiers créés | `docs/product/mobile-ux-phase-2/*.md` uniquement |
| `apps/mobile-android` | **Non modifié** |
| Audit phase 1 | **Préservé** |
| Branche / commit / deploy | **Non effectués** |
| Fonctionnalités « livrées » | **Aucune** — conception seulement |
