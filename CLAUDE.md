# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Build Commands

```bash
# Build all binaries
make build

# Run tests
make test

# Format and lint
make lint   # runs fmt + vet

# Run a single test
go test -v -race ./internal/storage/...

# Run seed node
./bin/seed -config configs/seed.json

# Run user node
./bin/user -config configs/user.json

# Run with seed data initialization
./bin/seed -init-seed
```

## Architecture Overview

Polyant is a distributed P2P knowledge system for AI agents. Key architectural decisions:

### Layered Structure

- **`cmd/`** - Entry points: `seed` (seed node), `user` (user node), `pactl` (CLI tool)
- **`internal/api/`** - HTTP layer: handlers, router, middleware
- **`internal/core/`** - Business logic: user, category, rating, email
- **`internal/network/`** - P2P layer: host, DHT, protocol, sync
- **`internal/storage/`** - Persistence: model, kv (BadgerDB), index (search)
- **`internal/auth/`** - Authentication: Ed25519 signatures, RBAC
- **`pkg/`** - Shared utilities: config, errors, logger, crypto

### Key Patterns

**Storage Interface Pattern** (`internal/storage/store.go`):
- All storage operations use interfaces (EntryStore, UserStore, etc.)
- Two implementations: `MemoryStore` (testing) and `BadgerStore` (production)
- Handlers depend on interfaces, not concrete implementations

**P2P Protocol** (`internal/network/protocol/`):
- AWSP (Polyant Sync Protocol) over libp2p streams
- Message types: Handshake, Query, SyncRequest, PushEntry, RatingPush
- Codec handles serialization (currently JSON, not protobuf)

**Authentication** (`internal/api/middleware/auth.go`):
- Ed25519 signature-based authentication
- Request signing: `METHOD + "\n" + PATH + "\n" + TIMESTAMP + "\n" + SHA256(BODY)`
- Headers: `X-Polyant-PublicKey`, `X-Polyant-Timestamp`, `X-Polyant-Signature`

**Entry Content Signing** (`internal/api/handler/entry_handler.go`):
- Entries have independent content signatures for sync verification
- Signature content: `SHA256(title + "\n" + content + "\n" + category)`

### Node Types

- `seed` - Bootstrap nodes with initial data, stay online
- `local` - Standard user nodes
- `user` - Lightweight client nodes

### Configuration

Config loaded from JSON + environment variables (prefix `POLYANT_`):
```bash
POLYANT_NODE_TYPE=seed
POLYANT_NETWORK_API_PORT=8080
```

### Chinese Text Support

The search index uses gojieba for Chinese word segmentation. A stub implementation exists for environments without CGO.
