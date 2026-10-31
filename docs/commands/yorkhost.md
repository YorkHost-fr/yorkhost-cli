## yorkhost

CLI officiel YorkHost : VPS, serveurs de jeu, facturation, support, sandboxes

### Description

yorkhost pilote votre compte YorkHost depuis le terminal : VPS, serveurs de jeu,
services, factures, tickets, anti-DDoS, commandes et sandboxes agents.

Commencez par vous connecter (navigateur, A2F requise) :

  yorkhost login

Puis par exemple :

  yorkhost service list
  yorkhost vps restart 1234 --wait
  yorkhost game get mon-serveur -o json | jq .address
  yorkhost invoice list --status unpaid

Documentation : https://github.com/YorkHost-fr/yorkhost-cli

### Options

```
      --api-url string   URL de l'API (env YORKHOST_API_URL)
      --debug            journal des requêtes HTTP sur stderr (sans secrets)
  -h, --help             aide de yorkhost
      --no-color         désactiver les couleurs (env NO_COLOR)
  -o, --output string    format de sortie : table, wide, json, yaml (env YORKHOST_OUTPUT)
  -p, --profile string   profil de configuration (env YORKHOST_PROFILE)
  -q, --quiet            n'afficher que le résultat
  -v, --version          version de yorkhost
  -y, --yes              répondre oui aux confirmations (env YORKHOST_YES=1)
```

### Voir aussi

* [yorkhost account](yorkhost_account.md)	 - Profil du compte (e-mail, crédit, impayés)
* [yorkhost antiddos](yorkhost_antiddos.md)	 - Anti-DDoS : attaques détectées, IP protégées et règles
* [yorkhost api](yorkhost_api.md)	 - Requête brute sur l'API client (comme gh api)
* [yorkhost auth](yorkhost_auth.md)	 - Sessions et identifiants (status, token, login, logout)
* [yorkhost catalog](yorkhost_catalog.md)	 - Produits commandables et tarifs
* [yorkhost completion](yorkhost_completion.md)	 - Script d'autocomplétion pour bash, zsh, fish ou PowerShell
* [yorkhost config](yorkhost_config.md)	 - Réglages et profils (plusieurs comptes, comme aws --profile)
* [yorkhost game](yorkhost_game.md)	 - Serveurs de jeu : état, démarrage, arrêt, métriques
* [yorkhost invoice](yorkhost_invoice.md)	 - Factures : liste, détail, PDF, paiement
* [yorkhost login](yorkhost_login.md)	 - Se connecter (navigateur + A2F, ou clé API)
* [yorkhost logout](yorkhost_logout.md)	 - Se déconnecter (révoque la session OAuth, oublie les identifiants)
* [yorkhost mcp](yorkhost_mcp.md)	 - Brancher un assistant IA (Claude Code, Cursor, VS Code…) sur votre compte
* [yorkhost order](yorkhost_order.md)	 - Commander un produit
* [yorkhost sandbox](yorkhost_sandbox.md)	 - Sandboxes agents : VM d'essai jetables (SSH via yorkhost.sh)
* [yorkhost service](yorkhost_service.md)	 - Tous vos services (VPS, dédiés, jeux, web…)
* [yorkhost ticket](yorkhost_ticket.md)	 - Support : tickets, réponses
* [yorkhost version](yorkhost_version.md)	 - Version du CLI (--check : nouvelle version disponible ?)
* [yorkhost vps](yorkhost_vps.md)	 - VPS : état, démarrage, arrêt, métriques, SSH
* [yorkhost whoami](yorkhost_whoami.md)	 - Compte et identifiants utilisés

