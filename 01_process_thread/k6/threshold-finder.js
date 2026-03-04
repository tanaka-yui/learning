import http from "k6/http";
import { check, sleep } from "k6";
import { Counter, Rate, Trend } from "k6/metrics";
import { textSummary } from "https://jslib.k6.io/k6-summary/0.0.1/index.js";
import exec from "k6/execution";

// ============================================================
// Configuration
// ============================================================

const BASE_URL = __ENV.BASE_URL || "http://localhost:8081";
const ENDPOINT = __ENV.ENDPOINT || "/heavy";
const REQUEST_TIMEOUT = __ENV.REQUEST_TIMEOUT || "30s";
const HEAVY_CALC_N = __ENV.HEAVY_CALC_N || "";

// Threshold finder parameters
const START_VUS = parseInt(__ENV.START_VUS || "1", 10);
const MAX_VUS = parseInt(__ENV.MAX_VUS || "100", 10);
const STEP_SIZE = parseInt(__ENV.STEP_SIZE || "5", 10);
const STEP_DURATION = __ENV.STEP_DURATION || "30s";
const TIMEOUT_THRESHOLD_MS = parseInt(__ENV.TIMEOUT_THRESHOLD || "5000", 10);

// ============================================================
// Custom metrics
// ============================================================

const timeoutCounter = new Counter("timeout_count");
const timeoutRate = new Rate("timeout_rate");
const errorRate = new Rate("error_rate");
const serverDuration = new Trend("server_duration_ms", true);
const stepTimeoutRate = new Trend("step_timeout_rate");

// ============================================================
// Generate stages dynamically
// ============================================================

function generateStages() {
  const stages = [];

  // First stage: start at START_VUS
  stages.push({ duration: STEP_DURATION, target: START_VUS });

  // Ramp up in steps
  for (let vus = START_VUS + STEP_SIZE; vus <= MAX_VUS; vus += STEP_SIZE) {
    // Quick ramp to next step (5s transition)
    stages.push({ duration: "5s", target: vus });
    // Hold at this level
    stages.push({ duration: STEP_DURATION, target: vus });
  }

  // Ensure we hit MAX_VUS if not already
  const lastTarget = stages[stages.length - 1].target;
  if (lastTarget < MAX_VUS) {
    stages.push({ duration: "5s", target: MAX_VUS });
    stages.push({ duration: STEP_DURATION, target: MAX_VUS });
  }

  // Ramp down
  stages.push({ duration: "10s", target: 0 });

  return stages;
}

const stages = generateStages();

// ============================================================
// k6 options
// ============================================================

export const options = {
  scenarios: {
    threshold_finder: {
      executor: "ramping-vus",
      startVUs: 0,
      stages: stages,
      gracefulRampDown: "10s",
    },
  },
  thresholds: {
    http_req_failed: ["rate<1"], // Don't fail the test — we want to find the limit
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

  console.log(`=== Threshold Finder ===`);
  console.log(`Target: ${BASE_URL}${ENDPOINT}`);
  console.log(`Server language: ${healthBody.language}`);
  console.log(`VU range: ${START_VUS} → ${MAX_VUS} (step: ${STEP_SIZE})`);
  console.log(`Step duration: ${STEP_DURATION}`);
  if (HEAVY_CALC_N) {
    console.log(`HEAVY_CALC_N: ${HEAVY_CALC_N} (via query param)`);
  }
  console.log(`Timeout threshold: ${TIMEOUT_THRESHOLD_MS}ms`);
  console.log(`Total stages: ${stages.length}`);
  console.log(`========================`);

  return { language: healthBody.language };
}

// ============================================================
// Main test function
// ============================================================

// Each VU logs only its first timeout to avoid noise
let firstTimeoutLogged = false;

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
    if (!firstTimeoutLogged) {
      firstTimeoutLogged = true;
      console.warn(
        `⚠ Timeout at ~${exec.instance.vusActive} VUs: ${res.timings.duration.toFixed(0)}ms > ${TIMEOUT_THRESHOLD_MS}ms`
      );
    }
  }

  // Track timeout rate per step for summary analysis
  stepTimeoutRate.add(isTimeout ? 1 : 0);

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
  const avgDuration = data.metrics.http_req_duration ? data.metrics.http_req_duration.values.avg : 0;
  const maxDuration = data.metrics.http_req_duration ? data.metrics.http_req_duration.values.max : 0;
  const timeoutRateValue = data.metrics.timeout_rate ? data.metrics.timeout_rate.values.rate : 0;

  const report = [
    "",
    "╔══════════════════════════════════════════════════════════════╗",
    "║               THRESHOLD FINDER RESULTS                      ║",
    "╠══════════════════════════════════════════════════════════════╣",
    `║  VU Range:          ${START_VUS} → ${MAX_VUS} (step: ${STEP_SIZE})`.padEnd(63) + "║",
    `║  Timeout Threshold: ${TIMEOUT_THRESHOLD_MS}ms`.padEnd(63) + "║",
    "╠══════════════════════════════════════════════════════════════╣",
    `║  Total Requests:    ${totalRequests}`.padEnd(63) + "║",
    `║  Timeout Count:     ${timeoutCount}`.padEnd(63) + "║",
    `║  Timeout Rate:      ${(timeoutRateValue * 100).toFixed(2)}%`.padEnd(63) + "║",
    `║  Avg Duration:      ${avgDuration.toFixed(0)}ms`.padEnd(63) + "║",
    `║  P95 Duration:      ${p95.toFixed(0)}ms`.padEnd(63) + "║",
    `║  P99 Duration:      ${p99.toFixed(0)}ms`.padEnd(63) + "║",
    `║  Max Duration:      ${maxDuration.toFixed(0)}ms`.padEnd(63) + "║",
    "╠══════════════════════════════════════════════════════════════╣",
  ];

  if (timeoutCount > 0) {
    report.push(`║  ⚠ Timeouts detected! (>${TIMEOUT_THRESHOLD_MS}ms)`.padEnd(63) + "║");
    report.push(`║  Review the k6 output above for per-stage breakdown.`.padEnd(63) + "║");
  } else {
    report.push(`║  ✓ No timeouts detected up to ${MAX_VUS} VUs`.padEnd(63) + "║");
    report.push(`║  Consider increasing MAX_VUS for further testing.`.padEnd(63) + "║");
  }

  report.push("╚══════════════════════════════════════════════════════════════╝");
  report.push("");

  const timestamp = new Date().toISOString().replace(/[:.]/g, "-");

  return {
    stdout: textSummary(data, { indent: " ", enableColors: true }) + "\n" + report.join("\n"),
    [`results/threshold_${timestamp}.json`]: JSON.stringify(data, null, 2),
  };
}
