# Architecture — comment fonctionne le CLI

Ce document explique ce qui se passe entre la commande que vous tapez et votre VPS : qui
parle à qui, comment l'authentification fonctionne, et où se trouvent les garde-fous.

## Vue d'ensemble

```
                         ┌──────────────────────── yorkhost-api (api.yorkhost.fr) ────────────────────────┐
 Terminal                │                                                                                │
┌───────────────┐  HTTPS │  /client/oauth/*   serveur d'autorisation OAuth 2.1 ──┐                        │
│ yorkhost      │ ──────▶│  /client/v1/*      API client publique                │                        │
│               │ Bearer │      │ 1. clé/jeton valide ? compte actif ? A2F ?     │                        │
└───────────────┘        │      │ 2. route ouverte aux clés ? permission (scope)?│                        │
        │                │      │ 3. service autorisé par la clé et à vous ?     │                        │
        │                │      │ 4. limites (par clé, par action, par IP)       │                        │
        ▼                │      │ 5. journal (visible dans l'espace client)      │                        │
 ~/.config/yorkhost/     │      ▼                                                 ▼                        │
   config.yaml           │  handlers métier ──▶ HostBill (services, factures, commandes, comptes)         │
 (profils, jetons, 0600) │                  ──▶ Proxmox (VPS) · panel de jeu (serveurs) · support · DDoS  │
                         └────────────────────────────────────────────────────────────────────────────────┘
                                     ▲
 Navigateur ── connexion + A2F + consentement ── espace client HostBill (clients.yorkhost.fr)
```

Trois briques :

1. **Ce CLI** (Go, un binaire statique). Il traduit les commandes en requêtes HTTP, met en
   forme les réponses et gère vos identifiants localement. **Aucune logique métier, aucun
   secret embarqué** : il n'a pas plus de droits que vous dans l'espace client.
2. **L'API client** (`yorkhost-api`, routes `/client/v1`). Elle authentifie chaque requête,
   applique les permissions et appelle les back-ends. C'est la même API que le serveur MCP
   (`/client/mcp`) utilisé par les assistants IA et que l'app mobile : une seule
   implémentation, donc un seul comportement.
3. **L'espace client HostBill** (module *Clés API*). C'est là que vous créez des clés,
   choisissez leurs permissions et services, consultez leur journal et déconnectez les
   applications — y compris le CLI.

## Ce que fait une commande

Exemple : `yorkhost vps restart web-prod --wait`.

1. **Configuration** (`internal/config`) : lecture de `~/.config/yorkhost/config.yaml`,
   choix du profil (`--profile` > `YORKHOST_PROFILE` > profil actif), de l'URL d'API et du
   format de sortie.
2. **Résolution de l'ID** : `web-prod` n'est pas un nombre, donc le CLI liste vos services
   (`GET /client/v1/services`) et cherche un VPS de ce nom (exact, puis préfixe unique).
   Un nom ambigu est refusé avec la liste des candidats.
3. **Confirmation** (`internal/prompt`) : redémarrer coupe les connexions, donc le CLI
   demande `[o/N]`. Hors terminal (script, CI) il refuse sans `--yes` plutôt que de rester
   bloqué.
4. **Jeton** (`internal/auth`) : clé API telle quelle, ou jeton OAuth rafraîchi s'il expire
   dans moins d'une minute (voir plus bas).
5. **Requête** (`internal/api`) : `POST /client/v1/vps/1234/power {"action":"restart"}`
   avec `Authorization: Bearer …` et un `User-Agent` `yorkhost-cli/<version>`.
6. **Côté API** : clé valide, compte actif, A2F active, permission `vps:power`, service 1234
   autorisé par la clé et à vous, limite de 10 redémarrages par minute non atteinte →
   l'action est lancée sur Proxmox, et la requête est inscrite au journal de la clé.
7. **Attente** (`--wait`) : le CLI interroge `GET /client/v1/vps/1234` toutes les 3 s
   jusqu'à l'état `running` (5 min au plus, `--timeout`).
8. **Rendu** (`internal/output`) : message sur stdout, progression sur stderr ; en `-o json`,
   l'état final du VPS.

## Authentification

Deux méthodes, détaillées pour l'utilisateur dans [authentication.md](authentication.md).

### Connexion par navigateur (OAuth 2.1 + PKCE)

C'est le même mécanisme que `gcloud auth login` ou `az login` : une application native avec
redirection *loopback* (RFC 8252).

```
yorkhost login                    yorkhost-api                         navigateur / HostBill
     │ ouvre 127.0.0.1:<port libre>     │                                      │
     │── POST /client/oauth/register ──▶│ app publique, redirect_uri exacte     │
     │◀───────────── client_id ─────────│                                      │
     │  verifier aléatoire, challenge = SHA-256(verifier), state aléatoire      │
     │── ouvre /client/oauth/authorize?client_id&redirect_uri&challenge&state ─▶│
     │                                  │── redirige vers l'espace client ────▶│
     │                                  │   connexion + A2F + choix des         │
     │                                  │   permissions et des services         │
     │◀──────── 127.0.0.1:<port>/callback?code=…&state=… ──────────────────────│
     │  vérifie state                   │                                      │
     │── POST /client/oauth/token (code + verifier) ─▶│ vérifie PKCE, code unique │
     │◀──── access_token (1 h) + refresh_token (30 j) │                          │
     │  enregistre dans config.yaml (0600), puis GET /client/v1/account          │
```

Points de sécurité :

- **PKCE S256 obligatoire** : un code intercepté est inutilisable sans le `verifier`, qui ne
  quitte jamais le CLI.
- **`state`** aléatoire vérifié au retour : un callback forgé est rejeté.
- Le serveur local n'écoute que sur **127.0.0.1**, sur un port choisi au hasard, et seulement
  le temps de la connexion (10 min au plus).
- L'**A2F** est exigée par l'espace client, et l'API la revérifie à chaque requête.
- Vous choisissez sur la page de consentement **quelles permissions et quels services** le CLI
  peut utiliser ; `--scopes` permet d'en demander moins dès le départ.

### Rafraîchissement automatique

Le jeton d'accès vaut 1 h ; le *refresh token* 30 jours glissants et **ne sert qu'une fois**
(chaque rafraîchissement en renvoie un nouveau, et un ancien rejoué plus d'une minute après
révoque toute la session — protection contre le vol de jeton).

Le CLI rafraîchit donc avec précaution (`internal/auth/tokens.go`) :

1. Si le jeton expire dans moins d'une minute — ou si l'API répond 401 — le CLI prend un
   **verrou** (`config.yaml.lock`, création exclusive, abandonné après 30 s si un process est
   mort).
2. Il **relit** le fichier : si un autre `yorkhost` lancé en parallèle a déjà rafraîchi, il
   réutilise ce jeton au lieu de consommer le refresh token une seconde fois.
3. Sinon il appelle `POST /client/oauth/token` et **enregistre immédiatement** le nouveau
   couple (écriture atomique : fichier temporaire puis renommage).
4. Si le serveur répond `invalid_grant` (session révoquée, expirée, mot de passe changé), les
   jetons sont effacés et le CLI demande un nouveau `yorkhost login`.

`yorkhost logout` révoque la session côté serveur (`/client/oauth/revoke`) avant d'oublier les
jetons ; vous pouvez aussi la couper dans *Clés API → Applications connectées*.

### Clé API

Une clé `yhk_client_v1_<id>_<secret>` créée dans l'espace client est envoyée telle quelle. Elle
ne s'expire pas toute seule (sauf date choisie à la création), peut être limitée à certaines
IP, et se révoque dans l'espace client. `YORKHOST_API_KEY` a priorité sur le profil : idéal en CI.

## Client HTTP et relances

`internal/api` applique des règles simples et sûres :

| Situation | Comportement |
|---|---|
| `429 Too Many Requests` | attente du `Retry-After` (≤ 30 s) puis nouvelle tentative, toutes méthodes : la limite est vérifiée avant toute action, rien n'a été exécuté |
| erreur réseau, `502`/`503`/`504` | jusqu'à 2 nouvelles tentatives **pour les lectures (GET) seulement** ; une commande ou un redémarrage n'est jamais rejoué automatiquement |
| `401` avec un jeton OAuth | un rafraîchissement puis une nouvelle tentative |
| erreur API | message de l'API + code + conseil (`missing_scope` → quelle permission ajouter, `mfa_required` → lien A2F…) et [code de sortie](scripting.md#codes-de-sortie) dédié |

`--debug` affiche chaque requête (méthode, URL, statut, durée, taille) sans jamais afficher
l'en-tête `Authorization`.

## Commandes et paiements

Commander (`order create`, `sandbox keep`, `sandbox claim`) suit le contrat de l'API en deux
temps :

1. **Devis** : `POST` sans `confirmation_token` → prix, frais, montant dû, et un jeton de
   confirmation valable 10 min, lié à votre clé, au produit, au cycle et au prix. Rien n'est
   commandé.
2. **Confirmation** (`[o/N]` ou `--yes`) : même `POST` avec le jeton → commande et facture.
   Le jeton ne sert qu'une fois ; un plafond de dépense journalier s'applique côté API.

La facture est réglée par le crédit du compte s'il suffit ; sinon le CLI affiche le **lien de
la page de paiement sécurisée** de l'espace client (3-D Secure, PayPal…). Aucun moyen de
paiement ne transite par le CLI ni par l'API.

## Organisation du code

```
cmd/yorkhost/          point d'entrée (signal Ctrl-C → contexte annulé)
internal/
  api/                 client HTTP : auth Bearer, erreurs, relances, pagination
  auth/                OAuth (enregistrement, PKCE, callback loopback, jeton, révocation)
                       et source de jetons avec rafraîchissement sous verrou
  config/              fichier YAML, profils, réglages, écriture atomique, verrou
  output/              table / wide / json / yaml, formats (€, Mo, %, dates), couleurs
  prompt/              confirmations, saisie masquée ; jamais de question hors terminal
  cli/                 une commande cobra par ressource (vps, game, sandbox, invoice…)
  version/             version injectée au build (goreleaser) ou lue du module
docs/commands/         référence générée (make docs)
scripts/               install.sh, install.ps1
```

Les tests (`go test ./...`) couvrent le client HTTP, le flux OAuth complet contre un faux
serveur d'autorisation (dont deux rafraîchissements concurrents), la configuration et, de bout
en bout, les commandes principales contre un faux serveur `/client/v1` reprenant les formats de
réponse réels.

## Ce que le CLI ne fait pas (encore)

Les routes de l'API client actuelles couvrent la lecture, l'alimentation (start/stop/restart),
la facturation, le support, l'anti-DDoS, les commandes et les sandboxes. Les permissions
`vps:manage`, `vps:reinstall`, `game:console`, `game:files:read`, `game:manage`,
`game:reinstall` et `dns:*` existent déjà mais ne servent encore à aucune commande du CLI :
les commandes correspondantes (réinstallation, console, fichiers de jeu, DNS…) arriveront
plus tard. Le CLI demande ces permissions dès la connexion, pour ne pas avoir à se reconnecter.
