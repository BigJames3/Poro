# Dossier pour visa avocat — Q-03

**Ce n’est pas un visa.** Un modèle de langage ne peut pas certifier des
conditions opposables, ni remplacer un avocat inscrit dans les pays de
lancement (CI, SN, CM, NG). Les fichiers `01`–`03` restent des
**brouillons produit v0.1-draft**.

Ce dossier prépare la relecture : questions à trancher, pièces, et ce
qu’il est interdit de faire avant la signature.

## 1. Pièces à relire

| Fichier | Objet |
|---|---|
| [01-conditions-utilisation.md](01-conditions-utilisation.md) | Contrat utilisateur / vendeur |
| [02-politique-confidentialite.md](02-politique-confidentialite.md) | Traitements de données réellement implémentés |
| [03-mentions-legales.md](03-mentions-legales.md) | Éditeur / hébergeur — **identité vide** |
| [ADR-0004](../architecture/adr/0004-decisions-produit-lancement.md) | Pays, SMS, paiements |
| [ADR-0012](../architecture/adr/0012-marketplace-vague-3.md) | COD, devises, agrément |
| [ADR-0013](../architecture/adr/0013-cycle-de-vie-du-compte.md) + [0018](../architecture/adr/0018-effacement-du-compte-dans-les-services.md) | Suppression et conservation |
| [ADR-0015](../architecture/adr/0015-controle-acces-stockage-objet.md) | Médias publics / privés |
| [ADR-0019](../architecture/adr/0019-feed-trending-public.md) | Fil tendances sans compte |

## 2. Informations que le produit n’a pas (à fournir au conseil)

- Raison sociale, forme, capital, immatriculation, siège, représentant.
- Contact support (e-mail / téléphone) et DPO le cas échéant.
- Hébergeur staging / production (non choisi).
- Âge minimum d’inscription.
- Date d’effet et numéro de version opposable.
- Variante anglaise Nigeria (Q-11, hors v1 FR).

**Ne pas inventer** ces champs pour « remplir » les mentions.

## 3. Questions à trancher par le conseil

1. Droit applicable et for pour quatre pays (OHADA / common law NG).
2. Qualification hébergeur vs éditeur des contenus utilisateurs.
3. Âge minimum et capacité (dont Nigeria).
4. Base légale des traitements (contrat, intérêt légitime, obligation) ;
   le brouillon ne pose pas de case RGPD (pas de SDK pub).
5. Affichage de l’IP des sessions (Q-09).
6. Durée et base de conservation des commandes après effacement (Q-19,
   proposition interne : acte uniforme comptable OHADA, ~10 ans).
7. Encaissement pour compte de vendeurs si Wave / paiement en ligne
   (agrément BCEAO / COBAC / CBN — ADR-0012).
8. Licence de diffusion HLS et interdiction d’enregistrer une photo
   produit (Q-MED-01 / 02) : le brouillon refuse l’action in-app.
9. COD présenté comme « non payé » : formulation opposable.
10. Obligation ou non d’une case d’acceptation à l’inscription
    (`accepted_tos_at` n’existe pas).

## 4. Interdit tant que le visa n’est pas signé

- Présenter les textes dans l’app comme conditions en vigueur.
- Soumettre l’APK aux stores avec ces corps.
- Cocher Q-03 « clos » dans l’état d’exécution.
- Livrer le lot M3 (Enregistrer une photo).

## 5. Après visa

1. Numéroter (v1.0) et dater l’effet.
2. Compléter les mentions (identité réelle).
3. Autoriser **séparément** l’embedding dans Android Studio.
4. Conserver ce dossier et les brouillons comme historique.
