import { tool } from "@strands-agents/sdk";
import { z } from "zod";
import { createTask, listTasks, updateTask, deleteTask } from "./taskTools.js";

export const createTaskTool = tool({
  name: "createTask",
  description: "新しいタスクを作成する",
  inputSchema: z.object({
    title: z.string(),
    description: z.string(),
    priority: z.enum(["low", "medium", "high"]),
  }),
  callback: (input) => createTask(input),
});

export const listTasksTool = tool({
  name: "listTasks",
  description: "タスク一覧を取得する",
  callback: () => listTasks(),
});

export const updateTaskTool = tool({
  name: "updateTask",
  description: "タスクを更新する",
  inputSchema: z.object({
    id: z.string(),
    title: z.string().optional(),
    status: z.enum(["todo", "in_progress", "done"]).optional(),
    priority: z.enum(["low", "medium", "high"]).optional(),
  }),
  callback: ({ id, ...updates }) => updateTask(id, updates),
});

export const deleteTaskTool = tool({
  name: "deleteTask",
  description: "タスクを削除する",
  inputSchema: z.object({ id: z.string() }),
  callback: ({ id }) => deleteTask(id),
});
