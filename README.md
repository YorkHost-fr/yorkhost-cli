<h1 align="center">yorkhost — le CLI officiel YorkHost</h1>

<p align="center">
  Pilotez vos VPS, serveurs de jeu, factures, tickets et sandboxes agents depuis le terminal.<br>
  Un seul binaire, Linux · macOS · Windows · FreeBSD.
</p>

<p align="center">
  <a href="https://github.com/YorkHost-fr/yorkhost-cli/actions/workflows/ci.yml"><img src="https://github.com/YorkHost-fr/yorkhost-cli/actions/workflows/ci.yml/badge.svg" alt="CI"></a>
  <a href="https://github.com/YorkHost-fr/yorkhost-cli/releases/latest"><img src="https://img.shields.io/github/v/release/YorkHost-fr/yorkhost-cli?sort=semver" alt="Release"></a>
  <a href="LICENSE"><img src="https://img.shields.io/badge/licence-MIT-blue" alt="Licence MIT"></a>
</p>

```console
$ yorkhost login
Ouverture du navigateur pour vous connecter à votre espace client YorkHost…
✓ Connecté en tant que Ada Lovelace <ada@example.com> (profil "default").

$ yorkhost service list
ID     NOM          TYPE   STATUT   PRODUIT       ÉCHÉANCE
1234   web-prod     vps    active   VPS-2         2026-11-01
5678   fivem-rp     game   active   FiveM Ryzen   2026-11-12

$ yorkhost vps restart web-prod --wait
Redémarrer le VPS 1234 ? Les connexions en cours seront coupées. [o/N] o
Attente de l'état "running"…
✓ VPS 1234 : running.
```

## Sommaire

- [Installation](#installation)
- [Premiers pas](#premiers-pas)
- [Commandes](#commandes)
- [Sorties et scripts](#sorties-et-scripts)
- [Profils et configuration](#profils-et-configuration)
- [Comment ça marche](#comment-ça-marche)
- [Documentation complète](#documentation)
- [Contribuer](#contribuer)

## Installation

**Linux, macOS, FreeBSD**

```sh
curl -fsSL https://raw.githubusercontent.com/YorkHost-fr/yorkhost-cli/main/scripts/install.sh | sh
```

**Windows (PowerShell)**

```powershell
irm https://raw.githubusercontent.com/YorkHost-fr/yorkhost-cli/main/scripts/install.ps1 | iex
```

**Paquets Debian/Ubuntu, Fedora/RHEL, Alpine** : `.deb`, `.rpm` et `.apk` sur la
[page des releases](https://github.com/YorkHost-fr/yorkhost-cli/releases/latest) (autocomplétion et pages `man` incluses).

**Avec Go 1.26+**

```sh
go install github.com/YorkHost-fr/yorkhost-cli/cmd/yorkhost@latest
```

Détails, vérification des sommes de contrôle, autocomplétion : [docs/installation.md](docs/installation.md).

## Premiers pas

```sh
yorkhost login                 # navigateur + double authentification
yorkhost whoami                # compte et identifiants utilisés
yorkhost service list          # tous vos services
yorkhost vps get 1234          # état, IP, CPU, RAM, disque
yorkhost game restart fivem-rp # par ID ou par nom
```

Sur un serveur sans navigateur ou dans une CI, utilisez une **clé API** créée dans
l'espace client (*Clés API → Nouvelle clé*) :

```sh
yorkhost login --api-key                         # saisie masquée
export YORKHOST_API_KEY=yhk_client_v1_…          # ou par variable d'environnement
```

La double authentification doit être active sur le compte. Voir [docs/authentication.md](docs/authentication.md).

## Commandes

Forme générale, comme `scw`, `ovhcloud` ou `aws` : **`yorkhost <ressource> <verbe> [ID] [options]`**.

| Ressource | Verbes | Exemple |
|---|---|---|
| `login`, `logout`, `whoami`, `auth` | connexion, session, jeton | `yorkhost auth token` |
| `service` | `list`, `get` | `yorkhost service list --type game --status active` |
| `vps` | `list`, `get`, `start`, `stop`, `restart`, `metrics`, `ssh` | `yorkhost vps ssh 1234` |
| `game` | `list`, `get`, `start`, `stop`, `restart`, `kill`, `metrics`, `open` | `yorkhost game metrics fivem-rp` |
| `sandbox` | `list`, `get`, `create`, `delete`, `ssh`, `exec`, `ls`, `cat`, `upload`, `download`, `links`, `keep`, `claim`, `offers`, `key …`, `secret …` | `yorkhost sandbox create --ssh` |
| `antiddos` | `attacks`, `targets`, `status` | `yorkhost antiddos attacks --days 7` |
| `account` | `get` | `yorkhost account` |
| `invoice` | `list`, `get`, `pdf`, `pay` | `yorkhost invoice pdf 4242 --open` |
| `ticket` | `list`, `get`, `create`, `reply`, `types` | `yorkhost ticket create -s "Panne" -b @log.txt` |
| `catalog`, `order` | tarifs, commande (devis → confirmation) | `yorkhost order create 42 --cycle a` |
| `config` | `list`, `get`, `set`, `unset`, `profile …`, `path` | `yorkhost config set default_vps 1234` |
| `api` | requête brute (comme `gh api`) | `yorkhost api /services --paginate` |
| `mcp` | brancher Claude Code, Cursor, VS Code | `yorkhost mcp --client cursor` |
| `completion`, `version` | autocomplétion, version | `yorkhost version --check` |

Chaque commande a son aide : `yorkhost <commande> --help`. Référence complète, générée
depuis le code : [docs/commands](docs/commands/yorkhost.md).

**Garde-fous** : dans les commandes dédiées, arrêt, redémarrage, suppression et commande demandent une confirmation
(`--yes` pour l'automatiser). Une commande passe toujours par un **devis** avant de créer
la facture, et le paiement se fait sur la page sécurisée de l'espace client : aucun moyen de
paiement ne transite par le CLI. La commande avancée `yorkhost api` envoie toutefois les
requêtes brutes sans ces confirmations ; vérifiez toute écriture avant de l'exécuter.

## Sorties et scripts

```sh
yorkhost vps list -o json | jq -r '.[] | select(.status=="active") | .id'
yorkhost game get 5678 -o yaml
yorkhost service list -o wide          # colonnes supplémentaires
```

- `-o table` (défaut), `wide`, `json`, `yaml`. En `json`/`yaml`, c'est la réponse de l'API, telle quelle.
- Le résultat va sur **stdout**, les messages et erreurs sur **stderr** : les pipes restent propres.
- Codes de sortie stables : `3` authentification, `4` introuvable, `5` conflit, `6` limite atteinte, `7` API indisponible, `8` confirmation refusée…
- `yorkhost sandbox exec` renvoie le code de sortie de la commande exécutée.

Tout est détaillé dans [docs/scripting.md](docs/scripting.md) (variables d'environnement, CI, exemples).

## Profils et configuration

Plusieurs comptes ou environnements ? Un profil par compte, comme `aws --profile` :

```sh
yorkhost --profile pro login
yorkhost config profile use pro
YORKHOST_PROFILE=perso yorkhost invoice list
```

Configuration dans `~/.config/yorkhost/config.yaml` (droits `0600`) — voir [docs/configuration.md](docs/configuration.md).

## Comment ça marche

```
 yorkhost (ce dépôt)                          yorkhost-api (api.yorkhost.fr)            Back-ends
┌─────────────────────┐   HTTPS + Bearer     ┌────────────────────────────────┐      ┌───────────────┐
│ commandes cobra     │ ───────────────────▶ │ /client/v1/*   API client      │ ───▶ │ HostBill      │
│ client HTTP         │                      │  auth clé/jeton, scopes,       │      │ Proxmox (VPS) │
│ OAuth PKCE loopback │ ◀── /client/oauth ── │  restriction de services,      │      │ Panel de jeu  │
│ ~/.config/yorkhost  │                      │  limites, journal              │      │ Support, DDoS │
└─────────────────────┘                      └────────────────────────────────┘      └───────────────┘
```

Le CLI est un client de l'**API client publique** de YorkHost (`/client/v1`), la même que
le serveur MCP et l'app mobile. Il ne contient aucune logique métier ni aucun secret : chaque
contrôle (propriété du service, permission, A2F, plafonds) est fait côté API. Explication
détaillée — connexion OAuth, rafraîchissement des jetons, relances, sécurité :
[docs/architecture.md](docs/architecture.md).

## Documentation

| Guide | Contenu |
|---|---|
| [Installation](docs/installation.md) | script, paquets, Windows, `go install`, autocomplétion, mise à jour |
| [Authentification](docs/authentication.md) | navigateur (OAuth), clé API, CI, permissions, révocation |
| [Configuration](docs/configuration.md) | profils, réglages, variables d'environnement, priorités |
| [Scripts et automatisation](docs/scripting.md) | formats de sortie, codes de sortie, `jq`, CI, `yorkhost api` |
| [Architecture](docs/architecture.md) | fonctionnement interne, flux OAuth, sécurité |
| [Correspondance API](docs/api.md) | commande ↔ route `/client/v1` ↔ permission |
| [Référence des commandes](docs/commands/yorkhost.md) | toutes les commandes et options (générée) |
| [Publier une version](docs/releasing.md) | tags, GoReleaser, paquets |

## Contribuer

Les contributions sont les bienvenues : [CONTRIBUTING.md](CONTRIBUTING.md).
Faille de sécurité : [SECURITY.md](SECURITY.md) (signalement privé).
Un souci sur un service ou une facture plutôt que sur le CLI ? `yorkhost ticket create`.

## Licence

[MIT](LICENSE) © YorkHost
