import Fastify from "fastify";
import { Redis } from "ioredis";
import { mastra } from "./agent.js";
import { prioritize } from "./skills/prioritize.js";
import { summarize } from "./skills/summarize.js";

const app = Fastify({ logger: true });
const redis = new Redis(process.env.REDIS_URL ?? "redis://localhost:6379");
const PORT = 4002;

const agent = mastra.getAgent("taskAgent");

app.post<{ Body: { message: string; sessionId: string } }>("/chat", async (req, reply) => {
  const { message, sessionId } = req.body;

  const historyRaw = await redis.get(`session:${sessionId}:history`);
  const history: Array<{ role: string; content: string }> = historyRaw
    ? JSON.parse(historyRaw)
    : [];

  if (message.includes("優先") || message.includes("prioritize")) {
    const result = prioritize();
    history.push({ role: "user", content: message });
    history.push({ role: "assistant", content: result });
    await redis.set(`session:${sessionId}:history`, JSON.stringify(history));
    return reply.send({ response: result });
  }

  if (message.includes("サマリ") || message.includes("summarize")) {
    const result = summarize();
    history.push({ role: "user", content: message });
    history.push({ role: "assistant", content: result });
    await redis.set(`session:${sessionId}:history`, JSON.stringify(history));
    return reply.send({ response: result });
  }

  const result = await agent.generate(message);
  const response = result.text;

  history.push({ role: "user", content: message });
  history.push({ role: "assistant", content: response });
  await redis.set(`session:${sessionId}:history`, JSON.stringify(history));

  return reply.send({ response });
});

app.listen({ port: PORT, host: "0.0.0.0" }, (err) => {
  if (err) {
    app.log.error(err);
    process.exit(1);
  }
});
