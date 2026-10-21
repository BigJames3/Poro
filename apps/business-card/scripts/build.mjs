#!/usr/bin/env node
// Builds the card into dist/: pages, vCard, manifest, service worker and the
// _headers file. Environment:
//   PUBLIC_URL    final https:// address of the card (canonical link, link
//                 previews, QR code). Optional in dev; required with --prod.
//   CARD_PROFILE  profile file, default profile.json.

import { createHash } from 'node:crypto';
import { cp, mkdir, readFile, readdir, rm, writeFile } from 'node:fs/promises';
import { dirname, join, relative, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';

import { headersFile } from '../src/lib/headers.js';
import { validateProfile, vcardFileName } from '../src/lib/profile.js';
import { renderCard, renderManifest, renderNotFound } from '../src/lib/render.js';
import { buildVCard } from '../src/lib/vcard.js';

const root = resolve(dirname(fileURLToPath(import.meta.url)), '..');
const dist = join(root, 'dist');
const prod = process.argv.includes('--prod');

/**
 * @param {string} msg
 * @returns {never}
 */
function fail(msg) {
  console.error(`build: ${msg}`);
  process.exit(1);
}

/** @param {string | undefined} raw */
function parsePublicUrl(raw) {
  if (!raw) {
    if (prod) fail('PUBLIC_URL est requis avec --prod (ex. https://carte.foodsco.ci/)');
    return undefined;
  }
  let url;
  try {
    url = new URL(raw);
  } catch {
    return fail(`PUBLIC_URL invalide : ${raw}`);
  }
  const local = ['localhost', '127.0.0.1'].includes(url.hostname);
  if (url.protocol !== 'https:' && !(local && !prod)) fail('PUBLIC_URL doit être en https://');
  if (url.search || url.hash) fail('PUBLIC_URL ne doit contenir ni ?requête ni #ancre');
  if (!url.pathname.endsWith('/')) url.pathname += '/';
  return url.href;
}

const profilePath = resolve(root, process.env.CARD_PROFILE ?? 'profile.json');
/** @type {import('../src/lib/profile.js').Profile} */
let profile;
try {
  profile = JSON.parse(await readFile(profilePath, 'utf8'));
} catch (e) {
  profile = fail(`lecture de ${relative(root, profilePath)} : ${/** @type {Error} */ (e).message}`);
}
const errors = validateProfile(profile);
if (errors.length) fail(`profil invalide :\n  - ${errors.join('\n  - ')}`);

const publicUrl = parsePublicUrl(process.env.PUBLIC_URL);
const basePath = publicUrl ? new URL(publicUrl).pathname : '/';

await rm(dist, { recursive: true, force: true });
await mkdir(join(dist, 'vendor'), { recursive: true });
await mkdir(join(dist, 'assets'), { recursive: true });

/** @type {Map<string, string | Buffer>} relative path → content, for the version hash */
const files = new Map();
/** @param {string} path @param {string | Buffer} content */
async function emit(path, content) {
  await mkdir(dirname(join(dist, path)), { recursive: true });
  await writeFile(join(dist, path), content);
  files.set(path, content);
}

// Static assets: icons, link preview image, optional photo.
const assetsDir = join(root, 'assets');
await cp(assetsDir, join(dist, 'assets'), { recursive: true });
for (const entry of await readdir(assetsDir, {
  recursive: true,
  withFileTypes: true,
})) {
  if (!entry.isFile()) continue;
  const rel = relative(root, join(entry.parentPath, entry.name)).split('\\').join('/');
  files.set(rel, await readFile(join(root, rel)));
}
for (const required of [
  'assets/icons/icon-192.png',
  'assets/icons/icon-512.png',
  'assets/og-image.jpg',
]) {
  if (!files.has(required)) fail(`${required} manquant : lancer npm run assets`);
}

let photo;
let photoUrl;
if (profile.person.photo) {
  const rel = `assets/${profile.person.photo}`;
  const buf = /** @type {Buffer | undefined} */ (files.get(rel));
  if (!buf) fail(`photo introuvable : ${rel}`);
  else {
    // Contact apps refuse or truncate large embedded photos.
    if (buf.length > 200 * 1024)
      fail(`${rel} dépasse 200 Ko : la réduire (ex. 400×400, JPEG qualité 80)`);
    photo = {
      base64: buf.toString('base64'),
      type: /** @type {'JPEG' | 'PNG'} */ (/\.png$/i.test(rel) ? 'PNG' : 'JPEG'),
    };
    photoUrl = `${basePath}${rel}`;
  }
}

// Theme: the profile's colours replace the defaults in the stylesheet.
let css = await readFile(join(root, 'src/styles.css'), 'utf8');
for (const [token, value] of [
  ['--brand', profile.theme.brand],
  ['--brand-strong', profile.theme.brandDark],
]) {
  const re = new RegExp(`(\\n\\s*${token}: )#[0-9a-fA-F]{6};`);
  if (!re.test(css)) fail(`jeton ${token} introuvable dans styles.css`);
  css = css.replace(re, `$1${value};`);
}
await emit('styles.css', css);

for (const name of ['app.js', 'qr.js', 'card-image.js']) {
  await emit(name, await readFile(join(root, 'src/client', name), 'utf8'));
}
await emit(
  'vendor/qrcode.mjs',
  await readFile(join(root, 'node_modules/qrcode-generator/dist/qrcode.mjs'), 'utf8'),
);

const vcfName = vcardFileName(profile);
await emit(vcfName, buildVCard(profile, { photo, cardUrl: publicUrl }));
await emit('_headers', headersFile(vcfName));
await emit('robots.txt', 'User-agent: *\nAllow: /\n');

// Pages reference assets with ?v=<hash of the assets>, so a new build is
// never served stale CSS or JS from an HTTP cache.
const hashOf = (/** @type {Iterable<[string, string | Buffer]>} */ entries) => {
  const h = createHash('sha256');
  for (const [path, content] of [...entries].sort(([a], [b]) => a.localeCompare(b)))
    h.update(path).update(content);
  return h.digest('hex').slice(0, 12);
};
const assetVersion = hashOf(files);
const opts = { basePath, publicUrl, version: assetVersion, photoUrl };
await emit('manifest.webmanifest', renderManifest(profile, opts));
await emit('index.html', renderCard(profile, opts));
await emit('404.html', renderNotFound(profile, opts));

const precache = [
  './',
  '404.html',
  'styles.css',
  'app.js',
  'qr.js',
  'card-image.js',
  'vendor/qrcode.mjs',
  'manifest.webmanifest',
  vcfName,
  'assets/icons/icon.svg',
  'assets/icons/icon-192.png',
  ...(photoUrl ? [`assets/${profile.person.photo}`] : []),
];
const version = hashOf(files);
const sw = (await readFile(join(root, 'src/sw.js'), 'utf8'))
  .replace("'__VERSION__'", JSON.stringify(version))
  .replace('/* PRECACHE */ []', JSON.stringify(precache));
if (sw.includes('__VERSION__') || sw.includes('/* PRECACHE */'))
  fail('src/sw.js : marqueurs de version introuvables');
await writeFile(join(dist, 'sw.js'), sw);

console.log(
  `build: dist/ prêt (version ${version}, ${files.size + 1} fichiers, ` +
    `URL publique : ${publicUrl ?? 'non définie, liens relatifs à l’hôte'})`,
);
