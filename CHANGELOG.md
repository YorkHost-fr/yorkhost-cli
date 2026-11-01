# Changelog

Toutes les évolutions notables du CLI. Format : [Keep a Changelog](https://keepachangelog.com/fr/1.1.0/),
versions : [SemVer](https://semver.org/lang/fr/).

## [Non publié]

## [0.2.0] - 2026-11-01

### Ajouté

- `antiddos rules list/create/move/delete` : règles anti-DDoS de vos IP (politiques disponibles,
  IP éligibles, création en attente d'application par le NOC, ordre, suppression avec
  confirmation ou `--yes`), en table ou `-o json`.
- `antiddos attacks/targets/status --ip` : attaques et cibles d'une seule adresse.

## [0.1.1] - 2026-10-20

### Ajouté

- Publication en un clic : *Actions → Release → Run workflow* calcule la prochaine version
  d'après les commits (`scripts/next-version.sh`), la tague et la publie.

### Corrigé

- Sécurisation du téléchargement des factures : écriture temporaire, remplacement uniquement
  après succès et option `--force`.
- Documentation du fait que `yorkhost api` contourne les confirmations des commandes dédiées.

## [0.1.0] - 2026-10-19

### Ajouté

- Première version du CLI `yorkhost`, client de l'API client YorkHost (`/client/v1`).
- Connexion par navigateur (OAuth 2.1 + PKCE, redirection loopback, A2F) avec rafraîchissement
  automatique sous verrou, ou par clé API (`--api-key`, `--with-token`, `YORKHOST_API_KEY`).
- Profils multiples (`--profile`, `config profile …`) et réglages (`config get/set/unset`).
- `service`, `vps` (dont `metrics` et `ssh`), `game` (dont `kill`, `open`), avec `--wait`
  et désignation par ID ou par nom.
- `invoice` (liste, détail, PDF, paiement), `ticket` (liste, lecture, création, réponse),
  `antiddos`, `catalog` et `order create` (devis puis confirmation).
- `sandbox` : création, SSH via `yorkhost.sh`, `exec`, fichiers (`ls`, `cat`, `upload`,
  `download`), lien bureau, passage en VPS (`keep`, `claim`), clés SSH et secrets du compte.
- `api` (requêtes brutes), `mcp` (configuration Claude Code / Cursor / VS Code), `version --check`,
  autocomplétion bash/zsh/fish/PowerShell avec IDs dynamiques.
- Sorties `table`, `wide`, `json`, `yaml` ; codes de sortie documentés.
- Releases GoReleaser (Linux, macOS, Windows, FreeBSD ; deb, rpm, apk), scripts d'installation
  avec vérification SHA-256.

[Non publié]: https://github.com/YorkHost-fr/yorkhost-cli/compare/v0.2.0...HEAD
[0.2.0]: https://github.com/YorkHost-fr/yorkhost-cli/compare/v0.1.1...v0.2.0
[0.1.1]: https://github.com/YorkHost-fr/yorkhost-cli/compare/v0.1.0...v0.1.1
[0.1.0]: https://github.com/YorkHost-fr/yorkhost-cli/releases/tag/v0.1.0
