import { Mastra } from "@mastra/core/mastra";
import { taskAgent } from "./agents/task-agent.js";

export const mastra = new Mastra({
  agents: { taskAgent },
  server: {
    host: "0.0.0.0",
    port: 4001,
    cors: {
      origin: "*",
      allowMethods: ["GET", "POST", "OPTIONS"],
      allowHeaders: ["Content-Type"],
    },
  },
});
