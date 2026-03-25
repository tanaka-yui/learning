import { VoltAgent } from "@voltagent/core";
import { honoServer } from "@voltagent/server-hono";
import { taskAgent } from "./agent.js";

new VoltAgent({
  agents: { taskAgent },
  server: honoServer({
    port: 4006,
  }),
});
