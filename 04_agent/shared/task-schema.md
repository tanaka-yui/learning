# 共通タスクAPI仕様

## タスクデータ構造

```json
{
  "id": "string (uuid)",
  "title": "string",
  "description": "string",
  "priority": "low | medium | high",
  "status": "todo | in_progress | done",
  "createdAt": "ISO8601 datetime"
}
```

## エンドポイント

| Method | Path | 説明 |
|--------|------|------|
| GET | /tasks | タスク一覧取得 |
| POST | /tasks | タスク作成 |
| PUT | /tasks/:id | タスク更新 |
| DELETE | /tasks/:id | タスク削除 |
| POST | /chat | Agent会話 |

## リクエストボディ

### POST /tasks

```json
{
  "title": "string (必須)",
  "description": "string (必須)",
  "priority": "low | medium | high (必須)"
}
```

`id` と `createdAt` はサーバーが生成する。

### PUT /tasks/:id

部分更新（PATCH相当）。更新したいフィールドのみ送信する。

```json
{
  "title": "string (省略可)",
  "description": "string (省略可)",
  "priority": "low | medium | high (省略可)",
  "status": "todo | in_progress | done (省略可)"
}
```

### POST /chat

```json
{
  "message": "string (必須)",
  "sessionId": "string (必須, クライアントが生成するUUID)"
}
```

`sessionId` はクライアントが生成し、会話セッションを識別する。Redis にこのIDをキーとして会話履歴を保存する。
