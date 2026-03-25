# AI Agent フレームワーク比較

タスク管理Agentを5つのフレームワークで実装した結果の比較。

## 比較表

| 観点 | mastra | mastra-fastify | strands-python | strands-typescript | claude-agent-sdk |
|------|--------|----------------|----------------|--------------------|------------------|
| **言語** | TypeScript | TypeScript | Python | TypeScript | TypeScript |
| **HTTPサーバー** | Mastra組み込み（Hono） | Fastify（手動） | FastAPI（手動）| Fastify（手動） | Fastify（手動） |
| **バックエンドポート** | 4001 | 4002 | 4003 | 4004 | 4005 |
| **フロントエンドポート** | 5001 | 5002 | 5003 | 5004 | 5005 |
| **ツール定義** | `createTool()` + zodスキーマ | `createTool()` + zodスキーマ | `@tool` デコレータ | `tool()` + zodスキーマ | 手動JSONスキーマ |
| **スキル管理** | `Workspace` + `LocalFilesystem` | `Workspace` + `LocalFilesystem` | `AgentSkills` プラグイン | 手動ファイル読み込み | 手動ファイル読み込み |
| **メモリ管理** | `@mastra/memory` + PostgreSQL | `@mastra/memory` + PostgreSQL | なし（AgentCore Memory必須） | なし（AgentCore Memory必須） | なし |
| **モデル設定** | `createAmazonBedrock()(modelId)` | `createAmazonBedrock()(modelId)` | `BedrockModel(model_id=...)` | `new BedrockModel({modelId: ...})` | `new AnthropicBedrock()` |
| **Agent構築** | `new Mastra({server, agents})` | `new Mastra({agents})` + Fastify | `Agent(model=..., tools=[...], plugins=[...])` | `new Agent({model, tools, systemPrompt})` | 手動ループ実装 |
| **ツール実行** | フレームワーク自動 | フレームワーク自動 | フレームワーク自動 | フレームワーク自動 | 手動ループ |
| **型安全性** | ✅ 高い（Zod） | ✅ 高い（Zod） | ✅ Python型ヒント | ✅ 高い（Zod） | ⚠️ 中（手動キャスト） |
| **セットアップ難易度** | ★★☆ | ★★★ | ★★☆ | ★★☆ | ★☆☆ |

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

## 参考

今回は5つのAI Agentフレームワークを比較しましたが、他にも多くのフレームワークやツールが存在します。
[langfuse](https://langfuse.com/integrations) を見ると、さらに多くのフレームワークやツールがリストアップされているので、興味がある方はぜひチェックしてみてください。