# AgentWiki 部署指南

## 快速开始

```bash
# 1. 克隆并构建
git clone https://github.com/daifei0527/agentwiki.git
cd agentwiki
make build

# 2. 启动节点（使用默认配置）
./bin/agentwiki

# 3. 验证服务运行
curl http://localhost:18531/api/v1/node/status
```

## 系统要求

### 硬件要求

| 类型 | 最低配置 | 推荐配置 |
|------|----------|----------|
| CPU | 1 核 | 2 核+ |
| 内存 | 100 MB | 512 MB+ |
| 磁盘 | 100 MB | 根据数据量增长（建议 SSD） |

> **注意**: 种子节点建议使用推荐配置，以支持更多并发连接和数据同步。

### 软件要求

- **操作系统**: Linux, macOS, Windows
- **Go 版本**: 1.22+ （仅源码编译需要）
- **依赖**: 无外部数据库依赖
  - 内置 Pebble KV 存储（默认）或 BadgerDB
  - 内置 Bleve 全文搜索引擎
  - 中文分词依赖 gojieba（需要 CGO）

> **CGO 依赖说明**:
> 项目使用 gojieba 进行中文分词，需要 CGO 支持。如果编译环境没有 CGO，会使用 stub 实现（分词功能降级）。在生产环境建议安装 CGO 工具链以获得完整的中文搜索支持。
>
> ```bash
> # Ubuntu/Debian
> sudo apt-get install build-essential
>
> # CentOS/RHEL
> sudo yum groupinstall "Development Tools"
>
> # macOS (Xcode Command Line Tools)
> xcode-select --install
> ```

## 安装方式

### 方式一：下载预编译二进制

从 [Releases](https://github.com/daifei0527/agentwiki/releases) 页面下载对应平台的二进制文件。

```bash
# Linux
wget https://github.com/daifei0527/agentwiki/releases/download/v1.0.0/agentwiki-linux-amd64
chmod +x agentwiki-linux-amd64
mv agentwiki-linux-amd64 /usr/local/bin/agentwiki

# macOS
wget https://github.com/daifei0527/agentwiki/releases/download/v1.0.0/agentwiki-darwin-amd64
chmod +x agentwiki-darwin-amd64
mv agentwiki-darwin-amd64 /usr/local/bin/agentwiki

# Windows
# 下载 agentwiki-windows-amd64.exe
```

### 方式二：从源码构建

```bash
# 克隆仓库
git clone https://github.com/daifei0527/agentwiki.git
cd agentwiki

# 安装依赖
go mod download

# 编译
make build

# 编译结果在 bin/ 目录下
ls bin/
# agentwiki  awctl
```

### 方式三：交叉编译多平台

```bash
# 编译所有平台版本
make cross-compile

# 编译特定平台
make build-linux     # Linux
make build-darwin    # macOS
make build-windows   # Windows
```

## 配置

### 配置文件结构

创建配置文件 `~/.agentwiki/config.json`：

```json
{
  "node": {
    "type": "local",
    "name": "my-node",
    "data_dir": "~/.agentwiki/data",
    "log_dir": "~/.agentwiki/logs",
    "log_level": "info"
  },
  "network": {
    "listen_port": 18530,
    "api_port": 18531,
    "seed_nodes": [],
    "dht_enabled": true,
    "mdns_enabled": true
  },
  "sync": {
    "auto_sync": true,
    "interval_seconds": 300,
    "mirror_categories": ["tech"],
    "max_local_size_mb": 1024
  },
  "sharing": {
    "allow_mirror": true,
    "bandwidth_limit_mb": 100
  },
  "user": {
    "private_key_path": "~/.agentwiki/keys",
    "auto_register": true
  },
  "smtp": {
    "enabled": false
  },
  "api": {
    "enabled": true,
    "cors": true
  }
}
```

### 配置说明

#### 节点配置 (node)

| 字段 | 类型 | 说明 |
|------|------|------|
| type | string | 节点类型：`seed`（种子节点）、`local`（本地节点） |
| name | string | 节点名称，用于标识 |
| data_dir | string | 数据存储目录 |
| log_dir | string | 日志目录 |
| log_level | string | 日志级别：`debug`, `info`, `warn`, `error` |

#### 网络配置 (network)

| 字段 | 类型 | 说明 |
|------|------|------|
| listen_port | int | P2P 监听端口 |
| api_port | int | HTTP API 端口 |
| seed_nodes | []string | 种子节点地址列表 |
| dht_enabled | bool | 是否启用 DHT 发现 |
| mdns_enabled | bool | 是否启用本地 mDNS 发现 |

#### 同步配置 (sync)

| 字段 | 类型 | 说明 |
|------|------|------|
| auto_sync | bool | 是否自动同步 |
| interval_seconds | int | 同步间隔（秒） |
| mirror_categories | []string | 镜像的分类列表 |
| max_local_size_mb | int | 本地存储上限（MB） |

#### 存储配置 (storage)

| 字段 | 类型 | 说明 |
|------|------|------|
| kv_type | string | KV 存储类型：`pebble`（默认）、`badger` |
| search_type | string | 搜索引擎类型：`bleve`（默认）、`memory` |

### 环境变量

配置可通过环境变量覆盖，前缀为 `AGENTWIKI_`：

```bash
# 节点配置
export AGENTWIKI_NODE_TYPE=seed
export AGENTWIKI_NODE_NAME=my-node
export AGENTWIKI_NODE_DATA_DIR=/data/agentwiki

# 网络配置
export AGENTWIKI_NETWORK_API_PORT=8080
export AGENTWIKI_NETWORK_LISTEN_PORT=18530
export AGENTWIKI_NETWORK_SEED_NODES=/ip4/192.168.1.1/tcp/18530/p2p/QmXXX

# 同步配置
export AGENTWIKI_SYNC_AUTO_SYNC=true
export AGENTWIKI_SYNC_INTERVAL_SECONDS=300

# 存储配置
export AGENTWIKI_STORAGE_KV_TYPE=pebble
export AGENTWIKI_STORAGE_SEARCH_TYPE=bleve
```

环境变量命名规则：`AGENTWIKI_<节>_<字段>`，使用下划线连接。例如：
- `AGENTWIKI_NODE_TYPE` → `node.type`
- `AGENTWIKI_NETWORK_API_PORT` → `network.api_port`

## 运行模式

### 命令行参数

```bash
./bin/agentwiki [选项]

选项:
  -config <file>      配置文件路径 (JSON 格式)
  -init-seed          初始化种子数据并退出
  -memory             使用内存存储（仅用于测试）
  -service            作为系统服务运行
  -version            显示版本信息
```

示例：

```bash
# 使用默认配置运行
./bin/agentwiki

# 指定配置文件
./bin/agentwiki -config /etc/agentwiki/config.json

# 初始化种子数据
./bin/agentwiki -init-seed -config seed-config.json

# 使用内存存储（测试用）
./bin/agentwiki -memory

# 作为系统服务运行
./bin/agentwiki -service
```

### 直接运行

```bash
# 使用默认配置
./bin/agentwiki

# 指定配置文件
./bin/agentwiki -config ~/.agentwiki/config.json

# 初始化种子数据
./bin/agentwiki -init-seed

# 显示帮助
./bin/agentwiki -help
```

### 系统服务 (Linux systemd)

创建服务文件 `/etc/systemd/system/agentwiki.service`：

```ini
[Unit]
Description=AgentWiki P2P Knowledge Node
After=network.target

[Service]
Type=simple
User=agentwiki
Group=agentwiki
WorkingDirectory=/opt/agentwiki
ExecStart=/opt/agentwiki/bin/agentwiki -config /opt/agentwiki/config.json
Restart=on-failure
RestartSec=5
LimitNOFILE=65536

[Install]
WantedBy=multi-user.target
```

启动服务：

```bash
# 创建用户
sudo useradd -r -s /bin/false agentwiki

# 创建目录
sudo mkdir -p /opt/agentwiki/{data,logs,keys}
sudo chown -R agentwiki:agentwiki /opt/agentwiki

# 安装服务
sudo cp agentwiki.service /etc/systemd/system/
sudo systemctl daemon-reload
sudo systemctl enable agentwiki
sudo systemctl start agentwiki

# 查看状态
sudo systemctl status agentwiki

# 查看日志
sudo journalctl -u agentwiki -f
```

### Docker 部署

创建 `Dockerfile`：

```dockerfile
FROM golang:1.22-alpine AS builder
WORKDIR /app
COPY . .
RUN go build -o agentwiki ./cmd/agentwiki

FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /app
COPY --from=builder /app/agentwiki .
COPY configs/default.json config.json
EXPOSE 18530 18531
CMD ["./agentwiki", "-config", "config.json"]
```

构建并运行：

```bash
# 构建镜像
docker build -t agentwiki:latest .

# 运行容器
docker run -d \
  --name agentwiki \
  -p 18530:18530 \
  -p 18531:18531 \
  -v agentwiki-data:/app/data \
  agentwiki:latest
```

### Docker Compose

创建 `docker-compose.yml`：

```yaml
version: '3.8'

services:
  agentwiki:
    image: agentwiki:latest
    ports:
      - "18530:18530"
      - "18531:18531"
    volumes:
      - ./data:/app/data
      - ./config.json:/app/config.json:ro
    restart: unless-stopped
    environment:
      - AGENTWIKI_NODE_NAME=agentwiki-docker
```

## 节点类型

AgentWiki 支持三种节点类型，适用于不同场景：

### 本地节点 (local)

标准用户节点，适合个人使用：

```json
{
  "node": {
    "type": "local",
    "name": "my-local-node"
  }
}
```

特点：
- 完整的 P2P 功能
- 可配置数据同步和镜像
- 适合个人知识库管理

### 种子节点 (seed)

用于网络引导和数据同步的公共节点：

```json
{
  "node": {
    "type": "seed",
    "name": "seed-node-1"
  },
  "network": {
    "listen_port": 18530,
    "dht_enabled": true
  },
  "sharing": {
    "allow_mirror": true,
    "bandwidth_limit_mb": 500
  }
}
```

特点：
- 提供 DHT 引导服务
- 初始数据源
- 高可用性要求
- 建议部署在公网服务器

种子节点启动时会自动初始化种子数据：

```bash
./bin/agentwiki -config seed-config.json -init-seed
```

## 种子节点部署指南

种子节点是 AgentWiki 网络的核心基础设施，负责：
- 提供 DHT 引导服务
- 初始数据分发
- 网络拓扑稳定

### 部署前准备

1. **服务器要求**：
   - 公网 IP 地址
   - 稳定的网络连接
   - 建议配置：2 核 CPU，1GB+ 内存，50GB+ SSD

2. **域名（可选）**：
   - 配置 DNS 解析
   - 便于节点发现和维护

### 部署步骤

1. **创建种子节点配置** `/opt/agentwiki/config.json`：

```json
{
  "node": {
    "type": "seed",
    "name": "seed-node-1",
    "data_dir": "/opt/agentwiki/data",
    "log_dir": "/opt/agentwiki/logs",
    "log_level": "info"
  },
  "network": {
    "listen_port": 18530,
    "api_port": 18531,
    "seed_nodes": [],
    "dht_enabled": true,
    "mdns_enabled": false
  },
  "storage": {
    "kv_type": "pebble",
    "search_type": "bleve"
  },
  "sync": {
    "auto_sync": true,
    "interval_seconds": 60
  },
  "sharing": {
    "allow_mirror": true,
    "bandwidth_limit_mb": 500,
    "max_concurrent": 50
  }
}
```

2. **创建系统服务用户**：

```bash
sudo useradd -r -s /bin/false agentwiki
sudo mkdir -p /opt/agentwiki/{data,logs,keys,seed-data}
sudo chown -R agentwiki:agentwiki /opt/agentwiki
```

3. **准备种子数据**：

```bash
# 将初始知识库数据放入 seed-data 目录
cp -r /path/to/seed-data/* /opt/agentwiki/seed-data/
chown -R agentwiki:agentwiki /opt/agentwiki/seed-data
```

4. **安装并启动服务**：

```bash
# 使用 awctl 安装服务
./bin/awctl service install \
  --name agentwiki \
  --config /opt/agentwiki/config.json

# 启动服务
./bin/awctl service start --name agentwiki

# 查看状态
./bin/awctl service status --name agentwiki
```

5. **获取节点地址**：

```bash
# 查看日志获取 Peer ID
journalctl -u agentwiki | grep "node_id"

# 完整的多地址格式
# /ip4/<公网IP>/tcp/18530/p2p/<PeerID>
```

### 多种子节点配置

部署多个种子节点提高可用性：

```json
{
  "network": {
    "seed_nodes": [
      "/ip4/seed1.example.com/tcp/18530/p2p/QmSeed1...",
      "/ip4/seed2.example.com/tcp/18530/p2p/QmSeed2..."
    ]
  }
}
```

### 监控与维护

```bash
# 检查服务状态
systemctl status agentwiki

# 查看实时日志
journalctl -u agentwiki -f

# 检查端口监听
netstat -tlnp | grep agentwiki

# 监控资源使用
top -p $(pgrep agentwiki)
```

### 高可用配置

建议部署至少 3 个种子节点：
- 不同地理位置
- 不同网络提供商
- 负载均衡或 DNS 轮询

### 用户节点 (user)

轻量级客户端节点，适合资源受限环境：

```json
{
  "node": {
    "type": "user",
    "name": "light-client"
  },
  "sync": {
    "auto_sync": false
  }
}
```

特点：
- 最小化资源占用
- 按需同步数据
- 适合嵌入式设备或移动端

## CLI 工具 (awctl)

`awctl` 是 AgentWiki 的命令行管理工具，用于管理知识库条目、用户、同步、镜像等功能。

### 全局选项

```bash
awctl [全局选项] <命令>

选项:
  -c, --config <file>    配置文件路径
  -d, --data <dir>       数据目录
  -s, --server <url>     API 服务器地址 (默认: http://localhost:8080)
  --key-dir <dir>        密钥目录 (默认: ~/.agentwiki/keys)
```

### 服务管理命令

```bash
# 安装为系统服务
awctl service install --name agentwiki --config /etc/agentwiki/config.json

# 启动服务
awctl service start --name agentwiki

# 停止服务
awctl service stop --name agentwiki

# 重启服务
awctl service restart --name agentwiki

# 查看服务状态
awctl service status --name agentwiki

# 查看服务日志
awctl service logs --name agentwiki -f --tail 100

# 卸载服务
awctl service uninstall --name agentwiki
```

### 密钥管理命令

```bash
# 生成新密钥对
awctl key generate

# 查看当前公钥
awctl key show
```

### 用户管理命令

```bash
# 注册新用户
awctl user register --name "my-agent" --email "agent@example.com"

# 查看用户信息
awctl user get <user-id>

# 列出所有用户
awctl user list
```

### 条目管理命令

```bash
# 搜索条目
awctl search "人工智能"

# 获取条目详情
awctl entry get <entry-id>

# 创建条目
awctl entry create --title "新条目" --category "tech" --content "内容..."

# 更新条目
awctl entry update <entry-id> --title "更新标题"

# 删除条目
awctl entry delete <entry-id>
```

### 同步命令

```bash
# 触发同步
awctl sync trigger

# 查看同步状态
awctl sync status
```

### 常用命令示例

```bash
# 查看节点状态
./bin/awctl status

# 搜索知识库
./bin/awctl -s http://localhost:18531 search "Go 语言"

# 连接远程服务器
./bin/awctl -s http://192.168.1.100:18531 status
```

## 数据管理

### 存储架构

AgentWiki 使用分层存储架构：

| 层级 | 组件 | 说明 |
|------|------|------|
| KV 存储 | Pebble（默认）/ BadgerDB | 存储条目、用户、评分等结构化数据 |
| 搜索引擎 | Bleve（默认） | 全文搜索索引，支持中文分词 |
| 文件系统 | 种子数据 | 初始种子数据文件 |

### 数据目录结构

```
~/.agentwiki/
├── config.json         # 配置文件
├── data/               # 数据目录
│   ├── kv/             # Pebble/BadgerDB KV 存储
│   ├── search.bleve/   # Bleve 搜索索引
│   └── seed-data/      # 种子数据（种子节点）
├── logs/               # 日志目录
│   └── agentwiki.log
└── keys/               # 密钥目录
    └── private.key     # Ed25519 私钥
```

### 存储配置

```json
{
  "storage": {
    "kv_type": "pebble",      // 或 "badger"
    "search_type": "bleve"    // 或 "memory"（仅测试用）
  }
}
```

Pebble vs BadgerDB：
- **Pebble**: 默认推荐，更小的内存占用，更好的写性能
- **BadgerDB**: 兼容选项，适合从旧版本迁移

### 数据备份

#### 方式一：文件系统备份

```bash
# 停止服务
systemctl stop agentwiki

# 备份数据目录
tar -czf agentwiki-backup-$(date +%Y%m%d-%H%M%S).tar.gz ~/.agentwiki/data

# 备份密钥（重要！）
cp -r ~/.agentwiki/keys ~/.agentwiki/keys-backup

# 重启服务
systemctl start agentwiki
```

#### 方式二：增量备份

```bash
# 使用 rsync 增量同步
rsync -av --delete ~/.agentwiki/data/ /backup/agentwiki-data/
```

#### 方式三：定时备份脚本

创建 `/etc/cron.daily/agentwiki-backup`：

```bash
#!/bin/bash
BACKUP_DIR="/backup/agentwiki"
DATE=$(date +%Y%m%d)
RETENTION_DAYS=7

# 创建备份
tar -czf ${BACKUP_DIR}/agentwiki-${DATE}.tar.gz ~/.agentwiki/data

# 清理旧备份
find ${BACKUP_DIR} -name "agentwiki-*.tar.gz" -mtime +${RETENTION_DAYS} -delete
```

### 数据恢复

```bash
# 停止服务
systemctl stop agentwiki

# 恢复数据
tar -xzf agentwiki-backup-20240101.tar.gz -C ~/

# 确保权限正确
chown -R agentwiki:agentwiki ~/.agentwiki

# 启动服务
systemctl start agentwiki
```

### 数据迁移

迁移到新服务器：

1. **源服务器**：
   ```bash
   # 停止服务
   systemctl stop agentwiki
   
   # 打包数据
   tar -czf agentwiki-migration.tar.gz ~/.agentwiki/data ~/.agentwiki/keys
   ```

2. **传输数据**：
   ```bash
   scp agentwiki-migration.tar.gz user@new-server:/tmp/
   ```

3. **目标服务器**：
   ```bash
   # 安装 AgentWiki
   # 创建目录
   mkdir -p ~/.agentwiki
   
   # 解压数据
   tar -xzf /tmp/agentwiki-migration.tar.gz -C ~/
   
   # 启动服务
   ./bin/agentwiki -config ~/.agentwiki/config.json
   ```

### 存储维护

#### 重建搜索索引

如果搜索出现问题，可以重建索引：

```bash
# 停止服务
systemctl stop agentwiki

# 删除旧索引
rm -rf ~/.agentwiki/data/search.bleve

# 启动服务（会自动重建索引）
systemctl start agentwiki
```

#### 压缩存储

Pebble 存储支持手动压缩：

```bash
# 通过 API 触发（如果实现了相关接口）
curl -X POST http://localhost:18531/api/v1/admin/compact
```

## 监控与日志

### 日志级别

- `debug`: 详细调试信息
- `info`: 常规运行信息（推荐）
- `warn`: 警告信息
- `error`: 仅错误信息

### 查看日志

```bash
# 实时查看日志
tail -f ~/.agentwiki/logs/agentwiki.log

# systemd 服务日志
journalctl -u agentwiki -f
```

### 健康检查

```bash
# 检查节点状态
curl http://localhost:18531/api/v1/node/status

# 检查 API 可用性
curl http://localhost:18531/api/v1/categories
```

## 网络配置

### 防火墙设置

需要开放以下端口：

```bash
# P2P 端口
sudo ufw allow 18530/tcp

# API 端口
sudo ufw allow 18531/tcp
```

### 多节点组网

1. 启动种子节点：

```bash
# 在 seed-node-1 上
./bin/agentwiki -config seed-config.json
```

2. 配置本地节点连接种子节点：

```json
{
  "network": {
    "seed_nodes": [
      "/ip4/seed-ip/tcp/18530/p2p/seed-peer-id"
    ]
  }
}
```

3. 启动本地节点：

```bash
./bin/agentwiki -config local-config.json
```

## 安全建议

1. **密钥保护**: 私钥文件权限设置为 600
   ```bash
   chmod 600 ~/.agentwiki/keys/private.key
   ```

2. **网络隔离**: API 端口不对外暴露，仅开放 P2P 端口

3. **定期备份**: 定期备份数据目录和密钥

4. **日志审计**: 定期检查日志文件

5. **更新维护**: 及时更新到最新版本

## 故障排除

### 常见问题

#### P2P 连接问题

**问题：节点无法连接到网络**

排查步骤：
1. 检查网络连接
   ```bash
   ping <seed-node-ip>
   ```

2. 检查防火墙
   ```bash
   sudo ufw status
   sudo ufw allow 18530/tcp
   ```

3. 验证种子节点地址格式
   ```bash
   # 正确格式
   /ip4/192.168.1.1/tcp/18530/p2p/QmXXX...
   ```

4. 检查 NAT 穿透
   - 确保路由器支持 UPnP
   - 或手动配置端口转发

**问题：DHT 发现失败**

解决方案：
- 确保至少连接到一个在线的种子节点
- 检查 `dht_enabled` 配置
- 等待 DHT 引导完成（可能需要几分钟）

#### 存储问题

**问题：数据库打开失败**

错误信息：`resource temporarily unavailable`

原因：另一个实例正在运行

解决方案：
```bash
# 检查是否有进程占用
lsof ~/.agentwiki/data/kv

# 终止旧进程
pkill -f agentwiki

# 或清理锁文件
rm -f ~/.agentwiki/data/kv/LOCK
```

**问题：磁盘空间不足**

```bash
# 检查磁盘使用
df -h ~/.agentwiki/data

# 清理日志
find ~/.agentwiki/logs -name "*.log" -mtime +30 -delete

# 限制存储大小
# 在配置中设置 max_local_size_mb
```

**问题：搜索索引损坏**

症状：搜索返回错误或空结果

解决方案：
```bash
# 重建搜索索引
rm -rf ~/.agentwiki/data/search.bleve
systemctl restart agentwiki
```

#### API 问题

**问题：API 无响应**

检查：
1. 端口是否被占用
   ```bash
   netstat -tlnp | grep 18531
   ```

2. 服务是否运行
   ```bash
   curl http://localhost:18531/api/v1/node/status
   ```

3. 查看日志
   ```bash
   journalctl -u agentwiki -n 50
   ```

**问题：认证失败**

错误：`invalid signature`

解决方案：
- 检查密钥文件是否存在
- 确认时间戳在有效范围内（5分钟内）
- 验证签名算法是否正确

#### 同步问题

**问题：同步失败**

检查：
- 日志中的错误信息
- 磁盘空间是否充足
- 分类配置是否正确

```bash
# 查看同步相关日志
grep -i sync ~/.agentwiki/logs/agentwiki.log
```

**问题：数据不一致**

解决方案：
1. 触发完整同步
2. 如需要，清除本地数据重新同步

### 日志分析

```bash
# 实时查看日志
tail -f ~/.agentwiki/logs/agentwiki.log

# systemd 服务日志
journalctl -u agentwiki -f

# 搜索错误日志
grep -i error ~/.agentwiki/logs/agentwiki.log

# 查看最近 100 行
tail -100 ~/.agentwiki/logs/agentwiki.log

# 按时间过滤
journalctl -u agentwiki --since "1 hour ago"
```

### 性能调优

#### 内存优化

```json
{
  "sync": {
    "max_local_size_mb": 512
  },
  "sharing": {
    "bandwidth_limit_mb": 50,
    "max_concurrent": 5
  }
}
```

#### 连接优化

```json
{
  "network": {
    "dht_enabled": true,
    "mdns_enabled": false
  }
}
```

- 禁用 mDNS 可减少局域网广播
- 调整 `max_concurrent` 控制并发连接

### 调试模式

启用详细日志：

```bash
# 方式一：配置文件
{
  "node": {
    "log_level": "debug"
  }
}

# 方式二：环境变量
export AGENTWIKI_NODE_LOG_LEVEL=debug
```

### 健康检查

```bash
# 检查节点状态
curl http://localhost:18531/api/v1/node/status

# 检查 API 可用性
curl http://localhost:18531/api/v1/categories

# 检查 P2P 连接
curl http://localhost:18531/api/v1/node/peers
```

### 常用诊断命令

```bash
# 查看进程状态
ps aux | grep agentwiki

# 查看端口监听
netstat -tlnp | grep agentwiki

# 查看系统资源
top -p $(pgrep agentwiki)

# 检查文件描述符
lsof -p $(pgrep agentwiki) | wc -l
```
