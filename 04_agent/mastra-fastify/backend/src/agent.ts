import { Mastra } from "@mastra/core/mastra";
import { Agent } from "@mastra/core/agent";
import { bedrock } from "@ai-sdk/amazon-bedrock";
import { createTaskTool, listTasksTool, updateTaskTool, deleteTaskTool } from "./tools/index.js";

const agent = new Agent({
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
});

export const mastra = new Mastra({
  agents: { taskAgent: agent },
});
