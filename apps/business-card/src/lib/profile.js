// Validation of profile.json and the values derived from it. Shared by the
// build (Node) and covered by unit tests: a malformed profile stops the build
// instead of publishing a card with a broken call, e-mail or map link.

/**
 * @typedef {{ network: string, url: string, label?: string }} Social
 * @typedef {{
 *   display: string, street?: string, district?: string, locality?: string,
 *   poBox?: string, postalCode?: string, region?: string, country?: string, mapsQuery?: string
 * }} Address
 * @typedef {{
 *   slug: string,
 *   lang: string,
 *   person: { honorific?: string, givenName: string, familyName: string, title: string, photo?: string | null },
 *   organization: { name: string, legalForm?: string, website?: string, city?: string, country?: string },
 *   contact: {
 *     phone?: { e164: string, display: string },
 *     whatsapp?: { e164: string, message?: string },
 *     email?: string,
 *     address?: Address
 *   },
 *   socials: Social[],
 *   theme: { brand: string, brandDark: string }
 * }} Profile
 */

/** Networks with an icon in the card; anything else is refused at build time. */
export const SOCIAL_NETWORKS = /** @type {const} */ ({
  linkedin: 'LinkedIn',
  facebook: 'Facebook',
  instagram: 'Instagram',
  x: 'X',
  youtube: 'YouTube',
  tiktok: 'TikTok',
});

const E164 = /^\+[1-9]\d{6,14}$/;
// Deliberately simple: one @, no spaces, a dot in the domain. RFC 5322 in
// full accepts addresses no mail client can open from a mailto: link.
const EMAIL = /^[^\s@<>"]+@[^\s@<>"]+\.[^\s@<>"]+$/;
const SLUG = /^[a-z0-9]+(?:-[a-z0-9]+)*$/;
const HEX = /^#[0-9a-f]{6}$/i;
// JPEG or PNG only: vCard 3.0 has no WebP type for the embedded photo.
const PHOTO = /^[a-z0-9][a-z0-9._/-]*\.(?:jpe?g|png)$/i;

/**
 * @param {unknown} value
 * @returns {value is string}
 */
function isText(value) {
  return typeof value === 'string' && value.trim().length > 0;
}

/** @param {unknown} value */
function isHttpsUrl(value) {
  if (typeof value !== 'string') return false;
  try {
    const url = new URL(value);
    return url.protocol === 'https:' && url.hostname.includes('.');
  } catch {
    return false;
  }
}

/**
 * Checks every field the page, the vCard and the links rely on.
 * @param {any} p
 * @returns {string[]} human-readable errors; empty when the profile is valid
 */
export function validateProfile(p) {
  /** @type {string[]} */
  const errors = [];
  /** @param {boolean} ok @param {string} msg */
  const check = (ok, msg) => {
    if (!ok) errors.push(msg);
  };
  if (!p || typeof p !== 'object') return ['le profil doit être un objet JSON'];

  check(typeof p.slug === 'string' && SLUG.test(p.slug), 'slug : minuscules, chiffres et tirets');
  check(isText(p.lang), 'lang : code de langue requis (ex. "fr")');

  const person = p.person ?? {};
  check(isText(person.givenName), 'person.givenName requis');
  check(isText(person.familyName), 'person.familyName requis');
  check(isText(person.title), 'person.title requis');
  check(
    person.photo == null ||
      (typeof person.photo === 'string' &&
        PHOTO.test(person.photo) &&
        !person.photo.includes('..')),
    'person.photo : chemin relatif vers un .jpg ou .png dans assets/',
  );

  const org = p.organization ?? {};
  check(isText(org.name), 'organization.name requis');
  check(
    org.website == null || isHttpsUrl(org.website),
    'organization.website : URL https:// requise',
  );

  const contact = p.contact ?? {};
  if (contact.phone != null) {
    check(E164.test(contact.phone.e164 ?? ''), 'contact.phone.e164 : format international +225…');
    check(isText(contact.phone.display), 'contact.phone.display requis');
  }
  if (contact.whatsapp != null) {
    check(
      E164.test(contact.whatsapp.e164 ?? ''),
      'contact.whatsapp.e164 : format international +225…',
    );
  }
  if (contact.email != null) check(EMAIL.test(contact.email), 'contact.email invalide');
  if (contact.address != null)
    check(isText(contact.address.display), 'contact.address.display requis');
  check(
    contact.phone != null || contact.email != null || contact.whatsapp != null,
    'au moins un moyen de contact (téléphone, e-mail ou WhatsApp) est requis',
  );

  check(Array.isArray(p.socials), 'socials : tableau requis (peut être vide)');
  for (const [i, s] of (Array.isArray(p.socials) ? p.socials : []).entries()) {
    check(Object.hasOwn(SOCIAL_NETWORKS, s?.network), `socials[${i}].network inconnu`);
    check(isHttpsUrl(s?.url), `socials[${i}].url : URL https:// requise`);
  }

  const theme = p.theme ?? {};
  check(HEX.test(theme.brand ?? ''), 'theme.brand : couleur #rrggbb');
  check(HEX.test(theme.brandDark ?? ''), 'theme.brandDark : couleur #rrggbb');
  return errors;
}

/** @param {Profile} p */
export function fullName(p) {
  return `${p.person.givenName} ${p.person.familyName}`;
}

/** @param {Profile} p */
export function organizationName(p) {
  const { name, legalForm } = p.organization;
  return legalForm ? `${name} ${legalForm}` : name;
}

/**
 * First letter of the first given name and of the family name, for the avatar.
 * @param {Profile} p
 */
export function initials(p) {
  const first = (/** @type {string} */ s) => Array.from(s.trim())[0] ?? '';
  return (first(p.person.givenName) + first(p.person.familyName)).toLocaleUpperCase(p.lang);
}

/** @param {string} e164 */
export function telHref(e164) {
  return `tel:${e164}`;
}

/** @param {{ e164: string, message?: string }} wa */
export function whatsappHref(wa) {
  const base = `https://wa.me/${wa.e164.slice(1)}`;
  return wa.message ? `${base}?text=${encodeURIComponent(wa.message)}` : base;
}

/** @param {string} email */
export function mailtoHref(email) {
  return `mailto:${email}`;
}

/** @param {Address} a */
export function mapsHref(a) {
  return `https://www.google.com/maps/search/?api=1&query=${encodeURIComponent(a.mapsQuery || a.display)}`;
}

/** Website shown without scheme or trailing slash: "www.foodsco.ci". @param {string} url */
export function displayUrl(url) {
  const u = new URL(url);
  return (u.host + u.pathname).replace(/\/$/, '');
}

/** File name offered when saving the contact, ASCII only for old Android browsers. @param {Profile} p */
export function vcardFileName(p) {
  return `${p.slug}.vcf`;
}
