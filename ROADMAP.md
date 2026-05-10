# AetherStream — Roadmap to Production

**Version:** 0.2.0  
**Date:** 2026-05-10  
**Status:** Alpha — feature-complete, security-hardening phase  
**Goal:** Transform AetherStream from an alpha-grade media server into a production-ready, secure, observable, and scalable platform.

---

## Executive Summary

AetherStream (49 packages, 50/50 tests passing, 0 gosec HIGH/MEDIUM/LOW in core) has a solid foundation: adaptive streaming, HLS/DASH, DLNA, Live TV/DVR, WebSocket realtime, OAuth2, API keys, secure store (AES-256-GCM), Prometheus metrics, and a React Web UI. However, the security audit revealed **4 HIGH**, **7 MEDIUM**, and **12 LOW** severity findings that must be resolved before any production deployment. The most critical risks are configuration-level (hardcoded secrets, overly permissive CORS, broken session timeout) and missing authentication on media streaming routes.

This roadmap is organized into three horizons:

1. **Court terme (1-2 mois)** — Security lockdown, P0 fixes, production baseline
2. **Moyen terme (3-6 mois)** — Performance, observability, hardening, missing features
3. **Long terme (6-12 mois)** — Scale, ecosystem, enterprise readiness

For each objective: **Priorite**, **Difficulte**, **Impact**, **Dependances**.

---

## 1. Objectifs a court terme (1-2 mois)

> **Theme:** *Security lockdown + production baseline.*  
> **Cible:** Deployable en production restreinte (LAN / petit VPS) avec confiance.

### 1.1 Remediation P0 — Securite critique

| ID | Objectif | Priorite | Difficulte | Impact | Dependances |
|----|----------|----------|------------|--------|-------------|
| S1 | **Retirer le fallback JWT secret en dur** — Si `AETHERSTREAM_AUTH_SECRET` n'est pas defini, le serveur DOIT refuser de demarrer. Generer un secret aleatoire 256-bit au premier demarrage et le persister avec permissions 0600. | P0 | Facile | Critique | — |
| S2 | **Corriger CORS wildcard + credentials** — Retirer `"*"` de `AllowOrigins`. Rendre les origines autorisees configurables via env/config. | P0 | Facile | Critique | — |
| S3 | **Reparer le middleware SessionTimeout** — `echo.Context` est recree par requete ; le middleware actuel est inoperant. Implementer un stockage de session cote serveur (cookie HttpOnly SameSite=Strict + timestamp serveur en DB/Redis/cookie chiffre). | P0 | Moyenne | Critique | — |
| S4 | **Validation de chemins subtitle/thumbnail** — `lang` et `itemID` utilisateur-controles passes a `ExtractSubtitleToFile` et `thumbSvc.Path` sans sanitisation. Whitelist `lang` (`^[a-zA-Z]{2,3}(-[a-zA-Z]{2})?$`), valider `itemID` (UUID/alphanum), verifier que le chemin retourne reste sous le repertoire autorise (`filepath.Rel` + check `..`). | P0 | Moyenne | Critique | — |

### 1.2 Remediation P1 — Securite importante

| ID | Objectif | Priorite | Difficulte | Impact | Dependances |
|----|----------|----------|------------|--------|-------------|
| S5 | **Utiliser le vrai user ID/role dans le token de login** — `pkg/api/api.go:162` hardcode `"admin-1"` et `"admin"`. Utiliser les valeurs retournees par `GetUserByUsername()`. | P1 | Facile | Haut | — |
| S6 | **Protection mutex + cleanup pour OAuth state map** — `pkg/oauth/oauth.go` utilise un `map` non protege en concurrence. Ajouter `sync.RWMutex` + goroutine de nettoyage periodique. | P1 | Facile | Haut | — |
| S7 | **Restreindre les permissions de repertoire/fichier** — Remplacer tous `os.MkdirAll(..., 0755)` par `0750` (ou `0700` pour transcodes/thumbnails). Idem pour `os.WriteFile`/`os.OpenFile` (`0640`/`0600`). gosec G301/G302/G306. | P1 | Facile | Haut | — |
| S8 | **Ajouter l'authentification sur les routes de streaming** — `/videos/:id/stream`, `/videos/:id/hls/...`, etc. sont enregistrees sur le routeur public `e`. Les deplacer derriere le groupe `api` protege ou ajouter `s.auth.Middleware()`. | P1 | Facile | Haut | S5 |
| S9 | **Deriver la cle securestore avec PBKDF2/Argon2id** — Actuellement la cle est tronquee a 32 bytes sans key stretching. Ajouter un sel unique par chiffrement et deriver via PBKDF2 (100k+ iterations) ou Argon2id. | P1 | Moyenne | Haut | — |
| S10 | **Hacher les API keys avec bcrypt ou HMAC+pepper** — SHA-256 non sale est trop rapide. Migrer vers bcrypt (couteux mais sur) ou HMAC-SHA256 avec un pepper serveur. | P1 | Moyenne | Haut | — |
| S11 | **Mettre a jour `golang.org/x/net` vers >= v0.55.0** — CVE-2025-22872 (HTTP/2 CONTINUATION flood). | P1 | Facile | Haut | — |

### 1.3 Tests & Qualite

| ID | Objectif | Priorite | Difficulte | Impact | Dependances |
|----|----------|----------|------------|--------|-------------|
| T1 | **Atteindre 70% de couverture de tests** — Actuellement 50 packages testes mais couverture inconnue. Generer un rapport `coverage.out`, identifier les packages sous-testes (API, stream, auth, config). | P1 | Moyenne | Haut | — |
| T2 | **Ajouter des tests de securite** — Tests pour path traversal, JWT forgery avec mauvais secret, CORS preflight, brute-force lockout, CSRF token validation. | P1 | Moyenne | Haut | S1-S4 |
| T3 | **Activer `-race` en CI** — `go test -race ./...` doit passer sans data race (notamment OAuth state map, WebSocket hub). | P1 | Facile | Moyen | S6 |

### 1.4 Documentation & Deploiement

| ID | Objectif | Priorite | Difficulte | Impact | Dependances |
|----|----------|----------|------------|--------|-------------|
| D1 | **Checklist de hardening pre-production** — Document `docs/SECURITY_HARDENING.md` : secrets requis, permissions fichiers, reverse proxy (nginx/traefik), TLS, fail2ban, firewall. | P1 | Facile | Moyen | S1-S11 |
| D2 | **Helm chart Kubernetes** — Deploiement stateful avec PVC pour DB et media, ConfigMap pour env vars, Secret pour auth/master key. | P2 | Moyenne | Moyen | D1 |
| D3 | **Docker Compose production** — Version avec reverse proxy (Traefik), Let's Encrypt, reseau isole, health checks, limits memoire/CPU. | P1 | Facile | Moyen | — |

---

## 2. Objectifs a moyen terme (3-6 mois)

> **Theme:** *Performance, observabilite, hardening avance, features manquantes.*  
> **Cible:** Serveur media robuste pour usage domestique / petit bureau (10-50 utilisateurs).

### 2.1 Performance & Scalabilite

| ID | Objectif | Priorite | Difficulte | Impact | Dependances |
|----|----------|----------|------------|--------|-------------|
| P1 | **Cache multi-niveaux** — Cache metadata (Redis/in-memory LRU) pour reduire les requetes SQLite. Cache thumbnails generees (CDN-compatible). Cache HLS playlists avec ETag. | P1 | Moyenne | Haut | — |
| P2 | **Pool de workers FFmpeg** — Limiter les jobs transcode par GPU/CPU. File d'attente priorisee (direct play > transcode). Monitoring du pool en temps reel. | P1 | Moyenne | Haut | — |
| P3 | **Benchmarks automatises** — Suite `pkg/benchmark` etendue : throughput streaming, latence HLS startup, charge API (k6/vegeta). Objectifs chiffres (p95 < 200ms API, < 2s HLS startup). | P2 | Moyenne | Moyen | T1 |
| P4 | **Brotli/Gzip compression** — Activer Brotli pour assets statiques et API JSON (deja present dans `pkg/performance/brotli.go` — l'activer par defaut). | P2 | Facile | Moyen | — |
| P5 | **Base de donnees migrable** — Support PostgreSQL en option pour les instances multi-serveur. Garder SQLite par defaut pour le mode standalone. | P2 | Difficile | Haut | — |

### 2.2 Observabilite & Monitoring

| ID | Objectif | Priorite | Difficulte | Impact | Dependances |
|----|----------|----------|------------|--------|-------------|
| M1 | **Dashboard Grafana** — JSON model pour metrics Prometheus : streams actifs, jobs transcode, erreurs API, latence DB, espace disque. | P1 | Facile | Haut | — |
| M2 | **Alerting (Prometheus Alertmanager)** — Regles : disque > 85%, erreur API > 5%, transcode queue > 10 jobs, auth failures > 100/min. | P1 | Moyenne | Haut | M1 |
| M3 | **Tracing distribue (OpenTelemetry)** — Instrumenter les handlers Echo, les appels FFmpeg, les requetes metadata TMDb/MusicBrainz. | P2 | Difficile | Moyen | — |
| M4 | **Health checks approfondis** — Endpoint `/health/ready` (DB accessible, FFmpeg present, espace disque OK) et `/health/live` (process up). | P1 | Facile | Moyen | — |
| M5 | **Log structured enrichi** — Correlation ID par requete, champs standardises (user_id, item_id, session_id). Export vers Loki/ELK. | P2 | Moyenne | Moyen | — |

### 2.3 Securite avancee

| ID | Objectif | Priorite | Difficulte | Impact | Dependances |
|----|----------|----------|------------|--------|-------------|
| S12 | **Authentification WebSocket** — `/ws` est public. Valider le token JWT pendant le handshake Upgrade ou deplacer derriere le groupe `api`. | P1 | Facile | Haut | S8 |
| S13 | **HSTS conditionnel** — Ne pas envoyer `Strict-Transport-Security` sur HTTP. Rendre `preload` opt-in via config. | P2 | Facile | Moyen | — |
| S14 | **Rate limiting avance** — Per-user (pas seulement per-IP) pour eviter les abus via proxy. Bucket token avec Redis pour multi-instance. | P2 | Moyenne | Moyen | — |
| S15 | **Content Security Policy (CSP)** — Header `Content-Security-Policy` strict pour l'interface web. | P2 | Facile | Moyen | — |
| S16 | **Audit log complet** — `pkg/audit/audit.go` etendu : loguer toutes les actions admin (CRUD users, libraries, API keys), exports, suppressions. Retention configurable. | P1 | Moyenne | Haut | — |
| S17 | **Scan de dependances automatise** — Integrer `govulncheck` et `snyk` dans la CI. Bloquer le merge si CVE HIGH/MEDIUM non resolue. | P1 | Facile | Haut | — |
| S18 | **Penetration test interne** — OWASP ZAP ou Burp Suite sur l'instance de staging. Rapport et remediation avant v1.0. | P2 | Difficile | Haut | S1-S16 |

### 2.4 Features manquantes

| ID | Objectif | Priorite | Difficulte | Impact | Dependances |
|----|----------|----------|------------|--------|-------------|
| F1 | **DASH complet** — `pkg/dash/dash.go` est un stub. Implementer la generation de manifests MPD, adaptation bitrate, segments init/media. | P1 | Difficile | Haut | — |
| F2 | **WebRTC streaming** — `pkg/webrtc` a un signaling stub. Completer l'echange ICE, DTLS, SRTP pour streaming P2P/navigateur. | P2 | Difficile | Moyen | — |
| F3 | **Chromecast / AirPlay** — `pkg/cast/chromecast.go` et `pkg/cast/airplay.go` sont stubs. Implementer la decouverte SSDP/mDNS et le controle de lecture. | P2 | Difficile | Moyen | — |
| F4 | **Clustering multi-serveur** — `pkg/cluster` a registry/replication stubs. Implementer election de leader, sync DB, repartition de charge transcode. | P2 | Difficile | Moyen | P5 |
| F5 | **Backup automatise** — `pkg/backup/backup.go` present mais limiter la taille de decompression (gosec G110). Chiffrement des backups avec le master key. | P1 | Moyenne | Haut | S9 |
| F6 | **Notifications push** — WebPush ou Firebase pour notifier fin de transcode, nouveaux episodes, erreurs systeme. | P2 | Moyenne | Moyen | — |
| F7 | **Multi-langue UI** — i18n React (fr, en, es, de). Detection automatique + choix manuel. | P2 | Facile | Moyen | — |

---

## 3. Objectifs a long terme (6-12 mois)

> **Theme:** *Scale, ecosystem, enterprise readiness.*  
> **Cible:** Plateforme media capable de servir des centaines d'utilisateurs, deployable en cloud, monetisable ou self-hostable avance.

### 3.1 Scalabilite & Cloud

| ID | Objectif | Priorite | Difficulte | Impact | Dependances |
|----|----------|----------|------------|--------|-------------|
| C1 | **Object Storage (S3/MinIO)** — Support stockage media sur S3 avec streaming via presigned URLs ou proxy range-request. | P2 | Difficile | Haut | P5 |
| C2 | **Transcode serverless / queue distribuee** — Envoyer les jobs FFmpeg vers une queue (NATS/RabbitMQ/SQS) consommee par des workers auto-scaling. | P2 | Difficile | Haut | P2 |
| C3 | **CDN integration** — Pousser les segments HLS/DASH et thumbnails vers un CDN (CloudFront, Bunny, KeyCDN). Invalidation selective. | P2 | Difficile | Moyen | C1 |
| C4 | **Multi-tenant** — Isolation des libraries, users, settings par organisation/tenant. Admin tenant vs admin global. | P2 | Difficile | Haut | S5 (RBAC avance) |

### 3.2 Ecosysteme & Integrations

| ID | Objectif | Priorite | Difficulte | Impact | Dependances |
|----|----------|----------|------------|--------|-------------|
| E1 | **Plugin marketplace** — `pkg/plugin` actuellement basique. API stable pour plugins, sandboxing (WASM ou gRPC), catalogue centralise. | P2 | Difficile | Moyen | — |
| E2 | **API tierce enrichie** — Integration Sonarr/Radarr/Lidarr pour auto-import. Integration Plex/Kodi/Jellyfin comme client source. | P2 | Moyenne | Moyen | — |
| E3 | **Mobile apps** — Applications iOS/Android natives (ou PWA avancee) avec offline sync, casting, notifications. | P2 | Difficile | Haut | F2, F3 |
| E4 | **Home Assistant / MQTT** — Exposer l'etat du serveur (lecture en cours, transcodes actifs) via MQTT pour automations domotique. | P3 | Moyenne | Faible | — |

### 3.3 Enterprise & Gouvernance

| ID | Objectif | Priorite | Difficulte | Impact | Dependances |
|----|----------|----------|------------|--------|-------------|
| G1 | **SSO / SAML / LDAP** — Authentification enterprise au-dela de OAuth2 (Google/GitHub). Support SAML 2.0 et LDAP/AD. | P2 | Difficile | Haut | S5 |
| G2 | **RBAC fin avec permission matrix** — Remplacer le modele binaire admin/user par un systeme de permissions granulaires (`users:read`, `users:write`, `transcode:manage`, etc.). | P1 | Difficile | Haut | S5 |
| G3 | **Compliance GDPR / CCPA** — Export de donnees utilisateur, suppression complete (right to erasure), consentement cookies, politique de retention. | P2 | Difficile | Moyen | S16 |
| G4 | **Certification de securite** — SOC 2 Type I ou ISO 27001 readiness pour les offres cloud payantes. | P3 | Difficile | Moyen | S18, G3 |
| G5 | **SLA / SLO definis** — Disponibilite 99.9%, p95 latence API < 100ms, temps de recovery < 5 min. Monitoring et rapports automatises. | P2 | Moyenne | Moyen | M1-M3 |

### 3.4 Performance extreme

| ID | Objectif | Priorite | Difficulte | Impact | Dependances |
|----|----------|----------|------------|--------|-------------|
| X1 | **Zero-copy streaming** — `sendfile` pour direct play, memory-mapped IO pour les gros fichiers. | P3 | Difficile | Moyen | — |
| X2 | **GPU transcoding multi-codec** — AV1 encode/decode, support Apple Silicon Media Engine, Intel QSV AV1. | P2 | Difficile | Haut | P2 |
| X3 | **Edge caching des playlists** — Cache HLS master/variant playlists en memoire avec invalidation par event (nouveau scan, nouvelle metadata). | P2 | Moyenne | Haut | P1 |

---

## 4. Matrice de dependances cles

```
S1 (JWT secret)  -->  S8 (auth streams), S12 (auth WS), T2 (sec tests)
S2 (CORS)        -->  D1 (hardening doc)
S3 (session)     -->  S16 (audit log)
S4 (path trav)   -->  T2 (sec tests)
S5 (login role)  -->  S8 (auth streams), G2 (RBAC matrix)
S6 (OAuth mutex) -->  T3 (race CI)
S9 (securestore) -->  F5 (backup chiffre)
P1 (cache)       -->  P3 (benchmarks), X3 (edge cache)
P2 (FFmpeg pool) -->  C2 (serverless transcode), X2 (GPU)
P5 (PostgreSQL)  -->  C4 (multi-tenant), F4 (cluster)
M1 (Grafana)     -->  M2 (alerting), G5 (SLO)
S18 (pentest)    -->  v1.0 release gate
```

---

## 5. Jalons (Milestones)

| Milestone | Date cible | Criteres de sortie |
|-----------|------------|-------------------|
| **v0.2.1 — Security Patch** | +2 semaines | S1, S2, S3, S4, S5, S6, S7, S11 corriges. gosec 0-0-0 maintenu. Tests passes. |
| **v0.3.0 — Hardened Alpha** | +1 mois | S8, S9, S10, S12, T1 (70% coverage), T2, T3, D1, D3 completes. Premiere deploiement LAN securise. |
| **v0.4.0 — Performance Beta** | +3 mois | P1, P2, P4, M1, M4, F1 (DASH), F5, S14, S15, S16, S17 en place. Benchmarks publies. |
| **v0.5.0 — Cloud Ready** | +6 mois | P5, M2, M3, M5, F2, F3, F4, C1, C2, G2 completes. Helm chart stable. Grafana dashboard public. |
| **v1.0.0 — Production** | +12 mois | Tous les items P0-P2 des 3 horizons realises. Pentest passe. SLA defini. Documentation complete. |

---

## 6. Indicateurs de succes (KPIs)

| KPI | Baseline | Cible v0.3 | Cible v0.5 | Cible v1.0 |
|-----|----------|------------|------------|------------|
| Couverture tests | ~?% (inconnu) | 70% | 80% | 85% |
| gosec findings | 0-0-0 (core) | 0-0-0 (all) | 0-0-0 | 0-0-0 |
| CVE non resolus | 1 (x/net) | 0 | 0 | 0 |
| Latence API p95 | Inconnue | < 300ms | < 200ms | < 100ms |
| Latence HLS startup | Inconnue | < 3s | < 2s | < 1.5s |
| Utilisateurs simultanes | 1-2 | 10 | 50 | 200+ |
| Temps de deploiement | Manuel | < 5 min (Docker) | < 2 min (Helm) | < 1 min (GitOps) |
| Uptime cible | N/A | 99% | 99.5% | 99.9% |

---

## 7. Risques et mitigations

| Risque | Probabilite | Impact | Mitigation |
|--------|-------------|--------|------------|
| Complexite FFmpeg / hardware accel | Moyenne | Haute | Tests matrice CI (VAAPI, NVENC, QSV). Fallback software garanti. |
| Dette technique securite non resolue | Faible | Critique | Gate P0 bloquant avant tout merge sur `main`. Audit mensuel. |
| Performance SQLite a l'echelle | Moyenne | Haute | PostgreSQL en option des v0.4. Benchmarks reguliers. |
| Fatigue developpement (49 packages) | Moyenne | Moyenne | Priorisation stricte P0-P1-P2. Refactoring guide par les benchmarks. |
| Dependances Go vulnerables | Moyenne | Haute | `govulncheck` en CI, miroir interne si necessaire. |

---

## 8. Comment contribuer

1. **P0 items** sont ouverts aux contributions externes — voir les issues taggees `security/p0`.
2. **Benchmarks** — Rejoindre le canal `#perf` pour partager des profils pprof.
3. **Documentation** — Toute amelioration de `docs/` est la bienvenue (meme typo).
4. **Plugins** — Soumettre des propositions d'API dans `docs/adr/`.

---

*Roadmap genere le 2026-05-10 par Hermes (SwarmForge) sur la base de :*
- *Security Audit Report (4 HIGH, 7 MEDIUM, 12 LOW)*
- *Health Check Report (score 3.7/10 -> progression vers 6.0+)*
- *50 packages Go, 50/50 tests passants, gosec 0-0-0 (core)*
- *Changelog v0.1.0 -> v0.2.0*

*Prochaine revision du roadmap : apres la sortie de v0.2.1 (securite patch).*
