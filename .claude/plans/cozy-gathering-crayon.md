# mastra-fastify の agents と skills を mastra に合わせて修正

## Context
mastra-fastify の agent 定義と skills 実装が mastra 本体と異なるアプローチで実装されている。mastra の実装を正として統一する。mastra-fastify は Fastify サーバーを使う点は維持するが、agent 設定・skills 形式を揃える。

## 変更内容

### 1. `agent.ts` の修正
**ファイル**: `04_agent/mastra-fastify/backend/src/agent.ts`

- Agent に `id: "task-agent"` を追加
- `bedrock()` を `createAmazonBedrock()` に変更し、credential provider chain を実装
- `workspace` を追加（workspace.ts からインポート）
- `memory` を追加（PostgresStore）
- `@aws-sdk/credential-providers` を依存関係に追加

### 2. `workspace.ts` の新規作成
**ファイル**: `04_agent/mastra-fastify/backend/src/workspace.ts`

mastra の `agents/workspace.ts` と同じ内容で作成:
- `LocalFilesystem` + `BM25` 検索
- `import.meta.url` ベースのパス解決
- skills ディレクトリは `../skills` を参照（backend/skills/ を指すように調整）

### 3. skills を Markdown 形式に変更
**削除**: `04_agent/mastra-fastify/backend/src/skills/prioritize.ts`, `summarize.ts`

**新規作成**:
- `04_agent/mastra-fastify/backend/skills/summarize/SKILL.md` (mastra からコピー)
- `04_agent/mastra-fastify/backend/skills/prioritize/SKILL.md` (mastra からコピー)

### 4. `index.ts` の修正
**ファイル**: `04_agent/mastra-fastify/backend/src/index.ts`

- skills の TypeScript import を削除
- キーワードマッチによる直接スキル呼び出しロジックを削除
- 全メッセージを agent.generate() に委譲（agent が workspace 経由で skills を利用）

### 5. `package.json` の依存関係追加
**ファイル**: `04_agent/mastra-fastify/backend/package.json`

追加:
- `@mastra/memory`
- `@mastra/pg`
- `@aws-sdk/credential-providers`

### 6. テストの更新
**ファイル**: `04_agent/mastra-fastify/backend/src/__tests__/tools.test.ts`

tools.test.ts はそのまま維持（taskTools のCRUDテストは変更なし）

## 検証方法
1. `cd 04_agent/mastra-fastify/backend && pnpm install`
2. `pnpm test` でテストが通ることを確認
3. Docker Compose でビルドが通ることを確認
