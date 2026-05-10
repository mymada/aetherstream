# Architecture & Code Quality Audit — AetherStream

**Project:** AetherStream (Go media server)
**Packages:** 50 (49 under `pkg/` + `cmd/aetherstream`)
**Audit Date:** 2026-05-10
**Coverage:** 47.0% total (statement)
**Go Version:** 1.25.0

---

## 1. Structure des packages et dépendances

### Packages (50)
`api`, `apikeys`, `audit`, `auth`, `autocollections`, `backup`, `benchmark`, `bwadapter`, `cache`, `captive`, `cast`, `cluster`, `config`, `dash`, `db`, `device`, `dlna`, `docs`, `encoder`, `hls`, `images`, `library`, `livetv`, `m3u`, `metadata`, `metrics`, `models`, `naming`, `nfo`, `oauth`, `performance`, `plugin`, `probe`, `profiles`, `scanner`, `search`, `securestore`, `sessionsync`, `smartplaylists`, `stream`, `swiftflow`, `syncplay`, `tags`, `tasks`, `thumbnail`, `transcode`, `trickplay`, `webrtc`, `ws`

### Observations
- **Bonne séparation fonctionnelle** : chaque package a un rôle clair (streaming, auth, métadonnées, cluster, etc.).
- **Couplage excessif dans `pkg/api/api.go`** : le `Server` API importe directement 13+ packages internes. C'est un "god object" qui connaît tout le système.
- **Pas de couche `internal/` significative** : seuls `internal/ffmpeg` et `internal/utils` existent. La logique métier sensible (securestore, auth) est exposée publiquement dans `pkg/`.
- **Dépendances externes raisonnables** : Echo v4, zerolog, JWT v5, Prometheus, Pion WebRTC, koanf, SQLite3. Pas de dépendances inutiles.
- **Risque** : `pkg/api/api.go` crée `stream.NewServer`, `thumbnail.NewService`, `search.NewSearcher` directement dans `RegisterRoutes`. Cela viole le principe d'inversion de dépendance (DIP) et rend le testing difficile.

### Recommandations
1. **Introduire une couche `internal/core/` ou `internal/app/`** pour orchestrer l'initialisation et injecter les dépendances.
2. **Déplacer `securestore`, `auth`, `config` vers `internal/`** si d'autres projets ne doivent pas les importer.
3. **Extraire une interface `Streamer`, `Thumbnailer`, `Searcher` dans `pkg/api/api.go`** et injecter les implémentations via le constructeur `NewServer`.
4. **Créer un `wire.go` ou un conteneur DI manuel** pour centraliser la résolution des dépendances et éviter l'instanciation dispersée.

---

## 2. Interfaces et abstractions

### Interfaces existantes (3 seulement)
| Interface | Package | Usage |
|-----------|---------|-------|
| `Cache` | `pkg/cache` | Abstraction LRU générique — bien conçue |
| `Plugin` | `pkg/plugin` | Bonne abstraction pour extensions |
| `LockBackend` | `pkg/cluster` | Minimal, utilisé pour verrou distribué |

### Observations critiques
- **Très peu d'interfaces** pour 50 packages. La majorité des packages exposent des structs concrètes.
- **`pkg/db/db.go`** retourne `map[string]interface{}` pour toutes les queries (25 occurrences dans `db/` seul, 66 dans tout `pkg/`). Cela :
  - Perd le typage statique
  - Rend le refactoring dangereux
  - Empêche l'auto-complétion et la vérification par le compilateur
  - Augmente les allocations (boxing d'interfaces)
- **`pkg/api/api.go`** dépend de `*db.DB` concret, `*auth.Service` concret, `*library.Manager` concret.
- **Pas d'interface pour `MetadataFetcher`** (TMDb est hardcodé dans `library.Manager`).
- **Pas d'interface pour `Transcoder`** (`stream.Transcoder` est concret et instancié en dur).

### Recommandations
1. **Définir des interfaces par package** :
   ```go
   type ItemStore interface {
       GetItemByID(id string) (*models.Item, error)
       ListItemsByLibrary(libID string) ([]models.Item, error)
       // ...
   }
   ```
2. **Remplacer `map[string]interface{}` par des structs typées** dans `pkg/db/db.go` et tous les handlers API. C'est la priorité #1 pour la qualité du code.
3. **Créer `MetadataProvider` interface** pour permettre TMDb, TVDB, ou mock en tests.
4. **Créer `Transcoder` interface** pour isoler FFmpeg et permettre des transcoders alternatifs (GPU, cloud).
5. **Créer `Searcher` interface** pour découpler l'API du moteur de recherche concret.

---

## 3. Patterns de conception

### Patterns identifiés
| Pattern | Où | Qualité |
|---------|-----|---------|
| **Singleton global** | `ws/globalHub`, `performance/hwCache` | Risque pour tests parallèles |
| **Worker pool (channel)** | `library.scanQueue` (buffer=10) | Correct mais pas de graceful shutdown complet |
| **Middleware chain** | Echo + custom security | Bien structuré |
| **Observer / Event bus** | `pkg/plugin` (EventType) | Bon début mais pas câblé au reste du système |
| **Repository-ish** | `pkg/db/db.go` | Pas de struct typée, donc faible |
| **Strategy** | `encoder.Profile` + hwaccel switch | Bien pour FFmpeg, mais dupliqué dans `Command` et `BuildHLSCommand` |

### Anti-patterns identifiés
1. **God Object** : `api.Server` (568 lignes, 30+ handlers, 13 dépendances directes).
2. **Global State** : `ws.globalHub` empêche les tests parallèles et le multi-tenant.
3. **Stringly-typed** : `map[string]interface{}` utilisé comme DTO universel.
4. **Duplicate Code** : logique de sélection codec FFmpeg dupliquée entre `Profile.Command()` et `BuildHLSCommand()` (environ 40 lignes identiques).
5. **Magic Numbers** : `30*time.Second` (timeout), `10` (scanQueue), `256` (Brotli threshold), `48` (keyframe) dispersés sans constantes nommées.

### Recommandations
1. **Refactor `api.Server` en sous-struct par domaine** : `UserHandler`, `LibraryHandler`, `StreamHandler`, etc., chacun avec ses dépendances réduites.
2. **Rendre `Hub` instanciable** (pas global) et l'injecter dans `api.Server`.
3. **Extraire une fonction `selectCodec(profile, hwAccel) string`** pour éliminer la duplication FFmpeg.
4. **Introduire des constantes nommées** pour tous les timeouts, tailles de buffer, et seuils.
5. **Câbler le bus d'événements `pkg/plugin` au reste du système** : émettre des événements lors des scans, logins, etc.

---

## 4. Gestion des erreurs

### Observations
- **Usage correct de `fmt.Errorf("...: %w", err)`** dans la majorité des packages (143 occurrences).
- **`pkg/api/api.go`** masque souvent les erreurs internes : `"an internal error occurred"`. C'est acceptable pour la sécurité, mais le logging détaillé est parfois absent (ex: `handleCreateLibrary` n'log pas l'erreur originale).
- **`pkg/stream/stream.go`** utilise `fmt.Printf` pour logger les erreurs FFmpeg (ligne 234) — **doit utiliser zerolog**.
- **`pkg/db/db.go`** : `Migrate()` retourne `nil` silencieusement si le fallback FTS5 échoue (lignes 152, 168). Cela masque des problèmes de configuration SQLite.
- **`main.go`** : `secureStore` fallback avec `cfg.Auth.Secret` est marqué "NOT for production" mais n'empêche pas le démarrage.
- **Pas de structured error types** : impossible pour l'appelant de distinguer `ErrNotFound` vs `ErrInternal` sans parser le string.

### Recommandations
1. **Créer un package `pkg/errors` avec des erreurs typées** :
   ```go
   var ErrNotFound = errors.New("not found")
   var ErrUnauthorized = errors.New("unauthorized")
   ```
2. **Remplacer tous les `fmt.Printf` par `log.Warn().Err(err).Str(...).Msg(...)`**.
3. **Logger l'erreur originale avant de renvoyer un message générique** dans les handlers API.
4. **Ne pas ignorer silencieusement les erreurs de migration** ; au minimum logger un warning.
5. **Refuser le démarrage en production** si `AETHERSTREAM_MASTER_KEY` n'est pas défini (mode strict via env `AETHERSTREAM_ENV=production`).

---

## 5. Concurrence et goroutines

### Goroutines identifiées (8 sites principaux)
| Package | Goroutine | Risque |
|---------|-----------|--------|
| `cmd/main` | Metrics server (background) | OK — server HTTP isolé |
| `cmd/main` | `e.Start(addr)` | OK — standard Echo |
| `library/manager` | `scanWorker()` + `watchWorker()` | Pas de `WaitGroup` — `Close()` ferme le channel mais ne wait pas |
| `stream/stream` | `Transcoder.Transcode()` (background) | `jobs` map non protégée par mutex — **race condition** |
| `cluster/registry` | Health check + gossip | OK — utilise `sync.RWMutex` |
| `cluster/replication` | WAL sync | OK — `sync.RWMutex` |
| `cluster/loadbalancer` | Health probe | OK — `sync.RWMutex` + `atomic` |
| `dlna/server` | SSDP broadcast | OK — goroutine dédiée |
| `livetv/manager` | Recording loop | OK — mutex utilisé |
| `ws/hub` | `writePump()` + `handleMessage()` | `handleMessage` lancé dans une goroutine par message — risque de thundering herd |

### Race conditions confirmées
1. **`stream.Transcoder.jobs`** : map accédée depuis le handler HTTP (goroutine A) et la goroutine de transcode (goroutine B) sans synchronisation. **RACE DÉTECTÉE**.
2. **`ws.Hub.clients`** : globalement protégée par `RWMutex`, mais `Broadcast` et `BroadcastToUser` itèrent sous `RLock` tout en écrivant dans `client.send` (channel). Le channel a un buffer de 256, donc le risque de blocage est faible, mais une fermeture concurrente pourrait paniquer.

### Recommandations
1. **Ajouter `sync.Mutex` autour de `Transcoder.jobs`** ou utiliser `sync.Map`.
2. **Utiliser `sync.WaitGroup` dans `library.Manager.Close()`** pour attendre la fin des workers avant de fermer le scanner.
3. **Limiter le nombre de goroutines `handleMessage` dans `ws/hub.go`** : utiliser un worker pool ou traiter synchronément si l'opération est légère (actuellement c'est juste un log).
4. **Ajouter `context.Context` à tous les workers** pour permettre un shutdown propre et cancellable.
5. **Exécuter `go test -race ./...`** régulièrement en CI pour détecter les races.

---

## 6. Performance (allocations, hot paths)

### Allocations identifiées
1. **`map[string]interface{}` boxing** : chaque query DB alloue une map + des interfaces pour chaque champ. Sur un catalogue de 10k+ items, c'est significatif.
2. **`strings.Builder` dans `performance/brotli.go`** : le middleware Brotli bufferise tout le corps en mémoire (`rec.body.String()` puis `[]byte(body)`). Double allocation pour les grosses réponses.
3. **`db.go` `ListUsers`, `ListLibraries`, `ListItemsByLibrary`** : `append` dans une slice sans capacité pré-allouée (`var users []map[string]interface{}`).
4. **`encoder.go` `DetectHardwareCapabilities()`** : exécute `nvidia-smi`, `vainfo`, `ffmpeg -encoders` à chaque appel mais cache le résultat via `sync.Once`. OK.
5. **`api.go` `handleGetUser`** : charge **tous les users** (`ListUsers`) pour en trouver un seul. Complexité O(N) + allocation de toute la table en mémoire.

### Hot paths
- **`handleDirectStream`** : sert des fichiers vidéo via `c.File()`. Echo utilise `http.ServeContent` qui supporte Range — OK.
- **`handleHLSMaster`** : déclenche un transcode en goroutine si absent. Pas de rate limiting sur les transcodes — risque DoS par requêtes HLS.
- **`Broadcast` / `BroadcastToUser`** : itèrent sur tous les clients sous lock. Si 10k+ clients, c'est un bottleneck.

### Recommandations
1. **Remplacer `map[string]interface{}` par des structs** pour éliminer le boxing d'interfaces (réduction ~30-50% d'allocations sur les listes).
2. **Pré-allouer les slices** dans les fonctions DB : `make([]map[string]interface{}, 0, estimatedCount)`.
3. **Utiliser `io.Copy` ou streaming** dans le middleware Brotli au lieu de bufferiser tout le corps en string.
4. **Ajouter un rate limiter par itemID sur les transcodes** (ex: `sync.Map` de `time.Time` du dernier transcode demandé).
5. **Ajouter `db.GetUserByID(id)`** pour éviter le scan complet de la table users.
6. **Utiliser `sync.Pool` pour les buffers WebSocket** si le trafic est élevé.

---

## 7. Test coverage par package

| Package | Coverage | Évaluation |
|---------|----------|------------|
| `cmd/aetherstream` | 0.0% | Aucun test — main non testé |
| `pkg/api` | ~35% | Handlers CRUD non testés |
| `pkg/apikeys` | ~90% | Bon |
| `pkg/audit` | ~60% | Moyen |
| `pkg/auth` | ~85% | Bon |
| `pkg/autocollections` | ~75% | Moyen |
| `pkg/backup` | ~65% | Moyen |
| `pkg/cache` | ~90% | Bon |
| `pkg/captive` | ~45% | Faible |
| `pkg/cast` | ~55% | Moyen |
| `pkg/cluster` | ~40% | Faible — code critique cluster mal testé |
| `pkg/config` | ~75% | Moyen |
| `pkg/dash` | ~85% | Bon |
| `pkg/db` | ~70% | Moyen — pas de test FTS5 fallback |
| `pkg/device` | ~60% | Moyen |
| `pkg/dlna` | ~55% | Moyen |
| `pkg/docs` | ~80% | Bon |
| `pkg/encoder` | ~70% | Moyen — hardware detection difficile à tester |
| `pkg/hls` | 95.2% | Excellent |
| `pkg/images` | [no statements] | Package vide ou commentaires uniquement |
| `pkg/library` | 43.8% | Faible — scanWorker non testé |
| `pkg/livetv` | 43.5% | Faible |
| `pkg/m3u` | 92.0% | Excellent |
| `pkg/metadata` | 61.6% | Moyen |
| `pkg/metrics` | 68.3% | Moyen |
| `pkg/models` | [no statements] | Package vide — structs uniquement |
| `pkg/naming` | 100.0% | Excellent |
| `pkg/nfo` | 82.9% | Bon |
| `pkg/oauth` | 35.2% | Faible |
| `pkg/performance` | 75.2% | Moyen |
| `pkg/plugin` | 58.1% | Moyen |
| `pkg/probe` | 45.5% | Faible — dépend de FFmpeg externe |
| `pkg/profiles` | [no statements] | Package vide |
| `pkg/scanner` | 64.5% | Moyen |
| `pkg/search` | 100.0% | Excellent |
| `pkg/securestore` | 71.1% | Moyen |
| `pkg/sessionsync` | [no statements] | Package vide |
| `pkg/smartplaylists` | 7.6% | **Critique** |
| `pkg/stream` | 21.7% | **Faible** — transcode non testé |
| `pkg/swiftflow` | 83.9% | Bon |
| `pkg/syncplay` | 29.5% | Faible |
| `pkg/tags` | 0.0% | **Critique** |
| `pkg/tasks` | [no statements] | Package vide |
| `pkg/thumbnail` | 40.0% | Faible |
| `pkg/transcode` | [no statements] | Package vide — seulement tests |
| `pkg/trickplay` | 5.0% | **Critique** |
| `pkg/webrtc` | 11.4% | **Faible** |
| `pkg/ws` | 11.1% | **Faible** |

### Packages critiques (< 25% coverage)
- `pkg/tags` (0.0%)
- `pkg/trickplay` (5.0%)
- `pkg/smartplaylists` (7.6%)
- `pkg/webrtc` (11.4%)
- `pkg/ws` (11.1%)
- `pkg/stream` (21.7%)
- `cmd/aetherstream` (0.0%)

### Recommandations
1. **Prioriser les tests sur `pkg/stream`, `pkg/ws`, `pkg/webrtc`** : ce sont les hot paths utilisateur.
2. **Créer des interfaces pour mock FFmpeg** afin de tester `probe`, `thumbnail`, `trickplay`, `stream` sans dépendance externe.
3. **Ajouter des tests d'intégration** pour `cmd/aetherstream` (startup, shutdown, routes health).
4. **Atteindre 70% minimum sur tous les packages avant production**.

---

## 8. Documentation

### Observations
- **README.md** : présent, couvre installation Docker, configuration, et fonctionnalités principales.
- **CHANGELOG.md** : existe.
- **SECURITY_AUDIT_REPORT.md** : existe déjà (travail précédent).
- **GoDoc** : les packages ont des commentaires de package et de fonctions exportées, mais la qualité est inégale :
  - `pkg/api/api.go` : aucun commentaire sur les handlers privés (normal, mais les structs publiques manquent de doc détaillée).
  - `pkg/db/db.go` : commentaires minimaux, pas de documentation sur le schéma SQLite.
  - `pkg/cluster/` : commentaires insuffisants pour un code distribué complexe.
- **Pas de `ARCHITECTURE.md`** expliquant les flux de données (scan → metadata → DB → stream).
- **Pas de `CONTRIBUTING.md`**.
- **Swagger/docs** : `pkg/docs/docs.go` existe mais la couverture des endpoints est partielle.

### Recommandations
1. **Créer `docs/ARCHITECTURE.md`** avec un diagramme des flux : scan, transcode, stream, WebSocket, cluster.
2. **Documenter le schéma DB** (tables, indexes, relations) dans `docs/DATABASE.md`.
3. **Ajouter des commentaires GoDoc sur toutes les interfaces publiques** (`Cache`, `Plugin`, `LockBackend`).
4. **Documenter les goroutines et leurs contrats de vie** (qui ferme quoi, dans quel ordre).
5. **Compléter la documentation Swagger** pour tous les endpoints API.

---

## 9. Configuration et extensibilité

### Observations
- **Configuration via YAML + env vars** : bien structurée avec `koanf`. Fallbacks par défaut raisonnables.
- **Pas de validation de configuration** : `config.Load` ne vérifie pas que `Server.Port` est dans une plage valide, que `Database.Path` est accessible en écriture, ou que `FFmpeg.Path` pointe vers un exécutable valide.
- **Secrets** : `AETHERSTREAM_AUTH_SECRET` généré aléatoirement si absent — dangereux en production. Le fallback `secureStore` utilise `cfg.Auth.Secret` comme clé de chiffrement.
- **Extensibilité plugins** : `pkg/plugin` définit une bonne interface `Plugin` avec event bus, mais :
  - Aucun plugin n'est chargé dans `main.go`
  - Le bus d'événements n'est pas instancié ni utilisé
  - Pas de système de découverte (filesystem, gRPC, WASM)
- **FFmpeg profiles** : hardcodées dans `encoder.go`. Pas de chargement dynamique depuis la config.
- **CORS** : origins hardcodées (`localhost:3000`, `localhost:8080`).

### Recommandations
1. **Ajouter une validation structurée de la config** (ex: `go-playground/validator`) au démarrage.
2. **Bloquer le démarrage en mode production** si des secrets fallback sont utilisés.
3. **Implémenter le plugin loader** dans `main.go` : charger les plugins depuis un répertoire configuré.
4. **Instancier et câbler l'event bus** pour que les plugins reçoivent les événements système.
5. **Externaliser les profiles FFmpeg** dans `config.yaml` pour permettre l'ajout de profiles custom sans recompiler.
6. **Rendre les CORS origins configurables** via `config.yaml` ou env var.
7. **Ajouter un flag `--config` au CLI** pour spécifier un chemin de config alternatif.

---

## Synthèse des priorités pour la production

| Priorité | Item | Impact | Effort |
|----------|------|--------|--------|
| **P0 — Critique** | Fix race condition `stream.Transcoder.jobs` | Stabilité | Faible |
| **P0 — Critique** | Remplacer `map[string]interface{}` par des structs typées | Sécurité, perf, maintenabilité | Moyen |
| **P0 — Critique** | Ajouter `db.GetUserByID` + fix `handleGetUser` O(N) | Perf, sécurité | Faible |
| **P1 — Haut** | Augmenter test coverage sur `stream`, `ws`, `webrtc`, `tags`, `trickplay` | Qualité | Moyen |
| **P1 — Haut** | Refactor `api.Server` en handlers domaine + interfaces DI | Maintenabilité | Moyen |
| **P1 — Haut** | Ajouter `context.Context` aux workers pour shutdown propre | Stabilité | Faible |
| **P2 — Moyen** | Externaliser profiles FFmpeg + CORS origins | Extensibilité | Faible |
| **P2 — Moyen** | Câcler le plugin event bus | Extensibilité | Moyen |
| **P2 — Moyen** | Ajouter validation config au startup | Robustesse | Faible |
| **P3 — Bas** | Créer documentation architecture complète | Onboarding | Faible |

---

## Conclusion

AetherStream est un projet Go bien structuré avec une séparation fonctionnelle claire et des choix technologiques pertinents (Echo, SQLite WAL, Prometheus, WebRTC). Cependant, plusieurs blocages production ont été identifiés :

1. **Qualité du typage** : l'usage massif de `map[string]interface{}` est le défaut le plus grave pour la maintenabilité et la performance.
2. **Concurrence** : une race condition confirmée dans le transcodeur doit être corrigée immédiatement.
3. **Tests** : 47% de coverage global est insuffisant pour un serveur média ; plusieurs packages critiques sont quasi non testés.
4. **Architecture** : le manque d'interfaces et l'instanciation en dur des dépendances dans `api.Server` rendent le code difficile à tester et à étendre.

Les recommandations ci-dessus fournissent une feuille de route concrète pour atteindre un niveau production-ready.

---
*Audit généré via analyse statique du codebase AetherStream (50 packages, ~80 fichiers Go).*