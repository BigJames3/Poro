import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import { test } from 'node:test';

import { buildVCard, escapeText, foldLine } from '../../src/lib/vcard.js';

const profile = JSON.parse(await readFile(new URL('../../profile.json', import.meta.url), 'utf8'));

/** Unfolds a vCard (RFC 2426 §2.6) into its logical lines. @param {string} v */
const unfold = (v) => v.replace(/\r\n /g, '').split('\r\n').filter(Boolean);

test('escapes TEXT special characters', () => {
  assert.equal(escapeText('a,b;c\\d\ne'), 'a\\,b\\;c\\\\d\\ne');
});

test('folds at 75 octets without splitting a UTF-8 character', () => {
  const line = `NOTE:${'É'.repeat(80)}`;
  const folded = foldLine(line);
  const enc = new TextEncoder();
  for (const [i, part] of folded.split('\r\n').entries()) {
    assert.ok(enc.encode(part).length <= 75, `ligne ${i} trop longue`);
    assert.ok(!part.includes('�'));
    if (i > 0) assert.ok(part.startsWith(' '));
  }
  assert.equal(folded.replace(/\r\n /g, ''), line);
});

test('builds a vCard 3.0 with every coordinate of the profile', () => {
  const v = buildVCard(profile);
  assert.ok(v.endsWith('\r\n'));
  assert.ok(!/[^\r]\n/.test(v), 'fins de ligne CRLF uniquement');
  const lines = unfold(v);
  assert.equal(lines[0], 'BEGIN:VCARD');
  assert.equal(lines[1], 'VERSION:3.0');
  assert.equal(lines.at(-1), 'END:VCARD');
  assert.ok(lines.includes('N:DIABATÉ;Bintou Saïda;;Madame;'));
  assert.ok(lines.includes('FN:Bintou Saïda DIABATÉ'));
  assert.ok(lines.includes('ORG:Foods’ Co S.A.'));
  assert.ok(
    lines.includes('TITLE:Responsable Customer service\\, Qualité et Coordination logistique'),
  );
  assert.ok(lines.includes('TEL;TYPE=CELL,VOICE,WORK:+2250768620497'));
  assert.ok(lines.includes('EMAIL;TYPE=INTERNET,WORK:responsablequalite@foodsco.ci'));
  assert.ok(lines.includes('URL;TYPE=WORK:https://www.foodsco.ci'));
  assert.ok(
    lines.some((l) => l.startsWith('ADR;TYPE=WORK:15 BP 292 Abidjan;Treichville;Zone 3\\, rue')),
  );
  // Same number for WhatsApp and phone: listed once.
  assert.equal(lines.filter((l) => l.startsWith('TEL')).length, 1);
});

test('adds the card link, the photo and the social profiles when present', () => {
  const p = structuredClone(profile);
  p.socials = [{ network: 'linkedin', url: 'https://www.linkedin.com/in/exemple' }];
  p.contact.whatsapp.e164 = '+2250102030405';
  const lines = unfold(
    buildVCard(p, {
      cardUrl: 'https://carte.example.ci/',
      photo: { base64: 'QUJD', type: 'JPEG' },
    }),
  );
  assert.ok(lines.includes('X-SOCIALPROFILE;TYPE=linkedin:https://www.linkedin.com/in/exemple'));
  assert.ok(lines.includes('NOTE:Carte de visite : https://carte.example.ci/'));
  assert.ok(lines.includes('PHOTO;ENCODING=b;TYPE=JPEG:QUJD'));
  assert.ok(lines.includes('TEL;TYPE=CELL:+2250102030405'));
});

test('omits absent coordinates instead of writing empty properties', () => {
  const p = structuredClone(profile);
  delete p.contact.address;
  delete p.organization.website;
  const v = buildVCard(p);
  assert.ok(!v.includes('ADR'));
  assert.ok(!v.includes('URL'));
});
