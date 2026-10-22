# Spécifications d'écrans

Périmètre : écrans **modifiés ou créés** par les lots 1 à 4. Les écrans existants non cités restent
tels quels (non-régression dans `08`). Chemins relatifs à `app/src/main/java/com/poro/app/`.

Exigences communes à tous les écrans :
- **États** : chargement (indicateur annoncé « Chargement »), erreur (`ErrorMessageText` + « Réessayer » pour les lectures), vide (texte explicite), succès.
- **Réseau instable** : toute action d'écriture désactive son bouton pendant l'appel ; la saisie est conservée en cas d'échec ; les délais 429 (`Retry-After`) sont affichés.
- **Accessibilité** : cibles ≥ 48 dp, titres en `heading`, icônes seules avec `contentDescription`, contraste AA (voir `06`), lisible à 200 % de police, utilisable avec TalkBack.
- **Petits écrans** : largeur de référence 360 dp ; aucun défilement horizontal hors carrousels.
- **Hors ligne** : aucune fonction hors ligne requise ; seules les saisies longues (livraison, signalement, formulaires de boutique et de produit) survivent à la mort du processus.

---

## S1. Fiche produit — ajout au panier additif (Lot 1, M1/M10/M11)

| Rubrique | Spécification |
|---|---|
| Objectif | Ajouter une quantité d'une déclinaison sans écraser ce qui est déjà au panier |
| Entrées / sorties | Depuis la vitrine ou un lien ; sortie : retour, ou « Voir le panier » depuis la confirmation |
| Données | `GET /products/{id}` ; quantité déjà au panier pour la déclinaison (`GET /cart`) |
| Actions | Choisir la déclinaison, la quantité (1..99 − déjà au panier), « Ajouter au panier » |
| API | `PUT /cart/items/{variantId}` `{quantity: min(déjà + choisie, 99)}` |
| États | Succès : snackbar « Ajouté. N au panier » + action « Voir le panier » ; déjà 99 : bouton désactivé « Quantité maximale atteinte » |
| Accès | Visiteur : feuille de connexion (Lot 4) ; propriétaire de la boutique : bouton remplacé par « C'est votre produit » |
| Erreurs | `variant_unavailable` (recharger la fiche), `cart_full`, `too_many_lines`, `own_shop`, réseau |
| Acceptation | Ajouter 2 puis 1 → 3 au panier ; à 99, aucun appel n'est envoyé |

## S2. Commande — affichage du total et du motif (Lot 1, M2/M6/M9)

| Rubrique | Spécification |
|---|---|
| Objectif | Montrer un état et un montant fidèles au serveur |
| Données | `OrderDto` : `status`, `total`, `subtotal_seen`, `items[].unit_price`, `unit_price_seen`, `cancel_reason`, `paid` |
| Règles | `total == null` → « Total estimé : X » + bandeau « Montant confirmé après vérification du stock » ; `unit_price ≠ unit_price_seen` → mention « Prix mis à jour par la boutique » sur la ligne ; `cancel_reason` traduit ; « Payée » **uniquement** si `status == paid` |
| Actions | Acheteur : annuler (`pending`/`confirmed`/`awaiting_payment`/`paid`), confirmer la réception (`shipped`) ; vendeur : expédier (`confirmed`/`paid`), annuler (avant expédition) |
| Erreurs | `order_status_conflict` → message + rechargement automatique de la commande |
| Acceptation | Une commande `pending` n'affiche jamais « Total : » sans « estimé » ; une commande COD livrée n'affiche jamais « Payée » |

## S3. Récapitulatif et confirmation de commande (Lot 1, M3/M5/M8)

| Rubrique | Spécification |
|---|---|
| Objectif | Commander en paiement à la livraison en sachant ce qui se passe ensuite |
| Validation locale | Nom 1..80, téléphone `^\+[1-9]\d{7,14}$`, ville 1..80, adresse 3..300 (obligatoire, pas de GPS), repère vide ou 3..300 |
| Après confirmation | Message « Commande envoyée. La boutique confirme la disponibilité des articles. » puis liste des commandes |
| Erreurs | `confirmation_required`, `address_book_full` (désactiver « Enregistrer l'adresse » et informer), `address_label_invalid`, `address_not_found`, `preview_*` (relecture) |
| Acceptation | Un repère de 2 caractères est refusé localement avec un message sous le champ |

## S4. Commandes reçues (vendeur) (Lot 1, M7)

| Rubrique | Spécification |
|---|---|
| Ligne | Destinataire (`delivery.full_name`, ou « Client » si effacé), ville, statut (`OrderStatusChip`), date, montant |
| Tri | Ordre serveur ; option locale « À expédier d'abord » sur les pages chargées (P2) |
| Acceptation | Aucune ligne n'affiche le nom de sa propre boutique |

## S5. Feuille de signalement (Lot 2) — voir `04` pour le contrat

| Rubrique | Spécification |
|---|---|
| Entrées | Vidéo (menu ⋯), commentaire (menu de ligne), profil public (barre) |
| Données | `target_type`, `target_id` ; motifs du contrat |
| États | Saisie, envoi, succès (201 / 200 doublon), erreur (422, 429, réseau, 503) |
| Accès | Connecté ; masqué sur ses propres contenus |
| Acceptation | Voir `04` § 6 |

## S6. Informations légales (Lot 3) — voir `05`

| Rubrique | Spécification |
|---|---|
| Écrans | `LegalIndexRoute`, `LegalDocumentRoute(id)` ; entrée « Informations légales » dans `SettingsScreen` et liens sur `PhoneEntryScreen`/`EmailRegisterScreen` |
| États | Brouillon (bandeau « À valider » permanent), absent, validé |
| Acceptation | Voir `05` § 5 |

## S7. Mode invité (Lot 4, **bloqué** par B1) — voir `03`

| Rubrique | Spécification |
|---|---|
| Démarrage | Sans session : fil Tendances en lecture seule (après B1) |
| Actions protégées | Feuille unique « Connectez-vous pour continuer » avec le motif ; retour à l'écran d'origine après connexion |
| Session | 401 en invité ≠ session expirée : pas de vidage de pile |
| Acceptation | Un visiteur ne déclenche aucune requête protégée ; aimer une vidéo ouvre la feuille ; après connexion, l'utilisateur revient sur la même vidéo |

## S8. Envoi de photos (Lot 1, M4 — après diagnostic)

| Rubrique | Spécification |
|---|---|
| Objectif | Avatar, logo et photos produit envoyés de façon fiable |
| États | Envoi en cours (indicateur sur la vignette), échec réseau (« Réessayer » sur la vignette), refus du stockage (« Photo refusée : JPEG, PNG ou WebP, 5 Mo au plus ») |
| Acceptation | Couper le réseau pendant l'envoi puis réessayer : la photo est envoyée une seule fois ; un produit avec photo peut être publié |
