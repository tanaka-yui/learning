import Fastify from "fastify";
import cors from "@fastify/cors";
import { createAgent } from "./agent.js";
import { readFileSync } from "fs";
import { join, dirname } from "path";
import { fileURLToPath } from "url";

const app = Fastify({ logger: true });
await app.register(cors, { origin: true });
const PORT = 4004;

const __dirname = dirname(fileURLToPath(import.meta.url));

const readSkill = (skillName: string): string => {
  const skillPath = join(__dirname, "../skills", skillName, "SKILL.md");
  return readFileSync(skillPath, "utf-8");
};

app.post<{ Body: { message: string; sessionId: string } }>("/chat", async (req, reply) => {
  const { message } = req.body;

  // strands-agentsはメモリ機能を持たない
  // 会話履歴の永続化にはAmazon Bedrock AgentCore Memoryが必要
  let prompt = message;
  if (message.includes("優先") || message.includes("prioritize")) {
    prompt = readSkill("prioritize");
  } else if (message.includes("サマリ") || message.includes("summarize")) {
    prompt = readSkill("summarize");
  }

  const agent = createAgent();
  const result = await agent.invoke(prompt);
  const response = result.toString();

  return reply.send({ response });
});

app.listen({ port: PORT, host: "0.0.0.0" }, (err) => {
  if (err) {
    app.log.error(err);
    process.exit(1);
  }
});
