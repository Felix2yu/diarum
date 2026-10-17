import js from '@eslint/js';
import prettier from 'eslint-config-prettier';
import svelte from 'eslint-plugin-svelte';
import { defineConfig } from 'eslint/config';
import globals from 'globals';
import ts from 'typescript-eslint';

export default defineConfig(
  // 构建产物与依赖不参与检查。build/ 是 adapter-static 的真实输出目录，
  // 漏掉它会把压缩产物当源码扫出上千条假错误。
  {
    ignores: ['build/', 'dist/', 'node_modules/', '.svelte-kit/', '.svelte-check/']
  },
  js.configs.recommended,
  ts.configs.recommended,
  svelte.configs.recommended,
  prettier,
  svelte.configs.prettier,
  {
    languageOptions: { globals: { ...globals.browser, ...globals.node } },
    rules: {
      // 类型由 svelte-check 负责，no-undef 在 TS 项目里会误报全局类型
      'no-undef': 'off',
      // 存量放宽：这几类在既有代码里成片存在，一次性改造等于重写前端。
      // 降为 warn 保留可见性，新代码不应再引入。
      '@typescript-eslint/no-explicit-any': 'warn',
      'svelte/prefer-svelte-reactivity': 'warn',
      // 同属「legacy 语法待迁移」信号，不是当前缺陷：
      // diary/[date]/+page.svelte 是纯 Svelte 4 写法（0 runes、56 个普通 let），
      // 迁 $effect 需把这些 let 全部改 $state，属重写级改动。
      // 告警本身在迁移时会自然消失，故降为 warn。
      'svelte/infinite-reactive-loop': 'warn',
      // Svelte 专用：$bindable 的默认值会被父组件双向绑定改写，legacy `$:` 的
      // 「块内自写状态」也是框架的标准写法，两者都不是「无用赋值」。
      'no-useless-assignment': 'off',
      // `_` 是「有意丢弃」的约定写法（each 的占位变量、忽略的回调参数）
      '@typescript-eslint/no-unused-vars': [
        'error',
        { argsIgnorePattern: '^_', varsIgnorePattern: '^_', caughtErrors: 'none' }
      ]
    }
  },
  {
    // 两类文件必须降级到 base no-unused-vars，TS 版会崩或误报：
    //
    // 1. 仍用 legacy `$:` 的文件（7 个）
    //    svelte-eslint-parser 为 `$:` 产生的 ComputedVariable 定义类型不在
    //    @typescript-eslint 8 的 DefinitionType 枚举内，规则走到 defToVariableType
    //    返回 undefined 后取 .type 直接崩：
    //      TypeError: Cannot read properties of undefined (reading 'type')
    //      at getDefinedMessageData (no-unused-vars.js)
    //
    // 2. 使用 $props() 的文件（13 个）
    //    base 规则不认 TS 的类型标注，会把 `$props()` 解构出的 prop 连同其默认值
    //    一并判为「已定义但从未使用」——而这些 prop 恰恰是父组件在传。
    //    反过来 TS 版规则对这些文件是正常的，所以方向与第 1 类相反。
    //
    // 收窄到这两个集合而非全量 svelte：其余文件保持 TS 版规则，判定才准确。
    files: [
      // 1. legacy `$:`
      'src/lib/components/share/DiarySharePreview.svelte',
      'src/lib/components/share/DiaryShareModal.svelte',
      'src/routes/settings/+page.svelte',
      'src/routes/tags/+page.svelte',
      'src/routes/diary/**',
      'src/routes/media/+page.svelte',
      // 2. $props()
      'src/lib/components/CityPicker.svelte',
      'src/lib/components/DiaryVersionHistoryModal.svelte',
      'src/lib/components/TextPolisher.svelte',
      'src/lib/components/WeatherDisplay.svelte',
      'src/lib/components/editor/MediaPicker.svelte',
      'src/lib/components/editor/TiptapEditor.svelte',
      'src/lib/components/calendar/Calendar.svelte',
      'src/lib/components/calendar/CalendarAnalysis.svelte',
      'src/lib/components/calendar/CalendarYearPicker.svelte',
      'src/lib/components/ui/Footer.svelte',
      'src/lib/components/ui/MoodIcon.svelte',
      'src/lib/components/ui/TableOfContents.svelte',
      'src/routes/+layout.svelte'
    ],
    rules: {
      '@typescript-eslint/no-unused-vars': 'off',
      'no-unused-vars': ['error', { args: 'none', caughtErrors: 'none', varsIgnorePattern: '^_' }]
    }
  },
  {
    files: ['**/*.svelte', '**/*.svelte.ts', '**/*.svelte.js'],
    languageOptions: {
      parserOptions: {
        extraFileExtensions: ['.svelte'],
        parser: ts.parser
      }
    }
  }
);
