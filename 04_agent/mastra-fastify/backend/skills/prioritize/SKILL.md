---
name: prioritize
description: 未完了タスクを優先度順（high > medium > low）に並べて表示する
version: 1.0.0
tags:
  - task-management
---

# タスク優先度順ソート

`listTasks` ツールを使用してタスクを取得し、ステータスが `done` 以外のものを優先度順に並べて返す。

優先度の順序: high > medium > low

出力形式:
```
1. [high] タスク名 (in_progress)
2. [medium] タスク名 (todo)
3. [low] タスク名 (todo)
```

未完了タスクがない場合は「未完了のタスクはありません。」と返す。
