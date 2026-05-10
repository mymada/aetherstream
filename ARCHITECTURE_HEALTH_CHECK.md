# Architecture Health Check — AetherStream

**Project:** AetherStream (Go media server)  
**Packages:** 51 (50 under `pkg/` + `cmd/aetherstream`)  
**Audit Date:** 2026-05-10  
**Go Version:** 1.25.0  
**Build:** OK (`go build ./...`)  
**Tests:** 50/51 packages avec tests (`pkg/compliance` sans tests)  
**Coverage:** 47.0% total (statement)  
**Gosec:** 0-0-0 (clean)  

---

## Table des scores par axe (0-100)

| Axe | Score | Poids | Pondéré |
|-----|-------|-------|---------|
| 1. Structure des packages | 72 | 15% | 10.8 |
| 2. Dépendances et couplage | 58 | 15% | 8.7 |
| 3. Patterns de conception | 55 | 10% | 5.5 |
| 4. Scalabilité | 48 | 15% | 7.2 |
| 5. Performance | 52 | 15% | 7.8 |
| 6. Maintenabilité | 50 | 15% | 7.5 |
| 7. Dette technique | 45 | 15% | 6.75 |
| **Score global** | **54.25** | 100% | **54.25** |

---

## 1. Structure des packages — Score: 72/100

### Forces
- **51 packages** avec séparation fonctionnelle claire : `api`, `auth`, `stream`, `cluster`, `webrtc`, `cast`, `dlna`, `livetv`, `tasks`, `webhooks`, `compliance`…
- Chaque domaine métier est isolé dans son propre package.
- `cmd/aetherstream` contient uniquement `main.go` (1 fichier, pas de logique métier).
- `migrations/` et `scripts/` sont bien séparés.
- Présence de packages transversaux bien identifiés : `models`, `config`, `cache`, `metrics`, `docs`.

### Faiblesses
- **`pkg/api/api.go` (648 lignes, 31 handlers)** : god object qui importe 13 packages internes directement et instancie `thumbnail.NewService`, `search.NewSearcher`, `stream.NewServer` dans `RegisterRoutes`.
- **Couche `internal/` quasi inexistante** : seuls `internal/ffmpeg` et `internal/utils` existent. La logique sensible (`securestore`, `auth`) est publique dans `pkg/`.
- **Packages vides ou quasi vides** : `pkg/images` (no statements), `pkg/profiles` (vide), `pkg/sessionsync` (vide), `pkg/tasks` (vide), `pkg/transcode` (seulement tests), `pkg/models` (structs uniquement). 6/51 packages = 12% de surface morte.
- **`pkg/compliance`** : package sans tests, sans code exécutable apparent.

### Recommandations
1. Introduire `internal/app/` pour orchestration DI et initialisation.
2. Déplacer `securestore`, `auth`, `config` vers `internal/` si non réutilisables.
3. Fusionner ou supprimer les packages vides (`images`, `profiles`, `sessionsync`, `tasks`, `transcode`).
4. Extraire `api.Server` en sous-handlers par domaine (`UserHandler`, `LibraryHandler`, `StreamHandler`).

---

## 2. Dépendances et couplage — Score: 58/100

### Forces
- **92 dépendances Go** (directes + indirectes) — raisonnable pour un media server.
- Pas de dépendances inutiles détectées.
- Stack cohérente : Echo v4, zerolog, JWT v5, Prometheus, Pion WebRTC, koanf, SQLite3 (mattn/go-sqlite3).
- Pas de cycles d'import détectés (`go build ./...` OK).

### Faiblesses
- **Couplage fort `api.Server` → 13 packages internes** : `db`, `auth`, `config`, `encoder`, `library`, `probe`, `search`, `securestore`, `stream`, `thumbnail`, `ws`, `cast`, `oauth`.
- **Pas d'interfaces de repository** : `api.Server` dépend de `*db.DB` concret, `*auth.Service` concret, `*library.Manager` concret.
- **Instanciation en dur** dans `NewServer` et `RegisterRoutes` : impossible de mocker pour les tests unitaires.
- **`pkg/library.Manager`** dépend de `metadata.TMDbClient` concret (pas d'interface `MetadataProvider`).
- **`pkg/stream.Transcoder`** dépend de `encoder.GetProfileByName` et `exec.Command("ffmpeg", ...)` en dur.

### Métriques
- 6 interfaces publiques dans tout `pkg/` (Cache, Plugin, LockBackend, SSOProvider, DB, ParentalDB) → **ratio interfaces/structs = 6/202 ≈ 3%** (extrêmement faible).
- 20 fichiers utilisent `map[string]interface{}` (57 occurrences).

### Recommandations
1. Définir des interfaces par domaine (`ItemStore`, `UserStore`, `MetadataProvider`, `Transcoder`).
2. Injecter les dépendances via le constructeur `NewServer`, jamais dans `RegisterRoutes`.
3. Créer un conteneur DI manuel ou utiliser `google/wire`.
4. Découpler `library.Manager` de `TMDbClient` via interface.

---

## 3. Patterns de conception — Score: 55/100

### Patterns identifiés
| Pattern | Où | Qualité |
|---------|-----|---------|
| Middleware chain | Echo + custom security | Bien |
| Worker pool (channel) | `library.scanQueue` (buffer=10) | Correct, pas de graceful shutdown complet |
| Observer / Event bus | `pkg/plugin` | Bon début mais non câblé au système |
| Repository-ish | `pkg/db/db.go` | Faible (pas de struct typée) |
| Strategy | `encoder.Profile` + hwaccel switch | Bien mais code dupliqué |
| Singleton global | `ws.globalHub`, `performance/hwCache` | Risque tests parallèles |

### Anti-patterns identifiés
1. **God Object** : `api.Server` (648 lignes, 31 handlers, 13 dépendances).
2. **Global State** : `ws.globalHub` empêche tests parallèles et multi-tenant.
3. **Stringly-typed** : `map[string]interface{}` utilisé comme DTO universel (57 occurrences).
4. **Duplicate Code** : logique codec FFmpeg dupliquée entre `Profile.Command()` et `BuildHLSCommand()` (~40 lignes).
5. **Magic Numbers** : `30*time.Second`, `10` (scanQueue), `256` (Brotli), `48` (keyframe) dispersés sans constantes.

### Recommandations
1. Refactor `api.Server` en handlers domaine avec DI réduit.
2. Rendre `Hub` instanciable et injectable.
3. Extraire `selectCodec(profile, hwAccel)` pour éliminer duplication FFmpeg.
4. Introduire des constantes nommées pour tous les timeouts et seuils.
5. Câbler le bus d'événements `pkg/plugin` au reste du système.

---

## 4. Scalabilité — Score: 48/100

### Forces
- **Clustering** : gossip UDP, registry de nœuds, load balancer, réplication WAL — architecture distribuée de base présente.
- **SQLite WAL mode** : permet lecture concurrente.
- **HLS/DASH** : streaming adaptatif natif.
- **Rate limiting par IP** : implémenté (`RateLimitByIP`).
- **Brotli/ETag middleware** : compression et cache HTTP.

### Faiblesses
- **SQLite monolithique** : pas de sharding, pas de read replicas. Limite verticale évidente pour un catalogue > 100k items.
- **`ws.Hub.clients`** : map globale avec `RWMutex`. Broadcast itère sur tous les clients sous lock. Bottleneck à 10k+ clients.
- **`stream.Transcoder`** : pas de rate limiting sur les transcodes — DoS par requêtes HLS possibles.
- **`library.scanQueue`** : buffer fixe de 10, pas de backpressure configurable.
- **Pas de load balancing côté stream** : un seul `stream.Server` par instance.
- **Pas de horizontal pod autoscaling** : pas de métriques custom pour K8s HPA.
- **`api.go` `handleGetUser`** ne charge plus toute la table (corrigé : utilise `GetUserByID`), mais `handleListUsers` charge tous les users sans pagination.

### Recommandations
1. Ajouter pagination sur toutes les listes API (`users`, `items`, `libraries`).
2. Remplacer `ws.globalHub` par un hub sharded par userID ou par room.
3. Ajouter un semaphore ou `sync.Map` de timestamps pour limiter les transcodes simultanés.
4. Évaluer PostgreSQL ou read replicas SQLite pour le scaling catalogue.
5. Exposer métriques custom pour HPA (streams actifs, CPU transcode).

---

## 5. Performance — Score: 52/100

### Forces
- **Hardware detection** : `encoder.DetectHardwareCapabilities()` avec `sync.Once` — pas de ré-exécution.
- **Cache LRU** : `pkg/cache` bien implémenté.
- **Brotli middleware** : compression des réponses.
- **Echo `c.File()`** : utilise `http.ServeContent` avec support Range — OK pour le direct streaming.
- **Benchmarks** : 5 benchmarks présents (`pkg/benchmark`).
- **Prometheus metrics** : HTTP requests, duration, active streams, DB queries, cache hits.

### Faiblesses
- **`map[string]interface{}` boxing** : chaque query DB alloue map + interfaces. Sur 10k+ items, allocations significatives.
- **Brotli double allocation** : `rec.body.String()` puis `[]byte(body)` — bufferise tout en mémoire.
- **DB slices sans pré-allocation** : `var users []User` au lieu de `make([]User, 0, estimatedCount)`.
- **`handleListUsers`** : charge toute la table users en mémoire sans pagination.
- **`ws.Broadcast`** : lock global + itération sur tous les clients.
- **Pas de `sync.Pool`** pour les buffers WebSocket ou transcode.

### Recommandations
1. Remplacer `map[string]interface{}` par des structs typées (réduction 30-50% allocations).
2. Pré-allouer les slices DB avec capacité estimée.
3. Streamer Brotli avec `io.Copy` au lieu de bufferiser en string.
4. Ajouter `sync.Pool` pour buffers WS et transcode.
5. Paginer toutes les listes API.

---

## 6. Maintenabilité — Score: 50/100

### Forces
- **Tests sur 50/51 packages** — bon taux de couverture fonctionnelle.
- **`fmt.Errorf("...: %w", err)`** : 113 occurrences — bonne pratique de wrapping.
- **README, CHANGELOG, SECURITY_AUDIT_REPORT** existent.
- **GoDoc** présent sur les packages et fonctions exportées.
- **Build Docker** multi-stage fonctionnel.
- **gosec 0-0-0** : pas de vulnérabilité détectée par l'analyseur statique.

### Faiblesses
- **Coverage 47%** — insuffisant pour production.
- **Packages critiques quasi non testés** :
  - `pkg/tags` : 0.0%
  - `pkg/trickplay` : 5.0%
  - `pkg/smartplaylists` : 7.6%
  - `pkg/webrtc` : 11.4%
  - `pkg/ws` : 11.1%
  - `pkg/stream` : 21.7%
  - `cmd/aetherstream` : 0.0%
- **Pas de `ARCHITECTURE.md`** expliquant les flux de données.
- **Pas de `CONTRIBUTING.md`**.
- **Swagger/docs** : couverture partielle des endpoints.
- **6 packages vides** : confusion pour les nouveaux développeurs.
- **TODO/FIXME** : peu (6 occurrences), mais certains critiques (ex: HMAC validation SwiftFlow).

### Recommandations
1. Atteindre 70% minimum sur tous les packages avant production.
2. Prioriser `pkg/stream`, `pkg/ws`, `pkg/webrtc` (hot paths utilisateur).
3. Créer `docs/ARCHITECTURE.md` avec diagramme des flux.
4. Compléter Swagger pour tous les endpoints.
5. Supprimer/fusionner les packages vides.

---

## 7. Dette technique — Score: 45/100

### Dettes identifiées

| Dette | Sévérité | Effort de remboursement |
|-------|----------|------------------------|
| `map[string]interface{}` universel (57 occ.) | **Critique** | Moyen |
| Race condition `stream.Transcoder.jobs` | **Critique** | Faible |
| `api.Server` god object (648 lignes) | **Haute** | Moyen |
| `ws.globalHub` singleton global | **Haute** | Faible |
| `fmt.Printf` dans `stream.go` | **Moyenne** | Faible |
| Pas de `context.Context` dans workers | **Moyenne** | Faible |
| Pas de validation config au startup | **Moyenne** | Faible |
| Plugin system non câblé | **Moyenne** | Moyen |
| CORS origins hardcodées | **Faible** | Faible |
| Profiles FFmpeg hardcodés | **Faible** | Faible |
| Pas de `GetUserByID` struct typé (corrigé partiellement) | **Faible** | Faible |
| `handleListUsers` sans pagination | **Moyenne** | Faible |
| `library.Manager.Close()` sans `WaitGroup` | **Moyenne** | Faible |
| `db.Migrate()` ignore silencieusement erreurs FTS5 | **Moyenne** | Faible |
| `secureStore` fallback avec secret dérivé | **Haute** | Faible |

### Recommandations prioritaires (P0 → P3)

| Priorité | Item | Impact | Effort |
|----------|------|--------|--------|
| **P0 — Critique** | Fix race condition `stream.Transcoder.jobs` | Stabilité | Faible |
| **P0 — Critique** | Remplacer `map[string]interface{}` par structs typées | Sécurité, perf, maintenabilité | Moyen |
| **P0 — Critique** | Bloquer démarrage production si `AETHERSTREAM_MASTER_KEY` absent | Sécurité | Faible |
| **P1 — Haut** | Refactor `api.Server` en handlers domaine + interfaces DI | Maintenabilité | Moyen |
| **P1 — Haut** | Augmenter coverage sur `stream`, `ws`, `webrtc`, `tags`, `trickplay` | Qualité | Moyen |
| **P1 — Haut** | Ajouter `context.Context` aux workers pour shutdown propre | Stabilité | Faible |
| **P2 — Moyen** | Pagination sur toutes les listes API | Scalabilité | Faible |
| **P2 — Moyen** | Câbler plugin event bus | Extensibilité | Moyen |
| **P2 — Moyen** | Validation config au startup | Robustesse | Faible |
| **P3 — Bas** | Documentation architecture complète | Onboarding | Faible |

---

## 8. Score global — 54/100

### Interprétation

**54/100 = « Prototype avancé, pas production-ready »**

AetherStream est un projet Go bien structuré avec une séparation fonctionnelle claire et des choix technologiques pertinents. Cependant, plusieurs blocages production ont été identifiés :

1. **Qualité du typage** : l'usage massif de `map[string]interface{}` est le défaut le plus grave pour la maintenabilité et la performance.
2. **Concurrence** : une race condition confirmée dans le transcodeur doit être corrigée immédiatement.
3. **Tests** : 47% de coverage global est insuffisant ; plusieurs packages critiques sont quasi non testés.
4. **Architecture** : le manque d'interfaces et l'instanciation en dur des dépendances dans `api.Server` rendent le code difficile à tester et à étendre.
5. **Scalabilité** : SQLite monolithique + hub WebSocket global + pas de rate limiting transcode = plafond de croissance visible.

### Seuils de maturité

| Seuil | Score | Statut |
|-------|-------|--------|
| MVP / Demo | 40-55 | ✅ Atteint |
| Beta / Early access | 55-70 | ❌ Non atteint |
| Production-ready | 70-85 | ❌ Non atteint |
| Enterprise / SRE | 85-100 | ❌ Non atteint |

### Feuille de route pour atteindre 70+ (production-ready)

1. **Sprint 1 (P0)** : Fix race condition, structs typées DB, secret strict mode → +8 points
2. **Sprint 2 (P1)** : Refactor DI + interfaces, coverage hot paths → +8 points
3. **Sprint 3 (P2)** : Pagination, config validation, plugin event bus → +5 points
4. **Sprint 4 (P3)** : Documentation, benchmarks charge, stress tests → +3 points

**Projection : 54 → 78 en 4 sprints** (estimation ~6-8 semaines à temps plein).

---

*Audit généré via analyse statique du codebase AetherStream (51 packages, 69 fichiers Go, 59 fichiers de test, 92 dépendances).*
