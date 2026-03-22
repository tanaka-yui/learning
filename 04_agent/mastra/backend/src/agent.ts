import { Agent } from "@mastra/core/agent";
import { anthropic } from "@ai-sdk/anthropic";
import { Memory } from "@mastra/memory";
import { PostgresStore } from "@mastra/pg";
import { createTaskTool, listTasksTool, updateTaskTool, deleteTaskTool } from "./tools/index.js";

const storage = new PostgresStore({
  connectionString: process.env.DATABASE_URL ?? "postgresql://postgres:postgres@localhost:5432/mastra",
});

export const taskAgent = new Agent({
  name: "TaskAgent",
  instructions: `あなたはタスク管理エージェントです。
ユーザーのタスク管理を支援します。
タスクの作成・一覧・更新・削除ができます。
優先度順の並び替えやサマリーも提供できます。`,
  model: anthropic("claude-sonnet-4-6"),
  tools: {
    createTask: createTaskTool,
    listTasks: listTasksTool,
    updateTask: updateTaskTool,
    deleteTask: deleteTaskTool,
  },
  memory: new Memory({ storage }),
});
