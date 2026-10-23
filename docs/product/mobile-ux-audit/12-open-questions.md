# Décisions produit ouvertes

Tout ce qui suit est **non tranché dans le code** ou **contradictoire entre documents**. Aucune hypothèse n’a été transformée en écran ou en API.

| ID | Question | Pourquoi ça bloque | Options constatées | Décideur probable |
|---|---|---|---|---|
| Q-01 | La marketplace est-elle dans le **premier** lancement public Android ? | ADR-0004 : P1. APK : onglet Boutique. Priorité P0 des écrans shop dépend de la réponse. | A : masquer l’onglet. B : garder COD. | Produit |
| Q-02 | Faut-il un **mode visiteur** (feed / vidéo / boutique sans compte) ? | **Tranché le 21 oct. 2026** : oui, via trending public ([ADR-0019](../../architecture/adr/0019-feed-trending-public.md)). Code feed **pas encore** modifié. G1 Android interdit tant que B-FEED n’est pas vert. | — | Produit (fait) |
| Q-03 | Où sont les **CGU, politique de confidentialité, mentions** des 4 pays ? | **Brouillons** dans `docs/legal/` (markdown local, v0.1-draft). Non opposables. Visa avocat + identité éditeur encore requis. Android reste « indisponible ». | Local versionné (retenu) vs URL. | Juridique (visa) |
| Q-04 | Que signifie **Devenir créateur** pour l’utilisateur ? | N’ajoute aucun droit de publication (ADR-0007). Badge + rôle JWT seulement. | Recopier le fait (« badge ») ou reporter le CTA jusqu’à un vrai bénéfice. | Produit |
| Q-05 | **ENTERPRISE** a-t-il un périmètre ? | Enum sans grant, sans UI, sans permission. | Reporter (recommandé) ou spécifier employés boutique (ADR-0001). | Produit |
| Q-06 | Un compte peut-il **lier téléphone et email** ? | Deux parcours d’identité disjoints, pas d’API de liaison. | Rester disjoint. Ou projet backend nouveau. | Produit + auth |
| Q-07 | L’email **doit-il** être vérifié pour vendre / publier ? | Aujourd’hui non. | Statu quo. Ou gate shop/upload. | Produit |
| Q-08 | Qui **suspend un compte** utilisateur et comment l’UI le dit ? | `SUSPENDED` sans API ; « contactez le support » sans canal. | SQL runbook. Ou API staff. Canal support **À confirmer**. | Ops + produit |
| Q-09 | Afficher l’**IP** des sessions ? | Donnée personnelle déjà dans `GET /sessions` et l’UI. | Garder (transparence). Masquer. | Juridique + UX |
| Q-10 | **Charte graphique** finale (logo, orange, typo) ? | `Theme.kt` se déclare provisoire. | Valider `#FF6A00` ou livrer une charte. | Marque |
| Q-11 | Langue UI **anglaise** pour le Nigeria ? | SMS +234 en EN, app en FR seulement. | FR partout. Ou `values-en` selon `country`/`language`. | Produit |
| Q-12 | `SHARE_BASE_URL` et domaine public ? | Partage sans lien. | Décider l’URL avant App Links. | Produit + infra |
| Q-13 | Faut-il une **caméra / enregistreur** in-app ? | Picker seulement, pas de permission caméra. | Picker suffisant au MVP (fait). Caméra = nouveau scope. | Produit |
| Q-14 | **Grille de vidéos** sur profil public ? | Pas d’API liste publique par auteur. | Attendre une API. Ne pas scraper le feed. | Backend + produit |
| Q-15 | Signalement : accusé et **délai** affichés ? | API sans SLA. | Confirmation minimale sans promesse. | Produit + modération |
| Q-16 | Les **admin/modérateurs** doivent-ils avoir une app distincte ? | Routes staff sur la gateway publique. | Interdire dans l’app grand public (recommandé). App staff plus tard. | Sécurité + produit |
| Q-17 | **Paiement à la livraison** dans les 4 pays, y compris Nigeria ? | COD est le seul moyen ; Wave ne couvre que CI/SN (ADR-0012). | COD 4 pays. Ou masquer shop au NG jusqu’à un provider. | Produit + juridique |
| Q-18 | Frais de livraison : **toujours** « payés au livreur » ? | Pas de calcul. | Statu quo v1. PORO-044 plus tard. | Produit |
| Q-19 | Durée de **conservation des commandes** après effacement | ADR-0018, validation juridique ouverte. | Ne pas afficher « tout est effacé » de façon absolue. | Juridique |
| Q-20 | ApplicationId Play Store définitif ? | Code : `com.poro.app`. Docs d’audit anciens : `com.example.myporo`. | Confirmer `com.poro.app`. | Mobile |
| Q-21 | Firebase / projet FCM ? | Backend devices prêt ; app sans projet. | Créer le projet ou retirer le switch push jusqu’alors. | Mobile + ops |
| Q-22 | `full_name` d’inscription : bug ou intention ? | Auth jette le nom. | Alimenter `display_name` user **ou** retirer le champ inscription. | Produit + backend |
| Q-23 | Alignement OpenAPI vs limites réelles (bio, titre vidéo) | UI trop large. | Corriger OpenAPI + Android (recommandé), pas élargir le serveur sans ADR. | Backend + mobile |
| Q-24 | Pays du profil (`country_code`) | Jamais posé à l’OTP. | Déduire de l’indicatif. Ou demander. Ou laisser null. | Produit + user |
| Q-25 | Hashtag : **page** dédiée ou recherche suffisante ? | Tap = nouvelle search. PORO-039 tendances. | Statu quo P2. | Produit |

## Hypothèses **non** retenues comme faits

- « Poro est déjà commercialisable tel quel » — non : report, legal, SMS réel, staging.
- « Créateur = seul habilités à publier » — faux (ADR-0007).
- « BUSINESS est exigé pour `POST /shops` » — faux.
- « Wave est disponible dans le checkout » — faux.
- « L’Android est un gabarit vide » — faux depuis le code actuel.
- « PENDING_DELETION est un status » — faux (`deletion_scheduled_at`).

## Documents à consolider (pas dupliquer)

| Document | Problème | Action proposée |
|---|---|---|
| `docs/api/ANDROID-INTEGRATION.md` | Dit gabarit vide, `com.example.myporo` | Mettre à jour l’état client ; garder les règles jetons/upload |
| `docs/architecture/00-etat-des-lieux.md` | Android « 36 fichiers, aucune API » | Recaler au prochain jalon (PORO-032) |
| `docs/audits/AUDIT-ETAT-AVANCEMENT.md` | 14/10/2026, Android 0 %, PORO-002/003 ensuite faits | Tamponner « historique » ; pointer cet audit UX pour le client |
| `.cursor/rules/project.md` | Marketplace « on ne fait pas encore » vs ADR-0004/0012 et code shop | Aligner les règles Cursor sur les ADR |
| Cette série `docs/product/mobile-ux-audit/` | Référence UX/UI mobile | Source pour designers et Android ; ne pas y recopier les OpenAPI |

## Informations impossibles à vérifier ici

- Comportement Africa’s Talking réel (tarifs, Sender ID) — ADR-0004.
- Contraste overlay sur une vidéo claire **sur appareil**.
- Gestes TalkBack vs pager vertical **sur appareil**.
- Timeouts OkHttp exacts (non relus ligne à ligne dans cet audit : **À confirmer**).
- Politique Play / App integrity / Play Integrity pour l’anti-SMS pumping (PORO-005 défi).
- Identité visuelle hors `#FF6A00` du thème.
