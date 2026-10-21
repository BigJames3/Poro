# Carte de visite numérique

Page web statique et PWA qui présente une carte de visite professionnelle :
identité, coordonnées, appel, WhatsApp, e-mail, itinéraire, enregistrement du
contact (vCard), partage du lien, QR code et export de la carte en image.
Elle reste consultable hors ligne après une première visite.

Refonte du fichier `index.html` d'origine (carte de Bintou Saïda DIABATÉ, Foods’ Co S.A.).

## Fonctionnement

```
profile.json ──► scripts/build.mjs ──► dist/
                   │                    ├─ index.html          carte (fonctionne sans JavaScript)
                   │                    ├─ 404.html            lien inexistant
                   │                    ├─ <slug>.vcf          fiche contact vCard 3.0
                   │                    ├─ app.js, qr.js, …    partage, QR code, image, hors ligne
                   │                    ├─ sw.js               service worker (cache versionné)
                   │                    ├─ manifest.webmanifest
                   │                    └─ _headers            en-têtes pour Netlify / Cloudflare Pages
                   └─ src/lib/*         validation, rendu HTML échappé, vCard, en-têtes
```

- **Une seule source de données** : `profile.json`. Le build la valide (téléphone
  au format international, e-mail, URL en `https://`, couleurs, réseaux connus) et
  s'arrête sur une erreur nommant le champ fautif.
- **Pas de serveur applicatif ni de base de données** : la carte est générée au
  build. Pour la modifier, on édite `profile.json`, on reconstruit et on redéploie.
- **Sans JavaScript**, la carte, les liens d'appel, d'e-mail, de WhatsApp, de
  carte et l'enregistrement du contact fonctionnent. Les boutons qui exigent du
  JavaScript (partage, QR code, image) restent masqués au lieu d'être inertes.
- **Partage** : feuille de partage native du téléphone (Web Share) ; sinon, fenêtre
  avec QR code et bouton « Copier ». Le lien partagé est `PUBLIC_URL`, ou à défaut
  l'adresse de la page sans paramètres.
- **QR code** : généré dans le navigateur (`qrcode-generator`, chargé à la demande),
  niveau de correction M, zone blanche de 4 modules, toujours noir sur blanc.
- **Image** : PNG 1080 px de large avec le QR code ; sur téléphone elle passe par
  la feuille de partage (enregistrement dans Photos), sur ordinateur elle se télécharge.
- **Hors ligne** : le service worker met en cache la page, la vCard, les scripts et
  les icônes ; la page est servie réseau d'abord, le reste cache d'abord.

## Commandes

Prérequis : Node.js 22 ou plus (la CI utilise Node 24).

```sh
cd apps/business-card
npm ci                  # installation
npm run dev             # build puis http://127.0.0.1:4173/
npm run build           # build de développement dans dist/
PUBLIC_URL=https://carte.foodsco.ci/ npm run build:prod   # build de production
npm run assets          # régénère icônes et image d'aperçu (après changement de nom ou de couleurs)

npm run format:check    # Prettier
npm run lint            # ESLint
npm run typecheck       # TypeScript sur le JavaScript (JSDoc)
npm test                # tests unitaires (node:test)
npm run test:e2e        # build + tests Playwright (Chromium bureau et mobile)
npm run check           # tout ce qui précède
```

`npm run assets` et `npm run test:e2e` utilisent Chromium via Playwright
(`npx playwright install chromium` une fois). Pour un Chromium déjà installé
ailleurs : `PLAYWRIGHT_CHROMIUM_PATH=/chemin/vers/chrome`.

## Modifier la carte

1. Éditer `profile.json` (champs décrits dans `src/lib/profile.js`).
   - `contact.phone.e164` / `contact.whatsapp.e164` : `+225…`, sans espaces.
   - `person.photo` : facultatif, chemin d'un JPEG ou PNG placé dans `assets/`
     (200 Ko au plus, carré de 400 px conseillé). Sans photo, l'avatar affiche
     les initiales.
   - `socials` : liste de `{ "network": "linkedin" | "facebook" | "instagram" | "x" | "youtube" | "tiktok", "url": "https://…" }`.
     Vide, la section n'est pas affichée.
   - `theme.brand` / `theme.brandDark` : couleurs de l'en-tête et des boutons.
2. Si le nom, l'entreprise ou les couleurs changent : `npm run assets`.
3. `npm run check`, puis redéployer.

Pour une autre personne : copier `profile.json`, puis `CARD_PROFILE=autre.json npm run build`.

## Déploiement

`PUBLIC_URL` (adresse finale, en `https://`) est exigé par `build:prod` : il sert
au lien canonique, aux aperçus de lien (WhatsApp, LinkedIn…), au QR code et à
la note de la vCard. Une adresse dans un sous-dossier (`https://exemple.ci/cartes/bintou/`)
est prise en charge.

### Docker (nginx)

```sh
docker build --build-arg PUBLIC_URL=https://carte.foodsco.ci/ -t poro-business-card .
docker run -p 8080:8080 poro-business-card     # http://localhost:8080, santé : /healthz
```

Image `nginx-unprivileged` (utilisateur non root, port 8080) avec les en-têtes de
`deploy/security-headers.conf`. Le TLS est terminé en amont (Traefik, répartiteur,
CDN).

### Hébergement statique (Netlify, Cloudflare Pages, GitHub Pages…)

Publier le dossier `dist/` produit par `build:prod`. Netlify et Cloudflare Pages
lisent `dist/_headers` (CSP, HSTS, type de la vCard). GitHub Pages ne permet pas
d'en-têtes : la CSP reste appliquée par la balise `<meta>`, mais pas HSTS ni
l'interdiction d'affichage dans un cadre.

### Contrôles avant mise en production

- [ ] `npm run check` passe ; `npm audit --omit=dev` sans vulnérabilité.
- [ ] `PUBLIC_URL` correct ; ouvrir l'URL et vérifier les en-têtes (`curl -I`).
- [ ] Aperçu du lien vérifié (ex. envoyer le lien dans WhatsApp).
- [ ] Sur un iPhone et un Android réels : appel, WhatsApp, enregistrement du
      contact, partage, scan du QR code avec l'appareil photo, installation
      sur l'écran d'accueil.
- [ ] Coordonnées relues avec la titulaire de la carte.

### Retour arrière

Le site est statique : redéployer le `dist/` (ou l'image Docker) de la version
précédente. Le service worker installe la nouvelle version au chargement suivant
et supprime l'ancien cache.

## Sécurité

- CSP stricte : uniquement des ressources du même domaine, aucun script ni style
  en ligne, aucun service tiers (polices système, pas de CDN, pas de traceur).
- Toutes les valeurs du profil sont échappées dans le HTML ; les données passées
  au script sont dans un bloc JSON qui ne peut pas fermer sa balise.
- Les URL du profil sont limitées à `https://` (pas de `javascript:`).
- Les liens externes s'ouvrent avec `rel="noopener noreferrer"`.
- En-têtes : HSTS, `nosniff`, `Referrer-Policy`, `Permissions-Policy`,
  `X-Frame-Options`/`frame-ancestors`, COOP, CORP. Un test vérifie que
  `deploy/security-headers.conf` et `_headers` envoient les mêmes valeurs que
  `src/lib/headers.js`.
- Les coordonnées publiées sont publiques par nature : ne rien mettre dans
  `profile.json` qui ne doive pas l'être.

## Tests

- **Unitaires** (`tests/unit`) : validation du profil, échappement vCard et
  repliement des lignes à 75 octets sans couper un caractère accentué,
  échappement HTML, absence de script en ligne, en-têtes, manifeste.
- **De bout en bout** (`tests/e2e`, Chromium bureau et Pixel 7) : contenu,
  liens d'appel / e-mail / WhatsApp / carte, vCard servie et valide, QR code
  décodé (jsQR) égal à l'URL de la carte, copie du lien, partage natif et repli,
  export PNG, fonctionnement sans JavaScript, page 404, en-têtes, accessibilité
  axe WCAG 2.1 AA en thème clair et sombre, navigation au clavier, hors ligne,
  manifeste ; 11 tailles d'écran de 280 à 1920 px (portrait et paysage) sans
  défilement horizontal, cibles tactiles de 44 px, zoom du texte à 200 %.

## Limites connues

- Firefox et Safari ne sont pas dans la CI (navigateurs non installés) ; ils
  s'ajoutent avec `PLAYWRIGHT_ALL_BROWSERS=1` après `npx playwright install`.
  Aucun test sur appareil iOS ou Android réel n'a été fait.
- Pas d'éditeur en ligne : la carte se modifie dans `profile.json`. Un éditeur
  avec comptes et base de données serait un autre produit (authentification,
  stockage des photos, modération) et n'est pas couvert ici.
- Aucune photo n'est fournie : l'avatar affiche les initiales. Ajouter une photo
  via `person.photo`.
- Outlook pour Windows lit mal les vCard 3.0 en UTF-8 : les accents peuvent y
  être altérés. iOS, Android, macOS et Gmail les lisent correctement.
