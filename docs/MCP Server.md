# Diarum MCP Server

## 概述

Diarum 提供了一个 MCP (Model Context Protocol) 服务器，允许外部 AI 助手（如 Claude Desktop）直接访问和查询日记数据。

## 功能

通过 MCP 协议，AI 助手可以：

- 查询指定日期的日记内容
- 按日期范围搜索日记
- 获取日记统计信息（总篇数、连续天数等）

## 配置

在 Claude Desktop 的配置文件中添加：

```json
{
  "mcpServers": {
    "diarum": {
      "command": "node",
      "args": ["<path-to>/mcp-server/dist/index.js"],
      "env": {
        "DIARUM_API_URL": "http://localhost:8090",
        "DIARUM_API_TOKEN": "<your-api-token>"
      }
    }
  }
}
```

### 环境变量

| 变量 | 必填 | 说明 |
| :--- | :--- | :--- |
| `DIARUM_API_URL` | 是 | Diarum 服务地址 |
| `DIARUM_API_TOKEN` | 是 | API Token（在 Diarum 设置页面获取） |

## 构建

```bash
cd mcp-server
npm install
npm run build
```

## 使用

构建完成后，在 Claude Desktop 中即可通过 MCP 协议访问日记数据。
