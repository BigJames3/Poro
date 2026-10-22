// Every user-facing French text of the service. Notifications store the text
// rendered when they are written: changing a text affects new ones only.

import type { Currency } from '../events/catalog';

export interface Message {
  title: string;
  body: string;
}

const EXCERPT_MAX = 100;

/** Name shown for an actor whose profile this service has not received. */
export const UNKNOWN_ACTOR = 'Quelqu’un';

/** Name shown for a shop this service has not received. */
export const UNKNOWN_SHOP = 'la boutique';

const REFUND_NOTICE = ' Ton remboursement est en cours.';

function capitalize(text: string): string {
  return text.charAt(0).toUpperCase() + text.slice(1);
}

/** Thousands separated by a narrow no-break space, as written in French. */
function group(value: number): string {
  return String(value).replace(/\B(?=(\d{3})+(?!\d))/g, '\u202f');
}

/**
 * An amount in minor units, written in French: XOF and XAF have no minor
 * unit, NGN is in kobo and shows them only when there are some.
 */
export function formatAmount(total: number, currency: Currency): string {
  if (currency === 'NGN') {
    const kobo = total % 100;
    const naira = group(Math.floor(total / 100));
    return `${kobo === 0 ? naira : `${naira},${String(kobo).padStart(2, '0')}`}\u00a0₦`;
  }
  return `${group(total)}\u00a0FCFA`;
}

function quote(excerpt: string): string {
  const text = excerpt.replace(/\s+/g, ' ').trim();
  const chars = Array.from(text);
  const cut = chars.length > EXCERPT_MAX ? `${chars.slice(0, EXCERPT_MAX).join('')}…` : text;
  return `« ${cut} »`;
}

export const messages = {
  like(actor: string, others: number): Message {
    if (others <= 0) {
      return { title: 'Nouveau j’aime', body: `${actor} a aimé ta vidéo` };
    }
    const rest = others === 1 ? '1 autre personne' : `${others} autres personnes`;
    return {
      title: 'Nouveaux j’aime',
      body: `${actor} et ${rest} ont aimé ta vidéo`,
    };
  },

  comment(actor: string, excerpt: string): Message {
    return {
      title: 'Nouveau commentaire',
      body: `${actor} a commenté ta vidéo : ${quote(excerpt)}`,
    };
  },

  reply(actor: string, excerpt: string): Message {
    return {
      title: 'Nouvelle réponse',
      body: `${actor} a répondu à ton commentaire : ${quote(excerpt)}`,
    };
  },

  follow(actor: string): Message {
    return { title: 'Nouvel abonné', body: `${actor} a commencé à te suivre` };
  },

  videoReady(title: string | undefined): Message {
    const name = title?.trim();
    return {
      title: 'Ta vidéo est en ligne',
      body:
        name === undefined || name === ''
          ? 'Elle est maintenant visible par tous.'
          : `${quote(name)} est maintenant visible par tous.`,
    };
  },

  orderNew(buyer: string, amount: string): Message {
    return {
      title: 'Nouvelle commande',
      body: `${buyer} a passé une commande de ${amount}, payée à la livraison.`,
    };
  },

  orderConfirmed(shop: string, amount: string): Message {
    return {
      title: 'Commande confirmée',
      body: `Ta commande chez ${shop} (${amount}) est confirmée. Tu paieras à la livraison.`,
    };
  },

  orderShipped(shop: string, tracking: string | null): Message {
    const follow = tracking?.trim();
    return {
      title: 'Commande expédiée',
      body:
        follow === undefined || follow === ''
          ? `${capitalize(shop)} a expédié ta commande.`
          : `${capitalize(shop)} a expédié ta commande. Suivi : ${quote(follow)}`,
    };
  },

  orderRejected(shop: string, refund: boolean): Message {
    return {
      title: 'Commande refusée',
      body:
        `${capitalize(shop)} n’a pas pu valider ta commande : un article n’est plus disponible.` +
        (refund ? REFUND_NOTICE : ''),
    };
  },

  orderCancelledBySeller(shop: string, refund: boolean): Message {
    return {
      title: 'Commande annulée',
      body: `${capitalize(shop)} a annulé ta commande.` + (refund ? REFUND_NOTICE : ''),
    };
  },

  orderPaymentTimeout(shop: string, refund: boolean): Message {
    return {
      title: 'Commande annulée',
      body:
        `Ta commande chez ${shop} est annulée faute de paiement dans le délai.` +
        (refund ? REFUND_NOTICE : ''),
    };
  },

  orderCancelledShopSuspended(shop: string, refund: boolean): Message {
    return {
      title: 'Commande annulée',
      body:
        `Ta commande chez ${shop} est annulée : la boutique a été suspendue.` +
        (refund ? REFUND_NOTICE : ''),
    };
  },

  orderCancelledByBuyer(buyer: string): Message {
    return { title: 'Commande annulée', body: `${buyer} a annulé sa commande.` };
  },

  /** Either party may be the erased account: the text names neither. */
  orderCancelledAccountDeleted(shop: string, refund: boolean): Message {
    return {
      title: 'Commande annulée',
      body:
        `Ta commande chez ${shop} est annulée suite à la suppression d’un compte.` +
        (refund ? REFUND_NOTICE : ''),
    };
  },

  orderCancelledAccountDeletedSeller(): Message {
    return {
      title: 'Commande annulée',
      body: 'Une commande a été annulée suite à la suppression d’un compte.',
    };
  },

  orderCompleted(buyer: string, amount: string): Message {
    return {
      title: 'Commande terminée',
      body: `La commande passée par ${buyer} (${amount}) est terminée.`,
    };
  },
};

/** The display name of an actor: display name, then @username, then a placeholder. */
export function actorName(
  profile: { displayName: string | null; username: string | null } | null,
): string {
  const display = profile?.displayName?.trim();
  if (display !== undefined && display !== '') {
    return display;
  }
  if (profile?.username) {
    return `@${profile.username}`;
  }
  return UNKNOWN_ACTOR;
}
