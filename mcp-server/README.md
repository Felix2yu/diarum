# Diarum MCP Server

A Model Context Protocol (MCP) server for the Diarum diary application, compatible with LobeHub and other MCP clients.

## Features

- Read diary entries by date
- List diary entries within a date range
- Create or update diary entries
- Search diary entries by keyword
- Get diary statistics (total entries, writing streak)
- Get all tags with usage counts

## Prerequisites

1. A running Diarum instance
2. API token enabled in Diarum (Settings > API Token)
3. Node.js 18+ installed

## Installation

```bash
cd mcp-server
npm install
npm run build
```

## Configuration

Set the following environment variables:

```bash
# Required: Your Diarum API token
export DIARUM_API_TOKEN="your-api-token-here"

# Optional: Diarum server URL (default: http://localhost:8090)
export DIARUM_BASE_URL="http://localhost:8090"
```

## Usage with LobeHub

### Option 1: Using npx (recommended)

Add to your LobeHub MCP configuration:

```json
{
  "mcpServers": {
    "diarum": {
      "command": "node",
      "args": ["/absolute/path/to/diarum/mcp-server/build/index.js"],
      "env": {
        "DIARUM_API_TOKEN": "your-api-token-here",
        "DIARUM_BASE_URL": "http://localhost:8090"
      }
    }
  }
}
```

### Option 2: Using the built binary

```bash
# Build first
npm run build

# Then configure in LobeHub
{
  "mcpServers": {
    "diarum": {
      "command": "/absolute/path/to/diarum/mcp-server/build/index.js",
      "env": {
        "DIARUM_API_TOKEN": "your-api-token-here"
      }
    }
  }
}
```

## Available Tools

### `get_diary`

Read a diary entry by date.

**Parameters:**
- `date` (string): Date in YYYY-MM-DD format

**Example:**
```json
{
  "date": "2025-01-15"
}
```

### `list_diaries`

List diary entries within a date range.

**Parameters:**
- `start` (string): Start date in YYYY-MM-DD format
- `end` (string): End date in YYYY-MM-DD format

**Example:**
```json
{
  "start": "2025-01-01",
  "end": "2025-01-31"
}
```

### `create_or_update_diary`

Create a new diary entry or update an existing one.

**Parameters:**
- `date` (string): Date in YYYY-MM-DD format
- `content` (string): Diary content in HTML format
- `mood` (string, optional): Mood emoji or label
- `weather` (string, optional): Weather emoji or label
- `tags` (string[], optional): Array of tags

**Example:**
```json
{
  "date": "2025-01-15",
  "content": "<p>Today was a great day!</p>",
  "mood": "😊",
  "weather": "☀️",
  "tags": ["travel", "happy"]
}
```

### `search_diaries`

Search diary entries by keyword.

**Parameters:**
- `query` (string): Search keyword or phrase

**Example:**
```json
{
  "query": "travel"
}
```

### `get_stats`

Get diary statistics (total entries and writing streak).

**Parameters:** None

### `get_tags`

Get all tags with their usage counts.

**Parameters:** None

## Development

```bash
# Install dependencies
npm install

# Build
npm run build

# Run (for testing)
DIARUM_API_TOKEN=xxx npm start
```

## License

MIT
