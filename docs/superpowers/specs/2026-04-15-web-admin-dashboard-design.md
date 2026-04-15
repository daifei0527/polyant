# Web 管理页面设计文档

> **创建日期**: 2026-04-15
> **状态**: 设计完成，待实现

## 概述

为 Polyant 添加 Web 管理页面和 CLI 管理工具，实现对用户、内容、数据的可视化管理。

### 设计目标

- **安全性优先**: 管理页面仅限本地访问，使用 Ed25519 认证
- **功能完整**: Web 页面和 CLI 工具功能对等
- **易部署**: 前端嵌入二进制，单文件分发
- **权限分级**: Lv0-Lv5 不同权限访问不同功能

### 技术选型

| 模块 | 技术 | 说明 |
|------|------|------|
| 前端框架 | Vue 3 + Vite + Element Plus | 渐进式加载，模块化开发 |
| 后端 API | Go + embed.FS | 静态文件嵌入，单二进制部署 |
| 认证方式 | Ed25519 → Session Token | 仅本地访问，安全性最高 |
| CLI 工具 | pactl | 原 awctl 更名，完整管理功能 |
| 部署方式 | 静态文件嵌入 | 单文件分发 |

---

## 1. 访问架构

### 1.1 安全设计

管理页面**仅限本地访问**，绑定 `127.0.0.1`，外部 IP 无法访问。

```
┌─────────────────────────────────────────────────────────────┐
│                         管理员                               │
├─────────────────────────────────────────────────────────────┤
│                                                              │
│   ┌─────────────────┐         ┌─────────────────┐           │
│   │  本地浏览器      │         │  远程终端        │           │
│   │  127.0.0.1:端口 │         │  SSH/远程连接    │           │
│   └────────┬────────┘         └────────┬────────┘           │
│            │                           │                     │
│            ▼                           ▼                     │
│   ┌─────────────────┐         ┌─────────────────┐           │
│   │  Web 管理页面    │         │  pactl CLI      │           │
│   │  (Vue 3)        │         │  (完整管理命令)  │           │
│   └────────┬────────┘         └────────┬────────┘           │
│            │                           │                     │
│            └───────────┬───────────────┘                     │
│                        ▼                                     │
│            ┌─────────────────────┐                           │
│            │  Admin API          │                           │
│            │  仅监听 127.0.0.1   │                           │
│            │  Ed25519 认证       │                           │
│            └─────────────────────┘                           │
└─────────────────────────────────────────────────────────────┘
```

### 1.2 安全措施

| 措施 | 说明 |
|------|------|
| **IP 绑定** | Admin API 仅监听 `127.0.0.1` |
| **Token 有效期** | 24 小时自动过期，页面关闭即失效 |
| **请求来源验证** | 检查 `Referer` / `Origin` 头 |
| **操作审计** | 所有管理操作记录审计日志 |
| **敏感操作确认** | 删除、封禁等操作需二次确认 |

---

## 2. 认证流程

### 2.1 Web 管理页面认证

```
┌─────────────────────────────────────────────────────────────────┐
│  1. 前端请求 /api/v1/admin/session/create                        │
│     - 后端读取 ~/.polyant/keys/private_key                       │
│     - 生成临时 Session Token (有效期 24h)                        │
│     - 返回 Token 给前端                                          │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│  2. 前端存储 Token (SessionStorage)                              │
│     - 后续请求携带 Authorization: Bearer <token>                 │
│     - 页面关闭后 Token 失效                                       │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│  3. 后端验证 Token + 权限等级                                     │
│     - 解析 Token 获取用户公钥                                     │
│     - 查询用户等级 (Lv0-Lv5)                                      │
│     - 根据路由要求检查权限                                        │
└─────────────────────────────────────────────────────────────────┘
```

### 2.2 CLI 认证

CLI 继续使用现有的 Ed25519 签名认证，支持远程服务器操作。

---

## 3. 权限等级

| 等级 | 角色 | 可访问功能 |
|------|------|------------|
| **Lv0** | 基础用户 | 个人中心（只读） |
| **Lv1** | 认证用户 | 个人中心、贡献内容 |
| **Lv2** | 活跃用户 | + 创建分类 |
| **Lv3** | 高级用户 | + 投票选举 |
| **Lv4** | 管理员 | + 用户管理、内容审核、数据统计 |
| **Lv5** | 超级管理员 | + 审计日志、选举管理、数据导入导出 |

---

## 4. 功能模块

### 4.1 用户管理 (Lv4+)

**页面**: `/admin/users`

**功能**:
- 用户列表（搜索、过滤、分页）
- 用户详情
- 封禁/解封用户
- 设置用户等级 (Lv5)

**API**:

| 功能 | 方法 | 路径 | 权限 |
|------|------|------|------|
| 用户列表 | GET | `/api/v1/admin/users` | Lv4+ |
| 用户详情 | GET | `/api/v1/admin/users/{pk}` | Lv4+ |
| 封禁用户 | POST | `/api/v1/admin/users/{pk}/ban` | Lv4+ |
| 解封用户 | POST | `/api/v1/admin/users/{pk}/unban` | Lv4+ |
| 设置等级 | PUT | `/api/v1/admin/users/{pk}/level` | Lv5 |

### 4.2 内容审核 (Lv4+)

**页面**: `/admin/entries`

**功能**:
- 条目列表（搜索、过滤、分页）
- 条目详情
- 删除条目

**API**:

| 功能 | 方法 | 路径 | 权限 |
|------|------|------|------|
| 条目列表 | GET | `/api/v1/admin/entries` | Lv4+ |
| 条目详情 | GET | `/api/v1/entry/{id}` | Lv4+ |
| 删除条目 | DELETE | `/api/v1/admin/entries/{id}` | Lv4+ |

### 4.3 数据统计 (Lv4+)

**页面**: `/admin/stats`

**功能**:
- 用户统计（总数、等级分布、活跃用户）
- 贡献统计（条目数、评分数、排名）
- 活跃趋势（近 7/30 天 DAU、注册趋势）

**API**:

| 功能 | 方法 | 路径 | 权限 |
|------|------|------|------|
| 用户统计 | GET | `/api/v1/admin/stats/users` | Lv4+ |
| 贡献统计 | GET | `/api/v1/admin/stats/contributions` | Lv4+ |
| 活跃趋势 | GET | `/api/v1/admin/stats/activity` | Lv4+ |

---

## 5. CLI 命令 (pactl)

### 5.1 命令结构

```bash
pactl
├── admin/                    # 管理命令组
│   ├── users/
│   │   ├── list             # 用户列表
│   │   ├── get <pk>         # 用户详情
│   │   ├── ban <pk> [--reason]      # 封禁用户
│   │   ├── unban <pk>       # 解封用户
│   │   └── level <pk> <level> [--reason]  # 设置等级
│   ├── entries/
│   │   ├── list             # 条目列表
│   │   ├── get <id>         # 条目详情
│   │   └── delete <id>      # 删除条目
│   ├── stats/
│   │   ├── users            # 用户统计
│   │   ├── contributions    # 贡献统计
│   │   └── activity [--days 7]  # 活跃趋势
│   └── status               # 系统状态
├── user/                     # 用户命令 (现有)
├── entry/                    # 条目命令 (现有)
├── sync/                     # 同步命令 (现有)
└── config/                   # 配置命令 (现有)
```

### 5.2 命令示例

```bash
# 用户管理
pactl admin users list --level 1 --limit 20
pactl admin users ban abc123... --reason "违规操作"
pactl admin users level abc123... 2 --reason "贡献达标"

# 内容审核
pactl admin entries list --category tech
pactl admin entries delete entry-123

# 数据统计
pactl admin stats users
pactl admin stats activity --days 30

# 系统状态
pactl admin status
```

### 5.3 输出格式

```bash
# 表格格式 (默认)
$ pactl admin users list
公钥              名称     等级  状态   贡献数
abc123def456...   User1    Lv2   正常   15
def456ghi789...   User2    Lv1   封禁   3

# JSON 格式 (脚本友好)
$ pactl admin users list --json
{"users": [...], "total": 100}
```

---

## 6. 前端实现

### 6.1 项目结构

```
web/admin/
├── public/
│   └── favicon.ico
├── src/
│   ├── main.js                 # 入口
│   ├── App.vue                 # 根组件
│   ├── router/
│   │   └── index.js            # 路由配置 (懒加载)
│   ├── stores/
│   │   ├── admin.js            # 用户/权限状态
│   │   └── app.js              # 应用状态
│   ├── api/
│   │   ├── request.js          # Axios 封装
│   │   ├── session.js          # 会话 API
│   │   ├── users.js            # 用户 API
│   │   ├── entries.js          # 条目 API
│   │   └── stats.js            # 统计 API
│   ├── views/
│   │   ├── Login.vue           # 登录页
│   │   ├── Layout.vue          # 管理布局
│   │   ├── users/
│   │   │   ├── List.vue        # 用户列表
│   │   │   └── Detail.vue      # 用户详情
│   │   ├── entries/
│   │   │   ├── List.vue        # 条目列表
│   │   │   └── Detail.vue      # 条目详情
│   │   └── stats/
│   │       ├── Index.vue       # 统计首页
│   │       └── components/     # 图表组件
│   ├── components/
│   │   ├── PermissionGuard.vue # 权限守卫
│   │   ├── Sidebar.vue         # 侧边栏
│   │   ├── Header.vue          # 顶栏
│   │   └── common/             # 公共组件
│   ├── utils/
│   │   ├── permission.js       # 权限工具
│   │   └── format.js           # 格式化工具
│   └── styles/
│       ├── index.scss          # 全局样式
│       └── variables.scss      # 变量定义
├── index.html
├── vite.config.js
├── package.json
└── .env
```

### 6.2 路由配置

```js
// router/index.js
const routes = [
  { path: '/login', component: () => import('@/views/Login.vue') },
  {
    path: '/',
    component: () => import('@/views/Layout.vue'),
    children: [
      {
        path: 'users',
        component: () => import('@/views/users/List.vue'),
        meta: { permission: 4 }  // Lv4+
      },
      {
        path: 'entries',
        component: () => import('@/views/entries/List.vue'),
        meta: { permission: 4 }
      },
      {
        path: 'stats',
        component: () => import('@/views/stats/Index.vue'),
        meta: { permission: 4 }
      }
    ]
  }
]
```

### 6.3 权限守卫组件

```vue
<!-- components/PermissionGuard.vue -->
<template>
  <div v-if="hasPermission">
    <slot />
  </div>
  <div v-else class="no-permission">
    <el-empty description="无访问权限" />
  </div>
</template>

<script setup>
import { computed } from 'vue'
import { useAdminStore } from '@/stores/admin'

const props = defineProps(['level'])
const adminStore = useAdminStore()
const hasPermission = computed(() => adminStore.userLevel >= props.level)
</script>
```

---

## 7. 后端实现

### 7.1 文件结构

```
internal/api/
├── admin/
│   ├── handler.go          # Admin 处理器
│   ├── session.go          # 会话管理
│   ├── middleware.go       # 权限中间件
│   └── static.go           # 静态文件服务 (embed.FS)
├── router/
│   └── router.go           # 注册 admin 路由
└── handler/
    └── admin_handler.go    # 现有管理员 API (保持兼容)

cmd/
├── polyant/                # 主服务 (嵌入管理页面)
└── pactl/                  # CLI 工具 (awctl 更名)

web/admin/                  # Vue 前端项目
├── dist/                   # 构建产物 (嵌入)
└── src/                    # 源码
```

### 7.2 Admin API 路由

```
/api/v1/admin/
├── session/
│   └── POST   /create              # 创建会话（仅 127.0.0.1）
├── users/
│   ├── GET    /                    # 用户列表
│   ├── GET    /:public_key         # 用户详情
│   ├── POST   /:public_key/ban     # 封禁用户
│   ├── POST   /:public_key/unban   # 解封用户
│   └── PUT    /:public_key/level   # 设置等级 (Lv5)
├── entries/
│   ├── GET    /                    # 条目列表
│   └── DELETE /:id                 # 删除条目
└── stats/
    ├── GET    /users               # 用户统计
    ├── GET    /contributions       # 贡献统计
    └── GET    /activity            # 活跃趋势
```

### 7.3 静态文件嵌入

```go
// internal/api/admin/static.go

//go:embed dist
var adminFS embed.FS

// AdminStaticHandler 静态文件服务
func AdminStaticHandler() http.Handler {
    return http.FileServer(http.FS(adminFS))
}
```

---

## 8. 构建与部署

### 8.1 构建流程

```
┌─────────────────────────────────────────────────────────────┐
│                     开发环境                                 │
├─────────────────────────────────────────────────────────────┤
│  make build-admin                                           │
│    1. cd web/admin && npm install && npm run build          │
│    2. 生成 dist/ 目录                                        │
│    3. go build -tags embed_admin ./cmd/polyant              │
└─────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────┐
│                     生产二进制                               │
├─────────────────────────────────────────────────────────────┤
│  polyant (单个二进制)                                        │
│    ├── /api/v1/*           → API 服务                       │
│    ├── /admin/*            → 管理页面 (嵌入)                │
│    ├── /admin/static/*     → 静态资源 (嵌入)                │
│    └── /                   → 首页 (现有)                    │
└─────────────────────────────────────────────────────────────┘
```

### 8.2 Makefile 扩展

```makefile
# 管理页面构建
.PHONY: build-admin
build-admin:
	cd web/admin && npm install && npm run build

# 完整构建 (包含管理页面)
.PHONY: build-full
build-full: build-admin
	go build -tags embed_admin -o bin/polyant ./cmd/polyant
	go build -o bin/pactl ./cmd/pactl

# 仅构建核心 (不含管理页面)
.PHONY: build
build:
	go build -o bin/polyant ./cmd/polyant
	go build -o bin/pactl ./cmd/pactl
```

### 8.3 配置扩展

```json
// configs/config.json
{
  "admin": {
    "enabled": true,
    "listen": "127.0.0.1:18531"
  }
}
```

---

## 9. 实现计划

### Phase 1: 基础设施

1. 创建 `web/admin/` Vue 项目
2. 配置 Vite + Element Plus
3. 实现静态文件嵌入
4. 实现会话管理 API

### Phase 2: CLI 工具

1. 重命名 `awctl` → `pactl`
2. 添加 `admin` 命令组
3. 实现用户管理命令
4. 实现内容审核命令
5. 实现数据统计命令

### Phase 3: Web 管理页面

1. 实现登录页面
2. 实现管理布局框架
3. 实现用户管理页面
4. 实现内容审核页面
5. 实现数据统计页面

### Phase 4: 测试与文档

1. 单元测试
2. 集成测试
3. 更新 README 和 API 文档
