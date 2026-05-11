# Audit Strategique Streaming - AetherStream v1.4

Date: 2026-05-11
Version auditee: v1.4
Scope: 4 protocoles de streaming vers TV/PC

---

## 1. Matrice Protocole x Etat x Effort

| Protocole | Etat Actuel | Maturite | Effort pour Production | Valeur Utilisateur | Risque |
|-----------|-------------|----------|------------------------|--------------------|--------|
| **DLNA/UPnP** | Fonctionnel (80%) | Beta | Faible (1-2 semaines) | Haute (TVs, consoles, box) | Faible |
| **Chromecast** | Stub / DIAL approximatif | Alpha | Moyen (3-4 semaines) | Haute (foyers Google) | Moyen |
| **WebRTC** | Signaling only, pas de media | POC | Eleve (6-8 semaines) | Moyenne (peer-to-peer) | Eleve |
| **Navigateur (TV)** | Fonctionnel via HLS.js | Production-ready | Trivial (deja pret) | Haute (tout ecran avec browser) | Faible |

---

## 2. Etat Detaillee par Protocole

### 2.1 DLNA/UPnP (pkg/dlna/server.go)

**Ce qui marche:**
- SSDP discovery actif (multicast 239.255.255.250:1900)
- NOTIFY announcements + reponses M-SEARCH
- Device description XML conforme UPnP MediaServer:1
- ContentDirectory SOAP: Browse root (libraries) + containers (items)
- ConnectionManager: GetProtocolInfo, GetCurrentConnectionIDs, GetCurrentConnectionInfo
- Content delivery HTTP avec path traversal protection
- Support video/audio/image avec content-type dynamique
- Pruning des devices stale

**Ce qui manque pour production:**
- Search retourne vide (stub)
- Pas de SCPD XML (service description) — certains clients TV le reclament
- Pas de eventing UPnP (pas bloquant pour lecture)
- Pas de transcodage a la volee (livre fichier brut; OK si MP4/H.264)
- Pas de range requests optimises pour le scrubbing

**Verdict:** Le plus proche de "ca marche". Une TV Samsung/LG ou une PS5/Xbox verra le serveur et pourra lire les fichiers compatibles.

### 2.2 Chromecast (pkg/cast/chromecast.go)

**Ce qui marche:**
- Discovery SSDP (ecoute multicast)
- Structure de session et device registry
- Generation d'URL HLS pour l'item

**Ce qui est stub / faux:**
- `launchAndLoad` fait un POST DIAL vers `/apps/DefaultMediaReceiver` puis marque la session "playing" sans attendre de confirmation reelle
- Pas de protocole Castv2 (protobuf over TLS sur port 8009)
- Pas de controle de volume, seek, pause/play reels
- mDNS discovery est un ticker vide (prune only)
- AirPlay (pkg/cast/airplay.go) a un controle /play et /stop plus complet que Chromecast, mais mDNS discovery absent aussi

**Verdict:** L'appareil sera decouvert, la session creee, mais le lancement reel sur le Chromecast est approximatif. Ca peut marcher sur certains receivers compatibles DIAL, pas sur un Chromecast standard.

### 2.3 WebRTC (pkg/webrtc/signaling.go)

**Ce qui marche:**
- Signaling WebSocket avec gorilla/websocket
- Echange SDP offer/answer via pion/webrtc v4
- ICE candidate forwarding
- Registry de peers concurrent-safe

**Ce qui manque totalement:**
- Aucune track media n'est ajoutee au PeerConnection
- Pas de capture/ffmpeg injection dans WebRTC
- Pas de data channel pour controle
- Pas de TURN server (STUN uniquement)
- Le code ferme le PC immediatement apres la negociation sans rien streamer

**Verdict:** C'est un signaling server fonctionnel mais sans media. Inutilisable pour du streaming video a ce stade.

### 2.4 Navigateur / Web UI (web/ui/src/components/MediaPlayer.tsx)

**Ce qui marche:**
- Lecteur HLS avec hls.js (fallback + native Safari)
- Selection de qualite (levels)
- Volume, fullscreen, play/pause
- Adaptive streaming via /videos/:id/adaptive.m3u8
- DASH manifest generation cote serveur

**Ce qui manque pour "TV-ready":**
- Pas de bouton "Cast to TV" dans l'UI
- Pas de remote control (API play/pause/seek depuis un autre device)
- Pas de detection de device sur le reseau local

**Verdict:** C'est le chemin le plus abouti. N'importe quel device avec un navigateur (TV Android, Apple TV via AirPlay mirroring, PC) peut ouvrir l'URL et lire. C'est deja "cast-ready" via l'URL partagee.

---

## 3. ICP (Ideal Customer Profile)

**Qui utilisera ca ?**

1. **Tech-savvy home user** avec un NAS local, une TV connectee (Samsung Tizen, LG webOS, Android TV) et un smartphone. Veut lire ses films sans passer par Plex/Kodi.
2. **Small office / event** qui veut diffuser une playlist sur un ecran via navigateur (digital signage leger).
3. **Famille multi-device** : parent lance sur le navigateur PC, enfant continue sur tablette. Besoin de resume cross-device (deja present via progress API).

**Ce qu'ils attendent en priorite:**
- Ouvrir l'app sur la TV et voir la bibliotheque (DLNA)
- Ou scanner un QR code / taper une URL sur la TV et lire (Navigateur)
- Chromecast est un "nice to have" pour les foyers Google
- WebRTC est un cas d'usage avance (partage a un ami exterieur), pas un besoin immediat

---

## 4. Priorisation Recommandee

### Phase 1: Rendre le produit "cast-ready" immediat (2-3 semaines)

1. **DLNA — Hardener et valider** (1 semaine)
   - Ajouter SCPD XML pour ContentDirectory et ConnectionManager
   - Tester avec une vraie TV (Samsung/LG) ou VLC en renderer
   - Ajouter range request support pour le scrubbing
   - Retourner des resultats dans Search (minimalement)
   - *Impact: eleve, effort: faible*

2. **Navigateur TV — Mode "leanback"** (1 semaine)
   - Ajouter une route /tv ou /remote qui affiche un QR code + URL simple
   - Creer une UI TV optimisee (gros boutons, navigation clavier/telecommande)
   - Ajouter un bouton "Play on TV" dans MediaPlayer qui ouvre l'URL /tv avec l'item precharge
   - *Impact: eleve, effort: faible*

### Phase 2: Chromecast reel (3-4 semaines)

3. **Chromecast — Implementer Castv2** (3-4 semaines)
   - Integrer une librairie Castv2 (ex: github.com/vishen/go-chromecast ou equivalent)
   - Ou utiliser le SDK Cast Sender cote web (plus simple: ajouter le bouton Cast standard de Google dans l'UI web)
   - *Shortcut recommande:* Au lieu d'implementer le protocole natif en Go, injecter le Google Cast SDK dans la Web UI. C'est 2 jours de travail vs 1 mois.
   - *Impact: eleve, effort: moyen (si shortcut pris)*

### Phase 3: WebRTC (6-8 semaines, optionnel)

4. **WebRTC — Media pipeline** (6-8 semaines)
   - Ajouter ffmpeg -> WebRTC track (VP8/H.264)
   - Gerer le transcodage a la volee
   - TURN server pour NAT traversal
   - *Impact: moyen, effort: eleve. A reserver pour v2.0 ou feature "partage externe".*

---

## 5. Dependances entre les 4 Options

```
Navigateur (HLS/DASH)  <-- base commune --+-->  DLNA (meme URLs HLS/files)
        |                                 |
        v                                 v
   Chromecast shortcut (Cast SDK)    Chromecast natif (Castv2)
        |                                 |
        +----> requiert HLS stable <-----+
                      |
                      v
              WebRTC (media pipeline)
                      |
              requiert transcodage live
```

**Dependances cles:**
- Tous les protocoles dependent du pipeline HLS/DASH existant. Si le transcodage est instable, tout est casse.
- DLNA et Navigateur partagent la meme source de contenu (fichiers bruts ou HLS). Pas de conflit.
- Chromecast (shortcut via Web UI) depend du navigateur fonctionnel.
- Chromecast natif pourrait reutiliser la logique DLNA de discovery (SSDP/mDNS).
- WebRTC est independant mais necessitera un nouveau pipeline de transcodage live (pas le meme que HLS segmente).

---

## 6. Recommandation Strategique Finale

**Pour rendre AetherStream "cast-ready" en 2 semaines:**

1. **Ne pas toucher a WebRTC** maintenant. C'est un trou sans fond pour un benefice incertain.
2. **Priorite 1: DLNA hardening** — c'est le seul protocole qui donnera l'experience "ca marche sur ma TV" sans intervention utilisateur.
3. **Priorite 2: Mode TV navigateur** — un QR code + URL /tv est plus universel que n'importe quel protocole proprietaire.
4. **Priorite 3: Chromecast via Cast SDK web** — ajouter le bouton Google Cast dans l'UI React. C'est le meilleur ROI.
5. **Priorite 4 (v2.0):** Chromecast natif Go + WebRTC media pipeline.

**Metrique de succes:**
- DLNA: detecte par 3 clients (VLC, TV Samsung, TV LG) et lecture d'un MP4 H.264 sans erreur.
- Navigateur TV: ouverture de /tv sur un navigateur TV, lecture en < 3 clicks.
- Chromecast: bouton Cast visible dans l'UI, lancement sur Google Home en < 5s.

---

*Rapport genere par audit automatique du codebase AetherStream v1.4.*
