## yorkhost antiddos rules

Règles anti-DDoS de vos IP (créées en attente, appliquées par le NOC)

### Description

Règles anti-DDoS de vos IP : politique Gcore appliquée au trafic qui correspond
(protocoles, ports, sources). Une nouvelle règle est créée avec le statut « pending » puis
appliquée par le NOC YorkHost. Les règles d'une IP sont évaluées dans l'ordre (position 1 en
premier).

### Options

```
  -h, --help   aide de yorkhost antiddos rules
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
* [yorkhost antiddos rules create](yorkhost_antiddos_rules_create.md)	 - Créer une règle (statut « pending » jusqu'à son application)
* [yorkhost antiddos rules delete](yorkhost_antiddos_rules_delete.md)	 - Supprimer une règle
* [yorkhost antiddos rules list](yorkhost_antiddos_rules_list.md)	 - Lister vos règles, politiques disponibles et IP éligibles
* [yorkhost antiddos rules move](yorkhost_antiddos_rules_move.md)	 - Changer l'ordre d'une règle parmi celles de son IP

