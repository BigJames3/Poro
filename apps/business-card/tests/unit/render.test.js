import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import { test } from 'node:test';

import { CSP, META_CSP, SECURITY_HEADERS, headersFile } from '../../src/lib/headers.js';
import {
  esc,
  inlineJson,
  renderCard,
  renderManifest,
  renderNotFound,
} from '../../src/lib/render.js';

const profile = JSON.parse(await readFile(new URL('../../profile.json', import.meta.url), 'utf8'));
const opts = { basePath: '/', version: 'test' };

test('escapes HTML in text and attributes', () => {
  assert.equal(
    esc(`<img src=x onerror="a">'&`),
    '&lt;img src=x onerror=&quot;a&quot;&gt;&#39;&amp;',
  );
});

test('inline JSON cannot close its script block', () => {
  const out = inlineJson({ v: '</script><script>alert(1)</script><!--' });
  assert.ok(!out.includes('<'));
  assert.deepEqual(JSON.parse(out), {
    v: '</script><script>alert(1)</script><!--',
  });
});

test('profile values are escaped in the page', () => {
  const p = structuredClone(profile);
  p.person.title = '<script>alert(1)</script>';
  p.organization.name = '"><img src=x onerror=alert(1)>';
  const html = renderCard(p, opts);
  assert.ok(!html.includes('<script>alert(1)'));
  assert.ok(!html.includes('<img src=x'));
  assert.ok(html.includes('&lt;script&gt;alert(1)&lt;/script&gt;'));
});

test('the page has no inline script or style the CSP would block', () => {
  for (const html of [renderCard(profile, opts), renderNotFound(profile, opts)]) {
    for (const [, attrs, body] of html.matchAll(/<script([^>]*)>([\s\S]*?)<\/script>/g)) {
      const dataBlock = /type="application\/(?:ld\+)?json"/.test(attrs);
      assert.ok(dataBlock || (attrs.includes('src=') && body === ''), `script inline : ${attrs}`);
    }
    assert.ok(!/<style[\s>]/.test(html));
    assert.ok(!/\sstyle="/.test(html));
    assert.ok(!/\son[a-z]+="/.test(html), 'gestionnaire d’événement inline');
  }
});

test('every external link opens with noopener', () => {
  const html = renderCard(profile, opts);
  for (const [tag] of html.matchAll(/<a [^>]*target="_blank"[^>]*>/g)) {
    assert.ok(tag.includes('rel="noopener noreferrer"'), tag);
  }
});

test('canonical URL and link-preview tags only with a public URL', () => {
  assert.ok(!renderCard(profile, opts).includes('og:image'));
  const html = renderCard(profile, {
    ...opts,
    publicUrl: 'https://carte.example.ci/',
  });
  assert.ok(html.includes('<link rel="canonical" href="https://carte.example.ci/">'));
  assert.ok(html.includes('content="https://carte.example.ci/assets/og-image.jpg"'));
});

test('the 404 page is not indexed and links back to the card', () => {
  const html = renderNotFound(profile, { ...opts, basePath: '/cartes/' });
  assert.ok(html.includes('<meta name="robots" content="noindex">'));
  assert.ok(html.includes('href="/cartes/"'));
});

test('the meta CSP is the header CSP without frame-ancestors', () => {
  assert.ok(CSP.includes("frame-ancestors 'none'"));
  assert.ok(!META_CSP.includes('frame-ancestors'));
  assert.ok(renderCard(profile, opts).includes(`content="${META_CSP}"`));
});

test('deploy/security-headers.conf sends the same security headers as _headers', async () => {
  const nginx = await readFile(
    new URL('../../deploy/security-headers.conf', import.meta.url),
    'utf8',
  );
  for (const [name, value] of Object.entries(SECURITY_HEADERS)) {
    assert.ok(
      nginx.includes(`add_header ${name} "${value}" always;`),
      `${name} absent ou différent dans security-headers.conf`,
    );
  }
  const file = headersFile('x.vcf');
  for (const [name, value] of Object.entries(SECURITY_HEADERS))
    assert.ok(file.includes(`  ${name}: ${value}`));
});

test('the manifest is valid JSON with installable icons', () => {
  /** @type {{ display: string, start_url: string, icons: { sizes: string, purpose?: string }[] }} */
  const m = JSON.parse(renderManifest(profile, opts));
  assert.equal(m.display, 'standalone');
  assert.equal(m.start_url, '/');
  assert.ok(m.icons.some((i) => i.sizes === '512x512' && i.purpose === 'maskable'));
  assert.ok(m.icons.some((i) => i.sizes === '192x192'));
});
