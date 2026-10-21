#!/usr/bin/env node
// Serves dist/ with the production headers (CSP, cache, content types), for
// local preview and the end-to-end tests. Not meant to face the internet:
// production runs behind nginx (deploy/) or a static host.

import { createReadStream } from 'node:fs';
import { stat } from 'node:fs/promises';
import { createServer } from 'node:http';
import { dirname, join, normalize, resolve, sep } from 'node:path';
import { fileURLToPath } from 'node:url';

import { SECURITY_HEADERS, cacheControl, contentType } from '../src/lib/headers.js';

const dist = resolve(dirname(fileURLToPath(import.meta.url)), '..', 'dist');
const port = Number(process.env.PORT ?? 4173);
const host = process.env.HOST ?? '127.0.0.1';

/** @param {string} path */
async function isFile(path) {
  try {
    return (await stat(path)).isFile();
  } catch {
    return false;
  }
}

const server = createServer(async (req, res) => {
  const send = (
    /** @type {number} */ status,
    /** @type {string} */ file,
    /** @type {string} */ pathname,
  ) => {
    res.writeHead(status, {
      ...SECURITY_HEADERS,
      'Content-Type': contentType(file),
      'Cache-Control': cacheControl(pathname),
    });
    if (req.method === 'HEAD') res.end();
    else createReadStream(file).pipe(res);
  };

  if (req.method !== 'GET' && req.method !== 'HEAD') {
    res.writeHead(405, { Allow: 'GET, HEAD', ...SECURITY_HEADERS }).end();
    return;
  }
  let pathname;
  try {
    pathname = decodeURIComponent(new URL(req.url ?? '/', 'http://x').pathname);
  } catch {
    res.writeHead(400, SECURITY_HEADERS).end();
    return;
  }
  // Resolved inside dist/ or refused: no ../ escape.
  const target = normalize(join(dist, pathname.endsWith('/') ? `${pathname}index.html` : pathname));
  if (target !== dist && !target.startsWith(dist + sep)) {
    send(404, join(dist, '404.html'), '/404.html');
    return;
  }
  if (await isFile(target)) send(200, target, pathname);
  else send(404, join(dist, '404.html'), '/404.html');
});

server.listen(port, host, () => console.log(`carte servie sur http://${host}:${port}/`));
