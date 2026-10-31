## yorkhost antiddos rules delete

Supprimer une règle

```
yorkhost antiddos rules delete <id> [options]
```

### Exemples

```
  yorkhost antiddos rules delete r_123
  yorkhost antiddos rules delete r_123 --yes
```

### Options

```
  -h, --help   aide de yorkhost antiddos rules delete
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

* [yorkhost antiddos rules](yorkhost_antiddos_rules.md)	 - Règles anti-DDoS de vos IP (créées en attente, appliquées par le NOC)

