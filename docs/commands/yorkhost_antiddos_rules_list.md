## yorkhost antiddos rules list

Lister vos règles, politiques disponibles et IP éligibles

```
yorkhost antiddos rules list [options]
```

### Exemples

```
  yorkhost antiddos rules list
  yorkhost antiddos rules list --ip 203.0.113.10 -o wide
  yorkhost antiddos rules list -o json   # état complet : politiques, plafonds, IP éligibles
```

### Options

```
  -h, --help        aide de yorkhost antiddos rules list
      --ip string   règles d'une seule adresse IP
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

