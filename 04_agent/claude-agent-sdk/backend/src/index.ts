import Fastify from "fastify";
import cors from "@fastify/cors";
import { runAgent } from "./agent.js";
import { readFileSync } from "fs";
import { join, dirname } from "path";
import { fileURLToPath } from "url";

const app = Fastify({ logger: true });
await app.register(cors, { origin: true });
const PORT = 4005;

const __dirname = dirname(fileURLToPath(import.meta.url));

const readSkill = (skillName: string): string => {
  const skillPath = join(__dirname, "../skills", skillName, "SKILL.md");
  return readFileSync(skillPath, "utf-8");
};

// メモリなし: sessionIdは受け取るが会話履歴は保存しない
app.post<{ Body: { message: string; sessionId: string } }>("/chat", async (req, reply) => {
  const { message } = req.body;

  let prompt = message;
  if (message.includes("優先") || message.includes("prioritize")) {
    prompt = readSkill("prioritize");
  } else if (message.includes("サマリ") || message.includes("summarize")) {
    prompt = readSkill("summarize");
  }

  const response = await runAgent(prompt);
  return reply.send({ response });
});

app.listen({ port: PORT, host: "0.0.0.0" }, (err) => {
  if (err) {
    app.log.error(err);
    process.exit(1);
  }
});
