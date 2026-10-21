# Questions ouvertes

Ne pas traiter une question comme tranchée. Les Q-01…25 de [ux-audit/12](../mobile-ux-audit/12-open-questions.md) restent la liste longue. Ici : celles qui **bloquent** le mapping backend→UI.

| ID | Question | Impact mapping | Reco |
|---|---|---|---|
| Q-01 | Boutique au lancement ? | Phase 2 : **visible** | Garder COD |
| Q-02 / B-FEED | Fil visiteur ? | Accueil sans compte | Trending public + quota IP **ou** wall |
| Q-03 | Textes légaux | Écran présent, corps vide | Juridique |
| Q-04 | Sens « créateur » | Chip vs droit | Badge seulement |
| Q-09 | Afficher l’IP sessions ? | Settings | Décision PII |
| Q-12 | SHARE_BASE_URL | Share sans lien | Configurer avant App Links |
| Q-14 | Grille profil public | API owner-only | Attendre endpoint |
| Q-16 | App staff ? | cases / admin | Interdit dans l’APK public |
| Q-21 | Firebase | devices + switch | Labelliser ou brancher |
| Q-22 | full_name jeté | Register | Backend ou retirer le champ |
| Q-23 | Limites OpenAPI vs code | 422 surprise | Aligner UI sur le **code** |
| Q-MED-01/02 | Enregistrer une photo | M3 | Non tant que TOS |
| Q-MED-06 | Post image social | Fil mixte | Contrat + ADR |
| Q-19 | Conservation commandes | Copie delete account | ADR-0018 juridique |

**Tranchées pour cette série**

- Marketplace images only ; pas de vidéo produit.
- Social cible = vidéo **ou** image ; **livraison** image bloquée.
- Cinq tabs conservés.
- Staff hors app.
- Pas de simulation (Wave, chat, favoris, offline, save HLS).

**Non inventé**

Canal support (`account_blocked`), SMS/SMTP prod, hébergeur staging, charte finale (Q-10), `values-en` (Q-11).
