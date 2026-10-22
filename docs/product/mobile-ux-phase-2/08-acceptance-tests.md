# Tests d'acceptation et de non-régression

Environnement de référence : `docker compose up -d --build` à jour de `main`, émulateur API 34+
avec `adb reverse tcp:8000 tcp:8000` et `tcp:9000`, `poro.debugApiBaseUrl=http://127.0.0.1:8000/`.
Deux comptes : **A** (acheteur) et **V** (vendeur, boutique ouverte, un produit publié avec photo).
Code OTP : `docker compose logs -f auth | Select-String "dev sms otp"`.

Chaque scénario indique le type de test : **U** = test JVM (ViewModel / dépôt, MockWebServer),
**M** = manuel sur émulateur, **CI** = `testDebugUnitTest assembleDebug`.

## 1. Marketplace (Lot 1)

| ID | Scénario | Étapes | Résultat attendu | Type |
|---|---|---|---|---|
| MK-01 | Ajout additif | A ajoute 2 du produit, revient, ajoute 1 | Panier : quantité 3 | U + M |
| MK-02 | Plafond 99 | Panier à 98, ajouter 5 | `PUT` envoie 99 ; message « Quantité maximale atteinte » | U |
| MK-03 | Propre produit | V ouvre son produit | Pas de bouton d'ajout ; « C'est votre produit » | M |
| MK-04 | Validation repère | Repère « ab » | Erreur sous le champ, aucun appel `preview` | U |
| MK-05 | Récapitulatif périmé | V change le prix entre preview et confirm | Retour à la relecture avec « Le récapitulatif a changé » | U + M |
| MK-06 | Double confirmation | Double appui rapide sur Confirmer | Une seule série de commandes (même `checkout_id`) | U + M |
| MK-07 | Message après commande | Confirmer | « Commande envoyée. La boutique confirme… » ; commande `pending` puis `confirmed` | M |
| MK-08 | Total estimé | Commande `pending` (`total` null) | « Total estimé : … » + bandeau | U |
| MK-09 | Jamais « Payée » en COD | Commande COD jusqu'à `completed` | Libellés : En attente → Confirmée → Expédiée → Livrée ; jamais « Payée » | U + M |
| MK-10 | Motif d'annulation | Commande `cancelled` `stock_rejected` | « Motif : article indisponible » | U |
| MK-11 | Conflit de statut | V expédie pendant que A annule | A reçoit le message de conflit et voit le statut rechargé | U + M |
| MK-12 | Liste vendeur | V ouvre « Commandes reçues » | Chaque ligne affiche destinataire et ville | M |
| MK-13 | Photo produit | V ajoute une photo puis publie | Photo visible, produit « En vente » | M |
| MK-14 | Photo hors réseau | Mode avion pendant l'envoi, puis Réessayer | Message réseau, puis succès ; une seule image | M |
| MK-15 | Boutique enregistrée | V modifie le nom | Message « Boutique enregistrée » ; un seul `PATCH` | M |
| MK-16 | Panier rechargé une fois | Ouvrir le panier | Un seul `GET /cart` dans Logcat | M |

## 2. Signalement (Lot 2)

| ID | Scénario | Résultat attendu | Type |
|---|---|---|---|
| RP-01 | Signaler une vidéo d'autrui, motif `spam` | 201 ; « Merci… » ; feuille fermée | U + M |
| RP-02 | Signaler à nouveau | 200 `created:false` ; « Vous avez déjà signalé… » | U |
| RP-03 | Sa propre vidéo | Entrée « Signaler » absente | M |
| RP-04 | Motif non choisi | Envoyer désactivé | U |
| RP-05 | Détail de 501 caractères | Saisie bloquée à 500 | U |
| RP-06 | Réseau coupé | Erreur, motif et détail conservés, Réessayer | U |
| RP-07 | 429 | Message avec délai | U |
| RP-08 | TalkBack | Feuille, motifs et bouton annoncés | M |

## 3. Écrans légaux (Lot 3)

| ID | Scénario | Résultat attendu | Type |
|---|---|---|---|
| LG-01 | Paramètres → Informations légales → Confidentialité | Document ouvert en 2 appuis | M |
| LG-02 | Document `draft` | Bandeau « À valider » toujours visible | U (rendu) + M |
| LG-03 | Document absent | État « bientôt disponible » | U |
| LG-04 | Police à 200 % | Texte lisible, pas de coupure | M |

## 4. Mode invité (Lot 4 — après B1)

| ID | Scénario | Résultat attendu | Type |
|---|---|---|---|
| GU-01 | Premier lancement sans compte | Fil Tendances affiché | M |
| GU-02 | Aimer en invité | Feuille de connexion ; aucune requête `PUT …/like` | U + M |
| GU-03 | Connexion depuis la feuille | Retour sur la même vidéo | M |
| GU-04 | 401 en invité | Pas de message « session expirée », pile conservée | U |

## 5. Non-régression (à chaque lot)

| ID | Parcours | Résultat attendu |
|---|---|---|
| NR-01 | Connexion SMS (code des logs) | Accueil affiché ; session conservée après redémarrage de l'app |
| NR-02 | Connexion email, inscription, mot de passe oublié | Inchangés |
| NR-03 | Fil Pour toi / Abonnements / Tendances | Lecture HLS, défilement vertical |
| NR-04 | Aimer, commenter, répondre, supprimer son commentaire, suivre, partager | Compteurs cohérents |
| NR-05 | Publier une vidéo | Progression, puis « prête » |
| NR-06 | Recherche vidéos, comptes, hashtags | Résultats et suggestions |
| NR-07 | Notifications, préférences (réactiver une préférence) | Préférence réactivée réellement côté serveur |
| NR-08 | Profil : pseudo, avatar, créateur ; sessions ; suppression du compte | Inchangés |
| NR-09 | Parcours marketplace complet A ↔ V | Commande créée, expédiée, reçue |
| NR-10 | CI | `testDebugUnitTest assembleDebug` vert ; aucun test désactivé |

Seuil de sortie d'un lot : tous les tests U et CI verts, scénarios M du lot et NR-01 à NR-09
exécutés et consignés (date, appareil, résultat, captures en cas d'échec).
