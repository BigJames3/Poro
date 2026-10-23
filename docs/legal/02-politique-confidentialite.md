# Politique de confidentialité — Poro

**Brouillon produit v0.1-draft — 21 octobre 2026.**  
**Ce texte n’a aucune valeur juridique.** Il inventorie les traitements
réellement mis en œuvre dans le code. Un DPO / avocat doit le viser avant
publication. Aucune case « j’accepte le RGPD » n’est ajoutée : l’app n’a
pas de SDK publicitaire (spec légale phase 2).

Responsable de traitement : **identité éditeur à compléter**
(voir [03-mentions-legales.md](03-mentions-legales.md)).

## 1. Données collectées

| Catégorie | Exemples | Source |
|---|---|---|
| Identifiants de compte | téléphone E.164, e-mail, mot de passe (Argon2id), `full_name` à l’inscription (non recopié dans le profil — Q-22) | vous |
| Profil | pseudo, nom affiché, bio, avatar, pays (souvent vide) | vous |
| Session | jeton d’accès / refresh, nom d’appareil, **adresse IP** (Q-09 ouvert) | appareil / gateway |
| Contenu | vidéos, titres, descriptions, commentaires, likes, partages, abonnements | vous |
| Boutique | nom, handle, description, logo, photos, prix, stock | vous |
| Commande | lignes, montants, devise, statut, contact de livraison (nom, téléphone, ville, adresse, GPS si vous l’envoyez) | vous |
| Signalements | cible, motif, commentaire ≤ 500 caractères | vous |
| Journal technique | `request_id`, logs sans OTP ni mot de passe | système |

Non collecté aujourd’hui : carnet de contacts du téléphone, position en
continu, SDK publicitaire, Firebase / FCM (Q-21 ouvert).

## 2. Finalités

- Créer et sécuriser le compte (OTP, limites anti-abus ADR-0017).
- Diffuser le fil, la recherche et les profils publics.
- Exécuter les commandes COD et informer acheteur / vendeur.
- Modérer les signalements et conserver un journal de décisions.
- Empêcher la fraude (pays OTP, quotas, révocation de session).

Base légale : **à qualifier par le juridique** (contrat, intérêt légitime,
obligation légale, consentement selon le pays). Côte d’Ivoire, Sénégal,
Cameroun et Nigeria n’ont pas le même régime que le RGPD.

## 3. Destinataires

- Services Poro (auth, user, video, feed, social, search, shop, order,
  notification, moderation) et la gateway Traefik.
- Africa’s Talking pour l’envoi d’OTP SMS (numéro, texte du SMS).
- SMTP (quand configuré) pour les codes e-mail.
- Stockage objet (SeaweedFS / S3 compatible) : HLS et photos catalogue en
  lecture GET publique ; originaux privés (ADR-0015).
- Pas de revente de fichiers clients à des annonceurs.

Le personnel de modération / admin accède à la file `cases` hors de l’app
grand public.

## 4. Durées

| Donnée | Durée dans le code |
|---|---|
| Compte actif | tant que le compte existe |
| Après demande de suppression | grâce de 30 jours, puis purge (ADR-0013) |
| Contact de livraison | 90 jours après la fin de la commande (ADR-0012) |
| Commandes (ids, montants, lignes) | conservées après effacement du compte ; **durée et base légale à viser** (ADR-0018, proposition interne : obligation comptable OHADA) |
| Signalements | journal conservé, signaleur pseudonymisé à l’effacement |
| Sessions | jusqu’à révocation ou expiration du refresh |
| OTP | quelques minutes, puis invalidé |

## 5. Transferts

Les OTP et l’hébergement peuvent sortir du pays de l’utilisateur (fournisseur
SMS, objet storage). **Localisation exacte des serveurs : à compléter** une
fois l’hébergeur choisi (staging / prod encore bloqués).

## 6. Vos droits

Selon le pays : accès, rectification, suppression, opposition. L’exercice
concret aujourd’hui :

- consulter et modifier le profil ;
- lister / révoquer les sessions ;
- supprimer le compte (J+30) ;
- les commandes et le journal de modération ne sont pas intégralement
  effacés (voir §4).

Demande hors app : **canal à compléter**.

L’app affiche l’IP des sessions. Masquer ce champ (Q-09) n’est pas tranché.

## 7. Mineurs

Âge minimum **non fixé**. Ne pas ouvrir le store tant que ce point n’est
pas visé.

## 8. Sécurité

JWT RS256, refresh par appareil, quota par IP et par compte, pas de secret
dans l’APK, `allowBackup=false`. Aucune garantie absolue.

## 9. Modifications

Toute version opposable portera un numéro et une date d’effet. Ce brouillon
peut changer sans préavis.
