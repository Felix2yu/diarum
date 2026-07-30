# Diarum 项目指令

## 项目概述

Diarum（吾身）是一个自托管的 AI 日记应用，核心理念是"一天一篇，打开即写，刚刚好"。名称源自《论语》"吾日三省吾身"。

- **定位**: 极简、私有、跨平台的每日日记工具
- **许可证**: Apache 2.0
- **版本**: v1.10+

## 技术栈

| 层级 | 技术 |
|------|------|
| 后端语言 | Go 1.26 |
| HTTP 框架 | Echo v5 |
| 数据库 | SQLite (modernc.org/sqlite, CGO-free, WAL mode) |
| 前端框架 | SvelteKit 2 + Svelte 5 (runes) |
| UI 样式 | Tailwind CSS 3 |
| 构建工具 | Vite 8 |
| AI 集成 | OpenAI-compatible API + chromem-go 向量数据库 (RAG) |
| MCP 服务器 | mcp-go (Model Context Protocol) |
| 部署 | Docker (多架构, GHCR) |

## 目录结构

```
diarum/
├── main.go                    # 应用入口 (serve, version 命令)
├── diarum.go                  # 版本/名称全局变量
├── go.mod / go.sum            # Go 依赖
├── Makefile                   # 构建自动化
├── internal/                  # Go 后端包
│   ├── api/                   # HTTP API 处理器 (19 个文件)
│   ├── auth/                  # 认证服务 (JWT)
│   ├── backup/                # 备份调度器
│   ├── chat/                  # AI 聊天服务
│   ├── config/                # 配置管理
│   ├── embedding/             # 向量数据库 + 嵌入服务 (RAG)
│   ├── logger/                # 自定义日志
│   ├── mcp/                   # MCP 服务器
│   ├── static/                # 嵌入的前端静态文件
│   ├── store/                 # SQLite 数据存储
│   └── weather/               # 天气服务 (Open-Meteo)
├── site/                      # 前端 (SvelteKit)
│   ├── src/
│   │   ├── lib/
│   │   │   ├── api/           # API 客户端函数
│   │   │   ├── components/    # Svelte 组件
│   │   │   ├── stores/        # Svelte stores (状态管理)
│   │   │   ├── types/         # TypeScript 类型定义
│   │   │   └── utils/         # 工具函数
│   │   └── routes/            # SvelteKit 页面 (文件路由)
│   └── build/                 # 编译后的前端输出
├── docs/                      # 设计文档 (中文)
└── .github/workflows/         # CI/CD
```

## 开发命令

### 构建

```bash
make build          # 构建前端和后端
make frontend       # 仅构建前端
make backend        # 仅构建后端 (依赖前端构建)
```

### 开发模式

需要两个终端分别运行：

```bash
make dev-frontend   # 前端开发服务器 (端口 5173)
make dev-backend    # 后端开发服务器 (使用 Air 热重载)
```

### 测试

```bash
make test           # 运行所有测试
make test-cover     # 运行测试并检查覆盖率 (阈值 93%)
```

### 其他

```bash
make clean          # 清理构建产物
make docker         # 构建 Docker 镜像
make version        # 显示当前版本
```

## 后端编码规范

### Go 风格

- 遵循标准 Go 风格 (gofmt)
- 使用 `internal/` 包组织私有代码
- 错误处理：使用 `errors.Is()` / `errors.As()` 进行错误比较
- 日志：使用 `internal/logger` 包 (Info, Error, Warn, Debug)

### API 设计

- RESTful API，前缀 `/api/v1/`
- 认证：JWT token (Bearer)
- 路由注册模式：`RegisterXxxRoutes(e *echo.Echo, s *store.Store, authMiddleware, ...)`
- 请求/响应使用 JSON
- 错误响应格式：`{"message": "...", "error": "..."}`

### 数据存储

- SQLite 数据库文件：`diarum.db` (WAL mode)
- Store 模式：`store.Store` 结构体封装所有数据库操作
- 嵌入式向量数据库：chromem-go 用于 RAG

## 前端编码规范

### Svelte/SvelteKit

- Svelte 5 (runes 响应式系统)
- SvelteKit 2 (文件路由)
- TypeScript 严格模式
- Tailwind CSS (utility-first)

### 状态管理

- Svelte stores (`writable`, `readable`, `derived`)
- 认证状态：`pb.authStore` (AuthStore 类)
- API 客户端：`site/src/lib/api/` 目录

### 组件规范

- 组件文件：`*.svelte`
- 工具函数：`site/src/lib/utils/`
- API 调用：`site/src/lib/api/`

## Git 工作流

### 分支命名

- `main` - 生产分支
- `release/**` - 发布分支
- `devel/**` / `dev/**` - 开发分支
- `feat/**` - 功能分支

### 提交规范

- 使用语义化提交信息
- 格式：`<type>(<scope>): <description>`
- 类型：feat, fix, docs, style, refactor, test, chore

### CI/CD

- GitHub Actions 自动运行测试
- 测试覆盖率上传到 Codecov
- Docker 多架构构建 (GHCR)

## 关键文件路径

### 后端

- 入口：`main.go`
- API 路由：`internal/api/*.go`
- 数据存储：`internal/store/store.go`
- 配置：`internal/config/`
- 认证：`internal/auth/`
- AI 功能：`internal/chat/`, `internal/embedding/`
- MCP 服务器：`internal/mcp/`

### 前端

- 页面路由：`site/src/routes/`
- API 客户端：`site/src/lib/api/`
- 组件：`site/src/lib/components/`
- 状态管理：`site/src/lib/stores/`
- 类型定义：`site/src/lib/types/`

## 注意事项

### 安全

- 不要在代码中硬编码密钥
- 使用环境变量或配置文件管理敏感信息
- JWT token 有过期时间
- API 需要认证 (除了公开路由)

### 性能

- 前端构建后会压缩 (zstd/brotli)
- SQLite 使用 WAL mode 提高并发性能
- 向量数据库使用增量构建

### 国际化

- 主要语言：中文
- 代码注释和文档使用中文
- API 响应使用英文 (或根据上下文)

## 常见任务

### 添加新的 API 端点

1. 在 `internal/api/` 创建或编辑相关文件
2. 使用 `RegisterXxxRoutes` 模式注册路由
3. 在 `main.go` 中调用注册函数
4. 添加相应的 store 方法 (如需要)
5. 编写测试

### 添加新的前端页面

1. 在 `site/src/routes/` 创建目录和 `+page.svelte`
2. 如需 API 调用，在 `site/src/lib/api/` 添加客户端函数
3. 如需新组件，在 `site/src/lib/components/` 创建

### 修改数据模型

1. 更新 `internal/store/store.go` 中的结构体和 SQL
2. 如有迁移需求，添加迁移逻辑
3. 更新相关的 API 处理器
4. 更新前端类型定义
