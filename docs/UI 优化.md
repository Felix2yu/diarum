# UI 优化文档

## 概述

本文档记录 Diarum 前端 UI 的优化内容，包括动画、交互、布局和视觉改进。

## 全局动画系统

### 新增动画

| 动画名 | 效果 | 用途 |
| :--- | :--- | :--- |
| `fade-in` | 淡入 + 上移 | 页面元素入场 |
| `slide-up` | 上滑淡入 | 弹窗、面板入场 |
| `scale-in` | 缩放淡入 | 模态框入场 |
| `shimmer` | 骨架屏闪烁 | 加载状态 |

### 交错动画

使用 `stagger-1` 到 `stagger-8` 工具类实现列表项错位入场效果。

## 页面优化

### 首页（Landing）
- 基于 IntersectionObserver 的滚动触发动画
- Feature 卡片 hover 微缩放
- AI 预览区左右分列动画

### 登录页
- Tab 切换改用滑块指示器
- 输入框/按钮统一 `rounded-xl`
- 按钮增加 `active:scale-[0.98]` 微缩放反馈

### 日记编辑器
- 编辑器卡片增加 `ring` 光晕
- 移动端面板增加 slide-up 入场动画
- 标签输入支持自动补全

### AI 助手
- 空状态改用渐变背景图标
- 聊天输入框/消息气泡样式统一

### 日历页
- 统计卡片增加 hover 阴影
- 最近条目 hover 边框增强
- 分析按钮支持横向滚动

### 标签云
- 改用圆角矩形卡片风格
- 支持按热度/名称排序
- 选中标签增加 ring 光晕

### 媒体库
- 网格圆角 `rounded-xl`
- 渐变覆盖层替代纯色
- 详情弹窗阴影增强

### 弹窗系统
- 往昔今朝/时空穿越/分析弹窗统一使用 portal 挂载
- 背景遮罩统一为 `hsl(0 0% 0% / 0.5)` + `backdrop-filter: blur(8px)`
- z-index 提升至 `2147483647`

## iOS 修复

### 页脚遮挡
- 移除页面容器 `safe-bottom`，改由 Footer 自身处理安全区内边距

### 弹窗穿透
- 背景遮罩从半透明主题色改为纯黑半透明
- 添加 `-webkit-backdrop-filter` 兼容前缀

### 按钮换行
- 分析按钮添加 `whitespace-nowrap` + `shrink-0`
- 容器添加 `overflow-x-auto` 横向滚动

## Svelte 5 语法迁移

全量替换所有组件中的旧事件语法：
- `on:click` → `onclick`
- `on:keydown` → `onkeydown`
- `on:input` → `oninput`
- `on:submit|preventDefault` → `onsubmit={(e) => { e.preventDefault(); ... }}`
- `on:click|stopPropagation` → `onclick={(e) => { e.stopPropagation(); ... }}`
