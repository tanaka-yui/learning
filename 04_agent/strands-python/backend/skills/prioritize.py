from tools.task_tools import task_store

PRIORITY_ORDER = {"high": 0, "medium": 1, "low": 2}


def prioritize() -> str:
    incomplete = [t for t in task_store if t["status"] != "done"]
    sorted_tasks = sorted(incomplete, key=lambda t: PRIORITY_ORDER.get(t["priority"], 99))
    if not sorted_tasks:
        return "未完了のタスクはありません。"
    return "\n".join(
        f"{i + 1}. [{t['priority']}] {t['title']} ({t['status']})"
        for i, t in enumerate(sorted_tasks)
    )
