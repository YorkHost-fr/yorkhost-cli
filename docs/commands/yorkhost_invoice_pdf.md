## yorkhost invoice pdf

Télécharger le PDF d'une facture

```
yorkhost invoice pdf <id> [options]
```

### Exemples

```
  yorkhost invoice pdf 4242                 # → facture-4242.pdf
  yorkhost invoice pdf 4242 --file ~/compta/ --open
```

### Options

```
  -f, --file string   fichier ou dossier de destination (- = stdout)
      --force         écraser le fichier de destination s'il existe
  -h, --help          aide de yorkhost invoice pdf
      --open          ouvrir le PDF après téléchargement
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

* [yorkhost invoice](yorkhost_invoice.md)	 - Factures : liste, détail, PDF, paiement

