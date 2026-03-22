import { Agent } from "@mastra/core/agent";
import { bedrock } from "@ai-sdk/amazon-bedrock";
import { Memory } from "@mastra/memory";
import { PostgresStore } from "@mastra/pg";
import { createTaskTool, listTasksTool, updateTaskTool, deleteTaskTool } from "./tools/index.js";

const storage = new PostgresStore({
  connectionString: process.env.DATABASE_URL ?? "postgresql://postgres:postgres@localhost:5432/mastra",
});

export const taskAgent = new Agent({
  id: "task-agent",
  name: "TaskAgent",
  instructions: `あなたはタスク管理エージェントです。
ユーザーのタスク管理を支援します。
タスクの作成・一覧・更新・削除ができます。
優先度順の並び替えやサマリーも提供できます。`,
  model: bedrock("anthropic.claude-sonnet-4-6-20251001-v1:0"),
  tools: {
    createTask: createTaskTool,
    listTasks: listTasksTool,
    updateTask: updateTaskTool,
    deleteTask: deleteTaskTool,
  },
  memory: new Memory({ storage }),
});
