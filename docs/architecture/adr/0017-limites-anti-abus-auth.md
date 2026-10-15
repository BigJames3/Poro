# ADR-0017 — Limites anti-abus d'auth : budget SMS et verrouillage du login

- Statut : accepté
- Date : 2026-10-15

## Contexte

L'audit du 14/10/2026 (PORO-005, PORO-006) a relevé deux abus possibles :

- **SMS pumping.** L'OTP n'était limité que par numéro (60 s entre deux codes, 5 par heure) et
  par IP (5 demandes par minute). Avec des IP tournantes et des numéros différents, souvent des
  plages de numéros surtaxés louées pour l'occasion, le coût SMS n'était pas borné.
- **Force brute répartie sur le mot de passe.** La connexion email n'était limitée que par IP :
  un compte pouvait être attaqué depuis autant d'adresses qu'on veut, et la re-saisie du mot de
  passe avant suppression du compte n'avait aucune limite propre.

## Décisions

### Budget SMS

Avant chaque envoi, quatre plafonds sont vérifiés puis consommés ensemble, de façon atomique
(script Lua Redis) :

| Plafond | Défaut | Variable | Dépassé |
|---|---|---|---|
| Par numéro et par jour UTC | 10 | `SMS_DAILY_LIMIT_PER_PHONE` | 429 `otp_throttled` |
| Par plage (numéro sans ses 3 derniers chiffres) et par heure | 20 | `SMS_HOURLY_LIMIT_PER_PREFIX` | 429 `otp_throttled` |
| Par indicatif pays et par heure | 1 000 | `SMS_HOURLY_LIMIT_PER_COUNTRY` | 429 `otp_throttled` |
| Global et par jour UTC (disjoncteur) | 5 000 | `SMS_DAILY_LIMIT` | 503 `sms_unavailable` |

- Un refus ne compte contre aucun plafond, et ne déclenche pas le délai de 60 s du numéro.
- Un SMS que le fournisseur n'a pas envoyé est rendu au budget.
- `Retry-After` indique la fin de la fenêtre atteinte.
- Le disjoncteur global coupe tous les OTP SMS jusqu'au jour UTC suivant et journalise une
  erreur ; la connexion par email reste possible. Couper plutôt que seulement alerter borne le
  coût d'une attaque.
- Métriques : `auth_sms_sent_total{country}` et `auth_sms_blocked_total{reason}`, pour les
  alertes de PORO-009.

### Verrouillage de la connexion par compte

- Un compte est désigné par son adresse email normalisée. La clé Redis est son empreinte
  SHA-256 : aucune adresse n'est stockée en clair.
- Le compteur vaut aussi pour une adresse inconnue : la réponse ne révèle pas si le compte
  existe.
- Après 5 échecs (`LOGIN_MAX_FAILURES`) en moins de 15 minutes (`LOGIN_FAILURE_WINDOW`), la
  connexion est bloquée 1 minute (`LOGIN_LOCK_BASE`). Chaque échec suivant double le blocage,
  jusqu'à 15 minutes (`LOGIN_LOCK_MAX`).
- Un blocage conserve les échecs pour sa durée plus la fenêtre : attendre la fin d'un blocage
  ne redonne pas cinq essais. Au plafond, chaque nouvel échec rebloque 15 minutes, soit environ
  4 essais par heure et par compte, quel que soit le nombre d'IP.
- Pendant un blocage, la réponse est 429 `login_throttled` avec `Retry-After`, **sans vérifier
  le mot de passe**.
- La re-saisie du mot de passe avant suppression du compte partage le même verrou : une session
  volée ne permet pas de deviner le mot de passe.
- Une connexion réussie ou une réinitialisation du mot de passe remet le compteur à zéro.
- La limite par IP existante reste en place.

### Reportés

- **Défi anti-robot** (Play Integrity ou captcha) sur la demande d'OTP : reporté à
  l'application Android, qui doit fournir le jeton.
- **Email d'alerte « tentatives de connexion »** : reporté aux emails transactionnels
  (PORO-037).

## Conséquences

- Les valeurs par défaut sont des estimations, à recalibrer avec le trafic réel et le prix des
  SMS Africa's Talking par pays.
- Un blocage peut gêner le vrai titulaire (un tiers peut le bloquer volontairement). Le
  blocage progressif limite la gêne, et le titulaire peut toujours réinitialiser son mot de
  passe ou se connecter par OTP.
- Redis est sur le chemin de la connexion et de l'OTP : s'il est indisponible, ces requêtes
  échouent (500). Auth en dépendait déjà pour ses limites.

## Alternatives écartées

- **Verrouillage dur** jusqu'à intervention : un tiers pourrait bloquer n'importe quel compte.
- **Clé par adresse en clair** : stockerait des données personnelles dans Redis sans besoin.
- **Seulement alerter** quand le budget global est atteint : le coût d'une attaque nocturne
  ne serait borné par rien.
