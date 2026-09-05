# 幕间 (diarum) MCP 日记批量编辑与文本纠正：现状分析与改进设计

> 目标：评估现有 MCP 实现，给出便于"通过 MCP 批量修改日记（内容 / 心情 / 情景 / 标签）"以及"将语音生成的日记自动修正为规范文字与格式"的优化方案，包含改进后的功能定义与调用方式。

---

## 一、现状盘点

### 1.1 现有 MCP 工具（`internal/mcp/mcp.go`）

| 工具 | 定位方式 | 能力 | 备注 |
|---|---|---|---|
| `create_diary` (L79) | 按 `date` | 创建或整条 upsert | 实为 upsert，但名为 create |
| `get_diary` (L170) | 按 `id` 或 `date` | 读取单条 | — |
| `delete_diary` (L213) | 按 `id` | 删除单条 | — |
| `list_recent_diaries` (L240) | 仅 `limit` | 最近 N 条 | 无日期范围 / 标签 / 游标分页 |
| `search_diaries` (L269) | `query` + `scenario` | 关键词搜索 | 无批量目标定位能力 |
| `get_tags` (L301) | — | 标签计数 | — |
| `get_stats` (L328) | — | 总数统计 | — |
| `get_weather` (L352) | `city` | 天气查询 | — |

### 1.2 数据层（`internal/store/store.go`）

- `Diary` 结构（L76）：`content / mood / mood_states / scenarios / weather / city / temp_min / temp_max / tags` 等字段。
- 写入入口只有两条：`UpsertDiary`（L1121，按 `date` 整条 upsert）与 `UpsertDiaryWeather`（L1218，按 `date` 仅写天气）。
- 局部更新能力：**没有** 按 `id` 的局部更新，也**没有**任何批量更新 / 批量删除函数。
- 写入后一律 `GetDiaryByID` 再查一次（读放大）。

### 1.3 文本纠正能力（仅在 REST 层，`internal/api/ai.go`）

- `POST /api/v1/ai/transcribe`（L191）：音频 → 文本，依赖外部 STT（Whisper 类接口）。
- `POST /api/v1/ai/polish`（L908）：文本整理，模式 `medium`（去口语词 / 纠错 / 分段）/ `strong`（深度改写）/ `custom`（自定义 prompt）。
- **MCP 层完全没有暴露上述任一能力**，Agent 无法通过 MCP 触发"转写→纠正→落库"。

---

## 二、核心问题与优化点

### 2.1 MCP 接口层面

1. **语义与命名不一致。** `create_diary` 实为 upsert（创建或覆盖），但名字暗示"只创建"。批量编辑场景下语义模糊，应显式拆分为 `upsert_diary` / `update_diary`（按 id 局部更新）/ `batch_update_diaries`。

2. **缺少"按 id 局部更新"工具。** 当前只能按 `date` upsert 整条。要编辑已有日记，必须 `get_diary` 拿到全量字段后再整条回写，存在**误覆盖**风险（天气、城市、温度被清零）。

3. **完全没有批量工具。** 批量修改内容 / 心情 / 标签无法在一次工具调用内完成，必须 N 次往返，Agent 编排成本高。

4. **文本纠正能力孤岛。** `transcribe` 与 `polish` 仅存在于 REST，MCP 未暴露。"语音日记自动修正"这一核心诉求在 MCP 侧无入口。

5. **列表不可分页 / 不可筛选。** `list_recent_diaries` 只支持 `limit`，批量处理大量日记时分批拉取、按标签 / 日期范围定位目标都很困难。

6. **缺少"预览 / 草稿"模式。** `polish` 只返回文本不落库；MCP 应提供"纠正并返回预览（不落库）"与"纠正并写入"两种模式，避免误写。

### 2.2 参数设计层面

1. **`content` 非可选，会静默清空内容（关键缺陷）。** MCP 侧 `content := req.GetString("content", "")`（L97）总是以 `""` 参与 upsert；当调用方只想更新 `mood` 时，`content` 为空串会**覆盖掉原有正文**。REST `POST /upsert` 同样把 `Content` 声明为必传 `string`（非指针）。局部更新应让 `content` 成为可选字段（指针 / 显式 `has`）。

2. **隐藏参数与 schema 不一致。** 处理器里用 `has("city") / has("temp_min") / has("temp_max")`（L121-135）探测字段，但 `create_diary` 的 schema（L79-88）**并未声明**这三个参数。客户端无法从工具定义中得知它们的存在，属于"隐式接口"。

3. **零值歧义处理重复且不统一。** MCP 用手写 `has()` 分支（L107-148），REST 用 JSON 指针（`Mood *int` 等）由框架绑定（L27-58）。两套逻辑易漂移。应定义**统一的 `DiaryPatch` 结构体**，MCP 与 REST 共用。

4. **`temp_min/max` 类型不安全。** MCP 用 `fmt.Sscanf(req.GetString("temp_min","0"), "%f", &v)`（L128）把数字当字符串解析，对 JSON number / 浮点不友好，易出错。应直接用数值类型接收。

5. **缺少运行时校验。** `date` 未做 `time.Parse("2006-01-02")` 强校验（REST 的 `by-date` 有，MCP 无）；`mood` 范围 [1,5] 仅写在 description，无运行时拦截。

6. **`\n` 双重转义是 workaround。** `content = strings.ReplaceAll(content, "\\n", "\n")`（L101）说明上层传输存在转义问题。应在 schema 明确 `content` 为纯文本或 HTML，并在框架层统一处理，而非每处打补丁。

7. **标签语义缺"追加 / 移除"。** store 写入时 `normalizeTags` 去重排序，但 MCP 层只能"全量覆盖"或"不动"。`update_diary` 应支持 `tags_op: replace | merge | remove`。

### 2.3 批量操作效率

1. **N 次往返 + 3N 次 SQL。** 每篇日记一次 `UpsertDiary` = `GetDiaryByDate`（SELECT）+ `UPDATE` + `GetDiaryByID`（SELECT）≈ 3 次查询。批量 100 篇 = 100 次往返 + 300 次 SQL。应提供 `batch_update_diaries`，服务端**单事务**内批量执行。

2. **无事务、无批量定位、无逐条结果。** 批量更新需要"按 id 列表 / 日期范围 / 标签 / 搜索条件"批量定位（当前只能逐条指定），并需返回 per-item 成功 / 失败以重试失败子集。当前单条返回整条 diary JSON，批量时响应体巨大。

3. **embedding 重建放大。** 日记变化会触发 `onDiaryChanged`（embedding 重建）。批量 100 篇 = 100 次重建；应合并为一次批量重建。

### 2.4 文本纠正流程

1. **转写与纠正分离且 MCP 未串联。** 应提供 `transcribe_audio` → `polish_diary`（仅文本纠正）→ `correct_voice_diary`（流水线：转写 → 语音专用预设纠正 → 可选落库）。

2. **缺"语音整理"专用预设。** 现有 `medium/strong/custom` 偏通用书面化。`correct_voice_diary` 应针对语音场景：去口语词 / 语气词、修正同音错别字、补断句与标点、自动分段，同时**尽量保留原文事实与口吻**。

3. **无格式规范化出口。** `polish` 只回纯文本，但日记 `content` 支持 HTML。批量写入时应统一格式（段落 `<p>`、换行 `<br>`），并保留"先预览后保存"。

4. **无幂等 / 去重。** 语音可能重复转写同一段；批量纠正应支持幂等（同 diary id 纠正结果可对比 / 缓存）。

---

## 三、改进后的功能定义

### 3.1 工具总览（建议）

| 工具 | 用途 |
|---|---|
| `upsert_diary` | 按 `date` 创建或整条写入（原 `create_diary`，语义明确化） |
| `get_diary` | 按 `id` / `date` 读取（保留） |
| `list_diaries` | 可筛选 / 分页的列表（替代 `list_recent_diaries`，支持 `date_range` / `tag` / `scenario` / 游标） |
| `update_diary` | **新增**：按 `id` 局部更新（content / mood / mood_states / scenarios / tags / weather …） |
| `batch_update_diaries` | **新增**：按选择器批量局部更新 |
| `delete_diary` / `batch_delete_diaries` | 单条 / 批量删除 |
| `search_diaries` | 保留，作为批量目标选择器之一 |
| `polish_diary` | **新增（来自 REST）**：文本纠正，支持 `apply` 落库与预览 |
| `transcribe_audio` | **新增（来自 REST）**：音频 → 文本 |
| `correct_voice_diary` | **新增**：语音日记修正流水线（转写 → 纠正 → 可选落库） |
| `get_tags` / `get_stats` / `get_weather` | 保留 |

### 3.2 统一的参数模型（store + MCP + REST 共用）

```go
// DiaryPatch 描述一次局部更新。nil 字段表示"不修改"。
type DiaryPatch struct {
    Content    *string   `json:"content,omitempty"`
    Mood       *int      `json:"mood,omitempty"`        // 1-5，运行时校验
    MoodStates *[]string `json:"mood_states,omitempty"`
    Scenarios  *[]string `json:"scenarios,omitempty"`
    Weather    *string   `json:"weather,omitempty"`
    City       *string   `json:"city,omitempty"`
    TempMin    *float64  `json:"temp_min,omitempty"`
    TempMax    *float64  `json:"temp_max,omitempty"`
    Tags       *[]string `json:"tags,omitempty"`
    TagsOp     string    `json:"tags_op,omitempty"`     // replace | merge | remove（仅当 Tags 非空时生效）
    ContentFormat string `json:"content_format,omitempty"` // text | html（控制写入规范化）
}

// BatchTargets 批量定位选择器（满足任一即可，互斥优先顺序：ids > date_range > tag > scenario > query）
type BatchTargets struct {
    IDs       []string `json:"ids,omitempty"`
    DateRange *struct {
        Start string `json:"start"` // YYYY-MM-DD
        End   string `json:"end"`
    } `json:"date_range,omitempty"`
    Tag       string `json:"tag,omitempty"`
    Scenario  string `json:"scenario,omitempty"`
    Query     string `json:"query,omitempty"` // 关键词搜索结果作为目标
}

type BatchOpts struct {
    DryRun           bool   `json:"dry_run,omitempty"`            // 只返回预览，不落库
    ContinueOnError  bool   `json:"continue_on_error,omitempty"`  // 单条失败继续
    TagsOp           string `json:"tags_op,omitempty"`
    ContentFormat    string `json:"content_format,omitempty"`
}

type BatchResult struct {
    ID     string `json:"id"`
    Status string `json:"status"` // ok | skipped | error
    Error  string `json:"error,omitempty"`
}
```

### 3.3 数据层需新增的接口

```go
// 按 id 的局部更新（替代"整条 upsert"做编辑）
func (s *Store) UpdateDiaryByID(id, owner string, p DiaryPatch) (*Diary, error)

// 批量更新：单事务内执行，返回逐条结果；dry_run 时仅 SELECT 目标并计算预览
func (s *Store) BatchUpdateDiaries(owner string, t BatchTargets, p DiaryPatch, o BatchOpts) ([]BatchResult, error)

// 批量删除
func (s *Store) BatchDeleteDiaries(owner string, ids []string) ([]BatchResult, error)
```

关键实现要点：
- 批量操作在**一个 `sql.Tx`** 中执行；UPDATE 用 `CASE` / 逐条参数化，避免 N 次往返。
- `TagsOp=merge` 时读现有 tags 并 `normalizeTags(append(existing, incoming...))`；`remove` 时过滤。
- `dry_run=true` 不写库，仅返回目标 id 列表与将要应用的 patch 摘要。
- 批量结束后**只触发一次** `onDiaryChanged`（合并 embedding 重建）。
- 不再对每篇逐条 `GetDiaryByID` 回查；结果从 patch + 目标 id 组装。

### 3.4 文本纠正流水线

`correct_voice_diary` 输入二选一：
- `audio_base64` / `audio_url`（先转写）；或
- `raw_text`（已转写，直接进入纠正）。

处理步骤：
1. 若提供音频 → 调用 STT（`transcribe_audio`）得到 `raw_text`；
2. 用"语音整理"预设 prompt 调用 `polish`（去口语词 / 语气词、修同音错别字、补断句与标点、自动分段，保留事实）；
3. 按 `content_format` 规范化（`text` → 段落 `<p>` / 换行 `<br>` 的 HTML）；
4. 返回 `{original, corrected, changes_summary, applied}`；`applied=true` 时调用 `update_diary` 落库。

`polish_diary` 暴露给 MCP，参数：`content`、`mode`（medium/strong/custom/voice）、`prompt`、`apply`（bool）、`target_diary_id`（可选，apply 时直接写回）。

---

## 四、调用方式示例（MCP tool call，JSON-RPC 风格）

### 4.1 按 id 局部更新心情 + 追加标签（不碰正文）

```json
{
  "tool": "update_diary",
  "arguments": {
    "id": "diary_abc123",
    "mood": 4,
    "tags": ["通勤", "阅读"],
    "tags_op": "merge"
  }
}
```
> 仅 `mood` 与 `tags` 被修改；`content` 等未传字段保持不变（修复了当前的清空缺陷）。

### 4.2 批量更新：给某标签下所有日记统一情景

```json
{
  "tool": "batch_update_diaries",
  "arguments": {
    "targets": { "tag": "出差" },
    "patch": { "scenarios": ["工作", "外出"], "tags_op": "replace" },
    "opts": { "dry_run": false, "continue_on_error": true }
  }
}
```
> 返回：`{ "results": [ {"id":"...","status":"ok"}, ... ], "applied": 23, "failed": 0 }`

### 4.3 批量纠正：把一批语音日记修正为规范文字（先预览）

```json
{
  "tool": "batch_update_diaries",
  "arguments": {
    "targets": { "ids": ["d1","d2","d3"] },
    "patch": { "content": "<<AUTO_CORRECT_VOICE>>" },
    "opts": { "dry_run": true, "content_format": "html" }
  }
}
```
> 服务端对每篇调用 `correct_voice_diary` 生成 `corrected` 预览并返回 diff；确认后再以 `dry_run:false` 落库。

### 4.4 单条语音日记修正并落库

```json
{
  "tool": "correct_voice_diary",
  "arguments": {
    "target_diary_id": "diary_xyz",
    "audio_base64": "<<BASE64>>",
    "apply": true,
    "content_format": "html"
  }
}
```
> 返回：`{ "original": "...口语原文...", "corrected": "<p>规范后的段落</p>", "changes_summary": "去除语气词 12 处、修正同音错字 3 处、补标点并分段", "applied": true }`

### 4.5 分页拉取目标（供批量前确认范围）

```json
{
  "tool": "list_diaries",
  "arguments": {
    "date_range": { "start": "2026-01-01", "end": "2026-03-31" },
    "tag": "随记",
    "limit": 50,
    "cursor": "eyJvZmZzZXQiOjUwfQ=="
  }
}
```

---

## 五、落地优先级与风险

**优先级 P0（修复正确性缺陷，工作量小）**
- 将 `content` / `mood` / `weather` / `city` / `temp_*` 改为可选（指针 + 统一 `DiaryPatch`），消除"局部更新清空正文"。
- 在工具 schema 中显式声明 `city` / `temp_min` / `temp_max`，移除 `has()` 隐式探测与 `\n` 转义 hack。
- 增加 `date` 格式与 `mood` 范围运行时校验。

**优先级 P1（核心批量能力）**
- 新增 `update_diary`（按 id 局部更新）、`batch_update_diaries`、`batch_delete_diaries`、`list_diaries`（可筛选分页）。
- 数据层实现单事务批量 + 合并 embedding 触发。

**优先级 P2（文本纠正 MCP 化）**
- 新增 `polish_diary` / `transcribe_audio` / `correct_voice_diary`，并把"语音整理"专用预设纳入 `polish` 的 mode。

**风险与注意**
- 批量操作必须沿用 `owner` 作用域（现有 store 的 WHERE 已带 owner），防止越权改他人日记。
- 批量响应只返回 per-item 状态摘要，不回传整条 diary，避免响应体过大；大批量按 `limit` 分批。
- `dry_run` 应作为批量修改的默认安全网，建议 Agent 工作流"先预览、再落库"。
