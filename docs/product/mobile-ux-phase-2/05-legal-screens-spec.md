# Axe C — Spécification des écrans légaux

Des **brouillons produit** existent dans `docs/legal/` (Q-03, 21 oct. 2026). Ils ne sont **pas opposables**. ADR-0004 cite les pays CI / SN / CM / NG. L’app Android continue d’afficher l’indisponibilité jusqu’au visa juridique et à l’identité éditeur.

## 1. Objectif

Permettre la consultation in-app des documents exigés par les stores et les lois de lancement, **lorsque des textes validés seront fournis**, et afficher un état d’indisponibilité honnête en attendant.

## 2. Documents visés

| ID | Document | Source actuelle | Version / date d’effet |
|---|---|---|---|
| LEG-TOS | Conditions d’utilisation | `docs/legal/01-conditions-utilisation.md` | 0.1-draft — **non opposable** |
| LEG-PRIVACY | Politique de confidentialité | `docs/legal/02-politique-confidentialite.md` | 0.1-draft — **non opposable** |
| LEG-MENTIONS | Mentions légales / éditeur | `docs/legal/03-mentions-legales.md` | 0.1-draft — identité éditeur **à compléter** |

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
