import { Agent } from "@voltagent/core";
import { createAmazonBedrock } from "@ai-sdk/amazon-bedrock";
import { createTaskTool, listTasksTool, updateTaskTool, deleteTaskTool } from "./tools/index.js";

const bedrock = createAmazonBedrock({
  region: process.env.AWS_REGION ?? "us-east-1",
  profile: process.env.AWS_PROFILE ?? "default",
});

export const taskAgent = new Agent({
  id: "task-agent",
  name: "task-agent",
  instructions: `あなたはタスク管理エージェントです。
ユーザーのタスク管理を支援します。
タスクの作成・一覧・更新・削除ができます。
優先度順の並び替えやサマリーも提供できます。`,
  model: bedrock("us.anthropic.claude-sonnet-4-6"),
  tools: [createTaskTool, listTasksTool, updateTaskTool, deleteTaskTool],
});
