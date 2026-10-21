import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import { test } from 'node:test';

import {
  displayUrl,
  initials,
  mapsHref,
  telHref,
  validateProfile,
  whatsappHref,
} from '../../src/lib/profile.js';

const profile = JSON.parse(await readFile(new URL('../../profile.json', import.meta.url), 'utf8'));
const clone = () => structuredClone(profile);

test('the committed profile is valid', () => {
  assert.deepEqual(validateProfile(profile), []);
});

test('refuses malformed fields with a message naming the field', () => {
  /** @type {[(p: any) => unknown, string][]} */
  const cases = [
    [(p) => (p.contact.phone.e164 = '07 68 62 04 97'), 'contact.phone.e164'],
    [(p) => (p.contact.whatsapp.e164 = '2250768620497'), 'contact.whatsapp.e164'],
    [(p) => (p.contact.email = 'pas-un-email'), 'contact.email'],
    [(p) => (p.organization.website = 'http://www.foodsco.ci'), 'organization.website'],
    [(p) => (p.organization.website = 'javascript:alert(1)'), 'organization.website'],
    [(p) => (p.theme.brand = 'red'), 'theme.brand'],
    [(p) => (p.slug = 'Bintou Saïda'), 'slug'],
    [(p) => (p.person.givenName = '  '), 'person.givenName'],
    [(p) => (p.person.photo = '../../etc/passwd.jpg'), 'person.photo'],
    [
      (p) => (p.socials = [{ network: 'myspace', url: 'https://myspace.com/x' }]),
      'socials[0].network',
    ],
    [(p) => (p.socials = [{ network: 'linkedin', url: 'javascript:alert(1)' }]), 'socials[0].url'],
  ];
  for (const [mutate, field] of cases) {
    const p = clone();
    mutate(p);
    const errors = validateProfile(p);
    assert.ok(
      errors.some((e) => e.startsWith(field)),
      `${field} : ${JSON.stringify(errors)}`,
    );
  }
});

test('requires at least one way to reach the person', () => {
  const p = clone();
  delete p.contact.phone;
  delete p.contact.whatsapp;
  delete p.contact.email;
  assert.ok(validateProfile(p).some((e) => e.includes('au moins un moyen de contact')));
});

test('refuses a non-object profile', () => {
  assert.equal(validateProfile(null).length, 1);
});

test('derives the links the card uses', () => {
  assert.equal(telHref('+2250768620497'), 'tel:+2250768620497');
  assert.equal(
    whatsappHref(profile.contact.whatsapp),
    'https://wa.me/2250768620497?text=Bonjour%20Madame%20DIABAT%C3%89%2C%20je%20vous%20contacte%20depuis%20votre%20carte%20de%20visite%20Foods%E2%80%99%20Co.',
  );
  assert.equal(whatsappHref({ e164: '+2250768620497' }), 'https://wa.me/2250768620497');
  assert.ok(
    mapsHref(profile.contact.address).startsWith(
      'https://www.google.com/maps/search/?api=1&query=Foods',
    ),
  );
  assert.equal(displayUrl('https://www.foodsco.ci/'), 'www.foodsco.ci');
  assert.equal(initials(profile), 'BD');
});
