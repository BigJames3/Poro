// Progressive enhancement of the card: sharing, QR code, image export and
// offline support. Without this script the card, its links and the vCard
// still work.

const base = new URL('./', import.meta.url);
const dataEl = document.getElementById('card-data');
/** @type {import('./card-image.js').CardData & { publicUrl: string | null, imageFileName: string }} */
const data = JSON.parse(dataEl?.textContent ?? '{}');

/** URL of the card itself, never the current URL with its query or hash. */
const cardUrl = new URL(data.publicUrl ?? base.href).href;

const dialog = /** @type {HTMLDialogElement | null} */ (document.getElementById('share-dialog'));
const toastEl = document.getElementById('toast');
const urlInput = /** @type {HTMLInputElement | null} */ (document.getElementById('share-url'));
const canShare = typeof navigator.share === 'function';

/** @type {ReturnType<typeof setTimeout> | undefined} */
let toastTimer;

/**
 * @param {string} message
 * @param {{ error?: boolean }} [opts]
 */
function toast(message, { error = false } = {}) {
  if (!toastEl) return;
  clearTimeout(toastTimer);
  toastEl.textContent = message;
  toastEl.classList.toggle('is-error', error);
  toastEl.classList.add('is-visible');
  toastTimer = setTimeout(() => {
    toastEl.classList.remove('is-visible');
    // Emptied after the fade so screen readers do not repeat it.
    setTimeout(() => (toastEl.textContent = ''), 250);
  }, 3200);
}

/** @param {unknown} e */
const isAbort = (e) => e instanceof DOMException && e.name === 'AbortError';

let qrRendered = false;

async function renderQr() {
  if (qrRendered) return;
  const box = document.getElementById('qr');
  if (!box) return;
  try {
    const { qrMatrix, qrSvg } = await import('./qr.js');
    box.replaceChildren(qrSvg(await qrMatrix(cardUrl)));
    qrRendered = true;
  } catch (e) {
    console.error(e);
    const msg = document.createElement('span');
    msg.className = 'qr__loading';
    msg.textContent = 'QR code indisponible hors connexion. Le lien ci-dessous fonctionne.';
    box.replaceChildren(msg);
  }
}

function openSheet() {
  if (!dialog) return;
  if (urlInput) urlInput.value = cardUrl;
  if (typeof dialog.showModal === 'function') dialog.showModal();
  else dialog.setAttribute('open', '');
  void renderQr();
}

function closeSheet() {
  if (!dialog) return;
  if (typeof dialog.close === 'function') dialog.close();
  else dialog.removeAttribute('open');
}

async function nativeShare() {
  try {
    await navigator.share({
      title: `${data.name} – ${data.organization}`,
      text: data.title,
      url: cardUrl,
    });
    return true;
  } catch (e) {
    if (isAbort(e)) return true;
    console.warn('navigator.share', e);
    return false;
  }
}

async function share() {
  if (canShare && (await nativeShare())) return;
  openSheet();
}

async function copyLink() {
  try {
    await navigator.clipboard.writeText(cardUrl);
    toast('Lien copié');
    return;
  } catch {
    // Clipboard API refused (insecure context, permissions): legacy path.
  }
  if (urlInput) {
    urlInput.focus();
    urlInput.select();
    const ok = document.execCommand?.('copy');
    toast(ok ? 'Lien copié' : 'Sélectionnez le lien pour le copier', {
      error: !ok,
    });
  }
}

/** @param {Blob} blob @param {string} name */
function download(blob, name) {
  const href = URL.createObjectURL(blob);
  const a = document.createElement('a');
  a.href = href;
  a.download = name;
  document.body.append(a);
  a.click();
  a.remove();
  setTimeout(() => URL.revokeObjectURL(href), 10_000);
}

/** @param {HTMLButtonElement} button */
async function exportImage(button) {
  button.disabled = true;
  button.setAttribute('aria-busy', 'true');
  try {
    await document.fonts?.ready;
    const { renderCardImage } = await import('./card-image.js');
    const blob = await renderCardImage(data, cardUrl);
    const file = new File([blob], data.imageFileName, { type: 'image/png' });
    // On touch devices the share sheet is how an image reaches the photo
    // library; on a desktop, a plain download is what people expect.
    const touch = window.matchMedia('(pointer: coarse)').matches;
    if (touch && navigator.canShare?.({ files: [file] })) {
      try {
        await navigator.share({ files: [file], title: data.name });
        return;
      } catch (e) {
        if (isAbort(e)) return;
      }
    }
    download(blob, data.imageFileName);
    toast('Image téléchargée');
  } catch (e) {
    console.error(e);
    toast('L’image n’a pas pu être créée. Réessayez.', { error: true });
  } finally {
    button.disabled = false;
    button.removeAttribute('aria-busy');
  }
}

/** @type {Record<string, (el: HTMLElement) => unknown>} */
const actions = {
  share: () => share(),
  qr: () => openSheet(),
  close: () => closeSheet(),
  copy: () => copyLink(),
  'native-share': () => nativeShare(),
  image: (el) => exportImage(/** @type {HTMLButtonElement} */ (el)),
};

document.addEventListener('click', (event) => {
  const el = /** @type {HTMLElement | null} */ (
    event.target instanceof Element ? event.target.closest('[data-action]') : null
  );
  const run = el && actions[el.dataset.action ?? ''];
  if (!el || !run) return;
  event.preventDefault();
  void run(el);
});

// A click on the backdrop (outside the sheet's box) closes it.
dialog?.addEventListener('click', (event) => {
  if (event.target !== dialog) return;
  const r = dialog.getBoundingClientRect();
  const inside =
    event.clientX >= r.left &&
    event.clientX <= r.right &&
    event.clientY >= r.top &&
    event.clientY <= r.bottom;
  if (!inside) closeSheet();
});

for (const el of document.querySelectorAll('[data-js-only]')) el.removeAttribute('hidden');
if (canShare) document.querySelector('[data-action="native-share"]')?.removeAttribute('hidden');

if ('serviceWorker' in navigator && window.isSecureContext) {
  window.addEventListener('load', () => {
    navigator.serviceWorker
      .register(new URL('sw.js', base), { scope: base.pathname })
      .catch((e) => {
        console.warn('service worker', e);
      });
  });
}
