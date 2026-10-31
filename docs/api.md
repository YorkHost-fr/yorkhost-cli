# Correspondance commandes ↔ API

Le CLI s'appuie uniquement sur l'API client publique `https://api.yorkhost.fr/client/v1`
(spécification OpenAPI dans le dépôt `yorkhost-api` : `docs/openapi-client-v1.yaml`). Ce
tableau indique, pour chaque commande, la route appelée et la permission requise.

| Commande | Route | Permission |
|---|---|---|
| `account`, `whoami`, contrôle après `login` | `GET /account` | `account:read` |
| `service list`, `vps list`, `game list`, noms de services | `GET /services` (toutes les pages) | `services:read` |
| `service get` | `GET /services/{id}` | `services:read` |
| `vps get`, `vps ssh`, `vps … --wait` | `GET /vps/{id}` | `vps:read` |
| `vps metrics` | `GET /vps/{id}/metrics` | `vps:read` |
| `vps start/stop/restart` | `POST /vps/{id}/power` `{"action"}` | `vps:power` |
| `game get`, `game open`, `game … --wait` | `GET /game-servers/{id}` | `game:read` |
| `game metrics` | `GET /game-servers/{id}/metrics` | `game:read` |
| `game start/stop/restart/kill` | `POST /game-servers/{id}/power` `{"action"}` | `game:power` |
| `invoice list` | `GET /invoices` (toutes les pages) | `billing:read` |
| `invoice get` | `GET /invoices/{id}` | `billing:read` |
| `invoice pdf` | `GET /invoices/{id}/pdf` → lien signé de 5 min | `billing:read` |
| `invoice pay` | `GET /invoices/{id}/payment-link` | `billing:read` |
| `ticket list` | `GET /tickets` (toutes les pages) | `tickets:read` |
| `ticket get` | `GET /tickets/{id}` | `tickets:read` |
| `ticket types` | `GET /ticket-departments` | `tickets:read` |
| `ticket create` | `POST /tickets` | `tickets:write` |
| `ticket reply` | `POST /tickets/{id}/reply` | `tickets:write` |
| `antiddos attacks/targets/status` | `GET /antiddos?days=&ip=` | `antiddos:read` |
| `antiddos rules list` | `GET /antiddos/rules?ip=` | `antiddos:read` |
| `antiddos rules create` | `POST /antiddos/rules` `{"ip","policy","proto_list","dport_list","sport_list","sip_list","description","position"}` | `antiddos:write` |
| `antiddos rules move` | `POST /antiddos/rules/{id}/move` `{"position"}` | `antiddos:write` |
| `antiddos rules delete` | `DELETE /antiddos/rules/{id}` (après confirmation) | `antiddos:write` |
| `catalog` | `GET /catalog` | `services:read` |
| `order create` | `POST /orders` (devis, puis avec `confirmation_token`) | `orders:write` |
| `sandbox list`, `sandbox offers` | `GET /sandboxes` | `sandbox:read` |
| `sandbox get`, `sandbox ssh` | `GET /sandboxes/{id}` | `sandbox:read` |
| `sandbox create` | `POST /sandboxes` | `sandbox:write` |
| `sandbox delete` | `DELETE /sandboxes/{id}` | `sandbox:write` |
| `sandbox links` | `POST /sandboxes/{id}/links` | `sandbox:write` |
| `sandbox exec` | `POST /sandboxes/{id}/exec` | `sandbox:write` |
| `sandbox ls` | `GET /sandboxes/{id}/files?path=&depth=` | `sandbox:read` |
| `sandbox cat`, `sandbox download` | `GET /sandboxes/{id}/files/content?path=` | `sandbox:read` |
| `sandbox upload` | `PUT /sandboxes/{id}/files/content` | `sandbox:write` |
| `sandbox keep` | `POST /sandboxes/{id}/keep` (devis, puis confirmation) | `orders:write` |
| `sandbox claim` | `POST /sandboxes/claim` (devis, puis confirmation) | `orders:write` |
| `sandbox key list/add/remove` | `GET/POST/DELETE /sandbox-keys` | `sandbox:read` / `sandbox:write` |
| `sandbox secret list/set/delete` | `GET /sandbox-secrets`, `PUT/DELETE /sandbox-secrets/{name}` | `sandbox:read` / `sandbox:write` |
| `api <chemin>` | n'importe quelle route `/client/*` | celle de la route |

## Authentification OAuth

| Étape | Route |
|---|---|
| Enregistrement du CLI (client public, redirection loopback) | `POST /client/oauth/register` |
| Page d'autorisation (redirige vers l'espace client) | `GET /client/oauth/authorize` |
| Échange du code / rafraîchissement | `POST /client/oauth/token` |
| Déconnexion | `POST /client/oauth/revoke` |

## Formats

- Erreurs : `{"error": "…", "code": "…"}` (`required_scope` en plus pour `missing_scope`).
- Listes paginées : `{"items": […], "page", "pageSize", "total", "hasMore"}` avec `?page=&limit=` (100 au plus).
- Champs : camelCase pour les comptes, services, factures, tickets, VPS et serveurs ;
  snake_case pour le catalogue, les commandes, l'anti-DDoS et les sandboxes — le CLI les
  restitue tels quels en `-o json`.

Le serveur MCP (`/client/mcp`, voir `yorkhost mcp`) expose ces mêmes routes comme outils pour
assistants IA, avec les mêmes clés et les mêmes permissions.
