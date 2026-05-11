# Audit Architecture Streaming AetherStream v1.4

**Date:** 2026-05-11
**Scope:** Modules cast (Chromecast/AirPlay), dlna (UPnP), webrtc, stream, et point d'entrée main.
**Méthode:** Lecture manuelle du code + tests + grep sur le repo local.

---

## 1. Résumé Exécutif

Le pipeline de streaming vers devices (TV) est **partiellement fonctionnel en DLNA, inopérant en cast/webrtc en l'état**. Le cœur de streaming HTTP (HLS/DASH/direct) est solide et bien sécurisé. Les modules cast et webrtc sont des **squelettes / stubs** qui simulent le protocole mais n'implémentent pas les interactions réelles nécessaires pour une TV.

| Module | Score Global | Statut Production |
|--------|-------------|-------------------|
| pkg/stream (HLS/DASH) | 78/100 | Utilisable avec ajustements |
| pkg/dlna | 62/100 | Fonctionnel basique, incomplet |
| pkg/cast (Chromecast) | 28/100 | **Non utilisable** |
| pkg/cast (AirPlay) | 25/100 | **Non utilisable** |
| pkg/webrtc | 22/100 | **Non utilisable** |

---

## 2. Scoring Détaillé par Module

### 2.1 pkg/stream (HLS/DASH/Direct) — Score 78/100

| Critère | Score | Commentaire |
|---------|-------|-------------|
| Complétude | 85 | Endpoints HLS master/variant/segment, DASH manifest/segment, direct stream, probe, burn-in, subtitles VTT. Manque la gestion d'état du transcode (pas de polling de progression). |
| Robustesse | 75 | Validation de chemin `filepath.Rel` contre `mediaRoot` sur tous les endpoints sensibles. Timeout de 30 min sur FFmpeg. Race condition sur les jobs de transcode corrigée (C2). Pas de rate-limit sur les segments. |
| Test Coverage | 70 | E2E test complet (`tests/e2e/streaming_test.go`). Tests unitaires sur burn-in et subtitles. Pas de tests sur les handlers de segment direct. |
| Sécurité | 85 | `filepath.Clean` + `filepath.Rel` + `strings.HasPrefix(..)` avant chaque lecture. `os.O_RDONLY` sur direct stream. Validation du langage avant utilisation dans commande FFmpeg. `#nosec G204` documentés avec justification. |
| Protocole respecté | 80 | HLS master/variant générés correctement. DASH MPD valide XML. Accept-Ranges sur le direct stream. Manque CORS restrictif (actuellement `*` sur DASH/HLS). |

**P0 (bloquant) AUCUN** pour le module stream seul.
**Quick wins:**
- Ajouter un endpoint `/videos/:id/transcode/status` pour savoir si le transcode est prêt (évite le polling m3u8 vide).
- Restreindre CORS aux origines configurées plutôt que `*`.

---

### 2.2 pkg/dlna (UPnP MediaServer) — Score 62/100

| Critère | Score | Commentaire |
|---------|-------|-------------|
| Complétude | 65 | SSDP NOTIFY + M-SEARCH fonctionnels. Device description XML valide. ContentDirectory Browse (root + library) et Search (vide). ConnectionManager (stubs). **Manque AVTransport** (play/pause/stop/seek) — essentiel pour le contrôle depuis une télécommande TV. |
| Robustesse | 60 | HTTP server sans gestion d'erreur fine sur `ListenAndServe`. Pas de retry SSDP. `buildBrowseResult` ne gère pas la pagination (StartingIndex/RequestedCount ignorés dans la logique de slicing). |
| Test Coverage | 80 | 20+ tests unitaires : description, browse root/library, XSS, SOAP error, ConnectionManager, content delivery, SSDP notify, start/stop. Bonne couverture pour un module de ce type. |
| Sécurité | 70 | `xmlEscape` sur les noms de librairies/items (test XSS passant). `isPathInMediaDirs` avant `http.ServeFile`. Pas de validation de `item.Path` à l'insertion DB (confiance implicite). |
| Protocole respecté | 55 | SSDP et SOAP UPnP corrects en surface. **AVTransport manquant** — la majorité des clients DLNA (TV Samsung, LG, Panasonic) exige AVTransport pour lancer la lecture. Sans ça, la TV peut voir le serveur mais pas lire. |

**P0:**
- **AVTransport absent** : une TV ne peut pas recevoir de commande PLAY/PAUSE/STOP. Le serveur DLNA n'est qu'un navigateur de fichiers, pas un renderer contrôlable.

**Quick wins:**
- Implémenter un service AVTransport minimal (SetAVTransportURI + Play).
- Respecter `StartingIndex`/`RequestedCount` dans `buildBrowseResult` pour les grosses bibliothèques.
- Ajouter des icônes réelles (actuellement `/icon48.png` 404).

---

### 2.3 pkg/cast (Chromecast) — Score 28/100

| Critère | Score | Commentaire |
|---------|-------|-------------|
| Complétude | 25 | Discovery SSDP simplifié (pattern matching "Google" dans les paquets UDP). Pas de vraie résolution mDNS `_googlecast._tcp`. **Pas de protocole Castv2** (protobuf/TLS sur port 8009). `launchAndLoad` fait un POST HTTP DIAL vers `/apps/DefaultMediaReceiver` mais ne suit pas avec la séquence LOAD média réelle. Play/Pause/Seek/Volume sont des stubs vides. |
| Robustesse | 30 | Goroutines de discovery démarrées sans supervision. `close(cc.stopCh)` non protégé contre double close. Pas de heartbeat vers le device. Pas de gestion de reconnect. |
| Test Coverage | 20 | 4 tests basiques (instanciation, struct fields). Aucun test de discovery réseau, aucun test de session, aucun mock du protocole. |
| Sécurité | 40 | Pas de validation TLS (normal, Castv2 n'est pas implémenté). Pas de surface d'attaque majeure car le module ne fait presque rien. |
| Protocole respecté | 15 | Le protocole Chromecast réel est **Castv2** (protobuf chiffré sur TLS 1.2, port 8009). Le code actuel utilise DIAL (HTTP port 8008) sans la suite Castv2. **Un Chromecast réel n'acceptera pas cette séquence.** |

**P0:**
- **Castv2 non implémenté** : le module ne peut pas piloter un Chromecast réel. C'est un simulateur/facade.
- **mDNS discovery absent** : `_googlecast._tcp` est le mécanisme principal de découverte.
- **Non branché dans main.go** : le `ChromecastController` n'est ni instancié ni démarré dans `cmd/aetherstream/main.go`.

**Quick wins:**
- Brancher `ChromecastController` dans `main.go` (instanciation + Start/Stop).
- Implémenter mDNS discovery avec `github.com/grandcat/zeroconf` ou `golang.org/x/net/mdns`.
- Intégrer une librairie Castv2 existante (ex: `github.com/vishen/go-chromecast`) plutôt que réinventer.

---

### 2.4 pkg/cast (AirPlay) — Score 25/100

| Critère | Score | Commentaire |
|---------|-------|-------------|
| Complétude | 20 | `sendPlay` envoie un POST `/play` avec `Content-Location` (format AirPlay 1). Pas de AirPlay 2 (protocol buffer + pairing). Discovery mDNS `_airplay._tcp` est un stub (ticker vide). Pas de mirroring, pas de feedback de position. |
| Robustesse | 25 | Mêmes défauts que Chromecast (pas de supervision, stubs). `sendPlay` utilise `text/parameters` mais ne gère pas la réponse 401/403 d'un device protégé par code. |
| Test Coverage | 15 | Tests d'instanciation uniquement. |
| Sécurité | 40 | Pas de surface d'attaque majeure. |
| Protocole respecté | 20 | AirPlay 1 partiellement respecté pour `/play`. AirPlay 2 (utilisé par les Apple TV récentes) est totalement absent. |

**P0:**
- **AirPlay 2 non supporté** : les Apple TV modernes nécessitent le protocole RAOP/AirPlay 2 avec pairing.
- **Non branché dans main.go**.

**Quick wins:**
- Brancher dans `main.go`.
- Utiliser une librairie existante pour AirPlay 1 (ex: `github.com/openairplay/go-airplay`) si besoin rapide.

---

### 2.5 pkg/webrtc (Signaling) — Score 22/100

| Critère | Score | Commentaire |
|---------|-------|-------------|
| Complétude | 15 | Signaling WebSocket SDP offer/answer + ICE candidate. **Aucune track média attachée** : le `PeerConnection` pion est créé mais on n'ajoute aucune `TrackLocal` (audio/vidéo). Le client WebRTC recevra une connexion vide. Pas de gestion de déconnexion propre côté track. Pas de TURN server (STUN uniquement). |
| Robustesse | 20 | `CheckOrigin: return true` sur l'upgrader WebSocket = **CORS ouvert à tous les domaines** (vulnérabilité CSWSH potentielle). Pas de heartbeat/ping-pong WebSocket. Pas de limite de peers. Goroutine `ReadMessage` bloque sans timeout. |
| Test Coverage | 25 | Tests d'instanciation et de concurrence basiques. Aucun test avec vraie connexion WebSocket, aucun test de négociation SDP complète. |
| Sécurité | 30 | `CheckOrigin` permissif = P0 sécurité. Pas d'authentification sur le endpoint `/webrtc/negotiate`. Pas de rate-limiting. |
| Protocole respecté | 25 | La négociation SDP est correcte côté signaling. Mais WebRTC sans track média ne sert à rien pour du streaming. |

**P0:**
- **Aucune track média** : le module ne stream rien. Il faut capturer/transmuxer la vidéo vers `webrtc.TrackLocalStaticSample`.
- **CheckOrigin=true** : n'importe quel site web peut ouvrir une connexion WebRTC vers le serveur (WebSocket hijacking + potentiel scan réseau via ICE).
- **Non branché dans main.go** : aucune route `/webrtc/negotiate` n'est enregistrée.

**Quick wins:**
- Remplacer `CheckOrigin` par une liste d'origines configurées.
- Ajouter une auth middleware sur le handler WebSocket.
- Implémenter l'ajout de track (ex: transmuxer HLS -> WebRTC avec `pion/webrtc` + `github.com/pion/interceptor`).

---

## 3. Analyse de cmd/aetherstream/main.go

**Constats:**
- Le `ChromecastController`, `AirPlayController` et `SignalingServer` **ne sont pas instanciés** dans `main.go`.
- Seul le `dlna.Server` est démarré (port `cfg.Server.Port+1`).
- Pas de route API REST pour lister les devices cast/airplay ni pour initier une session.
- Pas de gestion du cycle de vie des sessions cast (cleanup, heartbeat).

**Conséquence:** Même si les modules cast/airplay/webrtc étaient complets, l'utilisateur ne peut pas y accéder depuis l'application.

---

## 4. Liste des P0 (Bloquants pour usage réel)

| # | P0 | Module | Impact |
|---|----|--------|--------|
| 1 | **Castv2 non implémenté** | cast | Chromecast réel impossible à piloter |
| 2 | **AVTransport absent** | dlna | TV ne peut pas lancer la lecture via DLNA |
| 3 | **Aucune track média WebRTC** | webrtc | Connexion WebRTC vide, aucun streaming |
| 4 | **CheckOrigin=true sur WS** | webrtc | Vulnérabilité CSWSH / hijacking |
| 5 | **Modules cast/airplay/webrtc non branchés** | main | Fonctionnalités inaccessibles aux utilisateurs |
| 6 | **mDNS discovery absent** | cast | Chromecast/AirPlay non découverts sur le réseau local |

---

## 5. Quick Wins (Faible effort, haut impact)

1. **Brancher DLNA + cast + webrtc dans main.go** (~2h) : instancier les controllers, ajouter les routes, gérer le graceful shutdown.
2. **Sécuriser WebSocket origin** (~30min) : `CheckOrigin` basé sur `cfg.Server.AllowedOrigins`.
3. **Ajouter AVTransport stub minimal** (~4h) : `SetAVTransportURI` + `Play` pour rendre les TVs DLNA fonctionnelles.
4. **Pagination Browse DLNA** (~2h) : respecter `StartingIndex`/`RequestedCount`.
5. **Endpoint transcode status** (~2h) : éviter le polling m3u8 vide côté client.
6. **Intégrer librairie Castv2 existante** (~1-2j) : `go-chromecast` ou équivalent plutôt que réimplémenter protobuf/TLS.

---

## 6. Verdict

- **Streaming vers navigateur (HLS/DASH)** : OK, code de qualité moyenne-haute, sécurisé.
- **Streaming vers TV via DLNA** : Partiellement fonctionnel (browse visible, lecture manuelle possible si la TV supporte le direct HTTP, mais pas de contrôle standard).
- **Streaming vers Chromecast / AirPlay / WebRTC** : **Non fonctionnel en l'état**. Ce sont des placeholders architecturaux qui nécessitent une réimplémentation ou l'intégration de librairies tierces spécialisées.

**Recommandation:** Prioriser l'intégration d'une librairie Castv2 et l'ajout d'AVTransport DLNA avant toute annonce de "streaming vers TV".
