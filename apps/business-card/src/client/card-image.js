// PNG export of the card, drawn on a canvas from the same data as the page,
// with the QR code so that a forwarded image still leads to the live card.

import { drawQr, qrMatrix } from './qr.js';

const W = 1080;
const PAD = 88;
const FONT = 'system-ui, -apple-system, "Segoe UI", Roboto, "Helvetica Neue", Arial, sans-serif';
const INK = '#17181b';
const MUTED = '#5b5d65';
const LINE = '#e3e5ea';

/**
 * @typedef {{
 *   name: string, title: string, organization: string, legalForm: string, footer: string,
 *   initials: string, theme: { brand: string, brandDark: string },
 *   rows: { icon: string, label: string, value: string }[], icons: Record<string, string>
 * }} CardData
 */

/**
 * @param {CanvasRenderingContext2D} ctx
 * @param {string} text
 * @param {number} max
 */
function wrap(ctx, text, max) {
  const lines = [];
  let line = '';
  for (const word of text.split(/\s+/)) {
    const next = line ? `${line} ${word}` : word;
    if (line && ctx.measureText(next).width > max) {
      lines.push(line);
      line = word;
    } else {
      line = next;
    }
  }
  if (line) lines.push(line);
  return lines;
}

/**
 * @param {CardData} data
 * @param {string} url encoded in the QR code
 * @returns {Promise<Blob>}
 */
export async function renderCardImage(data, url) {
  const matrix = await qrMatrix(url);
  const canvas = document.createElement('canvas');
  canvas.width = W;
  // Taller than any card; cropped to the drawn height at the end.
  canvas.height = 2600;
  const ctx = /** @type {CanvasRenderingContext2D} */ (canvas.getContext('2d'));
  const { brand, brandDark } = data.theme;

  ctx.fillStyle = '#ffffff';
  ctx.fillRect(0, 0, W, canvas.height);

  // Brand band with its slanted edge.
  const g = ctx.createLinearGradient(0, 0, W, 420);
  g.addColorStop(0, brand);
  g.addColorStop(1, brandDark);
  ctx.fillStyle = g;
  ctx.beginPath();
  ctx.moveTo(0, 0);
  ctx.lineTo(W, 0);
  ctx.lineTo(W, 300);
  ctx.lineTo(0, 390);
  ctx.closePath();
  ctx.fill();

  // Logo: organisation name in a frame, as on the page.
  ctx.textBaseline = 'alphabetic';
  ctx.textAlign = 'left';
  ctx.font = `italic 700 60px ${FONT}`;
  const wName = ctx.measureText(data.organization).width;
  ctx.font = `600 32px ${FONT}`;
  const wForm = data.legalForm ? ctx.measureText(data.legalForm).width + 16 : 0;
  ctx.strokeStyle = '#ffffff';
  ctx.lineWidth = 6;
  ctx.strokeRect(PAD, 80, wName + wForm + 64, 100);
  ctx.fillStyle = '#ffffff';
  ctx.font = `italic 700 60px ${FONT}`;
  ctx.fillText(data.organization, PAD + 32, 152);
  if (data.legalForm) {
    ctx.font = `600 32px ${FONT}`;
    ctx.fillText(data.legalForm, PAD + 32 + wName + 16, 152);
  }

  // Avatar with initials, overlapping the band.
  const cx = W / 2;
  const cy = 400;
  ctx.beginPath();
  ctx.arc(cx, cy, 150, 0, Math.PI * 2);
  ctx.fillStyle = '#ffffff';
  ctx.fill();
  ctx.beginPath();
  ctx.arc(cx, cy, 136, 0, Math.PI * 2);
  ctx.fillStyle = '#f6f7f9';
  ctx.fill();
  ctx.fillStyle = brand;
  ctx.font = `700 104px ${FONT}`;
  ctx.textAlign = 'center';
  ctx.textBaseline = 'middle';
  ctx.fillText(data.initials, cx, cy + 4);
  ctx.textBaseline = 'alphabetic';

  let y = cy + 150 + 110;
  ctx.fillStyle = INK;
  ctx.font = `750 76px ${FONT}`;
  for (const line of wrap(ctx, data.name, W - PAD * 2)) {
    ctx.fillText(line, cx, y);
    y += 88;
  }
  y += 4;
  ctx.font = `400 40px ${FONT}`;
  for (const line of wrap(ctx, data.title, W - PAD * 2 - 40)) {
    ctx.fillText(line, cx, y);
    y += 54;
  }
  ctx.fillStyle = MUTED;
  ctx.font = `400 34px ${FONT}`;
  ctx.fillText(data.footer.split(' · ')[0], cx, y + 4);
  y += 70;

  // Coordinates.
  ctx.textAlign = 'left';
  const textX = PAD + 132;
  const textW = W - textX - PAD;
  data.rows.forEach((row, i) => {
    ctx.font = `650 40px ${FONT}`;
    const lines = wrap(ctx, row.value, textW);
    const h = Math.max(150, 96 + lines.length * 52);
    if (i > 0) {
      ctx.fillStyle = LINE;
      ctx.fillRect(PAD, y, W - PAD * 2, 3);
    }
    const mid = y + h / 2;
    ctx.fillStyle = brand;
    ctx.beginPath();
    ctx.arc(PAD + 46, mid, 46, 0, Math.PI * 2);
    ctx.fill();
    const path = data.icons[row.icon];
    if (path) {
      ctx.save();
      ctx.translate(PAD + 46 - 24, mid - 24);
      ctx.scale(2, 2);
      ctx.strokeStyle = '#ffffff';
      ctx.lineWidth = 2;
      ctx.lineCap = 'round';
      ctx.lineJoin = 'round';
      ctx.stroke(new Path2D(path));
      ctx.restore();
    }
    const top = mid - (34 + lines.length * 52) / 2;
    ctx.fillStyle = MUTED;
    ctx.font = `400 30px ${FONT}`;
    ctx.fillText(row.label, textX, top + 26);
    ctx.fillStyle = INK;
    ctx.font = `650 40px ${FONT}`;
    lines.forEach((l, k) => ctx.fillText(l, textX, top + 26 + 52 * (k + 1)));
    y += h;
  });

  // QR code and its caption.
  y += 40;
  const qrSize = 300;
  ctx.fillStyle = '#f6f7f9';
  ctx.fillRect(0, y, W, qrSize + 120);
  drawQr(ctx, matrix, PAD, y + 60, qrSize);
  ctx.fillStyle = INK;
  ctx.font = `700 40px ${FONT}`;
  ctx.fillText('Scannez pour ouvrir', PAD + qrSize + 56, y + 60 + qrSize / 2 - 10);
  ctx.fillText('la carte en ligne', PAD + qrSize + 56, y + 60 + qrSize / 2 + 44);
  y += qrSize + 120;

  // Footer and brand strip.
  ctx.fillStyle = MUTED;
  ctx.font = `400 30px ${FONT}`;
  ctx.textAlign = 'center';
  ctx.fillText(data.footer, cx, y + 70);
  y += 110;
  ctx.fillStyle = brand;
  ctx.fillRect(0, y, W, 24);
  y += 24;

  const out = document.createElement('canvas');
  out.width = W;
  out.height = y;
  /** @type {CanvasRenderingContext2D} */ (out.getContext('2d')).drawImage(canvas, 0, 0);
  return new Promise((resolve, reject) =>
    out.toBlob((b) => (b ? resolve(b) : reject(new Error('toBlob a échoué'))), 'image/png'),
  );
}
