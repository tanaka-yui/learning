import { Agent } from "@mastra/core/agent";
import { createAmazonBedrock } from "@ai-sdk/amazon-bedrock";
import { Memory } from "@mastra/memory";
import { PostgresStore } from "@mastra/pg";
import { createTaskTool, listTasksTool, updateTaskTool, deleteTaskTool } from "./tools/index.js";
import { workspace } from "./workspace.js";
import {
  createCredentialChain,
  fromContainerMetadata,
  fromEnv,
  fromIni,
  fromInstanceMetadata,
  fromNodeProviderChain,
  fromSSO,
} from '@aws-sdk/credential-providers'

const init = { logger: console }

const getProvider = ({ provider, profile }: { provider?: string; profile?: string }) => {
  switch (provider) {
    case 'ini':
      return createCredentialChain(fromEnv(init), fromIni(init))
    case 'sso':
      return fromSSO({
        profile: profile ?? 'default',
      })
    case 'container-metadata':
      return fromContainerMetadata()
    case 'instance-metadata':
      return fromInstanceMetadata()
    default:
      return fromNodeProviderChain()
  }
}

const storage = new PostgresStore({
  id: "mastra-pg-store",
  connectionString: process.env.DATABASE_URL ?? "postgresql://postgres:postgres@localhost:5432/mastra",
});

export const taskAgent = new Agent({
  id: "task-agent",
  name: "TaskAgent",
  instructions: `あなたはタスク管理エージェントです。
ユーザーのタスク管理を支援します。
タスクの作成・一覧・更新・削除ができます。
優先度順の並び替えやサマリーも提供できます。`,
  model: createAmazonBedrock({
    region: "us-east-1",
    credentialProvider: getProvider({ provider: process.env.AWS_PROVIDER, profile: process.env.AWS_PROFILE })
  })("us.anthropic.claude-sonnet-4-6"),
  tools: {
    createTask: createTaskTool,
    listTasks: listTasksTool,
    updateTask: updateTaskTool,
    deleteTask: deleteTaskTool,
  },
  workspace,
  memory: new Memory({ storage }),
});
