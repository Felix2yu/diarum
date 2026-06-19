# Changelog

本项目版本更新日志，按发布时间倒序排列。

---

## v1.4 - 2026-06-19

> 发布日期：2026-06-19
> 对应 commit：`671e092` — feat: AI日记分析支持自定义时间范围与关键词
> 对比基线：`v1.3 (479194f)` → `v1.4 (671e092)`，**11 个文件，+521 / -202 行**

### ✨ 新增 Features

- **AI 分析支持自定义时间区间** — 日历头部新增独立的「自定义分析」按钮，与周分析、月分析并列，可自由选择起止日期
- **关键词筛选** — 自定义分析支持逗号分隔的关键词，仅将命中关键词的日记交给模型，适用于主题式复盘（如"运动""心情""工作"）
- **提示词编辑器** — 在分析弹窗中可编辑传给模型的 system prompt，自定义分析风格；默认风格保持简洁
- **按时间正序投喂** — 提交给 AI 的日记内容由旧到新排列，并在系统提示词中明确引导关注日期维度与情绪/模式变化
- **历史分析列表** — 可在日历页查看所有已保存的 AI 分析（按周、月、自定义分类），打开任意一条可直接阅读或重新生成
- **页面标题全局居中** — 标题不再被左右导航按钮挤压，采用浮动层绝对居中布局

### 🔌 API 变更

- `POST /api/v1/ai/analysis` 新增 `period=custom` 支持，请求体新增 `customStart`、`customEnd`、`keywords` 字段
- 响应结构保持向后兼容：`{ period, startDate, endDate, keywords, summary, model }`
- `GET /api/v1/ai/analysis` 按用户返回所有已保存分析，供前端「历史分析」列表使用

### 🏗️ 数据模型 / 存储

- `period_analyses` 表新增 `keywords JSON DEFAULT '[]' NOT NULL`
- 新增唯一索引 `idx_period_analyses_user_period_start_keywords`，同一时间区间 + 关键词组合只保存一份最新结果
- 启动时自动迁移，缺失字段自动补默认值

### 🎨 前端界面调整

- [CalendarAnalysis.svelte](file:///workspace/site/src/lib/components/calendar/CalendarAnalysis.svelte) — 筛选面板与提示词编辑器条件渲染，容器全局滚动
- [Calendar.svelte](file:///workspace/site/src/lib/components/calendar/Calendar.svelte) — 头部新增自定义分析按钮
- [PageHeader.svelte](file:///workspace/site/src/lib/components/ui/PageHeader.svelte) — 标题改为绝对居中漂浮层
- [ai.ts](file:///workspace/site/src/lib/api/ai.ts) — `createAnalysis` 扩展自定义区间与关键词字段

### 📁 主要变更文件

- [diarum.go](file:///workspace/diarum.go) — `Version` 升级为 `v1.4`
- [CHANGELOG.md](file:///workspace/CHANGELOG.md) — 本文件
- [internal/api/ai.go](file:///workspace/internal/api/ai.go) — custom period + keywords 支持，时间正序，系统提示词优化
- [internal/store/store.go](file:///workspace/internal/store/store.go) — `period_analyses` 表结构升级
- [site/src/lib/api/ai.ts](file:///workspace/site/src/lib/api/ai.ts) — 前端 API 类型扩展
- [site/src/lib/components/calendar/Calendar.svelte](file:///workspace/site/src/lib/components/calendar/Calendar.svelte) — 自定义分析入口
- [site/src/lib/components/calendar/CalendarAnalysis.svelte](file:///workspace/site/src/lib/components/calendar/CalendarAnalysis.svelte) — 筛选 + 提示词 + 全局滚动
- [site/src/lib/components/ui/PageHeader.svelte](file:///workspace/site/src/lib/components/ui/PageHeader.svelte) — 标题绝对居中
- [site/src/routes/diary/[date]/+page.svelte](file:///workspace/site/src/routes/diary/[date]/+page.svelte) — 日期导航整行居中
- [site/src/routes/assistant/+page.svelte](file:///workspace/site/src/routes/assistant/+page.svelte) / [[id]/+page.svelte](file:///workspace/site/src/routes/assistant/[id]/+page.svelte) — 容器与间距微调
- [site/src/routes/search/+page.svelte](file:///workspace/site/src/routes/search/+page.svelte) / [settings/+page.svelte](file:///workspace/site/src/routes/settings/+page.svelte) — 容器与间距微调

---

## v1.3 - 2026-06-19

> 发布日期：2026-06-19
> 对应 commit：`131a141` — feat: 新增 AI 文本整理功能并优化星期显示 (#17)
> 对比基线：`v1.2 (685adc5)` → `v1.3 (131a141)`

### ✨ 新增 Features

- **AI 文本整理（TextPolisher）** — 在日记编辑页新增「AI 整理」入口，支持 3 种整理模式：中等（去语气词、纠错、自动分段）、强力（重组句子、精简冗余）、自定义（自行编写 system prompt）
- **双栏对比与一键回写** — 整理结果在右侧栏与原文并排展示字数对比，可一键应用到当前日记，保留上下文
- **AI 后端端点 `/api/v1/ai/polish`** — 支持传入 `mode` 与可选 `customPrompt`，返回结构化的 `PolishResult { content, model }`，并在用户未配置 AI Key / 端点时返回友好错误

### 🎨 前端界面 / 交互调整

- **日历星期表头统一为「一二三四五六日」** — [Calendar.svelte](file:///workspace/site/src/lib/components/calendar/Calendar.svelte) 与 [date.ts](file:///workspace/site/src/lib/utils/date.ts) 保持一致，告别周一/周日的显示不一致
- **`getCalendarDays` / `getWeekRange` 统一以周一为首** — 日历 padding 日与周范围计算均采用 `(day + 6) % 7` 偏移，保证周视图、年视图、日历视图对齐
- **日记编辑页工具栏新增 `TextPolisher` 模态框** — [routes/diary/[date]/+page.svelte](file:///workspace/site/src/routes/diary/[date]/+page.svelte) 从正文中截取纯文本作为 `sourceText`，整理完毕后回写

### 🔌 API 变更

- 新增 `POST /api/v1/ai/polish` — 接收 `{ text, mode, customPrompt? }`，返回 `{ content, model }`
- `GET /api/v1/version` / `GET /api/version` 保持向后兼容（v1.3 版本号在构建时注入）

### 📁 主要变更文件

- [CHANGELOG.md](file:///workspace/CHANGELOG.md) — 新增版本更新日志文件
- [.dockerignore](file:///workspace/.dockerignore) — 排除 CHANGELOG.md 及文档目录
- [diarum.go](file:///workspace/diarum.go) — `Version` 升级为 `v1.3`
- [internal/api/ai.go](file:///workspace/internal/api/ai.go) — 新增 `/polish` 端点
- [site/src/lib/components/TextPolisher.svelte](file:///workspace/site/src/lib/components/TextPolisher.svelte) — AI 文本整理模态组件
- [site/src/lib/api/ai.ts](file:///workspace/site/src/lib/api/ai.ts) — 新增 `polishText()` 与 `PolishResult`
- [site/src/lib/components/calendar/Calendar.svelte](file:///workspace/site/src/lib/components/calendar/Calendar.svelte) — 星期表头「一二三四五六日」
- [site/src/lib/utils/date.ts](file:///workspace/site/src/lib/utils/date.ts) — 周/日历统一以周一为首
- [site/src/routes/diary/[date]/+page.svelte](file:///workspace/site/src/routes/diary/[date]/+page.svelte) — 接入 TextPolisher

---

## v1.2 - 2026-06-18

> 发布日期：2026-06-18
> 对应 commit：`685adc5` — feat: 日记多标签与标签云功能 (#15)
> 对比基线：`v1.1 (4c9d6c1)` → `v1.2 (685adc5)`，**22 个文件，+823 / -102 行**

### ✨ 新增 Features

- **日记多标签支持** — 每条日记可添加任意数量的标签（支持中英文逗号分隔一次输入多个，自动去空格/去空串/去重）
- **标签云页面（`/tags`）** — 汇总所有日记标签，按出现次数动态缩放字号（0.85rem → 1.8rem），每个标签右侧显示数字徽标
- **按标签筛选日记** — 在标签云页面点击某个标签，下方即列出带有该标签的所有日记（日期、正文 snippet、标签、心情/天气）
- **标签搜索支持** — 搜索关键词同时命中日记正文与标签文本，匹配的标签在搜索结果中以主色徽章高亮
- **日记详情页标签管理** — 在 `[date]` 页直接输入标签，Enter/逗号提交、Backspace 空串时删除最后一个；标签随日记一并保存、读取时自动回填

### 🔌 API 变更

- 新增 `GET /api/v1/diaries/tags` — 返回 `{ tags: [{tag, count}], total }`
- 新增 `GET /api/v1/diaries/by-tag/:tag` — 返回指定标签下的全部日记
- `/api/v1/diaries`、`/api/v1/diaries/:date`、`/api/v1/diaries/search` 的返回对象新增 `tags: string[]` 字段
- `POST /api/public/diary`、`PUT /api/public/diary/:id` 的请求体新增 `tags: string[]`（`nil` 时默认 `[]`，更新时 `nil` 保留原有标签）
- 前端 `getTagCloud()`、`getDiariesByTag()`、`TagCount` 类型已在 [diaries.ts](file:///workspace/site/src/lib/api/diaries.ts) 中补齐

### 🏗️ 数据模型 / 存储

- `diaries` 表的 `tags` 字段由内部字符串升级为 JSON 数组（`tags JSON DEFAULT '[]' NOT NULL`），迁移时 `COALESCE(tags, '[]')` 保证向前兼容
- 新增 `tags` 表（`id`, `name`, `owner`, `created`, `updated`），建立联合唯一索引 `idx_tags_name_owner`
- 新增 `normalizeTags()`、`ListTagCounts()`、`ListDiariesByTag()` 等辅助函数；搜索条件扩展为 `content LIKE ? OR tags LIKE ?`
- 受函数签名变化牵连，多处调用 `UpsertDiary` / `InsertImportedDiary` 时末位补充 `nil`（空标签）参数

### 🎨 前端界面调整

- [Calendar.svelte](file:///workspace/site/src/lib/components/calendar/Calendar.svelte) 日历头部与日期格的 Tailwind 类拆为独立 `.weekdays-grid` / `.days-grid`，最大宽度限制为 `600px`，避免 2K 屏上被撑爆
- [TableOfContents.svelte](file:///workspace/site/src/lib/components/ui/TableOfContents.svelte) 顶部新增"标签"区块，支持已添加标签的徽章展示与删除按钮
- [search/+page.svelte](file:///workspace/site/src/routes/search/+page.svelte) 搜索结果项补充 `mood` / `weather` / `tags` 字段的可视化

---

## v1.1 - 2026-06-18

> 发布日期：2026-06-18
> 对应 commit：`4c9d6c1` — Merge pull request #14: 引入统一响应式布局系统并适配大屏
> 对比基线：`v1.0 (78ef202)` → `v1.1 (4c9d6c1)`，**12 个文件，+143 / -118 行**

### ✨ 新增 Features

- **统一响应式布局系统** — 在 [app.css](file:///workspace/site/src/app.css) 中新增全局响应式容器、流体字体与阅读宽度上限，使界面在 1280px / 1600px / 1920px / 2560px 下逐级放宽，告别 `max-w-6xl` 的超宽屏留白
- **Tiptap 编辑器撑满视口** — `routes/diary/[date]/+page.svelte` 改为纵向 flex 布局，编辑器占满剩余可视高度，告别"半截编辑器"
- **日历↔日记往返闭环** — 日历页支持 `?date=YYYY-MM-DD` 查询参数，从日记返回日历时自动聚焦到该日期所在月
- **登录页视觉放大** — 卡片 `max-w-md` → `max-w-lg`，Logo 与标题整体放大，响应式 padding `p-4 sm:p-8`
- **导航/页脚统一尺寸** — PageHeader 高度 `h-11 → h-14`，点击区从 `p-1.5 → p-2`；所有页面统一使用 `.container-responsive`，移除逐页定制的宽度管理
- **"自适应"主题文案** — ThemeToggle 将"跟随系统"改为"自适应"，配合简化图标，语义更直观

### 🔧 重构 / 清理

- `Footer.svelte` 由 75 行大幅精简到 25 行，移除 `onMount` + `fetchVersion()` 的版本号展示逻辑，改用全局响应式容器托管宽度
- 多个路由页面统一替换 `max-w-6xl mx-auto px-4 ...` / `max-w-5xl mx-auto ...` 为 `.container-responsive`，减少重复样式维护
- `TiptapEditor.svelte` 新增 `flex: 1 1 auto; height: 100%; display: flex; flex-direction: column`，编辑器 `resize: vertical` → `resize: none`

### 📐 响应式断点一览

| 断点 | 容器最大宽度 | 流体字体基线 |
|------|------------|----------|
| 默认 | 72rem | `clamp(15px, 0.92vw + 8px, 20px)` |
| ≥1280px | 80rem | 同上 |
| ≥1600px | 100rem | 同上 |
| ≥1920px | 120rem | `clamp(17px, ...)` |
| ≥2560px | 150rem | `clamp(19px, ...)` |

### 📁 主要变更文件

- [app.css](file:///workspace/site/src/app.css) — 响应式容器 / 流体字体 / 阅读宽度
- [Footer.svelte](file:///workspace/site/src/lib/components/ui/Footer.svelte) — 精简 + 容器替换
- [PageHeader.svelte](file:///workspace/site/src/lib/components/ui/PageHeader.svelte) — 尺寸调整
- [ThemeToggle.svelte](file:///workspace/site/src/lib/components/ui/ThemeToggle.svelte) — 文案与图标
- [TiptapEditor.svelte](file:///workspace/site/src/lib/components/editor/TiptapEditor.svelte) — 高度自适应
- [routes/+page.svelte](file:///workspace/site/src/routes/+page.svelte) / [diary/+page.svelte](file:///workspace/site/src/routes/diary/+page.svelte) / [diary/[date]/+page.svelte](file:///workspace/site/src/routes/diary/[date]/+page.svelte) / [login/+page.svelte](file:///workspace/site/src/routes/login/+page.svelte) / [media/+page.svelte](file:///workspace/site/src/routes/media/+page.svelte) / [search/+page.svelte](file:///workspace/site/src/routes/search/+page.svelte) / [settings/+page.svelte](file:///workspace/site/src/routes/settings/+page.svelte) — 统一容器替换

---

## v1.0 - 2026-06-17

> 发布日期：2026-06-17
> 对应 commit：`78ef202` — Update README.md
> 说明：Diarum 的首个正式版本，共 **132 个文件**，以下为该版本提供的完整功能清单

### ✨ 核心功能

- **每日一篇日记** — 基于日期 `YYYY-MM-DD` 索引，打开即跳转到今日日记，告别选择焦虑
- **Markdown 编辑与渲染** — 使用 Tiptap 富文本编辑器，支持标准 Markdown；日记正文与 AI 分析结果均渲染为 Markdown
- **媒体上传** — 为日记添加图片和文件；支持本地存储或 Chevereto 图床托管，可在设置页灵活切换
- **渐进式 Web 应用（PWA）** — 可安装为桌面/移动应用，离线可用，具备自动缓存与同步
- **一键分享** — 轻点即可生成日记分享链接
- **Memos Webhook 同步** — 接收 Memos 的新增 / 更新 / 删除事件，并回写到事件当日日记
- **AI 周期分析报告** — 配置 AI Key 后，支持对整周 / 整月日记自动生成分析报告
- **RAG 向量对话** — 日记自动向量化，可基于历史日记与 AI 对话，轻松完成今日复盘、周报、年终总结
- **全界面汉化** — 所有界面（日期格式、星期、月份、按钮文案等）均符合中文用户习惯
- **用户体系与权限** — 内置鉴权，支持多用户，数据按 owner 隔离
- **设置页** — 图床、AI、PWA、账号、外观主题等配置
- **搜索** — 按关键词搜索日记正文
- **标签页** — 独立的标签管理入口（v1.0 为基础版本，标签增强功能见 v2.3）
- **日历视图** — 按月查看日记写作分布与心情/天气概览
- **导入导出** — 支持将日记数据导出并在不同实例间迁移

### 🔧 技术栈

- 后端：Go 1.22+ + Echo（PocketBase 抽象层）
- 前端：Svelte + Vite + TypeScript + Tailwind CSS
- 数据库：SQLite（`diarum.db`，内置用户、日记、媒体、设置与 AI 对话数据迁移）
- 单二进制发布：前端资源通过 `embed` 直接打入二进制
- Docker：`ghcr.io/felix2yu/diarum:latest`

### 📦 部署

- `docker run -d -p 8090:8090 ghcr.io/felix2yu/diarum:latest`
- 数据目录：默认 `./pb_data`，可通过 `./diarum serve --data-dir=/custom/path` 或 `DIARUM_DATA_PATH` 覆盖
- 启动时若检测到旧版 `data.db` 且尚不存在 `diarum.db`，会自动迁移用户、日记、媒体元数据、设置与 AI 对话数据

### 📁 关键目录 / 文件

- [diarum.go](file:///workspace/diarum.go) — 应用入口与版本定义
- [main.go](file:///workspace/main.go) — `serve` / `version` 等命令行处理
- [internal/api/](file:///workspace/internal/api/) — 后端 API（日记、用户、AI、媒体、导入导出、OpenAPI 等）
- [internal/auth/](file:///workspace/internal/auth/) — 鉴权
- [internal/chat/](file:///workspace/internal/chat/) / [internal/embedding/](file:///workspace/internal/embedding/) — AI 对话与向量化
- [internal/config/](file:///workspace/internal/config/) / [internal/store/](file:///workspace/internal/store/) — 配置与数据存储
- [internal/static/](file:///workspace/internal/static/) — 前端静态资源嵌入
- [site/src/](file:///workspace/site/src/) — 前端 Svelte 组件与路由
