from pathlib import Path

import boto3
from strands import Agent, AgentSkills
from strands.models.bedrock import BedrockModel
from tools.task_tools import create_task, list_tasks, update_task, delete_task

boto_session = boto3.Session(
    region_name="us-west-1",
)
model = BedrockModel(
    model_id="us.anthropic.claude-sonnet-4-6",
    boto_session=boto_session,
)

BASE_DIR = Path(__file__).parent
skills_plugin = AgentSkills(skills=str(BASE_DIR / "skills"))

SYSTEM_PROMPT = """あなたはタスク管理エージェントです。
ユーザーのタスク管理を支援します。
タスクの作成・一覧・更新・削除ができます。
優先度順の並び替えやサマリーも提供できます。"""


def create_agent() -> Agent:
    return Agent(
        model=model,
        tools=[create_task, list_tasks, update_task, delete_task],
        plugins=[skills_plugin],
        system_prompt=SYSTEM_PROMPT,
    )
