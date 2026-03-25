import { createTool } from "@voltagent/core";
import { z } from "zod";
import { createTask, listTasks, updateTask, deleteTask } from "./taskTools.js";

export const createTaskTool = createTool({
  name: "createTask",
  description: "新しいタスクを作成する",
  parameters: z.object({
    title: z.string(),
    description: z.string(),
    priority: z.enum(["low", "medium", "high"]),
  }),
  execute: async (input) => createTask(input),
});

export const listTasksTool = createTool({
  name: "listTasks",
  description: "タスク一覧を取得する",
  parameters: z.object({}),
  execute: async () => listTasks(),
});

export const updateTaskTool = createTool({
  name: "updateTask",
  description: "タスクを更新する",
  parameters: z.object({
    id: z.string(),
    title: z.string().optional(),
    status: z.enum(["todo", "in_progress", "done"]).optional(),
    priority: z.enum(["low", "medium", "high"]).optional(),
  }),
  execute: async ({ id, ...updates }) => updateTask(id, updates),
});

export const deleteTaskTool = createTool({
  name: "deleteTask",
  description: "タスクを削除する",
  parameters: z.object({ id: z.string() }),
  execute: async ({ id }) => deleteTask(id),
});
