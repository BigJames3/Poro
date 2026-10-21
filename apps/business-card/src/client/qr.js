// QR code of the card's link. The encoder (52 kB) is only fetched when the
// visitor opens the share sheet or exports the image.

/** @typedef {{ size: number, isDark: (row: number, col: number) => boolean }} QrMatrix */

/**
 * @param {string} text ASCII text: the URL as `URL.href` serialises it
 * @returns {Promise<QrMatrix>}
 */
export async function qrMatrix(text) {
  const { qrcode } = await import('./vendor/qrcode.mjs');
  // Level M: survives a scratched print or a screen glare, still small for a URL.
  const qr = qrcode(0, 'M');
  qr.addData(text, 'Byte');
  qr.make();
  return { size: qr.getModuleCount(), isDark: (r, c) => qr.isDark(r, c) };
}

/**
 * One <path> for every dark module, built with DOM calls: nothing is parsed
 * as HTML.
 * @param {QrMatrix} m
 * @param {number} [quiet] margin in modules; the standard asks for 4
 */
export function qrSvg(m, quiet = 4) {
  const ns = 'http://www.w3.org/2000/svg';
  const total = m.size + quiet * 2;
  const svg = document.createElementNS(ns, 'svg');
  svg.setAttribute('viewBox', `0 0 ${total} ${total}`);
  svg.setAttribute('shape-rendering', 'crispEdges');
  svg.setAttribute('aria-hidden', 'true');
  const bg = document.createElementNS(ns, 'rect');
  bg.setAttribute('width', String(total));
  bg.setAttribute('height', String(total));
  bg.setAttribute('fill', '#ffffff');
  const path = document.createElementNS(ns, 'path');
  let d = '';
  for (let r = 0; r < m.size; r++) {
    for (let c = 0; c < m.size; c++) {
      if (m.isDark(r, c)) d += `M${c + quiet} ${r + quiet}h1v1h-1z`;
    }
  }
  path.setAttribute('d', d);
  path.setAttribute('fill', '#000000');
  svg.append(bg, path);
  return svg;
}

/**
 * @param {CanvasRenderingContext2D} ctx
 * @param {QrMatrix} m
 * @param {number} x
 * @param {number} y
 * @param {number} size side in pixels, quiet zone included
 */
export function drawQr(ctx, m, x, y, size) {
  const quiet = 4;
  const cell = size / (m.size + quiet * 2);
  ctx.fillStyle = '#ffffff';
  ctx.fillRect(x, y, size, size);
  ctx.fillStyle = '#000000';
  for (let r = 0; r < m.size; r++) {
    for (let c = 0; c < m.size; c++) {
      // Rounded outwards so neighbouring modules leave no hairline gap.
      if (m.isDark(r, c)) {
        ctx.fillRect(
          Math.floor(x + (c + quiet) * cell),
          Math.floor(y + (r + quiet) * cell),
          Math.ceil(cell),
          Math.ceil(cell),
        );
      }
    }
  }
}
