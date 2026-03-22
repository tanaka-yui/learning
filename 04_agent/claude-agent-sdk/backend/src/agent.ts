import Anthropic from "@anthropic-ai/sdk";
import AnthropicBedrock from "@anthropic-ai/sdk/bedrock";
import { createTask, listTasks, updateTask, deleteTask } from "./tools/taskTools.js";

const client = new AnthropicBedrock();

const toolDefinitions: Anthropic.Tool[] = [
  {
    name: "createTask",
    description: "新しいタスクを作成する",
    input_schema: {
      type: "object" as const,
      properties: {
        title: { type: "string" },
        description: { type: "string" },
        priority: { type: "string", enum: ["low", "medium", "high"] },
      },
      required: ["title", "description", "priority"],
    },
  },
  {
    name: "listTasks",
    description: "タスク一覧を取得する",
    input_schema: { type: "object" as const, properties: {} },
  },
  {
    name: "updateTask",
    description: "タスクを更新する。更新したいフィールドのみ指定する。",
    input_schema: {
      type: "object" as const,
      properties: {
        id: { type: "string" },
        title: { type: "string" },
        status: { type: "string", enum: ["todo", "in_progress", "done"] },
        priority: { type: "string", enum: ["low", "medium", "high"] },
      },
      required: ["id"],
    },
  },
  {
    name: "deleteTask",
    description: "タスクを削除する",
    input_schema: {
      type: "object" as const,
      properties: { id: { type: "string" } },
      required: ["id"],
    },
  },
];

type ToolInput = Record<string, string>;

const toolHandlers: Record<string, (input: ToolInput) => unknown> = {
  createTask: (input) =>
    createTask({
      title: input.title,
      description: input.description,
      priority: input.priority as "low" | "medium" | "high",
    }),
  listTasks: () => listTasks(),
  updateTask: ({ id, ...updates }) => {
    const typed: Partial<{ title: string; status: "todo" | "in_progress" | "done"; priority: "low" | "medium" | "high" }> = {};
    if (updates.title) typed.title = updates.title;
    if (updates.status) typed.status = updates.status as "todo" | "in_progress" | "done";
    if (updates.priority) typed.priority = updates.priority as "low" | "medium" | "high";
    return updateTask(id, typed);
  },
  deleteTask: ({ id }) => deleteTask(id),
};

export const runAgent = async (message: string): Promise<string> => {
  const messages: Anthropic.MessageParam[] = [{ role: "user", content: message }];

  while (true) {
    const response = await client.messages.create({
      model: "anthropic.claude-sonnet-4-6-20251001-v1:0",
      max_tokens: 4096,
      system: "あなたはタスク管理エージェントです。ユーザーのタスク管理を支援します。",
      tools: toolDefinitions,
      messages,
    });

    if (response.stop_reason === "end_turn") {
      const textBlock = response.content.find((b): b is Anthropic.TextBlock => b.type === "text");
      return textBlock?.text ?? "";
    }

    if (response.stop_reason === "tool_use") {
      messages.push({ role: "assistant", content: response.content });
      const toolResults: Anthropic.ToolResultBlockParam[] = response.content
        .filter((b): b is Anthropic.ToolUseBlock => b.type === "tool_use")
        .map((toolUse) => ({
          type: "tool_result" as const,
          tool_use_id: toolUse.id,
          content: JSON.stringify(
            toolHandlers[toolUse.name]?.(toolUse.input as ToolInput) ?? null
          ),
        }));
      messages.push({ role: "user", content: toolResults });
    } else {
      break;
    }
  }

  return "";
};
