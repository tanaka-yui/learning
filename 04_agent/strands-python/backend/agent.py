from strands import Agent
from strands.models.bedrock import BedrockModel
from tools.task_tools import create_task, list_tasks, update_task, delete_task

model = BedrockModel(model_id="anthropic.claude-sonnet-4-6-20251001-v1:0")

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
