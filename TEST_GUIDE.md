# AetherStream v1.5 — Guide de Test Streaming

## Démarrage rapide

```bash
cd /home/devuser/dev/aetherstream
export AETHERSTREAM_AUTH_SECRET=this-is-a-test-secret-32-chars-long
./aetherstream
```

Le serveur démarre sur :
- **App Web** : http://localhost:8081/app
- **Mode TV** : http://localhost:8081/tv
- **API** : http://localhost:8081/api
- **DLNA** : http://localhost:8082/device/description.xml
- **Métriques** : http://localhost:9090/metrics

## 1. Créer un compte

```bash
curl -X POST http://localhost:8081/auth/register \
  -H "Content-Type: application/json" \
  -d '{"username":"tonnom","password":"tonpass123"}'
```

## 2. Se connecter

```bash
curl -X POST http://localhost:8081/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username":"tonnom","password":"tonpass123"}'
```

Récupère le token dans la réponse.

## 3. Ouvrir l'app

Dans ton navigateur :
- http://localhost:8081/app → interface principale
- Login avec tes identifiants
- Browse les bibliothèques (vide au début)

## 4. Ajouter une bibliothèque

```bash
export TOKEN=eyJhbG... # ton token
curl -X POST http://localhost:8081/api/libraries \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"name":"Films","path":"./media","media_type":"movie"}'
```

## 5. Scanner la bibliothèque

```bash
curl -X POST http://localhost:8081/api/libraries/ID_DE_LA_LIB/scan \
  -H "Authorization: Bearer $TOKEN"
```

## 6. Lire une vidéo

Dans l'app web :
- Va dans "Films"
- Click sur la vidéo test
- Le lecteur s'ouvre avec HLS

## 7. Mode TV navigateur

Sur ta smart TV :
1. Ouvre le navigateur
2. Va sur http://IP_DE_TON_PC:8081/tv
3. Un QR code s'affiche
4. Sur ton téléphone, scanne le QR code (ouvre l'URL)
5. Le téléphone devient la télécommande
6. Choisis une vidéo sur le téléphone → elle joue sur la TV

## 8. Chromecast

Dans l'app web sur PC/téléphone :
1. Lance une vidéo
2. Click le bouton "Cast" (📡)
3. Sélectionne ton Google Home/Chromecast
4. La vidéo joue sur la TV

## 9. DLNA

Sur ta TV (Samsung/LG/Sony) :
1. Ouvre l'app "Sources" ou "Réseau"
2. Cherche "AetherStream" dans les serveurs DLNA
3. Browse les films
4. Sélectionne → lecture

## Test E2E automatisé

```bash
python3 scripts/test-e2e-streaming.py
```

## Arrêter le serveur

Ctrl+C dans le terminal où il tourne.
