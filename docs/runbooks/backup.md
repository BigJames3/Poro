# Runbook — sauvegardes et restauration (PORO-012)

## Ce qui est sauvegardé

| Donnée | Méthode | État |
|---|---|---|
| Bases Postgres `poro_*` (une par service) | `infra/backup/backup.sh` : `pg_dump` par base, chiffré `age` | prêt, testé en CI |
| Objets S3 (vidéos, avatars, images boutique) | réplication du bucket chez un second fournisseur | **à mettre en place avec l'hébergeur** |
| Redis (sessions de feed, limites de débit, registre de révocation) | aucune : données reconstruites ou expirant d'elles-mêmes | voulu |
| Topics Redpanda | aucune : l'outbox Postgres est la source ; un événement non publié repart du relais | voulu |

## Protection contre une compromission simultanée

- **Chiffrement avant écriture** : les dumps sont chiffrés avec `age` pour une ou plusieurs clés
  publiques (`BACKUP_AGE_RECIPIENTS`). La clé privée reste **hors ligne** : coffre du
  responsable technique, plus une copie papier ou une clé matérielle dans un second lieu.
  Ni le serveur de sauvegarde ni le bucket ne peuvent lire les données.
- **Autre compte, autre fournisseur** : le bucket de sauvegarde ne doit pas dépendre des
  identifiants de production. Une compromission de la production ne doit pas permettre
  d'effacer les sauvegardes.
- **Identifiants en écriture seule** : `BACKUP_S3_ACCESS_KEY_ID` peut seulement écrire (ni
  lecture, ni suppression). Les identifiants passent à `curl` par l'entrée standard, jamais sur
  la ligne de commande.
- **Immuabilité** : activer le versioning et l'*Object Lock* (mode *compliance*) sur le bucket,
  avec une durée égale à la rétention. La rétention distante est une règle de cycle de vie du
  bucket ; le script ne supprime jamais rien à distance.
- **Intégrité** : `MANIFEST` porte le SHA-256 de chaque fichier. Il est écrit et envoyé en
  dernier, donc une sauvegarde sans manifeste est incomplète. `age` authentifie aussi le
  contenu.

## RPO / RTO proposés (à valider par la direction)

| | Valeur proposée | Comment |
|---|---|---|
| RPO (perte maximale) | **24 h** au lancement | un `backup.sh` quotidien. Pour descendre à 15 min, utiliser le PITR (archivage WAL) du Postgres managé de l'hébergeur |
| RTO (remise en service) | **4 h** | restauration mesurée par `restore_verify.sh` (temps par base dans le rapport), plus redéploiement des services |
| Rétention | 14 copies quotidiennes locales, 35 jours distants (dont 4 hebdomadaires) | `BACKUP_RETENTION_DAYS`, règle de cycle de vie du bucket |

Les valeurs dépendent du coût accepté et des obligations légales de conservation (commandes) :
ce sont des **décisions métier et juridiques**, non prises ici.

## Mise en place

```bash
age-keygen -o poro-backup.key            # sur un poste hors ligne ; garder ce fichier secret
age-keygen -y poro-backup.key > recipients.txt
```

Tâche planifiée quotidienne, sur un hôte qui atteint Postgres en lecture (rôle avec
`pg_read_all_data`) :

```bash
PGHOST=… PGUSER=poro_backup PGPASSWORD=… \
BACKUP_AGE_RECIPIENTS=/etc/poro/recipients.txt BACKUP_DIR=/var/backups/poro \
BACKUP_S3_ENDPOINT=https://… BACKUP_S3_BUCKET=poro-backups \
BACKUP_S3_ACCESS_KEY_ID=… BACKUP_S3_SECRET_ACCESS_KEY=… \
infra/backup/backup.sh
```

`pg_dump` doit avoir une version supérieure ou égale à celle du serveur (16). Alerter si la tâche
n'a pas réussi depuis 26 h.

## Vérifier une sauvegarde (chaque semaine, et après chaque changement de schéma)

Sur un serveur Postgres **jetable** :

```bash
PGHOST=scratch-db RESTORE_ALLOW_HOST=scratch-db PGUSER=… PGPASSWORD=… \
BACKUP_AGE_IDENTITY=./poro-backup.key \
infra/backup/restore_verify.sh /var/backups/poro/20261015T020000Z
```

Le script vérifie les sommes, déchiffre, restaure chaque base dans `verify_<base>_<pid>`,
contrôle les tables et la table de migrations, puis supprime toujours les bases temporaires.
`RESTORE_ALLOW_HOST` doit répéter `PGHOST` : c'est un garde-fou contre une exécution sur la
production. Noter les durées : elles mesurent le RTO réel.

## Restaurer en production (incident)

Décision humaine, jamais automatique : elle écrase des données.

1. Arrêter les services qui écrivent dans la base concernée, et son relais outbox.
2. Vérifier la sauvegarde avec `restore_verify.sh` sur un serveur jetable.
3. Restaurer dans une **nouvelle** base, puis basculer la configuration du service dessus. Ne
   jamais restaurer par-dessus l'ancienne, que l'on garde pour analyse :
   ```bash
   createdb poro_order_restored
   age -d -i poro-backup.key <dir>/poro_order.dump.age | pg_restore --no-owner -d poro_order_restored
   ```
4. Redémarrer le service. Les migrations au démarrage complètent un schéma plus ancien.
5. Les événements produits après la sauvegarde ont déjà été consommés par les autres services :
   rejouer les événements utiles depuis Redpanda (rétention 7 jours) ou rapprocher à la main.
   Les inbox (`processed_events`) rendent un rejeu sans danger.

## Tests

`infra/backup/tests/run.sh` (job CI `backup`) exécute un aller-retour réel contre Postgres et
SeaweedFS. Il vérifie :

- le refus sans chiffrement ;
- l'absence de clair dans les fichiers et les droits `700` ;
- une restauration identique (500 lignes), avec les bases temporaires supprimées ;
- le refus avec une mauvaise clé, sur un fichier altéré ou sur un hôte non confirmé ;
- qu'aucune sauvegarde partielle ne reste après un échec ;
- une rétention limitée aux copies du script ;
- une copie distante identique.
