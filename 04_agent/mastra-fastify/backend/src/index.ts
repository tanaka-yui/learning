import Fastify from "fastify";
import cors from "@fastify/cors";
import { mastra } from "./agent.js";

const app = Fastify({ logger: true });
await app.register(cors, { origin: true });
const PORT = 4002;

const agent = mastra.getAgent("taskAgent");

app.post<{ Body: { message: string; sessionId: string } }>("/chat", async (req, reply) => {
  const { message, sessionId } = req.body;

  const result = await agent.generate(message);

  return reply.send({ response: result.text });
});

app.listen({ port: PORT, host: "0.0.0.0" }, (err) => {
  if (err) {
    app.log.error(err);
    process.exit(1);
  }
});
