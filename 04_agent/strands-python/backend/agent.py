from strands import Agent
from strands.models.anthropic import AnthropicModel
from tools.task_tools import create_task, list_tasks, update_task, delete_task

model = AnthropicModel(model_id="claude-sonnet-4-6")

SYSTEM_PROMPT = """あなたはタスク管理エージェントです。
ユーザーのタスク管理を支援します。
タスクの作成・一覧・更新・削除ができます。
優先度順の並び替えやサマリーも提供できます。"""


def create_agent(messages: list | None = None) -> Agent:
    return Agent(
        model=model,
        tools=[create_task, list_tasks, update_task, delete_task],
        system_prompt=SYSTEM_PROMPT,
        messages=messages,
    )
