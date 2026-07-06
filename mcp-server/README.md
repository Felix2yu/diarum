# Diarum MCP Server

Go 语言实现的 Diarum 日记 MCP 服务器，支持 HTTP 流式传输，允许 AI 客户端创建、更新、查询和分析日记。

## 功能特性

- **日记增删改查**：创建、读取、更新、删除日记条目
- **搜索过滤**：按关键词搜索，按心情/场景/标签过滤
- **统计分析**：日记总数、连续写作天数、日历热力图
- **AI 功能**：生成周期分析、与 AI 对话分析日记
- **媒体管理**：列出上传的媒体文件
- **导出功能**：导出日记为 ZIP 或 Markdown

## 安装

### 从源码构建

```bash
cd mcp-server
go build -o mcp-server
```

### 交叉编译

```bash
# macOS ARM64
GOOS=darwin GOARCH=arm64 go build -o mcp-server-darwin-arm64

# Linux AMD64
GOOS=linux GOARCH=amd64 go build -o mcp-server-linux-amd64

# Windows AMD64
GOOS=windows GOARCH=amd64 go build -o mcp-server-windows-amd64.exe
```

## 配置

### 环境变量

| 变量 | 必需 | 默认值 | 说明 |
|------|------|--------|------|
| `DIARUM_API_TOKEN` | 是 | - | 你的 Diarum API Token |
| `DIARUM_BASE_URL` | 否 | `http://localhost:8090` | Diarum 服务器地址 |
| `MCP_HOST` | 否 | `0.0.0.0` | MCP 服务器绑定地址 |
| `MCP_PORT` | 否 | `8080` | MCP 服务器端口 |

### 获取 API Token

1. 登录你的 Diarum 实例
2. 进入 设置 → API
3. 启用 API 访问并复制 Token

## Client Configuration

### 启动 MCP 服务器

```bash
# 设置环境变量
export DIARUM_API_TOKEN="your-api-token"
export DIARUM_BASE_URL="http://localhost:8090"  # 可选
export MCP_PORT="8080"  # 可选，默认 8080

# 启动服务器
./mcp-server
```

服务器启动后会显示：
- Streamable HTTP 端点：`http://localhost:8080/mcp`

### Claude Desktop

Add to your Claude Desktop configuration file:

**macOS**: `~/Library/Application Support/Claude/claude_desktop_config.json`
**Windows**: `%APPDATA%\Claude\claude_desktop_config.json`

```json
{
  "mcpServers": {
    "diarum": {
      "url": "http://localhost:8080/mcp"
    }
  }
}
```

### LobeHub

Add to your LobeHub MCP configuration:

```json
{
  "mcpServers": {
    "diarum": {
      "url": "http://localhost:8080/mcp"
    }
  }
}
```

### 其他 MCP 客户端

服务器使用 Streamable HTTP 传输（双向流式通信）。配置你的 MCP 客户端：
- **URL**: `http://localhost:8080/mcp`（或你配置的主机/端口）

### 测试连接

可以使用 curl 测试端点：

```bash
curl http://localhost:8080/mcp
```

## 可用工具

### 日记增删改查

| 工具 | 说明 |
|------|------|
| `create_diary` | 创建或更新日记条目 |
| `update_diary` | 更新现有日记 |
| `get_diary` | 按日期或 ID 获取日记 |
| `delete_diary` | 删除日记条目 |
| `list_recent_diaries` | 获取最近的日记 |
| `get_on_this_day` | 获取历史上的今天 |

### 搜索过滤

| 工具 | 说明 |
|------|------|
| `search_diaries` | 按关键词搜索日记 |
| `filter_diaries` | 按心情或场景过滤 |
| `get_tags` | 获取所有标签及使用次数 |
| `get_diaries_by_tag` | 获取特定标签的日记 |

### 统计

| 工具 | 说明 |
|------|------|
| `get_stats` | 获取总数和连续写作天数 |
| `get_calendar_heatmap` | 获取日历热力图数据 |

### AI 功能

| 工具 | 说明 |
|------|------|
| `analyze_period` | 生成周期分析报告 |
| `list_analyses` | 列出已保存的分析 |
| `ai_chat` | 与 AI 对话分析日记 |
| `build_embeddings` | 构建/重建向量嵌入 |
| `get_vector_stats` | 获取向量索引统计 |

### 媒体

| 工具 | 说明 |
|------|------|
| `list_media` | 列出上传的媒体文件 |

### 导出

| 工具 | 说明 |
|------|------|
| `export_diaries` | 导出日记为 ZIP 或 Markdown |

## 使用示例

### 创建日记

```
用户：帮我创建一篇关于今天徒步旅行的日记

AI（使用 create_diary 工具）：
{
  "date": "2024-01-15",
  "content": "今天去山里徒步了，天气特别好...",
  "mood": 5,
  "tags": ["徒步", "自然", "运动"],
  "weather": "晴天"
}
```

### 搜索日记

```
用户：找找我写过关于徒步的日记

AI（使用 search_diaries 工具）：
{
  "query": "徒步"
}
```

### 获取统计

```
用户：我今年写了多少篇日记？

AI（使用 get_stats 工具）：
{
  "timezone": "Asia/Shanghai"
}
```

### AI 分析

```
用户：分析一下我上个月的日记

AI（使用 analyze_period 工具）：
{
  "period": "month",
  "start_date": "2023-12-01",
  "end_date": "2023-12-31"
}
```

## 故障排除

### 服务器无法启动

- 确保 `DIARUM_API_TOKEN` 已设置
- 验证 Diarum 服务器正在运行
- 检查 API Token 是否有效
- 检查端口是否被占用（默认 8080）

### 客户端看不到工具

- 重启 MCP 客户端
- 验证 URL 配置正确（`http://localhost:8080/mcp`）
- 检查客户端日志中的连接错误

### API 错误

- 确保 Diarum 服务器正在运行
- 验证 API Token 有必要的权限
- 检查 MCP 服务器与 Diarum 之间的网络连接

### 测试连接

```bash
# 测试 Streamable HTTP 端点
curl http://localhost:8080/mcp

# 查看服务器状态
curl http://localhost:8080/health
```

## 开发

### 项目结构

```
mcp-server/
├── main.go              # 入口文件
├── config/config.go     # 配置处理
├── client/api.go        # Diarum API HTTP 客户端
├── tools/
│   ├── diary.go         # 日记 CRUD 工具
│   ├── search.go        # 搜索/过滤工具
│   ├── stats.go         # 统计工具
│   ├── ai.go            # AI 功能工具
│   ├── media.go         # 媒体工具
│   └── export_import.go # 导出工具
├── go.mod               # Go 模块
└── README.md            # 本文档
```

### 添加新工具

1. 在相应的 `tools/*.go` 文件中创建新函数
2. 使用 `mcp.NewTool()` 定义工具
3. 使用 `mcp.WithString()`、`mcp.WithNumber()` 等添加参数
4. 实现处理函数
5. 使用 `server.AddTool()` 注册工具

### 构建

```bash
go build -o mcp-server
```

### 测试

1. 启动 Diarum 服务器
2. 启动 MCP 服务器：
   ```bash
   export DIARUM_API_TOKEN="your-token"
   ./mcp-server
   ```
3. 测试连接：
   ```bash
   curl http://localhost:8080/mcp
   ```
4. 在 MCP 客户端中配置并测试工具

### Streamable HTTP 传输

服务器使用 Streamable HTTP 传输协议（MCP 最新标准）：
- **端点**：`/mcp` - 单一端点处理所有请求
- **双向流式通信**：真正的双向实时通信
- **比 SSE 更高效**：只需要一个端点，更简洁

## 许可证

MIT
