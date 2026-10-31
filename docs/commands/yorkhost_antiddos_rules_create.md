## yorkhost antiddos rules create

Créer une règle (statut « pending » jusqu'à son application)

```
yorkhost antiddos rules create [options]
```

### Exemples

```
  yorkhost antiddos rules create --ip 203.0.113.10 --policy "FiveM UDP" --proto udp --dport 30120
  yorkhost antiddos rules create --ip 203.0.113.10 --policy "Rate Limiter Low" --proto udp --dport 27015-27030 --position 1
  yorkhost antiddos rules create --ip 203.0.113.10 --policy "Allow List" --proto tcp --dport 22 --sip 198.51.100.7 --description "bastion"
```

### Options

```
      --description string   note libre
      --dport strings        ports de destination, ex. 30120 ou 27015-27030
  -h, --help                 aide de yorkhost antiddos rules create
      --ip string            IPv4 protégée (une des IP éligibles)
      --policy string        politique (voir rules list), ex. "FiveM UDP", Drop
      --position int         position parmi les règles de l'IP (1 = en premier ; défaut : en dernier)
      --proto strings        protocoles : tcp, udp, icmp, gre (répétable ou séparés par des virgules)
      --sip strings          adresses ou réseaux source (obligatoires pour Allow List)
      --sport strings        ports source
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

