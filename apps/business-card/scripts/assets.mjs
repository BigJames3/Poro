#!/usr/bin/env node
// Generates the icons and the link-preview image from profile.json, with the
// Chromium that Playwright drives. Run after changing the name, the
// organisation or the colours; the output is committed under assets/.

import { mkdir, readFile, writeFile } from 'node:fs/promises';
import { dirname, join, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';

import { chromium } from '@playwright/test';

import { fullName, initials, organizationName, validateProfile } from '../src/lib/profile.js';
import { esc } from '../src/lib/render.js';

const root = resolve(dirname(fileURLToPath(import.meta.url)), '..');
const profile = JSON.parse(
  await readFile(join(root, process.env.CARD_PROFILE ?? 'profile.json'), 'utf8'),
);
const errors = validateProfile(profile);
if (errors.length) throw new Error(errors.join('\n'));

const { brand, brandDark } = profile.theme;
const letters = esc(initials(profile));
const FONT = "system-ui, -apple-system, 'Segoe UI', Roboto, Arial, sans-serif";

/** @param {{ maskable?: boolean }} [o] */
const iconSvg = ({
  maskable = false,
} = {}) => `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 512 512">
  <defs><linearGradient id="g" x1="0" y1="0" x2="1" y2="1"><stop offset="0" stop-color="${brand}"/><stop offset="1" stop-color="${brandDark}"/></linearGradient></defs>
  <rect width="512" height="512" rx="${maskable ? 0 : 112}" fill="url(#g)"/>
  <text x="256" y="256" dy=".35em" text-anchor="middle" font-family="${FONT}" font-weight="700" font-size="${maskable ? 168 : 208}" fill="#fff" letter-spacing="4">${letters}</text>
</svg>
`;

const out = join(root, 'assets/icons');
await mkdir(out, { recursive: true });
await writeFile(join(out, 'icon.svg'), iconSvg());

const browser = await chromium.launch(
  process.env.PLAYWRIGHT_CHROMIUM_PATH
    ? { executablePath: process.env.PLAYWRIGHT_CHROMIUM_PATH }
    : {},
);
const page = await browser.newPage();

/** @param {string} svg @param {number} size @param {string} file */
async function png(svg, size, file) {
  await page.setViewportSize({ width: size, height: size });
  await page.setContent(
    `<html><body style="margin:0">${svg.replace('<svg ', `<svg width="${size}" height="${size}" `)}</body></html>`,
  );
  await page.screenshot({ path: file, omitBackground: true });
}

await png(iconSvg(), 192, join(out, 'icon-192.png'));
await png(iconSvg(), 512, join(out, 'icon-512.png'));
await png(iconSvg({ maskable: true }), 512, join(out, 'icon-maskable-512.png'));
// iOS adds its own rounded mask and needs an opaque square.
await png(iconSvg({ maskable: true }), 180, join(out, 'apple-touch-icon.png'));

// 1200×630 link preview (WhatsApp, LinkedIn, iMessage, Slack…).
await page.setViewportSize({ width: 1200, height: 630 });
await page.setContent(`<html><body style="margin:0;font-family:${FONT}">
<div style="width:1200px;height:630px;box-sizing:border-box;padding:72px 88px;color:#fff;display:flex;flex-direction:column;justify-content:space-between;background:radial-gradient(120% 140% at 100% 0%,rgba(255,255,255,.16),transparent 55%),linear-gradient(135deg,${brand},${brandDark})">
  <div style="display:inline-flex;align-self:flex-start;align-items:baseline;gap:10px;border:4px solid #fff;padding:10px 22px;font-size:44px;line-height:1.1">
    <span style="font-style:italic;font-weight:700">${esc(profile.organization.name)}</span>
    <span style="font-size:24px;font-weight:600">${esc(profile.organization.legalForm ?? '')}</span>
  </div>
  <div>
    <div style="font-size:72px;font-weight:750;letter-spacing:-1px;line-height:1.1">${esc(fullName(profile))}</div>
    <div style="margin-top:18px;font-size:34px;line-height:1.35;max-width:960px;opacity:.95">${esc(profile.person.title)}</div>
  </div>
  <div style="font-size:26px;opacity:.9">${esc(organizationName(profile))}${profile.organization.city ? ` · ${esc(profile.organization.city)}` : ''} — Carte de visite numérique</div>
</div></body></html>`);
await page.screenshot({ path: join(root, 'assets/og-image.jpg') });

await browser.close();
console.log('assets: icônes et image d’aperçu générées dans assets/');
