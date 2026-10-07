import { UNKNOWN_ACTOR, actorName, messages } from './messages.fr';

describe('French messages', () => {
  it('names actors by display name, then username', () => {
    expect(actorName({ displayName: ' Awa ', username: 'awa' })).toBe('Awa');
    expect(actorName({ displayName: '  ', username: 'awa' })).toBe('@awa');
    expect(actorName({ displayName: null, username: null })).toBe(UNKNOWN_ACTOR);
    expect(actorName(null)).toBe(UNKNOWN_ACTOR);
  });

  it('pluralizes grouped likes', () => {
    expect(messages.like('Awa', 0).body).toBe('Awa a aimé ta vidéo');
    expect(messages.like('Awa', 1).body).toBe('Awa et 1 autre personne ont aimé ta vidéo');
    expect(messages.like('Awa', 12).body).toBe('Awa et 12 autres personnes ont aimé ta vidéo');
  });

  it('quotes and shortens excerpts', () => {
    const long = 'é'.repeat(120);
    expect(messages.comment('Awa', 'Bravo\n  !').body).toBe(
      'Awa a commenté ta vidéo : « Bravo ! »',
    );
    expect(messages.reply('Awa', long).body).toBe(
      `Awa a répondu à ton commentaire : « ${'é'.repeat(100)}… »`,
    );
  });

  it('describes ready videos with or without a title', () => {
    expect(messages.videoReady(undefined).body).toBe('Elle est maintenant visible par tous.');
    expect(messages.videoReady('  ').body).toBe('Elle est maintenant visible par tous.');
    expect(messages.videoReady('Dakar').body).toBe('« Dakar » est maintenant visible par tous.');
    expect(messages.follow('Awa')).toEqual({
      title: 'Nouvel abonné',
      body: 'Awa a commencé à te suivre',
    });
  });
});
