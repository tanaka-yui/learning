# strands-python: AgentSkills プラグインによるスキル読み込み

## Context

現在 `main.py` で `SKILL.md` を手動で読み込みキーワードマッチでプロンプトに注入している。
Strands 公式の `AgentSkills` プラグインを使うことで、スキルのロードをフレームワーク標準の方式に統一する。

参照: https://strandsagents.com/docs/user-guide/concepts/plugins/skills/

## 変更内容

### 1. `agent.py` に `AgentSkills` プラグインを追加

- `from strands import AgentSkills` をインポート
- `AgentSkills(skills="./skills/")` でスキルディレクトリを一括ロード
- `Agent` の `plugins` パラメータに渡す

```python
from strands import Agent, AgentSkills
from strands.models.bedrock import BedrockModel
from tools.task_tools import create_task, list_tasks, update_task, delete_task
from pathlib import Path

model = BedrockModel(model_id="anthropic.claude-sonnet-4-6-20251001-v1:0")

SYSTEM_PROMPT = """あなたはタスク管理エージェントです。
ユーザーのタスク管理を支援します。
タスクの作成・一覧・更新・削除ができます。
優先度順の並び替えやサマリーも提供できます。"""

BASE_DIR = Path(__file__).parent
skills_plugin = AgentSkills(skills=str(BASE_DIR / "skills"))


def create_agent(messages: list | None = None) -> Agent:
    return Agent(
        model=model,
        tools=[create_task, list_tasks, update_task, delete_task],
        plugins=[skills_plugin],
        system_prompt=SYSTEM_PROMPT,
        messages=messages,
    )
```

### 2. `main.py` から手動スキル読み込みロジックを削除

- `read_skill` 関数を削除
- `chat` ハンドラ内のキーワードマッチ分岐（`if "優先" in ...`）を削除
- `prompt` は常に `req.message` をそのまま使う

## 対象ファイル

- `04_agent/strands-python/backend/agent.py` — AgentSkills プラグイン追加
- `04_agent/strands-python/backend/main.py` — 手動スキル読み込みの削除

## Verification

- Docker Compose でコンテナをビルド・起動し、`/chat` エンドポイントが正常に応答すること
- 「優先度順に並べて」「サマリーを見せて」等のメッセージでスキルが適用されること
