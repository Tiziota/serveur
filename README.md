# Serveur - Go Downloads

Site de téléchargement de fichiers en Go avec espace admin.

## Fonctions
- Interface moderne type SaaS (inspirée Canva)
- Liste publique de fichiers à télécharger
- Admin: connexion + ajout de fichiers via lien URL
- Données stockées dans `data/files.json`
- Déploiement Render prêt

## Lancer en local
```bash
go run .
```

Application dispo sur `http://localhost:10000` (ou PORT custom).

## Variables d'environnement
- `ADMIN_USER` (défaut: `admin`)
- `ADMIN_PASS` (défaut: `change-me-now`)
- `PORT` (défaut: `10000`)

## Déploiement Render
Render utilise `render.yaml`:
- Build: `go build -tags netgo -ldflags '-s -w' -o app`
- Start: `./app`

Ensuite:
1. Connecte le repo GitHub `Tiziota/serveur` à Render.
2. Configure `ADMIN_USER` et `ADMIN_PASS` dans les env vars Render.
3. Deploy automatique à chaque push.
