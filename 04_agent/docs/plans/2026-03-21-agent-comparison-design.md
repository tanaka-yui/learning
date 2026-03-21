# AI Agent フレームワーク比較 - 設計ドキュメント

## 概要

5つのAI Agentフレームワークを用いて、同一のタスク管理Agentを実装し、各フレームワークの特性を実践的に比較する。

## 比較対象フレームワーク

| フレームワーク | 言語 | ポート |
|---------------|------|--------|
| mastra | TypeScript | 4001 |
| mastra/core + fastify | TypeScript | 4002 |
| strands-agents/sdk-python + fastapi | Python | 4003 |
| strands-agents/sdk-typescript + fastify | TypeScript | 4004 |
| claude code agent sdk + fastify | TypeScript | 4005 |

## ユースケース: タスク管理Agent

全フレームワーク共通で以下の機能を実装する。

### ツール

- `createTask` — タスクを作成する
- `listTasks` — タスク一覧を取得する
- `updateTask` — タスクを更新する
- `deleteTask` — タスクを削除する

### スキル

- `prioritize` — 未完了タスクを重要度順に並び替えて返す
- `summarize` — 現在のタスク状況をテキストで要約する

### メモリ

- Redisに会話履歴を保存（キー: `session:{sessionId}:history`）
- `claude-agent-sdk` はメモリ未実装のため対象外

## プロジェクト構造

```
04_agent/
├── docker-compose.yml
├── Makefile
├── docs/
│   ├── plans/
│   │   └── 2026-03-21-agent-comparison-design.md
│   └── comparison.md          # 比較表・解説（実装後に作成）
├── shared/
│   └── task-schema.md         # 共通API仕様
├── mastra/                    # Port 4001
│   ├── src/
│   │   ├── tools/
│   │   ├── skills/
│   │   ├── agent.ts
│   │   └── index.ts
│   ├── Dockerfile
│   └── package.json
├── mastra-fastify/            # Port 4002
│   ├── src/
│   │   ├── tools/
│   │   ├── skills/
│   │   ├── agent.ts
│   │   └── index.ts
│   ├── Dockerfile
│   └── package.json
├── strands-python/            # Port 4003
│   ├── tools/
│   ├── skills/
│   ├── agent.py
│   ├── main.py
│   ├── Dockerfile
│   └── pyproject.toml
├── strands-typescript/        # Port 4004
│   ├── src/
│   │   ├── tools/
│   │   ├── skills/
│   │   ├── agent.ts
│   │   └── index.ts
│   ├── Dockerfile
│   └── package.json
└── claude-agent-sdk/          # Port 4005
    ├── src/
    │   ├── tools/
    │   ├── skills/
    │   ├── agent.ts
    │   └── index.ts
    ├── Dockerfile
    └── package.json
```

## API仕様（全フレームワーク共通）

### タスク管理エンドポイント

```
GET    /tasks          タスク一覧取得
POST   /tasks          タスク作成
PUT    /tasks/:id      タスク更新
DELETE /tasks/:id      タスク削除
```

### Agent会話エンドポイント

```
POST /chat
Body: { "message": string, "sessionId": string }
Response: { "response": string }
```

## データフロー

```
POST /chat
  ↓
Redisから会話履歴を取得（sessionId）
  ↓
Agent（LLM）が入力+履歴を元に判断
  ↓
ツール/スキルを実行（必要に応じて）
  ├── createTask / listTasks / updateTask / deleteTask
  ├── prioritize → タスクを重要度順にソート
  └── summarize → タスク状況のテキスト要約
  ↓
Agentが応答を生成
  ↓
Redisに会話履歴を保存
  ↓
レスポンス返却
```

## インフラ構成

- **Redis** (Port 6379): 会話履歴のインメモリストア
- **タスクデータ**: 各フレームワーク内のインメモリ（比較に集中するためDBなし）
- **Docker Compose**: 全サービスを統合管理

## 比較ドキュメントの観点

`docs/comparison.md` に以下の観点で比較表を作成する。

| 観点 | 内容 |
|------|------|
| セットアップ難易度 | 依存関係・設定ファイルの複雑さ |
| ツール定義 | ツールの定義方法・型安全性 |
| スキル管理 | スキルの抽象化方法 |
| メモリ管理 | 組み込みサポートの有無・Redisとの統合方法 |
| コード量 | 同機能を実現するための行数 |
| 型安全性 | TypeScript/Python型定義のサポート |
| 公式ドキュメント | 充実度・学習コスト |
| ユースケース適性 | 向いている用途・規模感 |
