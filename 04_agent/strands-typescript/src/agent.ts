import { Agent } from "@strands-agents/sdk";
import { AnthropicModel } from "@strands-agents/sdk/models/anthropic";
import { createTaskTool, listTasksTool, updateTaskTool, deleteTaskTool } from "./tools/index.js";
import type { MessageData } from "@strands-agents/sdk";

const model = new AnthropicModel({
  modelId: "claude-sonnet-4-6",
});

const SYSTEM_PROMPT = `あなたはタスク管理エージェントです。
ユーザーのタスク管理を支援します。
タスクの作成・一覧・更新・削除ができます。
優先度順の並び替えやサマリーも提供できます。`;

export const createAgent = (messages?: MessageData[]): Agent =>
  new Agent({
    model,
    tools: [createTaskTool, listTasksTool, updateTaskTool, deleteTaskTool],
    systemPrompt: SYSTEM_PROMPT,
    messages,
  });
