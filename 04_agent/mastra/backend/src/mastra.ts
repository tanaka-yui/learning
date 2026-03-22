import { Mastra } from "@mastra/core";
import { registerApiRoute } from "@mastra/core/server";
import { taskAgent } from "./agent.js";
import { prioritize } from "./skills/prioritize.js";
import { summarize } from "./skills/summarize.js";

export const mastra = new Mastra({
  agents: { taskAgent },
  server: {
    host: "0.0.0.0",
    cors: {
      origin: "*",
      allowMethods: ["GET", "POST", "OPTIONS"],
      allowHeaders: ["Content-Type"],
    },
    apiRoutes: [
      registerApiRoute("/chat", {
        method: "POST",
        handler: async (c) => {
          const { message, sessionId } = await c.req.json<{ message: string; sessionId: string }>();

          if (message.includes("優先") || message.includes("prioritize")) {
            return c.json({ response: prioritize() });
          }

          if (message.includes("サマリ") || message.includes("summarize")) {
            return c.json({ response: summarize() });
          }

          const result = await taskAgent.generate(message, {
            memory: { resource: "default-user", thread: sessionId },
          });

          return c.json({ response: result.text });
        },
      }),
    ],
  },
});
