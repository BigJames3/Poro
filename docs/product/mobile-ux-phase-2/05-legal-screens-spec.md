# Axe C2 — Spécification des écrans légaux

## 1. État réel

| Élément | Constat |
|---|---|
| Textes validés | **Aucun** dans le dépôt (CGU, politique de confidentialité, mentions, règles de la communauté, conditions de vente) |
| API de textes légaux | **Aucune** (`docs/api/*` et code des services : 0 occurrence de `terms`, `privacy`, `consent`, `cgu`) |
| Enregistrement du consentement | **Aucun** (ni route, ni table) |
| Site web | Aucun ; `SHARE_BASE_URL` vide |
| Écran Paramètres | `ui/profile/SettingsScreen.kt` : sessions, notifications, déconnexion, suppression du compte |

Conséquence : les écrans peuvent être préparés avec des **textes locaux provisoires**, mais aucun
consentement ne peut être prouvé côté serveur. **Un recueil de consentement « à l'inscription » ne
doit pas être présenté comme opposable tant que le backend ne l'enregistre pas** (décision L1, `10`).

## 2. Documents à prévoir

| Document | Identifiant | Accès | Statut du texte |
|---|---|---|---|
| Conditions d'utilisation | `terms` | Paramètres, écrans de connexion | « À valider » |
| Politique de confidentialité | `privacy` | Paramètres, écrans de connexion, suppression de compte | « À valider » |
| Règles de la communauté | `community` | Paramètres, feuille de signalement | « À valider » |
| Conditions de vente de la marketplace | `marketplace` | Paramètres, récapitulatif de commande, ouverture de boutique | « À valider » |

Liste à confirmer par le juridique (décision L2).

## 3. Écrans

### 3.1 Liste « Informations légales » (`LegalIndexRoute`)

| Rubrique | Contenu |
|---|---|
| Objectif | Retrouver chaque document en deux appuis depuis les Paramètres |
| Entrée | Paramètres → « Informations légales » ; liens des écrans de connexion |
| Données | Liste locale des documents (titre, version, date d'effet si connue) |
| États | Toujours disponible (local) ; un document sans texte validé affiche le bandeau « À valider » |
| Accessibilité | Titre `heading`, lignes de 48 dp minimum, ordre de lecture linéaire |

### 3.2 Document (`LegalDocumentRoute(id)`)

| Rubrique | Contenu |
|---|---|
| Objectif | Lire un texte, connaître sa version et sa date d'effet |
| Données | Titre, version, date d'effet, sections (titre + paragraphes), statut `draft`/`validated` |
| États | **Brouillon** : bandeau visible et non masquable « Texte provisoire — À valider, sans valeur juridique » ; **Absent** : « Ce document sera bientôt disponible » ; **Validé** : sans bandeau |
| Lecture | Texte natif Compose (pas de WebView), taille de police système respectée, défilement, sélection du texte possible |
| Accessibilité | Chaque section en `heading` ; contraste AA ; pas de texte dans des images |

### 3.3 Mention à l'inscription (provisoire)

Sur `PhoneEntryScreen` et `EmailRegisterScreen` : une ligne d'information, **sans case à cocher**,
« En continuant, vous acceptez les Conditions d'utilisation et la Politique de confidentialité »
avec deux liens. Statut : à valider (décision L1). Aucune donnée n'est envoyée.

## 4. Source des textes

| Option | Avantages | Limites | Recommandation |
|---|---|---|---|
| A. Ressources locales (`res/raw/legal_*.md` ou `strings`) versionnées dans l'app | Disponible hors ligne ; simple | Mise à jour = nouvelle version de l'app | **Pour la bêta** |
| B. Endpoint backend `GET /api/v1/legal/{id}` (version, date, contenu) + enregistrement du consentement | Mise à jour sans publication ; preuve d'acceptation | N'existe pas : service à créer | Avant le lancement public, si le juridique exige une preuve |

Format proposé pour l'option A (sans clause inventée) :

```markdown
---
id: privacy
version: 0.1-draft
effective: null          # date d'effet inconnue tant que non validé
status: draft            # draft | validated
---
# Politique de confidentialité
> Texte provisoire — À valider.
## 1. [Section à rédiger par le juridique]
[À valider]
```

## 5. Critères d'acceptation

1. Depuis les Paramètres, chaque document s'ouvre en deux appuis au plus.
2. Un document `draft` affiche toujours le bandeau « À valider » ; aucun texte provisoire n'apparaît sans ce bandeau.
3. Un document absent affiche l'état d'indisponibilité, sans écran vide.
4. Version et date d'effet sont affichées quand elles existent, « non définie » sinon.
5. Avec la taille de police système à 200 %, le texte reste lisible sans coupure horizontale.
6. Aucune case « J'accepte » n'est ajoutée tant que la décision L1 n'est pas prise.
