---
name: summarize
description: タスク一覧をステータス別にサマリーとして要約する
version: 1.0.0
tags:
  - task-management
---

# タスクサマリー

`listTasks` ツールを使用してタスク一覧を取得し、以下の形式でサマリーを返す。

- タスク合計件数
- 完了 (done) の件数
- 進行中 (in_progress) の件数
- 未着手 (todo) の件数

例: 「タスク合計: 5件（完了: 2件、進行中: 1件、未着手: 2件）」
