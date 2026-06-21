#!/usr/bin/env node

import { McpServer } from "@modelcontextprotocol/sdk/server/mcp.js";
import { StdioServerTransport } from "@modelcontextprotocol/sdk/server/stdio.js";
import { z } from "zod";

const DIARUM_BASE_URL = process.env.DIARUM_BASE_URL || "http://localhost:8090";
const DIARUM_API_TOKEN = process.env.DIARUM_API_TOKEN || "";

if (!DIARUM_API_TOKEN) {
  console.error("Error: DIARUM_API_TOKEN environment variable is required");
  process.exit(1);
}

const server = new McpServer({
  name: "diarum",
  version: "1.0.0",
});

interface Diary {
  id: string;
  date: string;
  content: string;
  mood: string;
  weather: string;
  tags: string[];
  created: string;
  updated: string;
}

interface DiaryResponse {
  id: string;
  date: string;
  content: string;
  mood: string;
  weather: string;
  tags: string[];
  exists: boolean;
  created?: string;
  updated?: string;
}

interface SearchResponse {
  diaries: Diary[];
  total: number;
}

interface StatsResponse {
  total: number;
  streak: number;
}

interface TagsResponse {
  tags: Array<{ name: string; count: number }>;
}

async function diarumFetch<T>(path: string, options?: RequestInit): Promise<T> {
  const url = `${DIARUM_BASE_URL}${path}`;
  const separator = path.includes("?") ? "&" : "?";
  const fullUrl = `${url}${separator}token=${DIARUM_API_TOKEN}`;

  const response = await fetch(fullUrl, {
    ...options,
    headers: {
      "Content-Type": "application/json",
      ...options?.headers,
    },
  });

  if (!response.ok) {
    const text = await response.text();
    throw new Error(`API error ${response.status}: ${text}`);
  }

  return response.json() as Promise<T>;
}

server.registerTool(
  "get_diary",
  {
    description:
      "Read a diary entry by date. Returns the diary content for the specified date.",
    inputSchema: {
      date: z
        .string()
        .describe('Date in YYYY-MM-DD format (e.g. "2025-01-15")'),
    },
  },
  async ({ date }) => {
    try {
      const diary = await diarumFetch<DiaryResponse>(
        `/api/v1/diaries?date=${date}`
      );

      if (!diary.exists) {
        return {
          content: [{ type: "text", text: `No diary entry found for ${date}.` }],
        };
      }

      const text = [
        `Date: ${diary.date}`,
        `Mood: ${diary.mood || "Not set"}`,
        `Weather: ${diary.weather || "Not set"}`,
        `Tags: ${diary.tags?.join(", ") || "None"}`,
        "",
        "Content:",
        diary.content || "(empty)",
      ].join("\n");

      return { content: [{ type: "text", text }] };
    } catch (error) {
      return {
        content: [
          {
            type: "text",
            text: `Error reading diary: ${error instanceof Error ? error.message : String(error)}`,
          },
        ],
        isError: true,
      };
    }
  }
);

server.registerTool(
  "list_diaries",
  {
    description:
      "List diary entries within a date range. Returns all diaries between start and end dates (inclusive).",
    inputSchema: {
      start: z
        .string()
        .describe('Start date in YYYY-MM-DD format (e.g. "2025-01-01")'),
      end: z
        .string()
        .describe('End date in YYYY-MM-DD format (e.g. "2025-01-31")'),
    },
  },
  async ({ start, end }) => {
    try {
      const result = await diarumFetch<{ diaries: DiaryResponse[]; total: number }>(
        `/api/v1/diaries?start=${start}&end=${end}`
      );

      if (result.total === 0) {
        return {
          content: [
            {
              type: "text",
              text: `No diary entries found between ${start} and ${end}.`,
            },
          ],
        };
      }

      const lines = result.diaries.map((d) => {
        const contentPreview = d.content
          ? d.content.replace(/<[^>]*>/g, "").substring(0, 100) + (d.content.length > 100 ? "..." : "")
          : "(empty)";
        return `${d.date} | Mood: ${d.mood || "-"} | ${contentPreview}`;
      });

      return {
        content: [
          {
            type: "text",
            text: `Found ${result.total} diary entries:\n\n${lines.join("\n")}`,
          },
        ],
      };
    } catch (error) {
      return {
        content: [
          {
            type: "text",
            text: `Error listing diaries: ${error instanceof Error ? error.message : String(error)}`,
          },
        ],
        isError: true,
      };
    }
  }
);

server.registerTool(
  "create_or_update_diary",
  {
    description:
      "Create a new diary entry or update an existing one. Uses upsert semantics - if a diary exists for the given date, it will be updated.",
    inputSchema: {
      date: z
        .string()
        .describe('Date in YYYY-MM-DD format (e.g. "2025-01-15")'),
      content: z.string().describe("Diary content in HTML format"),
      mood: z.string().optional().describe("Mood emoji or label (e.g. 😊, 😌)"),
      weather: z
        .string()
        .optional()
        .describe("Weather emoji or label (e.g. ☀️, 🌧️)"),
      tags: z
        .array(z.string())
        .optional()
        .describe("Array of tags (e.g. [\"travel\", \"food\"])"),
    },
  },
  async ({ date, content, mood, weather, tags }) => {
    try {
      const result = await diarumFetch<DiaryResponse>("/api/v1/diaries", {
        method: "POST",
        body: JSON.stringify({
          date,
          content,
          mood: mood || "",
          weather: weather || "",
          tags: tags || [],
        }),
      });

      return {
        content: [
          {
            type: "text",
            text: `Diary ${result.created ? "created" : "updated"} successfully for ${date}.\n\nDate: ${result.date}\nMood: ${result.mood || "Not set"}\nWeather: ${result.weather || "Not set"}\nTags: ${result.tags?.join(", ") || "None"}`,
          },
        ],
      };
    } catch (error) {
      return {
        content: [
          {
            type: "text",
            text: `Error saving diary: ${error instanceof Error ? error.message : String(error)}`,
          },
        ],
        isError: true,
      };
    }
  }
);

server.registerTool(
  "search_diaries",
  {
    description:
      "Search diary entries by keyword. Searches through diary content for matching text.",
    inputSchema: {
      query: z
        .string()
        .describe('Search keyword or phrase (e.g. "travel", "心情愉快")'),
    },
  },
  async ({ query }) => {
    try {
      const result = await diarumFetch<SearchResponse>(
        `/api/v1/diaries/search?q=${encodeURIComponent(query)}`
      );

      if (result.total === 0) {
        return {
          content: [
            {
              type: "text",
              text: `No diary entries found matching "${query}".`,
            },
          ],
        };
      }

      const lines = result.diaries.map((d) => {
        const contentPreview = d.content
          ? d.content.replace(/<[^>]*>/g, "").substring(0, 100) + (d.content.length > 100 ? "..." : "")
          : "(empty)";
        return `${d.date} | Mood: ${d.mood || "-"} | ${contentPreview}`;
      });

      return {
        content: [
          {
            type: "text",
            text: `Found ${result.total} diary entries matching "${query}":\n\n${lines.join("\n")}`,
          },
        ],
      };
    } catch (error) {
      return {
        content: [
          {
            type: "text",
            text: `Error searching diaries: ${error instanceof Error ? error.message : String(error)}`,
          },
        ],
        isError: true,
      };
    }
  }
);

server.registerTool(
  "get_stats",
  {
    description:
      "Get diary statistics including total count and current writing streak.",
    inputSchema: {},
  },
  async () => {
    try {
      const stats = await diarumFetch<StatsResponse>(
        "/api/v1/diaries/stats"
      );

      return {
        content: [
          {
            type: "text",
            text: `Diary Statistics:\n- Total entries: ${stats.total}\n- Current streak: ${stats.streak} days`,
          },
        ],
      };
    } catch (error) {
      return {
        content: [
          {
            type: "text",
            text: `Error fetching stats: ${error instanceof Error ? error.message : String(error)}`,
          },
        ],
        isError: true,
      };
    }
  }
);

server.registerTool(
  "get_tags",
  {
    description: "Get all tags with their usage counts.",
    inputSchema: {},
  },
  async () => {
    try {
      const result = await diarumFetch<TagsResponse>("/api/v1/diaries/tags");

      if (result.tags.length === 0) {
        return {
          content: [{ type: "text", text: "No tags found." }],
        };
      }

      const lines = result.tags.map((t) => `${t.name}: ${t.count} entries`);

      return {
        content: [
          {
            type: "text",
            text: `Tags:\n${lines.join("\n")}`,
          },
        ],
      };
    } catch (error) {
      return {
        content: [
          {
            type: "text",
            text: `Error fetching tags: ${error instanceof Error ? error.message : String(error)}`,
          },
        ],
        isError: true,
      };
    }
  }
);

async function main() {
  const transport = new StdioServerTransport();
  await server.connect(transport);
  console.error("Diarum MCP Server running on stdio");
}

main().catch((error) => {
  console.error("Fatal error in main():", error);
  process.exit(1);
});
