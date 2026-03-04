import http from "k6/http";
import { check, sleep } from "k6";
import { Counter, Rate, Trend } from "k6/metrics";
import { textSummary } from "https://jslib.k6.io/k6-summary/0.0.1/index.js";

// ============================================================
// Configuration
// ============================================================

const BASE_URL = __ENV.BASE_URL || "http://localhost:8081";
const ENDPOINT = __ENV.ENDPOINT || "/heavy";
const REQUEST_TIMEOUT = __ENV.REQUEST_TIMEOUT || "30s";
const HEAVY_CALC_N = __ENV.HEAVY_CALC_N || "";

// Autoscale test parameters
const WARMUP_VUS = parseInt(__ENV.WARMUP_VUS || "5", 10);
const PEAK_VUS = parseInt(__ENV.PEAK_VUS || "80", 10);
const WARMUP_DURATION = __ENV.WARMUP_DURATION || "30s";
const RAMP_UP_DURATION = __ENV.RAMP_UP_DURATION || "1m";
const PEAK_DURATION = __ENV.PEAK_DURATION || "3m";
const SUSTAIN_DURATION = __ENV.SUSTAIN_DURATION || "2m";
const COOLDOWN_DURATION = __ENV.COOLDOWN_DURATION || "2m";

const TIMEOUT_THRESHOLD_MS = parseInt(__ENV.TIMEOUT_THRESHOLD || "5000", 10);

// ============================================================
// Custom metrics
// ============================================================

const timeoutCounter = new Counter("timeout_count");
const timeoutRate = new Rate("timeout_rate");
const errorRate = new Rate("error_rate");
const serverDuration = new Trend("server_duration_ms", true);

// ============================================================
// Scenario: 4-phase autoscale test
//
// Phase 1 (Warmup):   Low load baseline       → HPA idle
// Phase 2 (Ramp-up):  Rapid increase to peak   → HPA triggers scale-out
// Phase 3 (Sustain):  Maintain peak load        → Observe scaled pods handling load
// Phase 4 (Cooldown): Gradual decrease          → HPA triggers scale-in
// ============================================================

export const options = {
  scenarios: {
    autoscale: {
      executor: "ramping-vus",
      startVUs: 0,
      stages: [
        // Phase 1: Warmup
        { duration: "10s", target: WARMUP_VUS },
        { duration: WARMUP_DURATION, target: WARMUP_VUS },

        // Phase 2: Ramp-up to peak
        { duration: RAMP_UP_DURATION, target: PEAK_VUS },

        // Phase 3: Sustain peak load
        { duration: PEAK_DURATION, target: PEAK_VUS },
        { duration: SUSTAIN_DURATION, target: PEAK_VUS },

        // Phase 4: Cooldown
        { duration: COOLDOWN_DURATION, target: WARMUP_VUS },
        { duration: "30s", target: 0 },
      ],
      gracefulRampDown: "15s",
    },
  },
  thresholds: {
    http_req_failed: ["rate<1"], // Don't fail — observe behavior
  },
};

// ============================================================
// Setup
// ============================================================

export function setup() {
  const healthRes = http.get(`${BASE_URL}/health`);
  const ok = check(healthRes, {
    "health check passed": (r) => r.status === 200,
  });

  if (!ok) {
    throw new Error(`Server at ${BASE_URL} is not responding to /health`);
  }

  const healthBody = JSON.parse(healthRes.body);
  const startTime = new Date().toISOString();

  console.log(`=== K8s Autoscale Test ===`);
  console.log(`Target: ${BASE_URL}${ENDPOINT}`);
  console.log(`Server language: ${healthBody.language}`);
  console.log(`Start time: ${startTime}`);
  console.log(`Phases:`);
  console.log(`  1. Warmup:   ${WARMUP_VUS} VUs for ${WARMUP_DURATION}`);
  console.log(`  2. Ramp-up:  → ${PEAK_VUS} VUs over ${RAMP_UP_DURATION}`);
  console.log(`  3. Peak:     ${PEAK_VUS} VUs for ${PEAK_DURATION} + ${SUSTAIN_DURATION}`);
  console.log(`  4. Cooldown: → ${WARMUP_VUS} VUs over ${COOLDOWN_DURATION}`);
  console.log(``);
  if (HEAVY_CALC_N) {
    console.log(`HEAVY_CALC_N: ${HEAVY_CALC_N} (via query param)`);
  }
  console.log(`TIP: Run 'make watch-hpa' in another terminal to observe scaling`);
  console.log(`==========================`);

  return { language: healthBody.language, startTime };
}

// ============================================================
// Main test function
// ============================================================

export default function () {
  const url = HEAVY_CALC_N
    ? `${BASE_URL}${ENDPOINT}?n=${HEAVY_CALC_N}`
    : `${BASE_URL}${ENDPOINT}`;
  const res = http.get(url, {
    timeout: REQUEST_TIMEOUT,
    tags: { endpoint: ENDPOINT },
  });

  check(res, {
    "status is 200": (r) => r.status === 200,
  });

  const isTimeout = res.timings.duration > TIMEOUT_THRESHOLD_MS;
  const isError = res.status !== 200;

  timeoutRate.add(isTimeout);
  errorRate.add(isError);
  if (isTimeout) {
    timeoutCounter.add(1);
  }

  if (res.status === 200) {
    try {
      const body = JSON.parse(res.body);
      if (body.durationMs !== undefined) {
        serverDuration.add(body.durationMs);
      }
    } catch (_e) {
      // ignore
    }
  }

  sleep(Math.random() * 0.5 + 0.25);
}

// ============================================================
// Summary
// ============================================================

export function handleSummary(data) {
  const totalRequests = data.metrics.http_reqs ? data.metrics.http_reqs.values.count : 0;
  const timeoutCount = data.metrics.timeout_count ? data.metrics.timeout_count.values.count : 0;
  const p95 = data.metrics.http_req_duration ? data.metrics.http_req_duration.values["p(95)"] : 0;
  const p99 = data.metrics.http_req_duration ? data.metrics.http_req_duration.values["p(99)"] : 0;
  const timeoutRateValue = data.metrics.timeout_rate ? data.metrics.timeout_rate.values.rate : 0;
  const errorRateValue = data.metrics.error_rate ? data.metrics.error_rate.values.rate : 0;

  const report = [
    "",
    "╔══════════════════════════════════════════════════════════════╗",
    "║             K8s AUTOSCALE TEST RESULTS                       ║",
    "╠══════════════════════════════════════════════════════════════╣",
    `║  Peak VUs:          ${PEAK_VUS}`.padEnd(63) + "║",
    `║  Total Requests:    ${totalRequests}`.padEnd(63) + "║",
    `║  Timeout Count:     ${timeoutCount}`.padEnd(63) + "║",
    `║  Timeout Rate:      ${(timeoutRateValue * 100).toFixed(2)}%`.padEnd(63) + "║",
    `║  Error Rate:        ${(errorRateValue * 100).toFixed(2)}%`.padEnd(63) + "║",
    `║  P95 Duration:      ${p95.toFixed(0)}ms`.padEnd(63) + "║",
    `║  P99 Duration:      ${p99.toFixed(0)}ms`.padEnd(63) + "║",
    "╠══════════════════════════════════════════════════════════════╣",
    `║  Check 'kubectl get hpa -n process-thread' for scaling      ║`,
    `║  history and 'kubectl get events -n process-thread' for      ║`,
    `║  scale-up/scale-down events.                                 ║`,
    "╚══════════════════════════════════════════════════════════════╝",
    "",
  ];

  const timestamp = new Date().toISOString().replace(/[:.]/g, "-");

  return {
    stdout: textSummary(data, { indent: " ", enableColors: true }) + "\n" + report.join("\n"),
    [`results/autoscale_${timestamp}.json`]: JSON.stringify(data, null, 2),
  };
}
