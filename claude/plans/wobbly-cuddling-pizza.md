# Mastra backend ディレクトリ構造修正

## Context

`mastra dev` CLI は `src/mastra/index.ts` をエントリーポイントとして期待する規約がある。現在の `backend` は `src/mastra.ts`、`src/agent.ts`、`src/tools/` というフラット構造になっており、Mastra の規約に合っていない。`example` ディレクトリの構造をリファレンスとして修正する。

## 修正内容

### 1. ディレクトリ構造を Mastra 規約に合わせる

**現在:**
```
src/
  mastra.ts          ← フラットファイル (削除済み、src/mastra/index.ts が既に存在)
  mastra/
    index.ts         ← 前回作成済み
  agent.ts
  tools/
    index.ts
    taskTools.ts
  skills/            ← デッドコード
    prioritize.ts
    summarize.ts
```

**修正後:**
```
src/
  mastra/
    index.ts         ← エントリーポイント (既存を修正)
    agents/
      task-agent.ts  ← src/agent.ts から移動
    tools/
      index.ts       ← src/tools/index.ts から移動
      taskTools.ts   ← src/tools/taskTools.ts から移動
```

### 2. 具体的な作業

| # | 作業 | ファイル |
|---|------|---------|
| 1 | `src/mastra.ts` を削除 (旧ファイル) | `src/mastra.ts` |
| 2 | `src/agent.ts` → `src/mastra/agents/task-agent.ts` に移動 | |
| 3 | `src/tools/` → `src/mastra/tools/` に移動 | |
| 4 | `src/skills/` を削除 (デッドコード) | |
| 5 | `src/mastra/index.ts` の import パスを修正 | `@mastra/core` → `@mastra/core/mastra` |
| 6 | `src/mastra/agents/task-agent.ts` の import パスを修正 | `@mastra/core/agent` はそのまま、tools の相対パス修正 |
| 7 | `src/mastra/index.ts` から agent の import パスを修正 | `../agent.js` → `./agents/task-agent.js` |
| 8 | `package.json` に `build` / `start` スクリプト追加 | |
| 9 | テストファイル `src/__tests__/tools.test.ts` の import パス修正 | |

### 3. import パス修正の詳細

**`src/mastra/index.ts`:**
```diff
- import { Mastra } from "@mastra/core";
- import { taskAgent } from "../agent.js";
+ import { Mastra } from "@mastra/core/mastra";
+ import { taskAgent } from "./agents/task-agent.js";
```

**`src/mastra/agents/task-agent.ts`:**
```diff
- import { createTaskTool, listTasksTool, updateTaskTool, deleteTaskTool } from "./tools/index.js";
+ import { createTaskTool, listTasksTool, updateTaskTool, deleteTaskTool } from "../tools/index.js";
```

**`src/__tests__/tools.test.ts`:**
- tools の import パスを `../mastra/tools/` に修正

### 4. package.json スクリプト追加

```diff
  "scripts": {
    "dev": "mastra dev",
+   "build": "mastra build",
+   "start": "mastra start",
    "test": "vitest run"
  },
```

## 検証

1. `docker compose up --build mastra` でサーバーが起動すること
2. フロントエンドからチャットメッセージを送信して応答が返ること
3. `Missing required file` エラーが出ないこと
