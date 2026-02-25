import Fastify from "fastify";

const server = Fastify({ logger: true });

const HEAVY_CALC_N = Number(process.env["HEAVY_CALC_N"] ?? "40");

const fibonacci = (n: number): number => {
  if (n <= 1) return n;
  return fibonacci(n - 1) + fibonacci(n - 2);
};

server.get("/health", async () => {
  return { status: "ok", language: "typescript" };
});

server.get("/heavy", async () => {
  const startedAt = new Date().toISOString();
  const start = performance.now();

  fibonacci(HEAVY_CALC_N);

  const end = performance.now();
  const finishedAt = new Date().toISOString();

  return {
    language: "typescript",
    threadId: `pid-${process.pid}`,
    startedAt,
    finishedAt,
    durationMs: Math.round(end - start),
  };
});

server.listen({ port: 3000, host: "0.0.0.0" }, (err) => {
  if (err) {
    server.log.error(err);
    process.exit(1);
  }
});
