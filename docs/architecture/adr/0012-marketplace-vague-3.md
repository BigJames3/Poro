# ADR-0012 — Vague 3 : marketplace par événements, paiement à la livraison et Wave

- Statut : accepté
- Date : 2026-10-09

## Contexte

La Vague 3 ajoute shop (NestJS, 8090), order (NestJS, 8091), payment (8092) et
analytics (Python, 8093). La mission cite Wave, Orange Money et CinetPay ;
[ADR-0004](0004-decisions-produit-lancement.md) retenait Wave et GeniusPay.
Wave ne couvre que la Côte d'Ivoire et le Sénégal.

## Décisions

1. **Ordre des PR** : prérequis (contrats, rôle BUSINESS, topics), shop,
   order, sécurité CI (P0-11 : gosec, Trivy, Dependabot), payment, analytics
   minimal, puis notifications marketplace.
2. **Saga par événements, sans appel synchrone** :

   | Étape | Événement | Effet |
   |---|---|---|
   | Paiement du panier | `poro.order.order.created` | Une commande par boutique |
   | Réservation | `poro.shop.stock.reserved` / `.rejected` | Shop retient le stock et fixe les prix |
   | Commande passée | `poro.order.order.placed` | Payment connaît le montant ; expire après 30 min sans paiement |
   | Paiement | `poro.payment.payment.succeeded` / `.failed` | Commande payée |
   | Expédition | `poro.order.order.shipped` | Suivi en texte libre, pas de transporteur |
   | Fin | `poro.order.order.completed` | Réception confirmée ou 7 jours après expédition ; montant dû au vendeur |
   | Annulation | `poro.order.order.cancelled` | Shop libère le stock ; payment rembourse si `paid` |
   | Remboursement | `poro.payment.refund.succeeded` | |

   Le catalogue est projeté par `poro.shop.shop.updated`,
   `poro.shop.product.updated` et `poro.shop.product.deleted` (instantanés
   complets, sans stock). Les prix vus par l'acheteur sont indicatifs : seuls
   ceux de `stock.reserved` font foi.
3. **Montants** : entiers en unités mineures (XOF et XAF n'en ont pas, NGN en
   kobo). Devise fixée par le pays de la boutique : XOF (CI, SN), XAF (CM),
   NGN (NG). Une commande n'a qu'une devise.
4. **Moyens de paiement v1** : paiement à la livraison, Wave, et un
   fournisseur simulé refusé hors dev. Le montant vient toujours de la
   commande ; seul un webhook vérifié ou une interrogation serveur du
   fournisseur confirme un paiement. CinetPay (Cameroun) et un fournisseur
   nigérian viendront derrière la même interface `PaymentProvider`.
5. **payment en Go seul** (au lieu de Go + NestJS) : un seul langage pour le
   service le plus sensible, avec le socle partagé (outbox, JWT, observabilité).
6. **Boutique liée à un compte** : une boutique appartient toujours à un
   compte auth (`owner_id`, le `sub` du JWT de celui qui l'ouvre) et un compte
   possède au plus une boutique en v1. Il n'existe pas de compte boutique
   séparé : le vendeur se connecte avec son compte habituel. Auth accorde le
   rôle BUSINESS à ce compte à la réception de `poro.shop.shop.created`, comme
   CREATOR, et le conserve si la boutique ferme.
7. **Reversement aux vendeurs** : journal des montants dus ; virements faits
   à la main par un admin en v1.
8. **Analytics** : v1 minimale en fin de vague (ventes vendeurs, engagement
   créateurs). Aucun événement de vue n'existe encore.

## Conséquences

- Les services NestJS (shop, order) tiennent leur propre catalogue TypeScript
  des événements, comme user et notification ; payment utilise `shared-go/events`.
- Une commande payée peut être annulée par le vendeur : le remboursement passe
  par l'API du fournisseur, sinon il est signalé pour traitement manuel.
- **À valider par un juriste avant d'ouvrir le paiement en production** :
  encaisser pour le compte de vendeurs peut exiger un agrément (BCEAO en zone
  XOF, COBAC en zone XAF, CBN au Nigeria) ou un agrégateur avec reversement
  séparé. Des accès sandbox Wave sont nécessaires pour tester l'intégration réelle.

## Complément du 2026-10-09 — service order

- Coordonnées de livraison (nom, téléphone, ville, adresse) visibles par
  l'acheteur et le vendeur seulement, effacées 90 jours après la fin de la
  commande.
- Moyens de paiement activés par configuration (`PAYMENT_METHODS`) : paiement
  à la livraison seul tant que le service payment n'existe pas.
- Le prix réservé par shop fait foi ; l'acheteur peut annuler tant que rien
  n'est expédié.
- Pas d'achat dans sa propre boutique ; pas d'annulation après expédition en
  v1 (litiges et retours plus tard).
- Un paiement arrivé après l'annulation republie `order.cancelled` avec
  `paid: true` pour déclencher le remboursement.

## Complément du 2026-10-09 — passage en caisse v2

- Paiement à la livraison seul pour l'instant ; le service payment est reporté.
- Passage en caisse en deux temps : récapitulatif (`preview`), puis
  confirmation explicite (`confirm`). Les commandes ne naissent qu'à la
  confirmation, et seulement si le panier et les prix n'ont pas bougé.
- Coordonnées pré-remplies : carnet d'adresses d'order, sinon nom (user) et
  téléphone (auth) lus avec le jeton de l'acheteur. C'est le seul appel
  synchrone entre services : lecture seule, délai court, jamais bloquant.
- Un seul champ « Nom et prénom ». Obligatoires : nom, téléphone, ville, et
  position GPS ou adresse ; indications facultatives.
- Frais de livraison : seulement mentionnés comme payés au livreur.
