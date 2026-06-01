# Changelog

All notable changes to Polyant will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [2.1.0] - 2026-06-01

### Added

#### 🤖 Agent Integration (Multi-Agent Support)

**Major update: Polyant now supports seamless access from mainstream AI agents.**

- **Shared SDK (`pkg/polysdk`)**: New Go SDK encapsulating Polyant API calls
  - Search, create, update, delete, rate operations
  - Ed25519 authentication support
  - Configuration loading and client factory

- **agentskills.io Standard Skills**: Standardized skills for Codex CLI and Hermes Agent
  - `polyant-search`: Search knowledge base for solutions and best practices
  - `polyant-save`: Save knowledge and experience to the knowledge base
  - `polyant-learn`: Learn from the knowledge base to improve skills
  - `polyant-rate`: Rate and review knowledge entries
  - `polyant-config`: Configure connection settings

- **OpenClaw-Specific Skills**: Adapted skills for OpenClaw format
  - Simplified Markdown format
  - Chinese-friendly trigger conditions
  - Direct pactl CLI integration

- **MCP Server (`polyant-mcp-server`)**: Universal integration layer for MCP-compatible agents
  - `polyant_search`: Search knowledge base
  - `polyant_create`: Create knowledge entries
  - `polyant_rate`: Rate knowledge entries

- **Unified Install Script**: Auto-detects installed agents and installs corresponding skills
  - Supports Claude Code, Codex, Hermes, OpenClaw
  - One-command installation for all skills

#### Supported Agents

| Agent | Integration | Status |
|-------|-------------|--------|
| Claude Code | Skills | ✅ Supported |
| Codex CLI | agentskills.io | ✅ Supported |
| Hermes Agent | agentskills.io | ✅ Supported |
| OpenClaw | Custom Skills | ✅ Supported |
| Other MCP Agents | MCP Server | ✅ Supported |

### Changed

#### Documentation Updates

- Updated SKILL.md with multi-agent integration guide
- Updated README.md with bilingual (Chinese/English) agent integration documentation
- Updated landing page with agent integration showcase
- Updated all links to use www.polyant.top domain

### Technical Details

#### New Files

```
pkg/polysdk/                      # Shared SDK
├── client.go                     # HTTP client
├── client_test.go                # Client tests (9 test cases)
├── types.go                      # Data types
├── errors.go                     # Error types
└── config.go                     # Config loading

cmd/polyant-mcp-server/           # MCP Server
├── main.go
├── server.go
├── server_test.go                # Server tests (6 test cases)
└── config.go

skills/agentskills/               # agentskills.io standard skills
├── polyant-search/
├── polyant-save/
├── polyant-learn/
├── polyant-rate/
└── polyant-config/

skills/openclaw/                  # OpenClaw-specific skills
├── polyant-search.md
├── polyant-save.md
├── polyant-learn.md
├── polyant-rate.md
└── polyant-config.md

scripts/
└── install-unified.sh            # Unified install script
```

---

## [2.0.1] - 2026-04-16

### Changed

#### Architecture Refactoring
- **Program Separation**: Split into three independent programs:
  - `seed` - Seed node for bootstrap servers (requires domain + TLS certificate)
  - `user` - User node for AI agents (auto-detects network environment)
  - `pactl` - CLI management tool
- **Removed Deprecated Programs**: Removed `polyant` main program and `awctl` CLI tool
- **Simplified Distribution**: Each release package contains all three programs

#### Web Admin Dashboard
- Complete web-based admin dashboard implementation
- User management: list, ban/unban users
- Statistics dashboard: user stats, contribution stats, activity trends
- Content moderation: entry list, delete entries
- Local-only access at `http://127.0.0.1:8080/admin/`

### Fixed
- Updated Makefile to build all three programs correctly
- Fixed admin handler tests
- Documentation updates to reflect new program structure

### Migration Guide (v1.0.0 → v2.0.1)

**Old command:**
```bash
./polyant -config configs/seed.json
./awctl entry list
```

**New command:**
```bash
./seed -config configs/seed.json   # For seed nodes
./user -config configs/user.json   # For user nodes
./pactl entry list                 # CLI tool
```

**Download changes:**
- Old: `polyant-1.0.0-linux-amd64.tar.gz` (single program)
- New: `polyant-2.0.1-linux-amd64.tar.gz` (contains seed, user, pactl)

---

## [1.0.0] - 2026-04-13

### Added

#### Core Features
- P2P distributed knowledge network using libp2p
- Ed25519 signature-based authentication system
- Knowledge entry CRUD operations with content signing
- Multi-level category hierarchy support
- Weighted rating system for knowledge entries
- Email verification for user account upgrades
- Full-text search with Chinese word segmentation (jieba)

#### Network & Sync
- DHT-based peer discovery
- mDNS local network discovery
- AWSP (Polyant Sync Protocol) for data synchronization
- Version vector-based conflict resolution
- Incremental sync with mirror support
- Push notification for entries and ratings

#### Storage (Phase 6a)
- **Pebble KV 存储** - CockroachDB 团队开发的高性能嵌入式存储
- **Bleve 全文索引** - 纯 Go 实现的持久化搜索引擎
- 数据持久化支持，重启不丢失
- 支持中英文混合搜索
- In-memory storage for testing

#### User System
- Multi-level user roles (Lv0-Lv3)
- RBAC (Role-Based Access Control)
- Public key-based identity
- Email verification flow

#### API & Interface
- RESTful API with JSON responses
- CORS support for web clients
- Rate limiting middleware
- Request signing authentication
- CLI management tool (awctl)
- System service support (daemon mode)

#### Documentation
- API documentation
- Deployment guide
- Architecture design document

### Technical Details

#### Dependencies
- go-libp2p for P2P networking
- Pebble (CockroachDB) for embedded KV storage
- Bleve for full-text search
- gojieba for Chinese text processing
- testify for testing

#### Test Coverage
- Total coverage: **62.3%**
- protocol module: 76.5%
- daemon module: 81.6%
- storage/index: 83.4%
- auth: 90.6%

### Security
- Ed25519 digital signatures for content integrity
- Request signing for API authentication
- Configurable rate limiting
- Input validation and sanitization

### Platform Support
- Linux (amd64, arm64)
- macOS (amd64, arm64)
- Windows (amd64)

---

## [0.1.0-dev] - 2026-03-XX

### Added
- Initial development version
- Basic project structure
- Core storage layer
- Simple API endpoints

---

## Release Notes

### v1.0.0

This is the first stable release of Polyant. The system provides a complete P2P knowledge management platform with the following highlights:

1. **Decentralized Architecture**: No central server required, all nodes are equal peers
2. **Content Integrity**: All entries are signed with Ed25519 keys
3. **Conflict Resolution**: Version vectors ensure data consistency across nodes
4. **Multi-language Support**: Chinese and English full-text search
5. **Extensible Design**: Plugin-based skill system for AI agent integration

### Upgrade from Development Version

If you were using the development version (0.1.0-dev), please:

1. Backup your data directory
2. Update the binary
3. Run with `-init-seed` if you want to reset seed data
4. The new version will migrate data automatically

### Known Issues

- P2P connection may be slow behind NAT (use relay or port forwarding)
- Large binary size due to CGO dependencies for Chinese segmentation
- Windows service support is limited

### Future Plans

- Web UI for knowledge management
- GraphQL API support
- IPFS integration for large file storage
- Mobile SDK
- More language support for search
