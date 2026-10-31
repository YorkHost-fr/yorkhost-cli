## yorkhost antiddos attacks

Attaques détectées sur vos IP

```
yorkhost antiddos attacks [options]
```

### Exemples

```
  yorkhost antiddos attacks --days 7
  yorkhost antiddos attacks --ip 203.0.113.10 -o json
```

### Options

```
      --days int    période en jours (1 à 365) (default 90)
  -h, --help        aide de yorkhost antiddos attacks
      --ip string   une seule adresse IP (cibles et attaques de cette IP)
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

* [yorkhost antiddos](yorkhost_antiddos.md)	 - Anti-DDoS : attaques détectées, IP protégées et règles

