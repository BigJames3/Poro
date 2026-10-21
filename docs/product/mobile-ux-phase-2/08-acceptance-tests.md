# Scénarios fonctionnels et non-régression

À exécuter **après** autorisation d’implémenter. Aujourd’hui : référence QA. Prérequis : gateway locale, comptes CI, boutique de test, **sans secrets dans les rapports**.

Les AC-AUTH / FEED / SOCIAL de [l’audit 10](../mobile-ux-audit/10-acceptance-criteria.md) restent la non-régression sociale. Ci-dessous : marketplace, report, légal, invité (quand débloqué).

## 1. Non-régression sociale (ne pas casser)

| ID | Scénario | Attendu |
|---|---|---|
| NR-01 | Login OTP + Home Pour toi | Pager, une lecture |
| NR-02 | Like / commentaire / follow | Compteurs persistés |
| NR-03 | Recherche compte → profil | PublicProfile |
| NR-04 | Upload MP4 court | Processing puis ready |
| NR-05 | Logout | Phone ; back quitte l’app |
| NR-06 | 5 onglets | Boutique toujours visible |

## 2. Marketplace

| ID | Scénario | Attendu |
|---|---|---|
| AC-MX-01 | Handle boutique active | Vitrine + produits |
| AC-MX-02 | Handle inconnu | `shop_not_found`, pas de crash |
| AC-MX-03 | 2× « Ajouter » qty 1 même variante | Panier **2** (après correctif) |
| AC-MX-04 | Qty 99 puis + | Reste 99 |
| AC-MX-05 | Article retiré de la vente | Ligne unavailable, checkout off |
| AC-MX-06 | Achat chez soi | `own_shop` |
| AC-MX-07 | Preview | Recap ; **0** commande `GET /orders` nouveau |
| AC-MX-08 | Confirm COD | Snackbar `checkout_placed` ; statut `confirmed` ; **pas** « Payée » |
| AC-MX-09 | Confirm 2× même preview | Mêmes `order_id` (idempotence) |
| AC-MX-10 | Modifier panier entre preview et confirm | `preview_outdated`, retour formulaire, pas de commande fantôme |
| AC-MX-11 | Airplane au confirm | Pas de succès ; retry ou liste vérifie |
| AC-MX-12 | Back récap | Livraison, champs conservés |
| AC-MX-13 | Cancel avant ship | `cancelled` + dialogue |
| AC-MX-14 | Ship vendeur | `shipped` ; acheteur voit « J’ai reçu » |
| AC-MX-15 | Confirm receipt | `completed` |
| AC-MX-16 | Publier produit 0 photo | `image_required` |
| AC-MX-17 | 10e photo | Acceptée après alignement UI |
| AC-MX-18 | Prix XOF | Entier « FCFA », pas de virgule |

## 3. Signalement (lot 2)

| ID | Scénario | Attendu |
|---|---|---|
| AC-MOD-01 | ⋮ vidéo → spam, sans détail | 201, snackbar envoyé, overlay intact |
| AC-MOD-02 | Même vidéo encore | 200, snackbar « déjà signalé », pas d’erreur rouge |
| AC-MOD-03 | Signaler son profil / sa vidéo | 422 `cannot_report_self` |
| AC-MOD-04 | Motif hors liste | Impossible (enum UI) |
| AC-MOD-05 | Détail 501 caractères | Bloqué local |
| AC-MOD-06 | Commentaire cible `comment` | 201 |
| AC-MOD-07 | 10e puis 11e / heure | 429 + attente |
| AC-MOD-08 | Dismiss sheet | 0 POST |
| AC-MOD-09 | Produit ⋮ report | **Absent** (pas d’API) |

## 4. Légal (lot 3)

| ID | Scénario | Attendu |
|---|---|---|
| AC-LEG-01 | Settings → documents | 3 lignes |
| AC-LEG-02 | Document non validé | Indisponible, **aucune** clause présentée comme opposable |
| AC-LEG-03 | Depuis Phone, ouvert légal | Pas de bounce login |
| AC-LEG-04 | Delete account toujours là | Inchangé |

## 5. Invité (lot 4 — seulement si trending public)

| ID | Scénario | Attendu |
|---|---|---|
| AC-GST-01 | Cold start sans jetons | Accueil Tendances **200**, pas Phone |
| AC-GST-02 | Pour toi visiteur | CTA login, pas d’appel For You ou 401 géré |
| AC-GST-03 | Like visiteur | Login ; après OTP retour vidéo |
| AC-GST-04 | Search visiteur | Résultats sans token |
| AC-GST-05 | Fiche produit visiteur + add | Login ; pas de PUT cart anonyme |
| AC-GST-06 | GET /users/me visiteur | **Aucun** appel |
| AC-GST-07 | Bearer pourri | 401, pas d’ignorer le header |
| AC-GST-08 | Connecté | Pour toi inchangé |

Si le backend n’est pas livré : **AC-GST-* non exécutés** ; l’app reste auth-wall (NR-05).

## 6. Sécurité / a11y smoke

| ID | Attendu |
|---|---|
| AC-SEC-01 | Pas de refresh token / OTP dans Logcat |
| AC-SEC-02 | Report n’affiche pas `case_id` |
| AC-SEC-03 | Routes `/moderation/cases` non appelées |
| AC-A11Y-01 | TalkBack ReportSheet : titre + motifs + CTA |
| AC-A11Y-02 | Checkout confirm annoncé comme bouton unique |

## 7. Hors ligne

Pas d’exigence « tout marche offline ». Échec réseau = `error_network` + Réessayer. Seul le légal packagé peut s’afficher sans réseau.
