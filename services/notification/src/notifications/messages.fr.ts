// Every user-facing French text of the service. Notifications store the text
// rendered when they are written: changing a text affects new ones only.

export interface Message {
  title: string;
  body: string;
}

const EXCERPT_MAX = 100;

/** Name shown for an actor whose profile this service has not received. */
export const UNKNOWN_ACTOR = 'Quelqu’un';

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
