import Fastify from "fastify";
import cors from "@fastify/cors";
import { runAgent } from "./agent.js";
import { prioritize } from "./skills/prioritize.js";
import { summarize } from "./skills/summarize.js";

const app = Fastify({ logger: true });
await app.register(cors, { origin: true });
const PORT = 4005;

// メモリなし: sessionIdは受け取るが会話履歴は保存しない
app.post<{ Body: { message: string; sessionId: string } }>("/chat", async (req, reply) => {
  const { message } = req.body;

  if (message.includes("優先") || message.includes("prioritize")) {
    return reply.send({ response: prioritize() });
  }

  if (message.includes("サマリ") || message.includes("summarize")) {
    return reply.send({ response: summarize() });
  }

  const response = await runAgent(message);
  return reply.send({ response });
});

app.listen({ port: PORT, host: "0.0.0.0" }, (err) => {
  if (err) {
    app.log.error(err);
    process.exit(1);
  }
});
