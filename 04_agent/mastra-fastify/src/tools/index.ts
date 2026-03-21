import { createTool } from "@mastra/core/tools";
import { z } from "zod";
import { createTask, listTasks, updateTask, deleteTask } from "./taskTools.js";

export const createTaskTool = createTool({
  id: "createTask",
  description: "新しいタスクを作成する",
  inputSchema: z.object({
    title: z.string(),
    description: z.string(),
    priority: z.enum(["low", "medium", "high"]),
  }),
  execute: async (inputData) => createTask(inputData),
});

export const listTasksTool = createTool({
  id: "listTasks",
  description: "タスク一覧を取得する",
  inputSchema: z.object({}),
  execute: async () => listTasks(),
});

export const updateTaskTool = createTool({
  id: "updateTask",
  description: "タスクを更新する",
  inputSchema: z.object({
    id: z.string(),
    title: z.string().optional(),
    status: z.enum(["todo", "in_progress", "done"]).optional(),
    priority: z.enum(["low", "medium", "high"]).optional(),
  }),
  execute: async ({ id, ...updates }) => updateTask(id, updates),
});

export const deleteTaskTool = createTool({
  id: "deleteTask",
  description: "タスクを削除する",
  inputSchema: z.object({ id: z.string() }),
  execute: async ({ id }) => deleteTask(id),
});
