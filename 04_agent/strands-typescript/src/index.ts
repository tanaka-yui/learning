import Fastify from "fastify";
import { Redis } from "ioredis";
import { createAgent } from "./agent.js";
import { prioritize } from "./skills/prioritize.js";
import { summarize } from "./skills/summarize.js";
import type { MessageData } from "@strands-agents/sdk";

const app = Fastify({ logger: true });
const redis = new Redis(process.env.REDIS_URL ?? "redis://localhost:6379");
const PORT = 4004;

app.post<{ Body: { message: string; sessionId: string } }>("/chat", async (req, reply) => {
  const { message, sessionId } = req.body;

  const historyRaw = await redis.get(`session:${sessionId}:history`);
  const history: MessageData[] = historyRaw ? JSON.parse(historyRaw) : [];

  if (message.includes("優先") || message.includes("prioritize")) {
    const result = prioritize();
    history.push(
      { role: "user", content: [{ text: message }] },
      { role: "assistant", content: [{ text: result }] }
    );
    await redis.set(`session:${sessionId}:history`, JSON.stringify(history));
    return reply.send({ response: result });
  }

  if (message.includes("サマリ") || message.includes("summarize")) {
    const result = summarize();
    history.push(
      { role: "user", content: [{ text: message }] },
      { role: "assistant", content: [{ text: result }] }
    );
    await redis.set(`session:${sessionId}:history`, JSON.stringify(history));
    return reply.send({ response: result });
  }

  const agent = createAgent(history.length > 0 ? history : undefined);
  const result = await agent.invoke(message);
  const response = result.toString();

  history.push(
    { role: "user", content: [{ text: message }] },
    { role: "assistant", content: [{ text: response }] }
  );
  await redis.set(`session:${sessionId}:history`, JSON.stringify(history));

  return reply.send({ response });
});

app.listen({ port: PORT, host: "0.0.0.0" }, (err) => {
  if (err) {
    app.log.error(err);
    process.exit(1);
  }
});
