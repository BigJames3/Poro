// vCard 3.0 (RFC 2426): the version every current iOS, Android, macOS and
// Outlook contact importer reads. 4.0 is still refused by some Android OEM
// contact apps.

import { fullName, organizationName } from './profile.js';

/**
 * Escapes a TEXT value: backslash, comma, semicolon and line breaks.
 * @param {string} value
 */
export function escapeText(value) {
  return value
    .replace(/\\/g, '\\\\')
    .replace(/,/g, '\\,')
    .replace(/;/g, '\\;')
    .replace(/\r\n|\r|\n/g, '\\n');
}

/**
 * Folds a content line at 75 octets (RFC 2426 §2.6) without splitting a
 * UTF-8 sequence: "DIABATÉ" must not be cut inside its "É".
 * @param {string} line
 */
export function foldLine(line) {
  const encoder = new TextEncoder();
  const parts = [];
  let current = '';
  let size = 0;
  for (const char of line) {
    const bytes = encoder.encode(char).length;
    // Continuation lines start with one space, which counts in their 75.
    const limit = parts.length === 0 ? 75 : 74;
    if (size + bytes > limit) {
      parts.push(current);
      current = '';
      size = 0;
    }
    current += char;
    size += bytes;
  }
  parts.push(current);
  return parts.join('\r\n ');
}

/**
 * @param {import('./profile.js').Profile} p
 * @param {{ photo?: { base64: string, type: 'JPEG' | 'PNG' }, cardUrl?: string }} [extra]
 * @returns {string} the vCard, CRLF line endings
 */
export function buildVCard(p, extra = {}) {
  const { person, organization: org, contact } = p;
  const lines = [
    'BEGIN:VCARD',
    'VERSION:3.0',
    `N:${[person.familyName, person.givenName, '', person.honorific ?? '', ''].map(escapeText).join(';')}`,
    `FN:${escapeText(fullName(p))}`,
    `ORG:${escapeText(organizationName(p))}`,
    `TITLE:${escapeText(person.title)}`,
  ];
  if (contact.phone) lines.push(`TEL;TYPE=CELL,VOICE,WORK:${contact.phone.e164}`);
  if (contact.whatsapp && contact.whatsapp.e164 !== contact.phone?.e164) {
    lines.push(`TEL;TYPE=CELL:${contact.whatsapp.e164}`);
  }
  if (contact.email) lines.push(`EMAIL;TYPE=INTERNET,WORK:${escapeText(contact.email)}`);
  if (org.website) lines.push(`URL;TYPE=WORK:${org.website}`);
  if (contact.address) {
    const a = contact.address;
    // ADR: PO box; extended; street; locality; region; postal code; country.
    const adr = [a.poBox, a.district, a.street, a.locality, a.region, a.postalCode, a.country];
    lines.push(`ADR;TYPE=WORK:${adr.map((v) => escapeText(v ?? '')).join(';')}`);
  }
  for (const s of p.socials) lines.push(`X-SOCIALPROFILE;TYPE=${s.network}:${s.url}`);
  if (extra.cardUrl) lines.push(`NOTE:${escapeText(`Carte de visite : ${extra.cardUrl}`)}`);
  if (extra.photo) lines.push(`PHOTO;ENCODING=b;TYPE=${extra.photo.type}:${extra.photo.base64}`);
  lines.push('END:VCARD');
  return lines.map(foldLine).join('\r\n') + '\r\n';
}
