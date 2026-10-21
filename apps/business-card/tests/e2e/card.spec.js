import AxeBuilder from '@axe-core/playwright';
import { expect, test } from '@playwright/test';
import jsQRModule from 'jsqr';

// CommonJS module: its default export is the function, typed as a namespace.
const jsQR = /** @type {typeof import('jsqr').default} */ (/** @type {unknown} */ (jsQRModule));

/** Console errors and uncaught exceptions per page; every test expects none. */
const consoleErrors = new WeakMap();

test.beforeEach(async ({ page }) => {
  /** @type {string[]} */
  const errors = [];
  page.on('console', (m) => {
    if (m.type() === 'error') errors.push(m.text());
  });
  page.on('pageerror', (e) => errors.push(e.message));
  consoleErrors.set(page, errors);
});

test.afterEach(async ({ page }) => {
  expect(consoleErrors.get(page)).toEqual([]);
});

test('shows the identity and every coordinate', async ({ page }) => {
  await page.goto('/');
  await expect(page).toHaveTitle('Bintou Saïda DIABATÉ – Foods’ Co S.A.');
  await expect(page.getByRole('heading', { level: 1 })).toHaveText('Bintou Saïda DIABATÉ');
  await expect(
    page.getByText('Responsable Customer service, Qualité et Coordination logistique'),
  ).toBeVisible();
  const details = page.locator('.details');
  await expect(details.getByRole('link', { name: /\+225 07 68 62 04 97/ })).toHaveAttribute(
    'href',
    'tel:+2250768620497',
  );
  await expect(
    details.getByRole('link', { name: /responsablequalite@foodsco\.ci/ }),
  ).toHaveAttribute('href', 'mailto:responsablequalite@foodsco.ci');
  await expect(details.getByRole('link', { name: /www\.foodsco\.ci/ })).toHaveAttribute(
    'href',
    'https://www.foodsco.ci',
  );
  await expect(details.getByRole('link', { name: /Treichville/ })).toHaveAttribute(
    'href',
    /google\.com\/maps\/search/,
  );
});

test('quick actions call, message on WhatsApp, e-mail and open the map', async ({ page }) => {
  await page.goto('/');
  const quick = page.getByRole('navigation', { name: /Contacter/ });
  await expect(quick.getByRole('link', { name: 'Appeler' })).toHaveAttribute(
    'href',
    'tel:+2250768620497',
  );
  const wa = quick.getByRole('link', { name: /WhatsApp/ });
  await expect(wa).toHaveAttribute(
    'href',
    /^https:\/\/wa\.me\/2250768620497\?text=Bonjour%20Madame/,
  );
  await expect(wa).toHaveAttribute('rel', 'noopener noreferrer');
  await expect(quick.getByRole('link', { name: 'E-mail' })).toHaveAttribute(
    'href',
    'mailto:responsablequalite@foodsco.ci',
  );
  await expect(quick.getByRole('link', { name: /Itinéraire/ })).toHaveAttribute('target', '_blank');
});

test('“Enregistrer le contact” serves a valid vCard', async ({ page, request }) => {
  await page.goto('/');
  const save = page.getByRole('link', { name: 'Enregistrer le contact' });
  const href = await save.getAttribute('href');
  expect(href).toBe('/bintou-saida-diabate.vcf');
  const res = await request.get(href ?? '');
  expect(res.status()).toBe(200);
  expect(res.headers()['content-type']).toBe('text/vcard; charset=utf-8');
  const text = (await res.text()).replace(/\r\n /g, '');
  expect(text).toMatch(/^BEGIN:VCARD\r\nVERSION:3\.0\r\n/);
  expect(text).toContain('FN:Bintou Saïda DIABATÉ\r\n');
  expect(text).toContain('TEL;TYPE=CELL,VOICE,WORK:+2250768620497\r\n');
  expect(text).toContain('EMAIL;TYPE=INTERNET,WORK:responsablequalite@foodsco.ci\r\n');
  expect(text.trimEnd().endsWith('END:VCARD')).toBe(true);
});

test('the QR code decodes to the card URL', async ({ page }) => {
  await page.goto('/');
  await page.getByRole('button', { name: 'QR code' }).click();
  const dialog = page.getByRole('dialog', { name: 'Partager la carte' });
  await expect(dialog).toBeVisible();
  const qr = dialog.getByRole('img', { name: /QR code/ });
  await expect(qr.locator('svg')).toBeVisible();
  const shot = await qr.screenshot();
  const decoded = await page.evaluate(async (b64) => {
    const img = new Image();
    img.src = `data:image/png;base64,${b64}`;
    await img.decode();
    const c = document.createElement('canvas');
    c.width = img.width;
    c.height = img.height;
    const ctx = /** @type {CanvasRenderingContext2D} */ (c.getContext('2d'));
    ctx.drawImage(img, 0, 0);
    return {
      w: c.width,
      h: c.height,
      data: Array.from(ctx.getImageData(0, 0, c.width, c.height).data),
    };
  }, shot.toString('base64'));
  const result = jsQR(Uint8ClampedArray.from(decoded.data), decoded.w, decoded.h);
  expect(result?.data).toBe('http://127.0.0.1:4173/');
  await expect(dialog.getByRole('textbox', { name: 'Lien de la carte' })).toHaveValue(
    'http://127.0.0.1:4173/',
  );
  await dialog.getByRole('button', { name: 'Fermer' }).click();
  await expect(dialog).toBeHidden();
  await page.getByRole('button', { name: 'QR code' }).click();
  await page.keyboard.press('Escape');
  await expect(dialog).toBeHidden();
});

test('“Copier” puts the card link in the clipboard', async ({ page, context, browserName }) => {
  test.skip(browserName !== 'chromium', 'permissions clipboard propres à Chromium');
  await context.grantPermissions(['clipboard-read', 'clipboard-write']);
  await page.goto('/');
  await page.getByRole('button', { name: 'QR code' }).click();
  await page.getByRole('button', { name: 'Copier' }).click();
  await expect(page.getByRole('status')).toHaveText('Lien copié');
  expect(await page.evaluate(() => navigator.clipboard.readText())).toBe('http://127.0.0.1:4173/');
});

test('“Partager” uses the native share sheet when available', async ({ page }) => {
  await page.addInitScript(() => {
    /** @type {any} */ (window).__shared = [];
    Object.defineProperty(navigator, 'share', {
      configurable: true,
      value: async (/** @type {ShareData} */ d) =>
        void (/** @type {any} */ (window).__shared.push(d)),
    });
  });
  await page.goto('/');
  await page.getByRole('button', { name: 'Partager', exact: true }).click();
  const shared = await page.evaluate(() => /** @type {any} */ (window).__shared);
  expect(shared).toEqual([
    {
      title: 'Bintou Saïda DIABATÉ – Foods’ Co',
      text: 'Responsable Customer service, Qualité et Coordination logistique',
      url: 'http://127.0.0.1:4173/',
    },
  ]);
  await expect(page.getByRole('dialog')).toBeHidden();
});

test('“Partager” falls back to the share sheet without Web Share', async ({ page }) => {
  await page.addInitScript(() => {
    Object.defineProperty(navigator, 'share', {
      configurable: true,
      value: undefined,
    });
  });
  await page.goto('/');
  await page.getByRole('button', { name: 'Partager', exact: true }).click();
  await expect(page.getByRole('dialog', { name: 'Partager la carte' })).toBeVisible();
  await expect(page.getByRole('button', { name: 'Partager via…' })).toBeHidden();
});

test('exports the card as a PNG image', async ({ page, isMobile }) => {
  test.skip(isMobile, 'sur mobile, l’image passe par la feuille de partage');
  await page.goto('/');
  const [download] = await Promise.all([
    page.waitForEvent('download'),
    page.getByRole('button', { name: 'Télécharger l’image de la carte' }).click(),
  ]);
  expect(download.suggestedFilename()).toBe('carte-de-visite-bintou-saida-diabate.png');
  const path = await download.path();
  const { readFile } = await import('node:fs/promises');
  const png = await readFile(path);
  expect(png.subarray(1, 4).toString()).toBe('PNG');
  expect(png.readUInt32BE(16)).toBe(1080);
  await expect(page.getByRole('status')).toHaveText('Image téléchargée');
});

test('works without JavaScript: content and contact links stay usable', async ({ browser }) => {
  const context = await browser.newContext({ javaScriptEnabled: false });
  const page = await context.newPage();
  await page.goto('/');
  await expect(page.getByRole('heading', { level: 1 })).toHaveText('Bintou Saïda DIABATÉ');
  await expect(page.getByRole('link', { name: 'Enregistrer le contact' })).toBeVisible();
  // The JS-only buttons stay hidden instead of doing nothing.
  await expect(page.getByRole('button', { name: 'QR code' })).toBeHidden();
  await context.close();
});

test('an unknown link shows the 404 page with a way back', async ({ page }) => {
  const res = await page.goto('/carte-qui-n-existe-pas');
  expect(res?.status()).toBe(404);
  // Chromium logs the 404 response itself: the only error expected here.
  const errors = consoleErrors.get(page);
  expect(errors).toEqual([expect.stringContaining('status of 404')]);
  errors.length = 0;
  await expect(page.getByRole('heading', { level: 1 })).toHaveText('Cette page n’existe pas');
  await page.getByRole('link', { name: /Voir la carte/ }).click();
  await expect(page.getByRole('heading', { level: 1 })).toHaveText('Bintou Saïda DIABATÉ');
});

test('sends the security headers', async ({ request }) => {
  const res = await request.get('/');
  const h = res.headers();
  expect(h['content-security-policy']).toContain("script-src 'self'");
  expect(h['content-security-policy']).toContain("frame-ancestors 'none'");
  expect(h['x-content-type-options']).toBe('nosniff');
  expect(h['referrer-policy']).toBe('strict-origin-when-cross-origin');
});

test('has no detectable accessibility violation (WCAG 2.1 AA)', async ({ page }) => {
  for (const scheme of /** @type {const} */ (['light', 'dark'])) {
    await page.emulateMedia({ colorScheme: scheme, reducedMotion: 'reduce' });
    await page.goto('/');
    const results = await new AxeBuilder({ page: /** @type {any} */ (page) })
      .withTags(['wcag2a', 'wcag2aa', 'wcag21a', 'wcag21aa'])
      .analyze();
    expect(
      results.violations.map(
        (v) => `${scheme}: ${v.id} ${v.nodes.map((n) => n.target).join(', ')}`,
      ),
    ).toEqual([]);
    await page.getByRole('button', { name: 'QR code' }).click();
    const dialog = await new AxeBuilder({ page: /** @type {any} */ (page) })
      .include('#share-dialog')
      .withTags(['wcag2a', 'wcag2aa'])
      .analyze();
    expect(dialog.violations.map((v) => `${scheme} dialog: ${v.id}`)).toEqual([]);
  }
});

test('is reachable with the keyboard alone', async ({ page, isMobile }) => {
  test.skip(isMobile, 'clavier : ordinateur');
  await page.goto('/');
  const focused = [];
  for (let i = 0; i < 12; i++) {
    await page.keyboard.press('Tab');
    focused.push(
      await page.evaluate(
        () =>
          document.activeElement?.getAttribute('aria-label') ||
          document.activeElement?.textContent?.trim(),
      ),
    );
  }
  expect(focused.slice(0, 7)).toEqual([
    'Appeler',
    'WhatsApp (nouvel onglet)',
    'E-mail',
    'Itinéraire (nouvel onglet)',
    'Enregistrer le contact',
    'Partager',
    'QR code',
  ]);
});

test('works offline after the first visit (PWA)', async ({ page, context }) => {
  await page.goto('/');
  await page.evaluate(async () => {
    await navigator.serviceWorker.ready;
  });
  // The first load is not controlled yet; reload so the worker serves it.
  await page.reload();
  await expect.poll(() => page.evaluate(() => !!navigator.serviceWorker.controller)).toBe(true);
  await context.setOffline(true);
  await page.reload();
  await expect(page.getByRole('heading', { level: 1 })).toHaveText('Bintou Saïda DIABATÉ');
  await page.getByRole('button', { name: 'QR code' }).click();
  await expect(page.locator('#qr svg')).toBeVisible();
  const vcf = await page.evaluate(async () => (await fetch('/bintou-saida-diabate.vcf')).text());
  expect(vcf).toContain('BEGIN:VCARD');
  await context.setOffline(false);
});

test('the web app manifest is installable', async ({ page, request }) => {
  await page.goto('/');
  const href = await page.locator('link[rel="manifest"]').getAttribute('href');
  const res = await request.get(href ?? '');
  expect(res.headers()['content-type']).toContain('application/manifest+json');
  const m = await res.json();
  expect(m.display).toBe('standalone');
  for (const icon of m.icons) expect((await request.get(icon.src)).status()).toBe(200);
});
