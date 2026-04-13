# Changelog

All notable changes to AgentWiki will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

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
- AWSP (AgentWiki Sync Protocol) for data synchronization
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

This is the first stable release of AgentWiki. The system provides a complete P2P knowledge management platform with the following highlights:

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
