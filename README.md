# Diarum

<p align="center">
  <img src="site/static/logo.png" alt="Diarum Logo" width="120" />
</p>

<p align="center">
  <em>一天一篇，打开即写，刚刚好。</em>
</p>

---
相较于原版做了如下修改：
1. 全界面中文本地化（日期格式、星期起始、按钮文案等均符合中文用户习惯）
2. API 新增日记修改、删除等接口
3. 新增 favicon 及应用图标
4. Docker 非 root 用户运行（UID:1000）
5. AI 周期分析报告（支持自定义时间范围与关键词，周报/月报自动生成并保存历史）
6. 日记与分析报告支持 Markdown 渲染
7. 完整 PWA 支持（离线模式、应用安装、自动更新提示）
8. 往昔今朝与时空穿越（日历视图中查看历年同日日记、随机翻阅历史日记）
9. 日记多标签系统与标签云（支持自动补全、按标签筛选、标签搜索高亮）
10. AI 文本整理（去语气词、纠错、自动分段，支持自定义 prompt）
11. 语音输入（Web Speech API 录音转文字）
12. 响应式布局优化（自适应大屏、流体字体、统一容器系统）
13. 日历年视图平铺布局 + 年月选择器
14. 全面 UI 优化（滚动动画、弹窗统一样式、移动端适配改进）
15. Svelte 5 升级（新事件语法、新响应式系统）
16. zstd + brotli 预压缩（静态资源传输体积减少 75-78%）
17. 清理 tiptap 死代码与未使用依赖（减少约 50MB node_modules）
18. MCP Server（Model Context Protocol）— 内置 MCP 服务端，支持 Cherry Studio、Claude Desktop 等 AI 客户端直接读写日记
19. Web Push 日记提醒通知 — 设置每日提醒时间，到点若当天还未写日记则推送系统通知（兼容 Apple APNs / Chrome / Firefox）
20. 循环重构 — 接入和风天气(QWeather)、定时自动获取与历史补全；自动日记备份并上传 S3
---

## 中文

### 关于

**吾身** (Diarum) - 一天一篇，打开即写，刚刚好。中文名取自"吾日三省吾身"，特点是每天有且仅有一篇日记，告别选择焦虑，致力于打造一款零负担、快记录、怡复盘的日记应用，记录独一无二的人生。

零负担，软件使用非常简单，登陆后打开首页即跳转到今日日记。快记录，打开立刻开始记录，自动保存。怡复盘，可以愉快的完成复盘、总结分析。轻松实现现代化 AI 加持的“吾日三省吾身”。

配置 AI Key 之后自动触发日记向量化，后续可以跟 AI LLM 结合日记开展对话 。自然快速地完成：

 - 今日复盘
 - 周报生成
 - 年终总结
 - 等等

基于 Go、SQLite 和现代 Web 技术构建，简洁、优雅、可自托管。

### 在线演示

无需安装即可体验 Diarum：

🌐 **演示站点**: https://demo.diarum.app/

📝 **演示账户**:
- 用户名: `demo`
- 密码: `demo@1234`

开发这款软件的初衷源自自己对日记的需求。现在市面上已经有很多优秀的日记和笔记软件。但都多少有点无法满足自己的需求。我期望的一个日记软件，是打开后立刻可以开始记录，不需要纠结文件名、标题、目录结构。最好是网页的，这样在各种设备都可以使用。我自己的设备涉及 MacBook 、HarmonyOS NEXT 、Android 、Arch Linux 、Windows 。只有网页应用能够很好的快速兼容这些平台。最好是可以很方便的自托管的，确保我自己对数据的掌控，且方便搬家。

于是就做了这样一款软件，英文名叫 Diarum ，中文名叫 “吾身”。使用 go+svelte 开发，轻快好用。花费了大量心思打磨移动端和桌面端的日记体验。现在我个人感觉使用体验已经比较丝滑，可以愉快的记录一天的各种事情。

在核心功能的基础上，集成了一个简单的 RAG 系统，配置好 AI KEY 和 MODEL 之后，会自动触发向量数据库的构建。这样一来跟内置的 AI 助手对话时，就可以将向量匹配到的日记放入上下文，方便的进行分析总结等。此外还提供了一个简单的 API 系统，可以方便的将日记数据对接到 n8n 这样的平台，实现自动化的周报、月报生成等灵活的工作流。

### 截图预览

| 桌面端浅色 | 桌面端深色 |
|:---:|:---:|
| ![桌面端浅色](site/static/screenshots/desktop-light.png) | ![桌面端深色](site/static/screenshots/desktop-dark.png) |

| 移动端浅色 | 移动端深色 |
|:---:|:---:|
| ![移动端浅色](site/static/screenshots/mobile-light.png) | ![移动端深色](site/static/screenshots/mobile-dark.png) |

### 主要功能

- 📝 **Markdown 支持** - 使用完整的 Markdown 格式记录每日想法
- 🖼️ **媒体上传** - 为日记条目添加图片和文件，支持 Chevereto 图床，灵活切换内置媒体管理器或外部图床
- 📱 **渐进式 Web 应用** - 支持安装到任意设备，离线可用，原生应用般的体验
- 📤 **一键分享** - 轻点即可分享日记内容
- 📥 **灵活导入** - 支持从导出 ZIP 或包含 `.md` 文件的 ZIP 导入日记，日期冲突时可差异对比并手动选择保留版本
- 🔄 **离线与自动同步** - 完整离线支持，自动缓存同步，实时查看数据同步状态
- 🏷️ **多标签系统** - 为日记添加标签，标签云可视化，按标签筛选与搜索
- 🗣️ **语音输入** - 支持录音转文字，快速记录
- 🤖 **AI 文本整理** - 一键去语气词、纠错、自动分段，支持自定义 prompt
- 📊 **AI 周期分析** - 自动生成周报/月报，支持自定义时间范围与关键词，历史报告管理
- 💬 **AI 对话 + RAG** - 基于向量检索的历史日记对话，附带引用追溯
- 📅 **往昔今朝 / 时空穿越** - 日历视图中回顾历年同日日记，随机翻阅历史记录
- 🔗 **Memos Webhook 同步** - 接收 Memos 新增、更新、删除 webhook 事件，并同步写入 memo 创建日期对应的日记
- 🤝 **MCP Server** - 内置 Model Context Protocol 服务端，支持 Cherry Studio、Claude Desktop 等 AI 客户端直接读写日记
- 🔔 **Web Push 日记提醒** - 设置每日提醒时间，到点若当天还未写日记则收到系统通知，不打扰已完成的记录（支持 Apple APNs / Chrome / Firefox）
- 🌤️ **智能天气** - 接入和风天气，定时自动获取、历史日期补全，天气与温度跨设备同步
- 🔒 **自动备份** - 定时导出日记并上传 S3，数据更安心
- 🔒 **自托管** - 完全掌控你的个人数据
- 🚀 **易于部署** - 单一二进制文件，内嵌前端，随处部署
- 💾 **原生 SQLite 后端** - 内置用户体系、本地媒体存储与旧数据自动迁移
- 🔧 **可配置** - 通过环境变量或命令行参数灵活配置数据目录

### 快速开始

#### 使用 Docker

```bash
docker run -d \
  --name diarum \
  -p 8090:8090 \
  ghcr.io/felix2yu/diarum:latest
```

在浏览器访问 `http://localhost:8090`

#### 使用 Docker 持久化数据

要持久化你的日记数据，需要挂载数据卷到数据目录：

```bash
docker run -d \
  --name diarum \
  -p 8090:8090 \
  -v /path/to/your/data:/app/data \
  ghcr.io/felix2yu/diarum:latest
```

#### 使用 Docker Compose

创建 `docker-compose.yml` 文件：

```yaml
services:
  diarum:
    image: ghcr.io/felix2yu/diarum:latest
    container_name: diarum
    ports:
      - "8090:8090"
    volumes:
      - ./data:/app/data
    environment:
      - DIARUM_DATA_PATH=/app/data
    restart: unless-stopped
```

运行：

```bash
docker compose up -d
```

### 配置说明

#### 数据目录

你可以通过三种方式配置数据目录（优先级从高到低）：

1. **命令行参数**：
   ```bash
   ./diarum serve --data-dir=/custom/path
   ```

2. **环境变量**：
   ```bash
   export DIARUM_DATA_PATH=/custom/path
   ./diarum serve
   ```

3. **默认值**：`./pb_data`（当前目录）

#### Docker 环境变量

- `DIARUM_DATA_PATH`：设置数据目录路径（默认：`/app/data`）

### 从源码构建

#### 前置要求

- Go 1.26 或更高版本
- Node.js 24 或更高版本
- zstd（用于静态资源预压缩）

#### 构建步骤

```bash
# 克隆仓库
git clone https://github.com/songtianlun/diarum.git
cd diarum

# 全量构建
make build

# 运行
./diarum serve
```

### 开发

```bash
# 启动前端开发服务器
make dev-frontend

# 启动后端开发服务器
make dev-backend
```

### 数据存储

Diarum 会在配置的数据目录下使用 `diarum.db` 保存应用数据。启动时如果检测到旧版 `data.db` 且尚不存在 `diarum.db`，会自动创建新数据库并迁移用户、日记、媒体元数据、设置和 AI 对话数据，同时保留旧数据库不变。

### MCP 集成

Diarum 内置 MCP (Model Context Protocol) 服务端，可通过 `/mcp` 端点与 AI 客户端交互，让 AI 直接读写你的日记。

#### 可用工具

| 工具 | 说明 |
|------|------|
| `create_diary` | 创建或更新指定日期的日记（支持心情、天气、标签等元数据） |
| `get_diary` | 按日期或 ID 获取日记 |
| `delete_diary` | 按 ID 删除日记 |
| `list_recent_diaries` | 获取最近的日记列表 |
| `search_diaries` | 按关键词搜索日记内容 |
| `get_tags` | 获取所有标签及使用次数 |
| `get_stats` | 获取日记统计数据 |
| `get_weather` | 获取中国城市天气预报 |

#### 获取 API Token

1. 登录 Diarum，进入**设置**页面
2. 在 **API Token** 区域点击**生成 Token**
3. 复制生成的 Token（格式：`xxxxx.xxxxx`）

#### 客户端配置

将下方 `<your-diarum-instance>` 替换为你的 Diarum 地址（如 `http://localhost:8090`），`<your-api-token>` 替换为上一步复制的 Token。

##### 通用格式（Cherry Studio / Trae / Cursor / Claude Desktop / Cline / 通义灵码）

以下客户端均使用相同的 `mcpServers` 配置格式：

- **Cherry Studio**：设置 → MCP 服务器 → 添加服务器，选择 **Streamable HTTP** 类型
- **Trae / TraeCode**：设置 → MCP 服务器 → 手动添加
- **Cursor**：项目根目录创建 `.cursor/mcp.json`，或编辑全局 `~/.cursor/mcp.json`
- **Claude Desktop**：编辑配置文件（macOS: `~/Library/Application Support/Claude/claude_desktop_config.json`，Windows: `%APPDATA%\Claude\claude_desktop_config.json`）
- **Cline (VS Code)**：插件设置 → MCP Servers → 添加 Server
- **通义灵码 / Lingma**：设置 → MCP 服务器 → 添加，选择 HTTP 类型

```json
{
  "mcpServers": {
    "diarum": {
      "url": "https://your-diarum-instance/mcp",
      "headers": {
        "Authorization": "Bearer <your-api-token>"
      }
    }
  }
}
```

##### OpenCode

编辑配置文件 `opencode.json`（项目根目录或 `~/.config/opencode/opencode.json`）：

```json
{
  "mcp": {
    "diarum": {
      "type": "remote",
      "url": "https://your-diarum-instance/mcp",
      "headers": {
        "Authorization": "Bearer <your-api-token>"
      }
    }
  }
}
```

##### WorkBuddy / 腾讯云代码助手 CodeBuddy

编辑配置文件 `~/.codebuddy/.mcp.json`（推荐）或项目目录下 `.workbuddy/mcp.json`：

```json
{
  "mcpServers": {
    "diarum": {
      "type": "http",
      "url": "https://your-diarum-instance/mcp",
      "headers": {
        "Authorization": "Bearer <your-api-token>"
      }
    }
  }
}
```

> **提示**：所有支持 MCP Streamable HTTP 或 SSE 协议的客户端均可接入。如客户端要求 SSE 类型，将 URL 末尾改为 `/mcp` 即可（Diarum 同时兼容两种协议）。

### 单元测试

[![codecov](https://codecov.io/github/Felix2yu/diarum/graph/badge.svg?token=YU69O21LXM)](https://codecov.io/github/Felix2yu/diarum)
![codecov-graph](https://codecov.io/github/Felix2yu/diarum/graphs/tree.svg?token=YU69O21LXM)

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

## Support

If you encounter any issues or have questions, please [open an issue](https://github.com/songtianlun/diarum/issues).

---

**Made with ❤️ by [songtianlun](https://github.com/songtianlun)**

## Star History

[![Star History Chart](https://api.star-history.com/svg?repos=songtianlun/diarum&type=timeline&legend=top-left)](https://www.star-history.com/#songtianlun/diarum&type=timeline&legend=top-left)
