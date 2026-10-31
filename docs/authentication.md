# Authentification

Le CLI agit en votre nom sur l'API client YorkHost. Deux façons de lui donner accès :

| Méthode | Pour qui | Durée | Révocation |
|---|---|---|---|
| **Navigateur** (`yorkhost login`) | votre poste | jeton 1 h renouvelé automatiquement, session 30 jours glissants | `yorkhost logout`, ou *Clés API → Applications connectées* |
| **Clé API** (`--api-key`, `YORKHOST_API_KEY`) | serveurs, CI, scripts | jusqu'à l'expiration choisie | *Clés API* dans l'espace client |

Dans les deux cas, la **double authentification (A2F)** doit être active sur le compte
(sinon : erreur `mfa_required`). Activez-la sur <https://clients.yorkhost.fr/mfa/>.

## Connexion par navigateur

```sh
yorkhost login
```

1. Le navigateur s'ouvre sur votre espace client.
2. Connectez-vous (A2F).
3. Choisissez les **permissions** et les **services** auxquels le CLI aura accès, puis validez.
4. Revenez au terminal : `✓ Connecté en tant que …`.

**Machine distante ou sans interface graphique** : `yorkhost login --no-browser` affiche le
lien. Le retour se fait sur `http://127.0.0.1:<port>` *de la machine qui exécute le CLI* :
ouvrez le lien dans un navigateur de cette machine, ou faites suivre le port, par exemple
`ssh -L <port>:127.0.0.1:<port> serveur`. Sur un serveur, une clé API est souvent plus simple.

**Moins de permissions** :

```sh
yorkhost login --scopes services:read,vps:read,vps:power
```

## Clé API

Créez une clé dans l'espace client : *Clés API → Nouvelle clé* (nom, permissions, services,
expiration, IP autorisées). Elle n'est affichée qu'une fois.

```sh
yorkhost login --api-key                       # saisie masquée, enregistrée dans le profil
echo "$KEY" | yorkhost login --with-token      # depuis un gestionnaire de secrets
export YORKHOST_API_KEY=yhk_client_v1_…        # sans rien enregistrer (prioritaire sur le profil)
```

`YORKHOST_TOKEN` est accepté comme alias de `YORKHOST_API_KEY`.

### En CI (GitHub Actions)

```yaml
- name: Redémarrer le serveur de jeu après déploiement
  env:
    YORKHOST_API_KEY: ${{ secrets.YORKHOST_API_KEY }}
  run: |
    curl -fsSL https://raw.githubusercontent.com/YorkHost-fr/yorkhost-cli/main/scripts/install.sh | sh
    yorkhost game restart 5678 --yes --wait
```

Bonnes pratiques : une clé par usage, permissions minimales, restreinte aux services concernés,
allowlist IP si l'adresse de sortie est fixe, expiration.

## Permissions (scopes)

| Permission | Ouvre |
|---|---|
| `account:read` | `account`, `whoami` |
| `services:read` | `service`, `vps list`, `game list`, `catalog`, résolution des noms |
| `billing:read` | `invoice list/get/pdf/pay` |
| `tickets:read` / `tickets:write` | `ticket list/get/types` / `ticket create/reply` |
| `vps:read` / `vps:power` | `vps get/metrics/ssh` / `vps start/stop/restart` |
| `game:read` / `game:power` | `game get/metrics/open` / `game start/stop/restart/kill` |
| `antiddos:read` / `antiddos:write` | `antiddos attacks/targets/status`, `antiddos rules list` / `antiddos rules create/move/delete` |
| `orders:write` | `order create`, `sandbox keep`, `sandbox claim` |
| `sandbox:read` / `sandbox:write` | `sandbox list/get/ls/cat/download/offers`, `key list`, `secret list` / le reste de `sandbox` |

Une permission manquante donne une erreur explicite :

```
Erreur : this API key lacks the required permission (HTTP 403, missing_scope)
  → cette clé n'a pas la permission "vps:power" : ajoutez-la dans l'espace client (Clés API) ou relancez yorkhost login
```

Correspondance complète commande ↔ route ↔ permission : [api.md](api.md).

## Vérifier, utiliser, révoquer

```sh
yorkhost whoami              # compte, profil, type d'authentification, échéance
yorkhost auth status         # identifiants de chaque profil (sans les secrets)
yorkhost auth token          # jeton courant, pour curl ou un autre outil
yorkhost logout              # révoque la session navigateur et oublie les identifiants
yorkhost logout --all        # tous les profils
```

`logout` n'invalide pas une **clé API** côté YorkHost (le CLI ne l'a pas créée) : révoquez-la
dans l'espace client si elle ne doit plus servir.

## Où sont stockés les identifiants ?

Dans le fichier de configuration (`yorkhost config path`), créé avec les droits `0600`
(lecture par vous seul). Ne le partagez pas et ne le versionnez pas. Le CLI n'écrit jamais un
jeton dans un journal : `--debug` masque l'en-tête `Authorization`, et `whoami` n'affiche que
l'identifiant public d'une clé.

Fonctionnement interne (PKCE, rafraîchissement sous verrou, rotation) : [architecture.md](architecture.md#authentification).
