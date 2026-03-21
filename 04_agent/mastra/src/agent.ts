import { Agent } from "@mastra/core/agent";
import { anthropic } from "@ai-sdk/anthropic";
import { createTaskTool, listTasksTool, updateTaskTool, deleteTaskTool } from "./tools/index.js";

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
});
