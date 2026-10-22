# Blocages et décisions requises

## 1. Blocages

| ID | Blocage | Bloque | Responsable | Débloqué quand |
|---|---|---|---|---|
| BL-1 | `docs/product/mobile-ux-audit/` (phase 1) absent du dépôt | Comparaison avec l'audit (L0-1) | Propriétaire | Dossier poussé sur une branche |
| BL-2 | Fil réservé aux comptes connectés (`feed`, 3 routes) | Mode invité (Lot 4) | Backend | B1 livré et contrat mis à jour |
| BL-3 | Envoi de photos en échec sur l'émulateur, cause non confirmée | Publication de produit, avatar (M4) | Propriétaire puis mobile | Journal `PoroMedia` transmis |
| BL-4 | Images Docker `auth` et `social` périmées ou en échec sur le poste de test | Sessions, compteurs, tests NR | Propriétaire | `docker compose up -d --build` puis services sains |
| BL-5 | PR #58 basée 6 commits avant `main` | Lot 5 (lectures groupées) | Mobile | Rebase ou fusion |
| BL-6 | Aucun texte légal validé, aucune API de consentement | Publication des écrans légaux comme définitifs | Juridique, backend | Textes validés ; décision L1 |

## 2. Décisions requises

| ID | Question | Options | Recommandation |
|---|---|---|---|
| B1 | Rendre une partie du fil publique ? | a) `trending` en jeton facultatif ; b) `for-you` non personnalisé public ; c) aucun mode invité | a), avec revue sécurité (B3) |
| B2 | Limiter les visiteurs derrière un CGNAT ? | a) limite par IP relevée ; b) jeton d'appareil anonyme | À évaluer par le backend avec des mesures réelles |
| M1 | « Ajouter au panier » additionne-t-il ? | a) additionner (plafond 99) ; b) libeller « Mettre N au panier » | a) |
| M2 | Accès « Voir le panier » après ajout | snackbar avec action / navigation directe | Snackbar avec action |
| R1 | Libellés français des 7 motifs de signalement | proposition dans `04` § 3 | Valider la proposition |
| R2 | Le signalement de produits ou de boutiques est-il requis au lancement ? | oui (backend à étendre) / non | Non pour la bêta |
| L1 | Mention d'acceptation à l'inscription | a) mention informative sans case ; b) case obligatoire + enregistrement serveur | a) pour la bêta ; b) avant le lancement public si le juridique l'exige |
| L2 | Liste définitive des documents légaux | `05` § 2 | Validation juridique |
| L3 | Source des textes | locaux (A) / endpoint (B) | A pour la bêta |
| DS1 | Contraste de l'orange (2,87:1 sous le blanc) | A : texte foncé sur orange ; B : orange soutenu `#C24E00` | Choix de marque ; l'une des deux est nécessaire pour l'AA |
| DS2 | Police de marque | système / police embarquée | Système (poids, réseaux lents) |
| U1 | Ajout de `androidx.work` pour l'envoi de vidéo | oui / non | Oui (Lot 5) |

## 3. Contrats ou informations manquants

| Sujet | Ce qui manque |
|---|---|
| Boutique fermée ou suspendue côté public | Réponse documentée (404 ? statut visible ?) de `GET /shops/{handle}` |
| Liste vendeur | Filtre par statut côté serveur (absent du contrat) |
| `paid` en paiement à la livraison | Moment où `paid`/`paid_at` passent à vrai pour une commande COD (non documenté) ; l'app ne l'utilise pas tant que ce n'est pas défini |
| Textes légaux | Contenu, versions, dates d'effet |
| Feed public | Contrat, limites, champs exposés (B1, B3) |

## 4. Ordre exact des tâches de développement (après autorisation)

1. L0-5 Reconstruire les images et rejouer NR-01 à NR-09 (référence).
2. L0-4 Diagnostiquer l'envoi de photos (`PoroMedia`).
3. L0-2 Mettre la PR #58 à jour avec `main`.
4. L1 : M4 → M1 → M2 → M3 → M5 → M8 → M9 → M6 → M7 → M10 → M11, avec leurs tests.
5. L2 : signalement (après R1).
6. L3 : écrans légaux (après L1, L2 juridiques).
7. L5 : lectures groupées (commentaires, compteurs), puis thème (après DS1), puis WorkManager.
8. L4 : mode invité, **uniquement** après B1 à B3 et autorisation.
9. L6 : selon les livraisons backend.
