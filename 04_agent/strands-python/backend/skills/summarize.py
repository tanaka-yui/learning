from tools.task_tools import task_store


def summarize() -> str:
    total = len(task_store)
    done = sum(1 for t in task_store if t["status"] == "done")
    in_progress = sum(1 for t in task_store if t["status"] == "in_progress")
    todo = sum(1 for t in task_store if t["status"] == "todo")
    return f"タスク合計: {total}件（完了: {done}件、進行中: {in_progress}件、未着手: {todo}件）"
