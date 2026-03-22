import Fastify from "fastify";
import { taskAgent } from "./agent.js";
import { prioritize } from "./skills/prioritize.js";
import { summarize } from "./skills/summarize.js";

const app = Fastify({ logger: true });
const PORT = 4001;

app.post<{ Body: { message: string; sessionId: string } }>("/chat", async (req, reply) => {
  const { message, sessionId } = req.body;

  if (message.includes("優先") || message.includes("prioritize")) {
    return reply.send({ response: prioritize() });
  }

  if (message.includes("サマリ") || message.includes("summarize")) {
    return reply.send({ response: summarize() });
  }

  // Mastra の組み込みメモリ（PostgreSQL）で会話履歴を自動管理
  const result = await taskAgent.generate(message, {
    memory: {
      resource: "default-user",
      thread: sessionId,
    },
  });

  return reply.send({ response: result.text });
});

app.listen({ port: PORT, host: "0.0.0.0" }, (err) => {
  if (err) {
    app.log.error(err);
    process.exit(1);
  }
});
