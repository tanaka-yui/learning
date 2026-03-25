# AI Agent フレームワーク比較

タスク管理Agentを5つのフレームワークで実装した結果の比較。

## 比較表

| 観点 | mastra | mastra-fastify | strands-python | strands-typescript | claude-agent-sdk | voltagent |
|------|--------|----------------|----------------|--------------------|------------------|-----------|
| **言語** | TypeScript | TypeScript | Python | TypeScript | TypeScript | TypeScript |
| **HTTPサーバー** | Mastra組み込み（Hono） | Fastify（手動） | FastAPI（手動）| Fastify（手動） | Fastify（手動） | VoltAgent組み込み（Hono） |
| **バックエンドポート** | 4001 | 4002 | 4003 | 4004 | 4005 | 4006 |
| **フロントエンドポート** | 5001 | 5002 | 5003 | 5004 | 5005 | 5006 |
| **ツール定義** | `createTool()` + zodスキーマ | `createTool()` + zodスキーマ | `@tool` デコレータ | `tool()` + zodスキーマ | 手動JSONスキーマ | `createTool()` + zodスキーマ |
| **スキル管理** | `Workspace` + `LocalFilesystem` | `Workspace` + `LocalFilesystem` | `AgentSkills` プラグイン | 手動ファイル読み込み | 手動ファイル読み込み | Toolkitsでグループ化 |
| **メモリ管理** | `@mastra/memory` + PostgreSQL | `@mastra/memory` + PostgreSQL | なし（AgentCore Memory必須） | なし（AgentCore Memory必須） | なし | 6プロバイダー対応（InMemory / PostgreSQL / LibSQL等）+ Semantic Search |
| **モデル設定** | `createAmazonBedrock()(modelId)` | `createAmazonBedrock()(modelId)` | `BedrockModel(model_id=...)` | `new BedrockModel({modelId: ...})` | `new AnthropicBedrock()` | `createAmazonBedrock()(modelId)`（AI SDK経由） |
| **Agent構築** | `new Mastra({server, agents})` | `new Mastra({agents})` + Fastify | `Agent(model=..., tools=[...], plugins=[...])` | `new Agent({model, tools, systemPrompt})` | 手動ループ実装 | `new VoltAgent({agents, server})` |
| **ツール実行** | フレームワーク自動 | フレームワーク自動 | フレームワーク自動 | フレームワーク自動 | 手動ループ | フレームワーク自動 |
| **型安全性** | ✅ 高い（Zod） | ✅ 高い（Zod） | ✅ Python型ヒント | ✅ 高い（Zod） | ⚠️ 中（手動キャスト） | ✅ 高い（Zod） |
| **セットアップ難易度** | ★★☆ | ★★★ | ★★☆ | ★★☆ | ★☆☆ | ★★☆ |
| **Observability** | Mastra Studio / Cloud + OTel + Langfuse等 | Mastra Studio / Cloud + OTel + Langfuse等 | OTelネイティブ + Jaeger / Langfuse等 | OTelネイティブ + Jaeger / Langfuse等 | Hooks経由で自作 | VoltOps Console + Langfuse + MLflow |

## Observability比較

AIエージェントの実行トレース・モニタリング・デバッグに関するフレームワークごとの対応状況。

| 機能 | mastra | strands | claude-agent-sdk | voltagent |
|---|:---:|:---:|:---:|:---:|
| **ビルトイントレーシング** | ✅ | ✅（OTelネイティブ） | ❌（手動） | ✅（VoltOps） |
| **OpenTelemetry対応** | ✅（GenAI Conventions準拠） | ✅（ネイティブ） | ❌ | ✅ |
| **トークン使用量追跡** | ✅ 自動 | ✅ 自動（キャッシュトークン含む） | ❌ | ✅ |
| **専用ダッシュボード** | Mastra Studio / Mastra Cloud | なし（外部ツール） | なし | VoltOps Console |
| **評価（Evals）** | Mastra Scorers（3種類） | GenAI Evaluation | ❌ | ✅ |
| **機密データマスキング** | ✅ SensitiveDataFilter | ❌ | ❌ | ❌ |

### 対応連携先

- **mastra**: Langfuse / MLflow / Braintrust / Datadog / New Relic / SigNoz / Dash0 / Traceloop / Laminar + 任意OTel互換プラットフォーム
- **strands**: Langfuse / Jaeger / AWS X-Ray / Zipkin / Opik / Grafana Tempo / Datadog / Arize AI + 任意OTel互換プラットフォーム
- **claude-agent-sdk**: コミュニティツール（[claude_telemetry](https://github.com/TechNickAI/claude_telemetry)）経由で Logfire / Sentry / Honeycomb / Datadog
- **voltagent**: Langfuse / MLflow + 任意OTel互換プラットフォーム

### 各フレームワークの特徴

**mastra** — Observabilityが最も統合的。Mastra Studio（ローカル）/Mastra Cloud（本番）という専用UIを持ち、公式でLangfuse・OTelエクスポーターをサポート。評価（Evals）機能も内蔵し、機密データの自動マスキングも提供する。

**strands** — OpenTelemetryネイティブ統合が最も深い。`Agent Span → Cycle Span → LLM Span → Tool Span` の階層トレースとキャッシュトークンを含む詳細なメトリクスが特徴。専用UIはなく、Jaeger等の外部ツールと組み合わせて使う。

**claude-agent-sdk** — ビルトインObservabilityなし。Hooksシステムとメッセージストリームで手動モニタリングするアプローチ。コミュニティツール `claude_telemetry` でOTel互換基盤への統合が可能。

**voltagent** — VoltOps Consoleで実行トレース・パフォーマンスをリアルタイム可視化。Langfuse / MLflow との公式連携も提供。

## 各フレームワークの詳細

### mastra

**パッケージ:** `@mastra/core` + `@mastra/memory` + `@mastra/pg` + `@ai-sdk/amazon-bedrock`

**特徴:**
- Mastra の**組み込みHTTPサーバー（Hono）**を使用 — Fastify などを手動でセットアップする必要がない
- `new Mastra({ agents, server })` だけでHTTPサーバーとルーティングが完結する
- `createTool()` でZodスキーマベースのツールを定義
- `Workspace` + `LocalFilesystem` でスキルディレクトリを管理し、Agentに渡す
- `Memory` + `PostgresStore`（`@mastra/pg`）で会話履歴をPostgreSQLに自動永続化
- AWS Bedrock経由で `createAmazonBedrock()` を使用し、クレデンシャルチェーンで認証

→ [`mastra/backend/src/mastra/index.ts`](./mastra/backend/src/mastra/index.ts) / [`mastra/backend/src/mastra/agents/task-agent.ts`](./mastra/backend/src/mastra/agents/task-agent.ts)

**向いているユースケース:**
- TypeScriptネイティブでAgentを構築し、HTTPサーバー管理もMastraに任せたい場合
- サーバー設定のボイラープレートを最小化したい場合
- PostgreSQLを既に使っているプロジェクトへの統合

---

### mastra-fastify

**パッケージ:** `@mastra/core` + `@mastra/memory` + `@mastra/pg` + `@ai-sdk/amazon-bedrock`（mastraと同一、追加で `fastify`）

**特徴:**
- `Mastra` クラスにAgentを登録しつつ、HTTPサーバーは**Fastifyで手動セットアップ**
- スキル管理・メモリ管理はmastraと同じ（`Workspace` + PostgreSQL）
- `mastra.getAgent()` でAgentを取得し、Fastifyルートハンドラーから呼び出す
- `Agent` を `Mastra` インスタンスで管理することで、複数Agentのオーケストレーションが可能

→ [`mastra-fastify/backend/src/agent.ts`](./mastra-fastify/backend/src/agent.ts) / [`mastra-fastify/backend/src/index.ts`](./mastra-fastify/backend/src/index.ts)

**向いているユースケース:**
- 既存のFastifyアプリケーションにAgentを組み込む場合
- Socket.IOなどのリアルタイム機能と組み合わせたい場合
- HTTPサーバーの挙動を細かく制御したい場合
- ElectronやNext.js API Routesなど、Agent機能のみを組み込みたい場合

---

### strands-python

**パッケージ:** `strands-agents` + `boto3` + `strands-agents[bedrock]`

**特徴:**
- `@tool` デコレータで関数をツールとして登録（Pythonらしい書き方）
- `BedrockModel(model_id=..., boto_session=...)` でAWS Bedrockを設定
- `AgentSkills(skills=str(skills_dir))` プラグインでスキルディレクトリを自動管理
- 呼び出しは `agent("message")` → `AgentResult`、`str(result)` でテキスト取得
- **メモリなし（ステートレス）** — strands-agents自体はメモリ機能を持たない。会話履歴の永続化には [Amazon Bedrock AgentCore Memory](https://aws.amazon.com/bedrock/agentcore/) との統合が必要

→ [`strands-python/backend/agent.py`](./strands-python/backend/agent.py)

**向いているユースケース:**
- Pythonエコシステムを活用したAgent開発
- `@tool` デコレータによる直感的なツール定義を好む場合
- データサイエンス・ML系のツールとの統合

---

### strands-typescript

**パッケージ:** `@strands-agents/sdk`

**特徴:**
- `tool()` ファクトリ関数でZodスキーマベースのツールを定義（Python版の `@tool` に対応）
- `BedrockModel` が `@strands-agents/sdk/bedrock` から利用可能
- `Agent.invoke(message)` → `AgentResult`、`result.toString()` でテキスト取得
- スキルはSDKのプラグイン機能ではなく、サーバー側で手動ファイル読み込みにより適用
- **メモリなし（ステートレス）** — strands-agents自体はメモリ機能を持たない。会話履歴の永続化には [Amazon Bedrock AgentCore Memory](https://aws.amazon.com/bedrock/agentcore/) との統合が必要

→ [`strands-typescript/backend/src/agent.ts`](./strands-typescript/backend/src/agent.ts)

**向いているユースケース:**
- Python版strandsと同じアーキテクチャをTypeScriptで実装したい場合
- Python↔TypeScriptで設計を共有するチーム

---

### claude-agent-sdk

**パッケージ:** `@anthropic-ai/sdk`（Bedrock拡張を含む）

**特徴:**
- フレームワークなし。`AnthropicBedrock` クライアントで `tool_use` 機能を直接使用し、Agentループを手動実装
- ツールはJSON Schemaで手動定義
- `stop_reason === "tool_use"` でツール実行ループを自分でハンドリング
- スキルはサーバー側でSKILL.mdファイルを読み込み、プロンプトに注入
- **メモリなし**（シンプルな実装）

→ [`claude-agent-sdk/backend/src/agent.ts`](./claude-agent-sdk/backend/src/agent.ts)

**向いているユースケース:**
- フレームワークに依存せず、Agentの動作を完全に制御したい場合
- 学習目的やカスタム動作が必要な場合
- 依存関係を最小限にしたい場合

---

### voltagent

**パッケージ:** `@voltagent/core` + `@voltagent/server-hono` + `@ai-sdk/amazon-bedrock`

**特徴:**
- TypeScriptファーストのAIエージェントフレームワーク。Vercel AI SDK上に構築されており、`createAmazonBedrock()(modelId)` でBedrockモデルをそのまま渡せる
- `new VoltAgent({ agents, server: honoServer({ port }) })` だけでHTTPサーバーとルーティングが完結する（Mastraと同様のアプローチ）
- `createTool()` + Zodスキーマでツールを定義。APIはMastraの `createTool()` と非常に類似
- 6種類のメモリプロバイダー（InMemory / PostgreSQL / LibSQL / Supabase等）に加え、Semantic SearchとWorking Memoryをサポート
- Supervisorパターンによるマルチエージェント（`subAgents` + `delegate_task` ツールの自動生成）
- **VoltOps Console** でエージェントの実行トレース・パフォーマンスをリアルタイム可視化。Langfuse / MLflow との公式連携も提供

→ [`voltagent/backend/src/index.ts`](./voltagent/backend/src/index.ts) / [`voltagent/backend/src/agent.ts`](./voltagent/backend/src/agent.ts)

**向いているユースケース:**
- Observabilityを重視するプロダクション向けAgent開発
- MastraライクなAPIでObservability機能・Voice・Guardrailsが最初から揃っている環境が必要な場合
- マルチエージェントのオーケストレーションをシンプルに実装したい場合

## 参考

今回は5つのAI Agentフレームワークを比較しましたが、他にも多くのフレームワークやツールが存在します。
[langfuse](https://langfuse.com/integrations) を見ると、さらに多くのフレームワークやツールがリストアップされているので、興味がある方はぜひチェックしてみてください。