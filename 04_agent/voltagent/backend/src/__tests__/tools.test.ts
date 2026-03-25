import { describe, it, expect, beforeEach } from "vitest";
import { taskStore, createTask, listTasks, updateTask, deleteTask } from "../tools/taskTools.js";

describe("taskTools", () => {
  beforeEach(() => {
    taskStore.length = 0;
  });

  it("creates a task with required fields", () => {
    const task = createTask({ title: "Test task", description: "desc", priority: "high" });
    expect(task.id).toBeDefined();
    expect(task.title).toBe("Test task");
    expect(task.status).toBe("todo");
  });

  it("lists all tasks", () => {
    createTask({ title: "Task 1", description: "", priority: "low" });
    createTask({ title: "Task 2", description: "", priority: "medium" });
    expect(listTasks()).toHaveLength(2);
  });

  it("updates a task", () => {
    const task = createTask({ title: "Old", description: "", priority: "low" });
    const updated = updateTask(task.id, { title: "New" });
    expect(updated?.title).toBe("New");
  });

  it("deletes a task", () => {
    const task = createTask({ title: "To delete", description: "", priority: "low" });
    expect(deleteTask(task.id)).toBe(true);
    expect(listTasks()).toHaveLength(0);
  });
});
