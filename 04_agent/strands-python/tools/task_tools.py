from uuid import uuid4
from datetime import datetime, timezone
from strands import tool

task_store: list[dict] = []


@tool
def create_task(title: str, description: str, priority: str) -> dict:
    """新しいタスクを作成する。priorityはlow/medium/highのいずれか。"""
    task = {
        "id": str(uuid4()),
        "title": title,
        "description": description,
        "priority": priority,
        "status": "todo",
        "createdAt": datetime.now(timezone.utc).isoformat(),
    }
    task_store.append(task)
    return task


@tool
def list_tasks() -> list:
    """タスク一覧を取得する。"""
    return list(task_store)


@tool
def update_task(id: str, title: str = "", status: str = "", priority: str = "") -> dict:
    """タスクを更新する。更新したいフィールドのみ指定する。"""
    for task in task_store:
        if task["id"] == id:
            if title:
                task["title"] = title
            if status:
                task["status"] = status
            if priority:
                task["priority"] = priority
            return task
    return {}


@tool
def delete_task(id: str) -> bool:
    """タスクを削除する。"""
    for i, task in enumerate(task_store):
        if task["id"] == id:
            task_store.pop(i)
            return True
    return False
