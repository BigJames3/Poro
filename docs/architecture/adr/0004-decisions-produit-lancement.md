# ADR-0004 — Périmètre de lancement, SMS et paiements

- Statut : accepté (décisions du propriétaire du 30 septembre 2026)
- Date : 2026-09-30

## Décisions

| Sujet | Décision |
|---|---|
| Périmètre MVP | **Social d'abord** (vidéo, feed, social, notifications, modération). La marketplace passe en P1, immédiatement après. |
| Pays de lancement | Côte d'Ivoire (+225), Sénégal (+221), Cameroun (+237), Nigeria (+234) |
| Fournisseur SMS | Africa's Talking |
| Paiements (P1) | Wave et GeniusPay en premier |

## Conséquences techniques

**SMS / OTP**
- `SMS_PROVIDER=africastalking` : `POST https://api.africastalking.com/version1/messaging`
  (form-urlencoded, en-tête `apiKey`). Le nom d'utilisateur `sandbox` sélectionne le bac à sable,
  interdit en `prod`.
- Seuls les statuts destinataire 100 (Processed), 101 (Success) et 102 (Queued) valent succès.
  403/404/406 (numéro invalide, type non supporté, liste noire) → `422 phone_unreachable` ;
  tout le reste (solde insuffisant, sender ID refusé, panne, clé invalide) → `503 sms_unavailable`,
  cause exacte dans les logs uniquement.
- Aucun renvoi automatique : un timeout peut quand même livrer le SMS, un renvoi enverrait un
  second code. Après un échec, le délai de 60 s est levé pour permettre un nouvel essai ;
  le plafond horaire (5/h) compte toujours la tentative.
- `OTP_ALLOWED_CALLING_CODES=+225,+221,+237,+234` : les autres pays reçoivent
  `422 phone_country_not_supported`. C'est la protection principale contre la fraude
  « SMS pumping » (envoi massif vers des numéros surtaxés), qui peut vider le solde en quelques heures.
- Texte du SMS : anglais pour +234, français pour les trois autres pays.

**À vérifier avec Africa's Talking avant la production** (je n'ai pas pu le confirmer : leur
documentation est derrière une protection anti-robot, et ces points sont contractuels) :
1. couverture et tarifs réels vers les quatre pays, notamment Sénégal et Cameroun ;
2. enregistrement du Sender ID alphanumérique `PORO` par pays (obligatoire au Nigeria, avec
   délais de plusieurs semaines) ;
3. route transactionnelle au Nigeria pour éviter le filtrage DND ;
4. alerte de solde bas côté compte Africa's Talking.

**Paiements**
- L'abstraction `PaymentProvider` du service commerce sera conçue autour de Wave et GeniusPay :
  initiation, redirection ou USSD, webhook signé, vérification serveur du statut, idempotence.
  Stripe reste possible plus tard derrière la même interface.
- Le frontend n'est jamais source de vérité : seul un webhook vérifié ou une interrogation
  serveur du fournisseur confirme un paiement.
