import { Agent } from "@strands-agents/sdk";
import { BedrockModel } from "@strands-agents/sdk/bedrock";
import { createTaskTool, listTasksTool, updateTaskTool, deleteTaskTool } from "./tools/index.js";

const model = new BedrockModel({
  modelId: "us.anthropic.claude-sonnet-4-6",
  region: process.env.AWS_REGION ?? "us-east-1",
  clientConfig: {
    profile: process.env.AWS_PROFILE ?? "default",
    region: process.env.AWS_REGION ?? "us-east-1",
  }
});

const SYSTEM_PROMPT = `あなたはタスク管理エージェントです。
ユーザーのタスク管理を支援します。
タスクの作成・一覧・更新・削除ができます。
優先度順の並び替えやサマリーも提供できます。`;

export const createAgent = (): Agent =>
  new Agent({
    model,
    tools: [createTaskTool, listTasksTool, updateTaskTool, deleteTaskTool],
    systemPrompt: SYSTEM_PROMPT,
  });
