import { expect, test } from '@playwright/test';

// Widths of real devices, portrait and landscape: iPhone SE (1st gen.),
// iPhone 13, Pixel 7, Galaxy Fold folded, iPad mini, iPad Pro, laptop, desktop.
const viewports = [
  [320, 568],
  [568, 320],
  [280, 653],
  [375, 667],
  [390, 844],
  [844, 390],
  [412, 915],
  [768, 1024],
  [1024, 768],
  [1366, 768],
  [1920, 1080],
];

test.describe('responsive', () => {
  test.skip(({ isMobile }) => isMobile, 'viewports fixés ci-dessous');

  for (const [width, height] of viewports) {
    test(`${width}×${height}: no horizontal scroll, nothing clipped`, async ({ page }) => {
      await page.setViewportSize({ width, height });
      await page.goto('/');
      const overflow = await page.evaluate(
        () => document.documentElement.scrollWidth - window.innerWidth,
      );
      expect(overflow).toBeLessThanOrEqual(0);

      // Every text block and control lies inside the card horizontally.
      const outside = await page.evaluate(() => {
        const card = /** @type {Element} */ (
          document.querySelector('.card')
        ).getBoundingClientRect();
        return [
          ...document.querySelectorAll(
            '.card a, .card button:not([hidden]), .card h1, .card p, .detail__value',
          ),
        ]
          .filter((el) => {
            const r = el.getBoundingClientRect();
            return r.width > 0 && (r.left < card.left - 0.5 || r.right > card.right + 0.5);
          })
          .map((el) => el.textContent?.trim().slice(0, 30));
      });
      expect(outside).toEqual([]);

      // The quick actions stay on one row.
      const tops = await page
        .locator('.quick__btn')
        .evaluateAll((els) => [
          ...new Set(els.map((e) => Math.round(e.getBoundingClientRect().top))),
        ]);
      expect(tops).toHaveLength(1);

      // Touch targets of at least 44×44 CSS px (WCAG 2.5.5, Apple HIG); half
      // a pixel of tolerance for sub-pixel layout.
      const small = await page.evaluate(() =>
        [...document.querySelectorAll('.quick__btn, .btn, .icon-btn, .link-btn, .detail')]
          .filter((el) => !el.closest('[hidden]') && !el.closest('dialog:not([open])'))
          .map((el) => ({
            el: el.textContent?.trim().slice(0, 20),
            r: el.getBoundingClientRect(),
          }))
          .filter(({ r }) => r.width > 0 && (r.width < 43.5 || r.height < 43.5))
          .map(({ el }) => el),
      );
      expect(small).toEqual([]);

      await page.getByRole('button', { name: 'QR code' }).click();
      const dialog = page.getByRole('dialog');
      const box = await dialog.boundingBox();
      expect(box && box.x >= 0 && box.x + box.width <= width).toBe(true);
      const dialogOverflow = await dialog.evaluate((d) => d.scrollWidth - d.clientWidth);
      expect(dialogOverflow).toBeLessThanOrEqual(0);
    });
  }

  test('text zoom to 200 % keeps every coordinate readable', async ({ page }) => {
    await page.setViewportSize({ width: 390, height: 844 });
    await page.goto('/');
    // Same effect as the browser's text-size setting; set through the CSSOM,
    // which the CSP allows, unlike an injected <style>.
    await page.evaluate(() => document.documentElement.style.setProperty('font-size', '200%'));
    const overflow = await page.evaluate(
      () => document.documentElement.scrollWidth - window.innerWidth,
    );
    expect(overflow).toBeLessThanOrEqual(0);
    await expect(page.getByText('15 BP 292 Abidjan', { exact: false })).toBeVisible();
  });
});
