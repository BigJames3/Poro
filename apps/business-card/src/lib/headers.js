// HTTP headers for every host: the build writes them to dist/_headers
// (Netlify, Cloudflare Pages), deploy/nginx.conf repeats them (a unit test
// keeps the two equal) and scripts/serve.mjs sends them in dev and e2e.

/** Everything is first-party: no CDN, no font service, no inline script. */
export const CSP = [
  "default-src 'none'",
  "script-src 'self'",
  "style-src 'self'",
  "img-src 'self' data: blob:",
  "connect-src 'self'",
  "manifest-src 'self'",
  "worker-src 'self'",
  "base-uri 'none'",
  "form-action 'none'",
  "frame-ancestors 'none'",
].join('; ');

/**
 * Copy in a <meta> tag for hosts that cannot set headers. frame-ancestors is
 * dropped: browsers ignore it there and log an error.
 */
export const META_CSP = CSP.split('; ')
  .filter((d) => !d.startsWith('frame-ancestors'))
  .join('; ');

/** @type {Record<string, string>} */
export const SECURITY_HEADERS = {
  'Content-Security-Policy': CSP,
  'Strict-Transport-Security': 'max-age=31536000; includeSubDomains',
  'X-Content-Type-Options': 'nosniff',
  'Referrer-Policy': 'strict-origin-when-cross-origin',
  'Permissions-Policy': 'camera=(), microphone=(), geolocation=(), payment=(), usb=()',
  'Cross-Origin-Opener-Policy': 'same-origin',
  'Cross-Origin-Resource-Policy': 'same-origin',
  'X-Frame-Options': 'DENY',
};

/**
 * Cache policy by path. HTML, the service worker and the vCard must be
 * revalidated so an edited card reaches visitors on their next load.
 * @param {string} pathname
 */
export function cacheControl(pathname) {
  if (pathname.startsWith('/assets/')) return 'public, max-age=604800';
  return 'no-cache';
}

/**
 * Content types the static hosts do not all get right.
 * @param {string} pathname
 */
export function contentType(pathname) {
  const ext = pathname.slice(pathname.lastIndexOf('.') + 1).toLowerCase();
  return (
    {
      html: 'text/html; charset=utf-8',
      css: 'text/css; charset=utf-8',
      js: 'text/javascript; charset=utf-8',
      mjs: 'text/javascript; charset=utf-8',
      json: 'application/json; charset=utf-8',
      webmanifest: 'application/manifest+json; charset=utf-8',
      vcf: 'text/vcard; charset=utf-8',
      svg: 'image/svg+xml',
      png: 'image/png',
      jpg: 'image/jpeg',
      jpeg: 'image/jpeg',
      webp: 'image/webp',
      ico: 'image/x-icon',
      txt: 'text/plain; charset=utf-8',
    }[ext] ?? 'application/octet-stream'
  );
}

/**
 * dist/_headers, in the Netlify / Cloudflare Pages syntax. Paths are relative
 * to the published dist/ folder, whatever PUBLIC_URL's path.
 * @param {string} vcfName
 */
export function headersFile(vcfName) {
  const block = (/** @type {string} */ path, /** @type {Record<string, string>} */ h) =>
    [path, ...Object.entries(h).map(([k, v]) => `  ${k}: ${v}`)].join('\n');
  return (
    [
      // Hosts merge every matching block: Cache-Control is set per path
      // only, otherwise /assets/* would get "no-cache, public, …".
      block('/*', SECURITY_HEADERS),
      block('/assets/*', { 'Cache-Control': cacheControl('/assets/') }),
      // No Content-Disposition: attachment, iOS Safari would file the
      // vCard in Downloads instead of opening its "add contact" sheet.
      block(`/${vcfName}`, { 'Content-Type': contentType(vcfName) }),
    ].join('\n\n') + '\n'
  );
}
