# AgentWiki 部署指南

## 系统要求

### 硬件要求

| 类型 | 最低配置 | 推荐配置 |
|------|----------|----------|
| CPU | 1 核 | 2 核+ |
| 内存 | 50 MB | 200 MB+ |
| 磁盘 | 5 MB | 根据数据量增长 |

### 软件要求

- **操作系统**: Linux, macOS, Windows
- **Go 版本**: 1.22+ （仅源码编译需要）
- **依赖**: 无外部数据库依赖（使用嵌入式 BadgerDB）

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

### 环境变量

配置可通过环境变量覆盖，前缀为 `AGENTWIKI_`：

```bash
export AGENTWIKI_NODE_TYPE=seed
export AGENTWIKI_NETWORK_API_PORT=8080
export AGENTWIKI_SYNC_AUTO_SYNC=true
```

## 运行模式

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

种子节点启动时会自动初始化种子数据：

```bash
./bin/agentwiki -config seed-config.json -init-seed
```

## CLI 工具 (awctl)

`awctl` 是 AgentWiki 的命令行管理工具。

### 常用命令

```bash
# 初始化配置
./bin/awctl init

# 查看节点状态
./bin/awctl status

# 安装为系统服务
./bin/awctl service install

# 启动服务
./bin/awctl service start

# 停止服务
./bin/awctl service stop

# 查看服务状态
./bin/awctl service status

# 卸载服务
./bin/awctl service uninstall
```

## 数据管理

### 数据目录结构

```
~/.agentwiki/
├── config.json       # 配置文件
├── data/             # 数据目录
│   ├── entries/      # 条目数据
│   ├── users/        # 用户数据
│   ├── ratings/      # 评分数据
│   └── index/        # 搜索索引
├── logs/             # 日志目录
│   └── agentwiki.log
└── keys/             # 密钥目录
    └── private.key
```

### 数据备份

```bash
# 备份数据目录
tar -czf agentwiki-backup-$(date +%Y%m%d).tar.gz ~/.agentwiki/data

# 恢复数据
tar -xzf agentwiki-backup-20240101.tar.gz -C ~/
```

### 数据迁移

1. 停止服务
2. 复制数据目录到新位置
3. 更新配置文件中的 `data_dir`
4. 启动服务

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

**Q: 节点无法连接到网络**

检查：
- 网络连接是否正常
- 防火墙是否开放端口
- 种子节点地址是否正确

**Q: 同步失败**

检查：
- 日志中的错误信息
- 磁盘空间是否充足
- 分类配置是否正确

**Q: API 无响应**

检查：
- API 端口是否被占用
- 服务是否正常运行
- 日志中是否有错误

### 日志分析

```bash
# 搜索错误日志
grep -i error ~/.agentwiki/logs/agentwiki.log

# 查看最近的日志
tail -100 ~/.agentwiki/logs/agentwiki.log
```
