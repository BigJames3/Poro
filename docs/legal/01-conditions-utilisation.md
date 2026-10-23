# Conditions d’utilisation — Poro

**Brouillon produit v0.1-draft — 21 octobre 2026.**  
**Ce texte n’a aucune valeur juridique.** Il décrit le service tel qu’il existe
dans le dépôt. Il deviendra opposable seulement après visa d’un avocat des
pays de lancement et publication d’une version numérotée avec date d’effet.

Poro est un service de clips courts et une place de marché à paiement à la
livraison, prévu pour la Côte d’Ivoire, le Sénégal, le Cameroun et le Nigeria
(indicatifs +225, +221, +237, +234 — ADR-0004).

## 1. Objet

Les présentes conditions régissent l’usage de l’application mobile Poro et de
l’API associée. Ouvrir un compte, publier un clip, ouvrir une boutique ou
passer une commande vaut acceptation **une fois ce document visé et publié**.
Tant qu’il est en brouillon, aucun engagement contractuel n’en découle.

## 2. Compte

- Création par téléphone (OTP SMS, Africa’s Talking) ou e-mail et mot de passe.
- Un e-mail non vérifié n’empêche pas la connexion (ADR-0013).
- Rôles : PERSONAL, CREATOR, BUSINESS, et rôles internes non exposés dans
  l’app grand public. **CREATOR est un badge** : il n’est pas exigé pour
  publier une vidéo (ADR-0007). BUSINESS est accordé après ouverture d’une
  boutique (ADR-0012).
- Âge minimum : **à compléter par le juridique** (non tranché).
- Sessions par appareil, révocables ; un jeton d’accès déjà émis peut rester
  valable quelques minutes après révocation selon ADR-0016.
- Suppression : `POST /me/delete` programme l’effacement à J+30 ; se
  reconnecter ou annuler avant l’échéance l’interrompt. Après purge, rien
  n’est restaurable (ADR-0013, ADR-0018).

## 3. Contenu social

- Formats acceptés aujourd’hui : vidéo MP4 / MOV / WebM, 256 Mio, 180 s.
  Une page « image seule » dans le fil **n’existe pas encore** (B-IMG-POST).
- Poro héberge et diffuse le clip en HLS public (`poro-videos`). L’original
  reste privé (`poro-video-sources`, ADR-0015).
- En publiant, vous accordez à Poro une licence non exclusive de héberger,
  transcoder et afficher le contenu pour le fonctionnement du service.
- Vous gardez vos droits. Vous garantissez disposer des droits sur le
  contenu et les musiques / visages / marques qu’il contient.
- Poro peut retirer un contenu signalé ou contraire aux règles (modération,
  ADR-0010). Un signalement n’emporte aucune promesse de délai (Q-15).
- **Enregistrer une vidéo** dans la galerie n’est pas offert : il n’existe
  pas de fichier MP4 public unique (B-VID-SAVE).

## 4. Boutique et commandes

- Une boutique par compte ; devise fixée par le pays de la boutique
  (XOF, XAF, NGN — ADR-0012).
- Photos produit : JPEG, PNG ou WebP, 5 Mio, 10 photos, ré-encodées en WebP.
  Pas de vidéo produit.
- Moyen de paiement v1 : **paiement à la livraison (COD)**. Une commande
  `confirmed` n’est **pas** « payée ». Wave n’est pas branché.
- Pas d’achat dans sa propre boutique.
- L’acheteur peut annuler tant que rien n’est expédié. Après expédition,
  pas d’annulation v1 (litiges plus tard).
- Les prix affichés sont indicatifs jusqu’à la réservation de stock.
- Frais de livraison : « payés au livreur » ; Poro ne calcule pas le tarif
  (Q-18).
- Reversements vendeurs : journal interne, virements manuels en v1.

## 5. Médias d’autrui (Q-MED-02)

- Les clips et photos **publiés** sont visibles par les comptes autorisés
  par l’API (et, pour les objets des buckets publics, par quiconque connaît
  l’URL).
- Vous n’acquérez **aucun droit** de réutiliser, vendre ou republier le
  contenu d’autrui.
- L’app **n’offre pas** d’action « Enregistrer » pour une photo produit
  (Q-MED-01 = non). Une copie obtenue hors app (navigateur, capture) n’est
  pas une autorisation.

## 6. Conduite

Interdits notamment : spam, nudité non autorisée, violence, harcèlement,
discours de haine, fraude. Signalement in-app : vidéo, commentaire, profil
(7 motifs). Pas de signalement produit (API absente).

## 7. Responsabilité

Poro fournit le service « en l’état ». Les ventes sont conclues entre
acheteur et vendeur ; Poro n’est pas dépositaire des fonds en v1 COD.
La responsabilité pour un contenu publié par un utilisateur reste celle
de l’auteur, sous réserve des obligations légales de l’hébergeur **à
qualifier par le juridique** dans chaque pays.

## 8. Droit applicable

**À compléter.** Pays de lancement : CI, SN, CM, NG. Une clause unique
peut être insuffisante ; le visa devra trancher le droit applicable et
le for.

## 9. Contact

Canal support : **à compléter** (aucun e-mail produit dans le dépôt).
