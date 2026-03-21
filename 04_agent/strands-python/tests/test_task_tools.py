import pytest
from tools.task_tools import task_store, create_task, list_tasks, update_task, delete_task


@pytest.fixture(autouse=True)
def clear_store():
    task_store.clear()
    yield
    task_store.clear()


def test_create_task():
    task = create_task(title="Test", description="desc", priority="high")
    assert task["status"] == "todo"
    assert task["title"] == "Test"
    assert "id" in task


def test_list_tasks():
    create_task(title="T1", description="", priority="low")
    create_task(title="T2", description="", priority="medium")
    assert len(list_tasks()) == 2


def test_update_task():
    task = create_task(title="Old", description="", priority="low")
    updated = update_task(task["id"], title="New")
    assert updated["title"] == "New"


def test_delete_task():
    task = create_task(title="Del", description="", priority="low")
    assert delete_task(task["id"]) is True
    assert len(list_tasks()) == 0
