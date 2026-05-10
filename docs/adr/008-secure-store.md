# ADR-008: Secure Store with AES-256-GCM

## Status

Accepted

## Context

AetherStream needs to store sensitive configuration values (API keys, passwords, webhook secrets) securely. Plain-text storage in SQLite or environment variables is insufficient for production deployments.

## Decision

Implement a **secure store** in `pkg/securestore` using **AES-256-GCM** encryption with a master key derived from `AETHERSTREAM_MASTER_KEY`.

## Rationale

- AES-256-GCM provides authenticated encryption (confidentiality + integrity).
- Go's `crypto/aes` and `crypto/cipher` packages are standard library, no external dependencies.
- Master key is provided at runtime via environment variable; never stored in the database.
- Encrypted values are stored in SQLite alongside regular config, transparently decrypted on read.

## Consequences

- Positive: Secrets are encrypted at rest. Database theft does not expose API keys.
- Positive: Transparent API — callers use `securestore.Get(key)` and `securestore.Set(key, value)` without managing crypto.
- Negative: If `AETHERSTREAM_MASTER_KEY` is lost, encrypted values are unrecoverable.
- Negative: Master key rotation requires re-encrypting all stored secrets.

## Alternatives Considered

- **HashiCorp Vault** — rejected to avoid external infrastructure dependency.
- **NaCl/libsodium** — considered but rejected to stay within Go standard library.
- **RSA hybrid encryption** — rejected; AES-256-GCM is sufficient for symmetric key use case.
