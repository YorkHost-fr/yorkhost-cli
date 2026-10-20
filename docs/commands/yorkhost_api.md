## yorkhost api

Requête brute sur l'API client (comme gh api)

### Description

Envoie une requête authentifiée à l'API client et affiche la réponse JSON.
Le chemin est relatif à /client/v1 s'il ne commence pas par /client/.

Attention : cette commande brute n'applique pas les confirmations de sécurité
des commandes dédiées. Une requête POST, PUT ou DELETE est envoyée directement ;
vérifiez la méthode, le chemin et les données avant de l'exécuter.

Spécification : https://github.com/YorkHost-fr/yorkhost-cli/blob/main/docs/api.md

```
yorkhost api <chemin> [options]
```

### Exemples

```
  yorkhost api /services
  yorkhost api /services --paginate
  yorkhost api -X POST /vps/1234/power -F action=restart
  yorkhost api -X POST /tickets -d '{"subject":"Test","body":"Bonjour"}'
```

### Options

```
  -d, --data string         corps JSON (texte, @fichier ou - pour stdin)
  -F, --field stringArray   champ du corps JSON clé=valeur (nombres et booléens typés)
  -h, --help                aide de yorkhost api
  -i, --include             afficher statut et en-têtes HTTP (stderr)
  -X, --method string       méthode HTTP (défaut GET, POST avec un corps)
      --paginate            parcourir toutes les pages (listes paginées)
```

### Options globales

```
      --api-url string   URL de l'API (env YORKHOST_API_URL)
      --debug            journal des requêtes HTTP sur stderr (sans secrets)
      --no-color         désactiver les couleurs (env NO_COLOR)
  -o, --output string    format de sortie : table, wide, json, yaml (env YORKHOST_OUTPUT)
  -p, --profile string   profil de configuration (env YORKHOST_PROFILE)
  -q, --quiet            n'afficher que le résultat
  -y, --yes              répondre oui aux confirmations (env YORKHOST_YES=1)
```

### Voir aussi

* [yorkhost](yorkhost.md)	 - CLI officiel YorkHost : VPS, serveurs de jeu, facturation, support, sandboxes

