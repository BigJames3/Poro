# Maquettes de direction

Ces images sont des **visuels de direction** générés à partir des specs, pas des captures Compose ni un prototype cliquable. L’acceptation se juge sur l’app et [10-design-acceptance-criteria.md](../10-design-acceptance-criteria.md), pas sur une superposition pixel.

| Fichier | Écran | Spec |
|---|---|---|
| `poro-design-system.jpg` | Tokens | [02](../02-design-system.md) |
| `poro-feed-home.jpg` | Fil Accueil | [05](../05-social-feed-design.md) |
| `poro-shop-home.jpg` | Hub boutique | [06](../06-marketplace-design.md) |
| `poro-product.jpg` | Fiche produit | [06](../06-marketplace-design.md) |
| `poro-checkout-review.jpg` | Récap COD | [06](../06-marketplace-design.md) |
| `poro-auth-phone.jpg` | Connexion téléphone | [03](../03-screen-redesign.md) |

## Écarts à ne pas implémenter

Les générateurs d’image ajoutent parfois du décor hors contrat. **Priorité au texte des specs.**

| Image | Hors spec | Règle |
|---|---|---|
| Shop home | Grande photo de marché en header | **Ne pas** ajouter : aucun asset ni API |
| Shop home | Champ « Rechercher un produit » | **Ne pas** : PORO-026 absent |
| Product | « Livraison à convenir » | **Ne pas** promettre un délai (pas de contrat) |
| Checkout | « Frais de livraison — Offerts » | **Ne pas** : les frais ne sont pas dans le total aujourd’hui |
| Design system | Hex hors tableau (`#C45C26`, etc.) | **Ignorer** ; seuls les tokens de [01](../01-visual-direction.md) §4 comptent |
| Feed | Casse « Pour Toi » | Label réel : `Pour toi` |

Ce qui **est** à viser dans les images : orange `#FF6A00`, vert succès, chrome sombre du fil, prix dominant, COD sans wallet, cinq onglets, un CTA par écran.
