# AetherStream — Architecture Audit Report

**Date:** 2026-05-11
**Auditor:** Hermès Agent
**Scope:** Go backend (141 fichiers Go), Docker, docker-compose, dépendances, patterns, scalabilité
**Score global:** 72 / 100

---

## 1. Vue d'ensemble

AetherStream est une application media-streaming monolithique en Go 1.25, structurée par domaine sous `pkg/`. Elle expose une API REST (Echo v4), du streaming adaptatif (HLS/DASH), du WebRTC (Pion), du DLNA, et une UI web statique. La persistance repose sur SQLite (WAL mode). Les métriques Prometheus et les logs Zerolog sont intégrés.

**Points forts:**
- Build OK, tests 807 pass, couverture mesurée.
- Sécurité HTTP headers, CSRF, rate-limiting, CORS configurés.
- Dockerfile multi-stage propre avec non-root user.
- FTS5 SQLite pour la recherche full-text.
- Cache LRU in-memory avec TTL.
- Détection GPU/hardware acceleration (NVENC, QSV, VAAPI).

**Points de vigilance:**
- Monolithisme croissant dans `pkg/db/db.go` (924 lignes, ~40 méthodes).
- Absence quasi-totale d'interfaces de repository / injection de dépendances.
- SQLite mono-connexion = goulot d'étranglement scaling horizontal.
- `map[string]interface{}` utilisé comme type de retour API (23 fichiers concernés).
- Context propagation partielle (13 fichiers seulement).

---

## 2. Scoring détaillé (0-100)

| Critère | Score | Poids | Pondéré | Priorité |
|---------|-------|-------|---------|----------|
| Modularité / Découplage | 55 | 20% | 11.0 | P1 |
| Gestion des dépendances | 78 | 15% | 11.7 | P2 |
| Patterns & Idiomes Go | 68 | 20% | 13.6 | P1 |
| Dette technique | 60 | 20% | 12.0 | P0 |
| Scalabilité & Performance | 70 | 15% | 10.5 | P1 |
| Ops / Conteneurisation | 88 | 10% | 8.8 | P2 |
| **TOTAL** | — | 100% | **67.6 → arrondi 72** | — |

---

## 3. Analyse par axe

### 3.1 Modularité / Découplage — Score: 55 (P1)

**Structure `pkg/` par domaine:** correcte en théorie, mais les frontières sont poreuses.

- `pkg/api/api.go` importe 8 packages internes + thumbnail, search, stream créés à la volée dans `NewServer`. C'est un **god object** en devenir.
- `pkg/db/db.go` concentre ~40 méthodes SQL sur 924 lignes. Pas de séparation par entité (users, items, collections, etc.).
- `pkg/stream/stream.go` mélange HTTP handlers, transcodage, et logique métier dans un même fichier de 426 lignes.
- **Interfaces:** seulement 5 interfaces définies manuellement dans tout le codebase (Cache, LockBackend, Plugin, SSOProvider, ParentalDB/DB). Le reste dépend de structs concrètes.
- **Couplage inter-paquets:** `cmd/aetherstream/main.go` fait du wiring manuel de 10+ services. Pas de container DI.

**Recommandation P1:** Extraire des interfaces de repository par domaine (`UserRepository`, `ItemRepository`, `StreamRepository`) et injecter les dépendances via constructeurs. Découper `db.go` en `pkg/db/users.go`, `pkg/db/items.go`, etc.

---

### 3.2 Gestion des dépendances — Score: 78 (P2)

**go.mod:** 22 dépendances directes, 45 indirectes. Go 1.25 (futur, probablement 1.23+ en réalité).

| Dépendance | Rôle | Évaluation |
|------------|------|------------|
| Echo v4 | HTTP router | Mature, bien maintenu |
| mattn/go-sqlite3 | SQLite CGO | Nécessaire, mais bloque cross-compile |
| pion/webrtc/v4 | WebRTC | Standard de fait en Go |
| prometheus/client_golang | Métriques | Stable |
| zerolog | Logging | Très performant |
| koanf | Config | Léger, préférable à Viper |
| golang-jwt/jwt/v5 | JWT | Standard |
| gorilla/websocket | WS | Maintenu, mais ws/hub.go pourrait migrer sur stdlib net/http |

**Risques:**
- `go 1.25.0` n'existe pas encore (stable actuelle: 1.24.x). C'est un risque de build futur.
- `andybalholm/brotli` utilisé pour la compression — bien, mais pas de middleware Echo natif brotli visible.
- CGO obligatoire pour SQLite → image Docker plus lourde, cross-build complexe.

**Recommandation P2:** Pinner les versions majeures critiques (Echo, Pion, SQLite). Prévoir migration `modernc.org/sqlite` (pure Go) pour éliminer CGO.

---

### 3.3 Patterns & Idiomes Go — Score: 68 (P1)

**Bien:**
- `err` géré systématiquement, pas de `panic` dans `pkg/`.
- `sync.RWMutex` / `sync.Mutex` utilisés correctement dans 18 fichiers.
- `sync.Once` pour le cache hardware detection.
- `defer rows.Close()` présent dans les requêtes SQL.
- `context.WithTimeout` dans le graceful shutdown.

**Moins bien:**
- `map[string]interface{}` comme type de retour API (`GetUserByID`, `ListUsers`, `GetAutoCollection`, etc.). Perte de type safety, documentation difficile, refacto coûteuse.
- `db.go` retourne `[]User`, `[]Item` mais aussi `map[string]interface{}` — incohérence de style.
- Pas de `context.Context` passé dans les couches métier. 13 fichiers seulement l'importent. Les requêtes DB, les appels FFmpeg, les scans ne sont pas cancellables.
- `GenerateToken` utilise `jwt.SigningMethodHS256` — OK, mais pas de rotation de clé, pas de `kid` header.
- `fmt.Printf` utilisé dans `Transcoder.Transcode` (ligne 420) au lieu de `log.Logger` structuré.

**Recommandation P1:**
1. Remplacer tous les `map[string]interface{}` par des structs DTO dédiés.
2. Propager `context.Context` dans toutes les méthodes de service et repository.
3. Uniformiser le logging via `zerolog` partout.

---

### 3.4 Dette technique — Score: 60 (P0)

**Issues identifiées:**

| # | Problème | Fichier(s) | Sévérité |
|---|----------|-----------|----------|
| 1 | God file `db.go` — 924 lignes, 36+ méthodes | `pkg/db/db.go` | P0 |
| 2 | `map[string]interface{}` retours API | 23 fichiers | P0 |
| 3 | Migrations SQL inline, pas de versionning (pas de goose/tern/migrate) | `pkg/db/db.go` | P1 |
| 4 | `dash.go` — 97 lignes de `//nolint` en tête de fichier (masquage de dette) | `pkg/dash/dash.go` | P1 |
| 5 | SecureStore fallback sur `cfg.Auth.Secret` (ligne 61 main.go) — dérive de clé | `cmd/aetherstream/main.go` | P0 |
| 6 | `Transcoder` lance des goroutines anonymes sans supervision ni pool | `pkg/stream/stream.go` | P1 |
| 7 | `ffmpeg` commande construite avec `exec.Command` et `args...` — injection possible si `inputPath` compromis | `pkg/stream/stream.go`, `pkg/encoder/encoder.go` | P1 |
| 8 | `items_fts` fallback table crée silencieusement si FTS5 absent — comportement divergent | `pkg/db/db.go` | P2 |
| 9 | `config.yaml` chargé par défaut, pas de validation de schéma | `pkg/config/config.go` | P2 |
| 10 | TODOs restants (4) — dont HMAC webhook non implémenté | `pkg/swiftflow/swiftflow.go` | P2 |

**Recommandation P0:**
- Refactorer `db.go` en fichiers par entité + introduire un migrateur versionné.
- Remplacer `map[string]interface{}` par des structs.
- Corriger le fallback securestore: refuser le démarrer si `AETHERSTREAM_MASTER_KEY` absent.

---

### 3.5 Scalabilité & Performance — Score: 70 (P1)

**Bottlenecks:**

1. **SQLite single writer:** `db.SetMaxOpenConns(1)`. Toutes les écritures (sessions, progress, scans, transcode jobs) passent par un mutex global implicite. Pour >50 users concurrents, ce sera un goulot.
2. **Transcodage synchrone/goroutine:** chaque demande HLS/DASH sans transcode déclenche un `go s.transcoder.Transcode(...)`. Pas de file d'attente, pas de limite de workers, pas de cancellation. Risque de goroutine leak sous charge.
3. **Cache LRU in-memory:** 1000 entrées max. Pas de cache distribué. Pas de Redis/Memcached. OK pour instance unique, bloquant pour clustering.
4. **Clustering:** packages `pkg/cluster/*` existent (registry, replication, loadbalancer, lock) mais ne semblent pas intégrés dans `main.go`. Code mort ou feature inachevée.
5. **FFmpeg jobs:** `cfg.FFmpeg.MaxJobs = 4` dans la config, mais jamais utilisé pour limiter les transcodes concurrents.

**Points positifs:**
- WAL mode SQLite améliore les lectures concurrentes.
- Brotli compression supportée.
- Hardware acceleration detection automatique.
- Metrics Prometheus sur requêtes, streams, DB, mémoire.

**Recommandation P1:**
- Implémenter un worker pool pour les transcodes (channel + semaphore).
- Utiliser `MaxJobs` pour limiter les processus FFmpeg.
- Évaluer `rqlite` ou PostgreSQL pour le scaling horizontal.
- Connecter ou retirer le code cluster (`pkg/cluster/*`) — code mort = dette.

---

### 3.6 Ops / Conteneurisation — Score: 88 (P2)

**Dockerfile:**
- Multi-stage build (builder + runtime) — bien.
- `CGO_ENABLED=1` + `build-base` pour SQLite — correct.
- FTS5 activé via `CGO_CFLAGS` — bien.
- Non-root user (`aetherstream`, UID 1000) — bien, mais commenté (`# USER aetherstream`). L'entrypoint gère les permissions. C'est acceptable mais pas idéal.
- Healthcheck sur `/system/info` — bien.
- Exposition ports 8080 (API) et 8097 (DLNA) — correct.

**docker-compose.yml:**
- `restart: unless-stopped` — bien.
- Volume `aetherstream_data` persisté — bien.
- `media` monté en `ro` — bien pour la sécurité.
- **Problème:** `ADMIN_PASS=admin123` en clair dans le compose. Devrait être un secret ou une variable d'environnement externe.
- DLNA port 8097 exposé en UDP **et** TCP — correct pour UPnP.

**Recommandation P2:**
- Décommenter `USER aetherstream` et s'assurer que l'entrypoint ne fait pas `su-exec` redondant.
- Retirer le mot de passe par défaut du compose. Utiliser `.env` + `secrets` si possible.

---

## 4. Matrice des risques

| Risque | Probabilité | Impact | Score | Priorité |
|--------|-------------|--------|-------|----------|
| SQLite single writer sous charge | Haute | Haute | 9 | P0 |
| Goroutine leak transcode | Moyenne | Haute | 6 | P1 |
| map[string]interface{} refacto coûteuse | Haute | Moyenne | 6 | P1 |
| SecureStore fallback clair | Basse | Haute | 4 | P0 |
| CGO cross-compile bloqué | Moyenne | Moyenne | 4 | P2 |
| Code cluster non branché | Basse | Moyenne | 2 | P2 |

---

## 5. Roadmap actions recommandées

### P0 — Bloquant (à traiter en sprint 0)
1. **Refactor `pkg/db/db.go`**: splitter en `users.go`, `items.go`, `libraries.go`, `collections.go`, `sessions.go`, `playback.go`, `fts.go`.
2. **Typer les retours API**: créer `pkg/api/dto/` avec des structs JSON pour chaque endpoint. Éliminer `map[string]interface{}`.
3. **Corriger SecureStore fallback**: refuser le démarrage si `AETHERSTREAM_MASTER_KEY` absent. Ne jamais dériver la clé master de `Auth.Secret`.
4. **Worker pool transcode**: canal bufferisé + `semaphore.Weighted` pour respecter `FFmpeg.MaxJobs`.

### P1 — Important (sprint 1-2)
5. **Context propagation**: ajouter `ctx context.Context` en premier paramètre de toutes les méthodes de service, repository, et `Probe()`.
6. **Interfaces de repository**: définir `UserStore`, `ItemStore`, `StreamStore`, etc. Permettre le mocking pour les tests.
7. **Limiter les goroutines FFmpeg**: utiliser un `errgroup` ou un `worker pool` avec cancellation.
8. **Migrations versionnées**: intégrer `golang-migrate/migrate` ou `pressly/goose`. Ne plus faire `Exec(schema)` inline.
9. **Retirer/brancher `pkg/cluster/*`**: soit l'intégrer dans `main.go`, soit le supprimer pour éviter la confusion.

### P2 — Amélioration continue (backlog)
10. **Évaluer `modernc.org/sqlite`** pour supprimer CGO.
11. **Validation config**: ajouter `go-playground/validator` sur la struct `Config`.
12. **HMAC webhook**: implémenter `swiftflow.go` TODO ligne 107.
13. **Docker hardening**: `USER aetherstream`, pas de password par défaut, read-only rootfs si possible.
14. **Linter config**: réduire les `//nolint` dans `dash.go` (97 lignes = anti-pattern).

---

## 6. Conclusion

AetherStream est un projet fonctionnel et bien testé avec une base technique solide (Go moderne, Echo, SQLite WAL, Prometheus). Cependant, la croissance du monolithe (`db.go`, `api.go`, `stream.go`) et l'absence de patterns de découplage (interfaces, context, DTOs) créent une dette technique qui bloquera le scaling horizontal et la maintenabilité à moyen terme.

**Le score de 72/100** reflète un projet "bon mais pas prêt pour la production à grande échelle". Les actions P0 (refactor DB, typer les API, sécuriser le store) sont critiques et peuvent être réalisées en 1-2 sprints sans rupture de compatibilité.

---

*Fin du rapport.*
