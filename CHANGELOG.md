# Changelog

## v1.1 - 2026-06-18

> 发布日期：2026-06-18
> 对应 commit：`4c9d6c1` — Merge pull request #14: 引入统一响应式布局系统并适配大屏

### ✨ 新增 Features

- **统一响应式布局系统** — 全面重写前端布局，使用统一的响应式设计系统，自动适配从手机到桌面大屏的各种设备尺寸
- **全界面汉化** — 完成所有界面中文化，日期、星期格式等细节符合中文用户习惯
- **API 新增修改与删除** — 日记数据 API 支持完整的 CRUD 操作
- **新增 favicon** — 应用图标/logo 完整配置，支持 PWA 添加到主屏幕
- **非 root 用户运行** — Docker 容器默认使用 UID: 1000 运行，更安全
- **AI 周期分析报告** — 配置 AI Key 后，支持对整周/整月日记自动生成分析报告
- **Markdown 渲染** — 日记正文与 AI 分析结果均支持标准 Markdown 渲染
- **RAG 向量对话** — 日记自动向量化，可基于历史日记与 AI 对话（今日复盘、周报、年终总结等）
- **Chevereto 图床集成** — 支持通过 Chevereto 图床托管图片，可在本地与 Chevereto 之间灵活切换
- **PWA 支持** — 可安装为桌面/移动应用，具备离线缓存与自动同步
- **Memos Webhook 同步** — 接收 Memos 的新增 / 更新 / 删除事件并回写到当天日记
- **SQLite 原生后端** — 内置用户体系、本地媒体存储与旧数据自动迁移
- **Docker 部署** — 提供官方镜像 `ghcr.io/felix2yu/diarum:latest` 与 docker-compose 示例
- **OpenAPI 接口** — 便于对接 n8n 等自动化工作流

### 🔧 技术实现

- 后端：Go 1.22+ + Echo
- 前端：Svelte + Vite + TypeScript + Tailwind CSS
- 数据库：SQLite（通过 PocketBase 抽象层）
- 单二进制发布，前端资源通过 `embed` 直接打入二进制

### 📁 关键目录 / 文件

- `diarum.go` — 应用入口与版本定义
- `main.go` — serve / version 等命令行处理
- `internal/api/` — 后端 API（日记、用户、AI、媒体、导入导出、OpenAPI 等）
- `internal/auth/` — 鉴权
- `internal/chat/` / `internal/embedding/` — AI 对话与向量化
- `internal/config/` / `internal/store/` — 配置与数据存储
- `site/src/` — 前端 Svelte 组件与路由

### ⚠️ 已知说明

- 数据目录默认使用 `./pb_data`，可通过命令行 `--data-dir` 或环境变量 `DIARUM_DATA_PATH` 自定义
- 启动时若检测到旧版 `data.db` 且不存在 `diarum.db`，将自动迁移用户、日记、媒体元数据、设置与 AI 对话数据
