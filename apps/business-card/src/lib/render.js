// Renders the static pages. Everything a visitor needs (name, coordinates,
// call / e-mail / WhatsApp links, the vCard) works without JavaScript; the
// script only adds sharing, the QR code and the image export.

import { META_CSP } from './headers.js';
import { FILLED, STROKE } from './icons.js';
import {
  SOCIAL_NETWORKS,
  displayUrl,
  fullName,
  initials,
  mailtoHref,
  mapsHref,
  organizationName,
  telHref,
  vcardFileName,
  whatsappHref,
} from './profile.js';

/** @param {unknown} value */
export function esc(value) {
  return String(value)
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;')
    .replace(/'/g, '&#39;');
}

/**
 * JSON for a <script type="application/json"> block: "</script>" or "<!--"
 * in a value must not end the block.
 * @param {unknown} data
 */
export function inlineJson(data) {
  return JSON.stringify(data)
    .replace(/</g, '\\u003c')
    .replace(/>/g, '\\u003e')
    .replace(/&/g, '\\u0026');
}

/** @param {keyof typeof STROKE} name */
function icon(name) {
  return `<svg class="i" viewBox="0 0 24 24" aria-hidden="true" focusable="false"><path d="${STROKE[name]}"/></svg>`;
}

/** @param {keyof typeof FILLED} name */
function mark(name) {
  const box = name === 'whatsapp' ? '0 0 24 24' : '-2 -2 28 28';
  return `<svg class="i i--fill" viewBox="${box}" aria-hidden="true" focusable="false"><path d="${FILLED[name]}"/></svg>`;
}

const EXTERNAL = 'target="_blank" rel="noopener noreferrer"';

/**
 * @typedef {{ basePath: string, publicUrl?: string, version: string, photoUrl?: string }} RenderOptions
 */

/**
 * Data the browser script needs; the page is its only source.
 * @param {import('./profile.js').Profile} p
 * @param {RenderOptions} o
 */
export function clientData(p, o) {
  const c = p.contact;
  return {
    name: fullName(p),
    title: p.person.title,
    organization: p.organization.name,
    legalForm: p.organization.legalForm ?? '',
    footer: footerText(p),
    initials: initials(p),
    theme: p.theme,
    publicUrl: o.publicUrl ?? null,
    imageFileName: `carte-de-visite-${p.slug}.png`,
    rows: [
      c.phone && { icon: 'phone', label: 'Téléphone', value: c.phone.display },
      c.email && { icon: 'mail', label: 'E-mail', value: c.email },
      p.organization.website && {
        icon: 'globe',
        label: 'Site web',
        value: displayUrl(p.organization.website),
      },
      c.address && { icon: 'pin', label: 'Adresse', value: c.address.display },
    ].filter(Boolean),
    icons: STROKE,
  };
}

/** @param {import('./profile.js').Profile} p */
function footerText(p) {
  const place = [p.organization.city, p.organization.country].filter(Boolean).join(', ');
  return [organizationName(p), place].filter(Boolean).join(' · ');
}

/** @param {import('./profile.js').Profile} p @param {RenderOptions} o */
function head(p, o, { title = '', description = '', noindex = false } = {}) {
  const b = o.basePath;
  const v = `?v=${o.version}`;
  const abs = (/** @type {string} */ path) =>
    o.publicUrl ? new URL(path.replace(/^\//, ''), o.publicUrl).href : null;
  const ogImage = abs('assets/og-image.jpg');
  return `<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1, viewport-fit=cover">
<meta http-equiv="Content-Security-Policy" content="${META_CSP}">
<meta name="referrer" content="strict-origin-when-cross-origin">
<title>${esc(title)}</title>
<meta name="description" content="${esc(description)}">
${noindex ? '<meta name="robots" content="noindex">\n' : ''}<meta name="color-scheme" content="light dark">
<meta name="theme-color" content="${esc(p.theme.brand)}" media="(prefers-color-scheme: light)">
<meta name="theme-color" content="#121316" media="(prefers-color-scheme: dark)">
${o.publicUrl && !noindex ? `<link rel="canonical" href="${esc(o.publicUrl)}">\n<meta property="og:url" content="${esc(o.publicUrl)}">\n` : ''}<meta property="og:type" content="profile">
<meta property="og:title" content="${esc(title)}">
<meta property="og:description" content="${esc(description)}">
<meta property="og:locale" content="${esc(p.lang === 'fr' ? 'fr_FR' : p.lang)}">
${ogImage ? `<meta property="og:image" content="${esc(ogImage)}">\n<meta property="og:image:width" content="1200">\n<meta property="og:image:height" content="630">\n<meta name="twitter:card" content="summary_large_image">\n` : ''}<link rel="manifest" href="${b}manifest.webmanifest${v}">
<link rel="icon" href="${b}assets/icons/icon.svg" type="image/svg+xml">
<link rel="icon" href="${b}assets/icons/icon-192.png" type="image/png" sizes="192x192">
<link rel="apple-touch-icon" href="${b}assets/icons/apple-touch-icon.png">
<meta name="apple-mobile-web-app-title" content="${esc(p.person.givenName.split(' ')[0])}">
<link rel="stylesheet" href="${b}styles.css${v}">`;
}

/**
 * @param {import('./profile.js').Profile} p
 * @param {RenderOptions} o
 */
export function renderCard(p, o) {
  const b = o.basePath;
  const c = p.contact;
  const name = fullName(p);
  const title = `${name} – ${organizationName(p)}`;
  const description = `${p.person.title} – ${organizationName(p)}${p.organization.city ? `, ${p.organization.city}` : ''}.`;

  const quick = [
    c.phone &&
      `<a class="quick__btn" href="${esc(telHref(c.phone.e164))}">${icon('phone')}<span>Appeler</span></a>`,
    c.whatsapp &&
      `<a class="quick__btn" href="${esc(whatsappHref(c.whatsapp))}" ${EXTERNAL} aria-label="WhatsApp (nouvel onglet)">${mark('whatsapp')}<span>WhatsApp</span></a>`,
    c.email &&
      `<a class="quick__btn" href="${esc(mailtoHref(c.email))}">${icon('mail')}<span>E-mail</span></a>`,
    c.address &&
      `<a class="quick__btn" href="${esc(mapsHref(c.address))}" ${EXTERNAL} aria-label="Itinéraire (nouvel onglet)">${icon('pin')}<span>Itinéraire</span></a>`,
  ].filter(Boolean);

  /** @param {string} href @param {keyof typeof STROKE} ic @param {string} label @param {string} value @param {boolean} [external] */
  const row = (href, ic, label, value, external = false) =>
    `<li><a class="detail" href="${esc(href)}"${external ? ` ${EXTERNAL}` : ''}><span class="detail__icon">${icon(ic)}</span><span class="detail__text"><span class="detail__label">${esc(label)}</span><span class="detail__value">${esc(value)}</span></span></a></li>`;
  const details = [
    c.phone && row(telHref(c.phone.e164), 'phone', 'Téléphone', c.phone.display),
    c.email && row(mailtoHref(c.email), 'mail', 'E-mail', c.email),
    p.organization.website &&
      row(p.organization.website, 'globe', 'Site web', displayUrl(p.organization.website), true),
    c.address && row(mapsHref(c.address), 'pin', 'Adresse', c.address.display, true),
  ].filter(Boolean);

  const socials = p.socials.length
    ? `<section class="socials" aria-labelledby="socials-title">
      <h2 class="section-title" id="socials-title">Réseaux sociaux</h2>
      <ul class="socials__list">${p.socials
        .map((s) => {
          const label = SOCIAL_NETWORKS[/** @type {keyof typeof SOCIAL_NETWORKS} */ (s.network)];
          return `<li><a class="social" href="${esc(s.url)}" ${EXTERNAL} aria-label="${esc(label)} (nouvel onglet)">${mark(/** @type {keyof typeof FILLED} */ (s.network))}</a></li>`;
        })
        .join('')}</ul>
    </section>`
    : '';

  const avatar = o.photoUrl
    ? `<img class="avatar" src="${esc(o.photoUrl)}" alt="Photo de ${esc(name)}" width="112" height="112" decoding="async">`
    : `<div class="avatar avatar--initials" aria-hidden="true">${esc(initials(p))}</div>`;

  const jsonLd = {
    '@context': 'https://schema.org',
    '@type': 'Person',
    name,
    givenName: p.person.givenName,
    familyName: p.person.familyName,
    jobTitle: p.person.title,
    worksFor: {
      '@type': 'Organization',
      name: organizationName(p),
      ...(p.organization.website && { url: p.organization.website }),
    },
    ...(c.phone && { telephone: c.phone.e164 }),
    ...(c.email && { email: c.email }),
    ...(c.address && {
      address: {
        '@type': 'PostalAddress',
        streetAddress: c.address.street ?? c.address.display,
        addressLocality: c.address.locality,
        addressCountry: c.address.country,
      },
    }),
    ...(o.publicUrl && { url: o.publicUrl }),
    ...(p.socials.length && { sameAs: p.socials.map((s) => s.url) }),
  };

  return `<!doctype html>
<html lang="${esc(p.lang)}">
<head>
${head(p, o, { title, description })}
<script type="module" src="${b}app.js?v=${o.version}"></script>
<script type="application/ld+json">${inlineJson(jsonLd)}</script>
<script type="application/json" id="card-data">${inlineJson(clientData(p, o))}</script>
</head>
<body>
<main class="page">
  <article class="card" aria-labelledby="card-name">
    <header class="hero">
      <p class="logo"><span class="logo__name">${esc(p.organization.name)}</span>${p.organization.legalForm ? ` <span class="logo__form">${esc(p.organization.legalForm)}</span>` : ''}</p>
    </header>
    <div class="identity">
      ${avatar}
      <h1 class="identity__name" id="card-name">${esc(name)}</h1>
      <p class="identity__title">${esc(p.person.title)}</p>
      <p class="identity__org">${esc(organizationName(p))}${p.organization.city ? ` · ${esc(p.organization.city)}` : ''}</p>
    </div>
    ${quick.length ? `<nav class="quick" aria-label="Contacter ${esc(name)}">${quick.join('')}</nav>` : ''}
    <div class="actions">
      <a class="btn btn--primary" href="${b}${esc(vcardFileName(p))}" data-action="save">${icon('userPlus')}<span>Enregistrer le contact</span></a>
      <div class="actions__row" data-js-only hidden>
        <button class="btn btn--secondary" type="button" data-action="share">${icon('share')}<span>Partager</span></button>
        <button class="btn btn--secondary" type="button" data-action="qr">${icon('qr')}<span>QR code</span></button>
      </div>
    </div>
    <section class="details" aria-labelledby="details-title">
      <h2 class="section-title" id="details-title">Coordonnées</h2>
      <ul class="details__list">${details.join('')}</ul>
    </section>
    ${socials}
    <footer class="card__foot">
      <p>${esc(footerText(p))}</p>
      <button class="link-btn" type="button" data-action="image" data-js-only hidden>${icon('download')}<span>Télécharger l’image de la carte</span></button>
    </footer>
  </article>
</main>
<dialog class="sheet" id="share-dialog" aria-labelledby="share-title">
  <div class="sheet__head">
    <h2 class="sheet__title" id="share-title">Partager la carte</h2>
    <button class="icon-btn" type="button" data-action="close" aria-label="Fermer">${icon('close')}</button>
  </div>
  <p class="sheet__hint">Faites scanner ce QR code pour ouvrir la carte sur un autre téléphone.</p>
  <div class="qr" id="qr" role="img" aria-label="QR code du lien de la carte"><span class="qr__loading">Génération du QR code…</span></div>
  <label class="field-label" for="share-url">Lien de la carte</label>
  <div class="link-field">
    <input class="link-field__input" id="share-url" type="url" readonly>
    <button class="btn btn--secondary btn--compact" type="button" data-action="copy">${icon('copy')}<span>Copier</span></button>
  </div>
  <button class="btn btn--primary" type="button" data-action="native-share" hidden>${icon('share')}<span>Partager via…</span></button>
</dialog>
<div class="toast" id="toast" role="status" aria-live="polite"></div>
</body>
</html>
`;
}

/**
 * Served by the host for any unknown path (a mistyped or outdated link).
 * @param {import('./profile.js').Profile} p
 * @param {RenderOptions} o
 */
export function renderNotFound(p, o) {
  const title = `Page introuvable – ${organizationName(p)}`;
  return `<!doctype html>
<html lang="${esc(p.lang)}">
<head>
${head(p, o, { title, description: 'Cette page n’existe pas.', noindex: true })}
</head>
<body>
<main class="page">
  <article class="card card--message" aria-labelledby="nf-title">
    <header class="hero hero--short">
      <p class="logo"><span class="logo__name">${esc(p.organization.name)}</span>${p.organization.legalForm ? ` <span class="logo__form">${esc(p.organization.legalForm)}</span>` : ''}</p>
    </header>
    <div class="message">
      <p class="message__code">404</p>
      <h1 class="message__title" id="nf-title">Cette page n’existe pas</h1>
      <p class="message__text">Le lien est peut-être incomplet ou a changé. La carte de visite reste disponible à son adresse habituelle.</p>
      <a class="btn btn--primary" href="${o.basePath}">${icon('arrowLeft')}<span>Voir la carte de ${esc(fullName(p))}</span></a>
    </div>
  </article>
</main>
</body>
</html>
`;
}

/**
 * @param {import('./profile.js').Profile} p
 * @param {RenderOptions} o
 */
export function renderManifest(p, o) {
  const name = fullName(p);
  return JSON.stringify(
    {
      name: `${name} – ${organizationName(p)}`,
      short_name: p.person.givenName.split(' ')[0],
      description: `Carte de visite de ${name}`,
      lang: p.lang,
      id: o.basePath,
      start_url: o.basePath,
      scope: o.basePath,
      display: 'standalone',
      background_color: '#eceef2',
      theme_color: p.theme.brand,
      icons: [
        {
          src: `${o.basePath}assets/icons/icon-192.png`,
          sizes: '192x192',
          type: 'image/png',
        },
        {
          src: `${o.basePath}assets/icons/icon-512.png`,
          sizes: '512x512',
          type: 'image/png',
        },
        {
          src: `${o.basePath}assets/icons/icon-maskable-512.png`,
          sizes: '512x512',
          type: 'image/png',
          purpose: 'maskable',
        },
        {
          src: `${o.basePath}assets/icons/icon.svg`,
          sizes: 'any',
          type: 'image/svg+xml',
        },
      ],
    },
    null,
    2,
  );
}
