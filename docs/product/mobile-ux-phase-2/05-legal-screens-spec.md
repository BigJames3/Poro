# Axe C — Spécification des écrans légaux

**Aucun texte juridique n’existe dans le dépôt** (recherche `privacy`, `terms`, `cgu`, `legal` : 0 fichier produit). ADR-0004 cite les lois CI / SN / CM / NG sans fournir de clauses. Tout contenu ci-dessous est un **placeholder marqué « À valider »**, pas une politique publiable.

## 1. Objectif

Permettre la consultation in-app des documents exigés par les stores et les lois de lancement, **lorsque des textes validés seront fournis**, et afficher un état d’indisponibilité honnête en attendant.

## 2. Documents visés

| ID | Document | Source actuelle | Version / date d’effet |
|---|---|---|---|
| LEG-TOS | Conditions d’utilisation | Absente | Absentes |
| LEG-PRIVACY | Politique de confidentialité | Absente | Absentes |
| LEG-MENTIONS | Mentions légales / éditeur | Absentes | Absentes |

Pas de consentement cookies / tracking dans le code (pas de SDK pub). **Ne pas inventer une case RGPD** tant que le juridique ne l’exige pas (Q-03).

La suppression de compte (ADR-0013) reste le parcours stores « delete account » déjà livré ; les écrans légaux ne le remplacent pas.

## 3. Modèle de données recommandé (app)

Tant qu’il n’y a pas d’API `GET /legal`, **ne pas créer de backend**. Contenu embarqué ou URL distante **À valider**.

Structure interne prévue :

```
LegalDocument(id, title, version: String?, effectiveAt: String?, bodyMarkdown: String?, unavailable: Boolean)
```

- Si `unavailable == true` ou `bodyMarkdown` null : écran d’indisponibilité, **pas** un faux texte.
- `version` / `effectiveAt` : n’afficher la ligne que si le juridique les fournit.
- Langue : FR uniquement (comme l’app). Variante EN Nigeria : Q-11, hors phase 2.

Placeholder body (à n’afficher **que** dans un build debug / flavour staging, jamais comme politique Play) :

> **À valider — ce texte n’a aucune valeur juridique.**  
> Les conditions d’utilisation et la politique de confidentialité de Poro seront publiées ici après validation.

## 4. Écrans

### UI-LEG-001 — Liste (`LegalRoute`)

| | |
|---|---|
| Objectif | Choisir un document |
| Entrées | Paramètres (`SettingsScreen`, section à ajouter) ; pied des écrans auth (`PhoneEntryScreen` / register) en `TextButton` 48 dp |
| Sortie | `LegalDocumentRoute(id)` ; back |
| Auth | Public (utile au visiteur et à l’inscription) |
| Liste | TOS, confidentialité, mentions |
| Vide / indispo | Chaque ligne reste tappable → écran document indisponible |

### UI-LEG-002 — Document (`LegalDocumentRoute`)

| État | UI |
|---|---|
| Chargement | Si fetch URL futur ; aujourd’hui contenu local immédiat |
| Disponible | Titre, version/date **si présentes**, corps scrollable |
| Indisponible | Titre + message « Document non encore validé. » + pas de clauses |
| Erreur réseau | Uniquement si source distante plus tard + Réessayer |

Pas de WebView obligatoire : `Text` Compose / markdown simple. Si le juridique livre une **URL HTTPS** canonique : `CustomTabs` (navigateur), pas de WebView interne produit.

Impression / partage : hors scope.

### Consentement

| Question | État |
|---|---|
| Case obligatoire à l’inscription | **Non établie** dans les règles produit actuelles |
| Blocage de l’app sans acceptation | Non |
| Journal d’acceptation côté auth | Pas d’API |

**Recommandation** : lien consultable depuis auth, **sans** bloquer OTP/register tant que le juridique n’impose pas un `accepted_tos_at`. Si imposé plus tard : case + version, à spécifier dans un complément — pas dans cette spec.

## 5. Navigation

Ajouter deux routes typées au graphe **existant** (`profileGraph` ou petit `legalGraph` appelé depuis auth et settings). Pas de nouvel onglet. Accessible **sans session** (nécessaire avant Q-02 et pour Play).

Aujourd’hui `isSignInScreen()` exclut tout hors auth : une `LegalRoute` ouverte depuis Phone ne doit **pas** déclencher `goToSignIn()` quand `SignedOut`. Règle : traiter `Legal*` comme les écrans sign-in (accessibles déconnecté) **ou** les retirer de l’observateur session.

## 6. Accessibilité

Titre `heading()` ; corps `bodyLarge` ; contraste thème ; pas d’images texte ; taille système respectée (typo M3).

## 7. Critères

- Aucun paragraphe n’est présenté comme opposable tant que le fichier n’est pas marqué validé par le métier.
- Paramètres exposent les trois liens même si indisponibles.
- Delete account et sessions inchangés.
- Pas d’appel réseau légal tant qu’aucune URL n’est fournie.
