# README.md更新 + VoltAgent実装プラン

## Context

04_agentのREADME.mdに以下2点を追加する:
1. **Observability（可観測性）の観点**を比較に追加
2. **VoltAgent**を比較対象に追加（実装サンプル付き、ポート4006/5006）

## 変更一覧

### 1. VoltAgent バックエンド実装

**ディレクトリ:** `04_agent/voltagent/backend/`

#### `package.json`
- `@voltagent/core`, `@voltagent/server-hono`, `@ai-sdk/amazon-bedrock`, `zod`, `uuid`
- VoltAgentはVercel AI SDK上に構築されているため、Bedrockは`@ai-sdk/amazon-bedrock`で対応
- dev: `tsx watch`, start: `tsx`

#### `src/agent.ts`
- `createAmazonBedrock()(modelId)` でBedrockモデルインスタンスを生成
- `createTool()` + Zodスキーマで4つのタスクツールを定義
- `Agent` インスタンスを作成し、エクスポート

#### `src/index.ts`
- `VoltAgent` + `honoServer({ port: 4006 })` で起動
- VoltAgent組み込みHTTPサーバー（Hono）を使用
- 自動生成エンドポイント: `POST /agents/:id/text`

#### `src/tools/taskTools.ts`
- 既存の`strands-typescript`と同じロジック（Task型、CRUD操作）を再利用

#### `src/tools/index.ts`
- `createTool()` で4つのツールを定義（VoltAgentのAPI）

#### `tsconfig.json`, `Dockerfile`
- 既存フレームワークと同じパターン

### 2. VoltAgent フロントエンド実装

**ディレクトリ:** `04_agent/voltagent/frontend/`

既存フレームワークと同じReact + Vite構成だが、APIコールを VoltAgent形式に変更:
- エンドポイント: `POST ${BACKEND_URL}/agents/task-agent/text`
- リクエストボディ: `{ input: "message" }`
- レスポンス: `{ success: true, data: { text: "..." } }`

ファイル一覧: `package.json`, `src/App.tsx`, `src/main.tsx`, `index.html`, `vite.config.ts`, `tsconfig.json`, `Dockerfile`

### 3. Docker Compose更新

`04_agent/docker-compose.yml` に追加:
- `voltagent` サービス（port 4006、profile: voltagent）
- `voltagent-frontend` サービス（port 5006、VITE_THEME_COLOR: #10b981、profile: voltagent）
- AWS認証: `${HOME}/.aws:/root/.aws:ro` + `AWS_REGION`/`AWS_PROFILE`

### 4. Makefile更新

`04_agent/Makefile` に追加:
- `voltagent:` ターゲット
- `all` / `down` に `--profile voltagent` 追加

### 5. README.md更新

`04_agent/README.md`:

#### 5a. 比較表の更新
- **新しい列**: `voltagent`
- **新しい行**: `Observability`

比較表に追加するVoltAgent値:
| 観点 | 値 |
|------|-----|
| 言語 | TypeScript |
| HTTPサーバー | VoltAgent組み込み（Hono） |
| バックエンドポート | 4006 |
| フロントエンドポート | 5006 |
| ツール定義 | `createTool()` + zodスキーマ |
| スキル管理 | Toolkitsでグループ化 |
| メモリ管理 | 6プロバイダー対応（InMemory, PostgreSQL等）+ Semantic Search |
| モデル設定 | `createAmazonBedrock()(modelId)`（AI SDK経由） |
| Agent構築 | `new VoltAgent({agents, server})` |
| ツール実行 | フレームワーク自動 |
| 型安全性 | ✅ 高い（Zod） |
| セットアップ難易度 | ★★☆ |

Observability行（全フレームワーク）:
| mastra/mastra-fastify | strands-python/typescript | claude-agent-sdk | voltagent |
|---|---|---|---|
| Mastra Studio/Cloud + OTel + Langfuse | OTelネイティブ + Jaeger/Langfuse | Hooks経由で自作 | VoltOps Console + Langfuse + MLflow |

#### 5b. Observability比較セクション新設

「## 比較表」と「## 各フレームワークの詳細」の間に挿入:

```
## Observability比較
```

詳細比較表:

| 機能 | mastra | strands | claude-agent-sdk | voltagent |
|---|:---:|:---:|:---:|:---:|
| **ビルトイントレーシング** | ✅ | ✅ (OTelネイティブ) | ❌ (手動) | ✅ (VoltOps) |
| **OpenTelemetry対応** | ✅ (GenAI Conventions) | ✅ (ネイティブ) | ❌ | ✅ |
| **トークン使用量追跡** | ✅ 自動 | ✅ 自動(キャッシュ含む) | ❌ | ✅ |
| **専用ダッシュボード** | Mastra Studio / Cloud | なし (外部ツール) | なし | VoltOps Console |
| **評価 (Evals)** | Mastra Scorers | GenAI Evaluation | ❌ | ✅ |
| **機密データマスキング** | ✅ SensitiveDataFilter | ❌ | ❌ | ❌ |

対応連携先（フレームワーク別）:

- **mastra**: Langfuse / MLflow / Braintrust / Datadog / New Relic / SigNoz / Dash0 / Traceloop / Laminar + 任意OTel互換プラットフォーム
- **strands**: Langfuse / Jaeger / AWS X-Ray / Zipkin / Opik / Grafana Tempo / Datadog / Arize AI + 任意OTel互換プラットフォーム
- **claude-agent-sdk**: コミュニティツール（claude_telemetry）経由で Logfire / Sentry / Honeycomb / Datadog
- **voltagent**: Langfuse / MLflow + 任意OTel互換プラットフォーム

各フレームワークの概要説明（3-4行ずつ）を追記。

#### 5c. VoltAgent詳細セクション追加

`claude-agent-sdk`セクションの後に追加:
- パッケージ: `@voltagent/core` + `@voltagent/server-hono` + `@ai-sdk/amazon-bedrock`
- 特徴: 6項目
- 参照ファイルリンク
- 向いているユースケース

## 作成ファイル一覧

```
04_agent/voltagent/
  backend/
    Dockerfile
    package.json
    tsconfig.json
    src/
      agent.ts
      index.ts
      tools/
        index.ts
        taskTools.ts
      __tests__/
        tools.test.ts
  frontend/
    Dockerfile
    index.html
    package.json
    tsconfig.json
    vite.config.ts
    src/
      App.tsx
      main.tsx
```

## 編集ファイル一覧

- `04_agent/docker-compose.yml`
- `04_agent/Makefile`
- `04_agent/README.md`

## 検証方法

1. `cd 04_agent && make voltagent` でDocker Composeが起動できるか確認
2. `http://localhost:4006` でVoltAgentバックエンドが応答するか
3. `http://localhost:5006` でフロントエンドが表示されるか
4. README.mdのマークダウンテーブルが正しくレンダリングされるか
