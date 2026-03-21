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

## POST /chat

Request: `{ "message": "string", "sessionId": "string" }`
Response: `{ "response": "string" }`
