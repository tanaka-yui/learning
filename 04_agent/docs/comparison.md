# AI Agent フレームワーク比較

タスク管理Agentを5つのフレームワークで実装した結果の比較。

## 比較表

| 観点 | mastra | mastra-fastify | strands-python | strands-typescript | claude-agent-sdk |
|------|--------|----------------|----------------|--------------------|------------------|
| **言語** | TypeScript | TypeScript | Python | TypeScript | TypeScript |
| **HTTPサーバー** | Fastify（手動） | Fastify（手動） | FastAPI | Fastify（手動） | Fastify（手動） |
| **ポート** | 4001 | 4002 | 4003 | 4004 | 4005 |
| **ツール定義** | `createTool()` + zodスキーマ | `createTool()` + zodスキーマ | `@tool` デコレータ | `tool()` + zodスキーマ | 手動JSONスキーマ |
| **スキル管理** | 独自関数 | 独自関数 | 独自関数 | 独自関数 | 独自関数 |
| **メモリ管理** | PostgreSQL（Mastra組み込み） | Redis（手動） | Redis（手動） | Redis（手動） | なし |
| **モデル設定** | `anthropic("claude-sonnet-4-6")` | `anthropic("claude-sonnet-4-6")` | `AnthropicModel(model_id=...)` | `new AnthropicModel({modelId: ...})` | `client.messages.create(model=...)` |
| **Agent構築** | `new Agent({...})` | `new Mastra({agents: {...}})` | `Agent(model=..., tools=[...])` | `new Agent({model, tools, systemPrompt})` | 手動ループ実装 |
| **ツール実行** | フレームワーク自動 | フレームワーク自動 | フレームワーク自動 | フレームワーク自動 | 手動ループ |
| **型安全性** | ✅ 高い（Zod） | ✅ 高い（Zod） | ✅ Python型ヒント | ✅ 高い（Zod） | ⚠️ 中（手動キャスト） |
| **セットアップ難易度** | ★★☆ | ★★★ | ★★☆ | ★★☆ | ★☆☆ |

## 各フレームワークの詳細

### mastra

**パッケージ:** `@mastra/core` + `@mastra/memory` + `@mastra/pg`

**特徴:**
- `createTool()` でZodスキーマベースのツールを定義
- `Agent` クラスに `memory: new Memory({ storage: new PostgresStore({...}) })` を渡すだけで会話履歴の永続化が完結
- `agent.generate(message, { memory: { resource, thread } })` で会話履歴の読み書きを自動管理 — Redis の手動操作が不要
- AI SDK (`@ai-sdk/anthropic`) を通じてAnthropicモデルを使用
- メモリ: PostgreSQL（ポート5432）

**メモリ設定の例:**
```typescript
import { Memory } from "@mastra/memory";
import { PostgresStore } from "@mastra/pg";

export const taskAgent = new Agent({
  // ...
  memory: new Memory({
    storage: new PostgresStore({
      connectionString: process.env.DATABASE_URL,
    }),
  }),
});

// 呼び出し時に sessionId を thread として渡すだけで自動保存
const result = await taskAgent.generate(message, {
  memory: { resource: "default-user", thread: sessionId },
});
```

**向いているユースケース:** TypeScriptネイティブでAgentを構築しつつ、会話履歴の永続化もフレームワークに任せたい場合。PostgreSQLを既に使っているプロジェクトへの統合が容易。

---

### mastra-fastify

**パッケージ:** `@mastra/core`（低レベルAPI使用）

**特徴:**
- `Mastra` クラスにAgentを登録し、`mastra.getAgent()` で取得
- `Agent` を `Mastra` インスタンスで管理することで、複数Agentのオーケストレーションが可能
- ツール定義はmastraと同一

**Agent構築の例:**
```typescript
const mastra = new Mastra({ agents: { taskAgent: agent } });
const agent = mastra.getAgent("taskAgent");
```

**向いているユースケース:** 複数のAgentを管理・オーケストレーションしたい場合。`Mastra` インスタンスをアプリケーション全体の中央管理レジストリとして使用する設計に向いている。

---

### strands-python

**パッケージ:** `strands-agents` + `anthropic`

**特徴:**
- `@tool` デコレータで関数をツールとして登録（Pythonらしい書き方）
- `Agent` の `messages` パラメータで会話履歴を初期化できる
- `AnthropicModel(model_id=...)` でモデルを設定
- 呼び出しは `agent("message")` → `AgentResult`、`str(result)` でテキスト取得

**ツール定義の例:**
```python
@tool
def create_task(title: str, description: str, priority: str) -> dict:
    """新しいタスクを作成する。"""
    ...
```

**向いているユースケース:** Pythonエコシステムを活用したAgent開発。`@tool` デコレータによる直感的なツール定義が特徴。データサイエンス・ML系のツールとの統合に強い。

---

### strands-typescript

**パッケージ:** `@strands-agents/sdk`

**特徴:**
- `tool()` ファクトリ関数でZodスキーマベースのツールを定義（Python版の `@tool` に対応）
- `Agent.invoke(message)` → `AgentResult`、`result.toString()` でテキスト取得
- `AnthropicModel` が `@strands-agents/sdk/models/anthropic` から利用可能
- 会話履歴を `messages` として渡してAgentを初期化

**ツール定義の例:**
```typescript
const createTaskTool = tool({
  name: "createTask",
  description: "新しいタスクを作成する",
  inputSchema: z.object({ title: z.string(), priority: z.enum([...]) }),
  callback: (input) => createTask(input),
});
```

**向いているユースケース:** Python版strandsと同じアーキテクチャをTypeScriptで実装したい場合。Python↔TypeScriptで設計を共有するチームに向いている。

---

### claude-agent-sdk

**パッケージ:** `@anthropic-ai/sdk`

**特徴:**
- フレームワークなし。Anthropic SDKの `tool_use` 機能を直接使用してAgentループを手動実装
- ツールはJSON Schemaで手動定義
- `stop_reason === "tool_use"` でツール実行ループを自分でハンドリング
- **メモリなし**（Redisを使用しないシンプルな実装）

**Agentループの例:**
```typescript
while (true) {
  const response = await client.messages.create({ tools, messages });
  if (response.stop_reason === "end_turn") return response;
  if (response.stop_reason === "tool_use") {
    // ツール実行 → 結果をmessagesに追加 → 繰り返し
  }
}
```

**向いているユースケース:** フレームワークに依存せず、Agentの動作を完全に制御したい場合。学習目的やカスタム動作が必要な場合に適している。依存関係が最小限。

---

## セットアップ難易度の比較

| フレームワーク | 依存パッケージ数 | 設定ファイル | 学習コスト |
|---------------|----------------|------------|----------|
| mastra | 多（@mastra/core + @mastra/memory + @mastra/pg） | package.json + tsconfig | 中（メモリはAPI1行） |
| mastra-fastify | 同上 | 同上 | やや高（Mastraクラスの理解が必要） |
| strands-python | 少（strands-agents + anthropic） | pyproject.toml | 低（Pythonらしい書き方） |
| strands-typescript | 中 | package.json + tsconfig | 中 |
| claude-agent-sdk | 最小（@anthropic-ai/sdk のみ） | package.json + tsconfig | 低（SDKのみ学習すればよい） |

## 推奨用途まとめ

| 用途 | 推奨フレームワーク |
|------|----------------|
| TypeScriptで素早くAgent開発 | **mastra** |
| 複数Agentのオーケストレーション | **mastra-fastify** |
| Pythonエコシステムとの統合 | **strands-python** |
| Python↔TypeScriptの設計共有 | **strands-typescript** |
| 最小依存・完全制御 | **claude-agent-sdk** |
