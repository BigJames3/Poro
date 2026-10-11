import { UNKNOWN_ACTOR, UNKNOWN_SHOP, actorName, formatAmount, messages } from './messages.fr';

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

  it('writes amounts in minor units the French way', () => {
    expect(formatAmount(0, 'XOF')).toBe('0\u00a0FCFA');
    expect(formatAmount(950, 'XOF')).toBe('950\u00a0FCFA');
    expect(formatAmount(12500, 'XAF')).toBe('12\u202f500\u00a0FCFA');
    expect(formatAmount(1234567890, 'XOF')).toBe('1\u202f234\u202f567\u202f890\u00a0FCFA');
    expect(formatAmount(1250000, 'NGN')).toBe('12\u202f500\u00a0₦');
    expect(formatAmount(1250050, 'NGN')).toBe('12\u202f500,50\u00a0₦');
    expect(formatAmount(5, 'NGN')).toBe('0,05\u00a0₦');
  });

  it('describes each step of an order', () => {
    expect(messages.orderNew('Awa', '12 500 FCFA')).toEqual({
      title: 'Nouvelle commande',
      body: 'Awa a passé une commande de 12 500 FCFA, payée à la livraison.',
    });
    expect(messages.orderConfirmed('Chez Fatou', '12 500 FCFA')).toEqual({
      title: 'Commande confirmée',
      body: 'Ta commande chez Chez Fatou (12 500 FCFA) est confirmée. Tu paieras à la livraison.',
    });
    expect(messages.orderShipped(UNKNOWN_SHOP, null).body).toBe(
      'La boutique a expédié ta commande.',
    );
    expect(messages.orderShipped('Chez Fatou', '  ').body).toBe(
      'Chez Fatou a expédié ta commande.',
    );
    expect(messages.orderShipped('Chez Fatou', 'Moto, arrivée vers 16 h').body).toBe(
      'Chez Fatou a expédié ta commande. Suivi : « Moto, arrivée vers 16 h »',
    );
    expect(messages.orderCompleted('Awa', '12 500 FCFA')).toEqual({
      title: 'Commande terminée',
      body: 'La commande passée par Awa (12 500 FCFA) est terminée.',
    });
    expect(messages.orderCancelledByBuyer('Awa')).toEqual({
      title: 'Commande annulée',
      body: 'Awa a annulé sa commande.',
    });
  });

  it('explains cancellations to the buyer, with the refund when paid', () => {
    expect(messages.orderRejected('Chez Fatou', false)).toEqual({
      title: 'Commande refusée',
      body: 'Chez Fatou n’a pas pu valider ta commande : un article n’est plus disponible.',
    });
    expect(messages.orderCancelledBySeller(UNKNOWN_SHOP, false).body).toBe(
      'La boutique a annulé ta commande.',
    );
    expect(messages.orderCancelledBySeller('Chez Fatou', true).body).toBe(
      'Chez Fatou a annulé ta commande. Ton remboursement est en cours.',
    );
    expect(messages.orderPaymentTimeout('la boutique', true).body).toBe(
      'Ta commande chez la boutique est annulée faute de paiement dans le délai. Ton remboursement est en cours.',
    );
  });
});
