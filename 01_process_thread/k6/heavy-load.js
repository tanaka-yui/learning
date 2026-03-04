import http from "k6/http";
import { check, sleep } from "k6";
import { Counter, Rate, Trend } from "k6/metrics";
import { textSummary } from "https://jslib.k6.io/k6-summary/0.0.1/index.js";

// ============================================================
// Configuration (all configurable via environment variables)
// ============================================================

const BASE_URL = __ENV.BASE_URL || "http://localhost:8081";
const ENDPOINT = __ENV.ENDPOINT || "/heavy";
const REQUEST_TIMEOUT = __ENV.REQUEST_TIMEOUT || "30s";
const HEAVY_CALC_N = __ENV.HEAVY_CALC_N || "";

// Test profile: "constant" | "ramping" | "stress" | "spike" | "soak"
const TEST_PROFILE = __ENV.TEST_PROFILE || "ramping";

const TARGET_VUS = parseInt(__ENV.TARGET_VUS || "50", 10);
const DURATION = __ENV.DURATION || "60s";
const RAMP_UP = __ENV.RAMP_UP || "30s";
const RAMP_DOWN = __ENV.RAMP_DOWN || "10s";
const MAX_VUS = parseInt(__ENV.MAX_VUS || "200", 10);
const SOAK_DURATION = __ENV.SOAK_DURATION || "5m";

const THRESHOLD_P95 = __ENV.THRESHOLD_P95 || "10000";
const THRESHOLD_P99 = __ENV.THRESHOLD_P99 || "15000";
const THRESHOLD_TIMEOUT_RATE = __ENV.THRESHOLD_TIMEOUT_RATE || "0.1";
const THRESHOLD_ERROR_RATE = __ENV.THRESHOLD_ERROR_RATE || "0.05";

// ============================================================
// Custom metrics
// ============================================================

const timeoutCounter = new Counter("timeout_count");
const timeoutRate = new Rate("timeout_rate");
const errorRate = new Rate("error_rate");
const serverDuration = new Trend("server_duration_ms", true);

// ============================================================
// Scenario definitions
// ============================================================

const scenarios = {
  constant: {
    constant: {
      executor: "constant-vus",
      vus: TARGET_VUS,
      duration: DURATION,
    },
  },

  ramping: {
    ramping: {
      executor: "ramping-vus",
      startVUs: 0,
      stages: [
        { duration: RAMP_UP, target: Math.floor(TARGET_VUS * 0.25) },
        { duration: RAMP_UP, target: Math.floor(TARGET_VUS * 0.5) },
        { duration: RAMP_UP, target: Math.floor(TARGET_VUS * 0.75) },
        { duration: RAMP_UP, target: TARGET_VUS },
        { duration: DURATION, target: TARGET_VUS },
        { duration: RAMP_DOWN, target: 0 },
      ],
      gracefulRampDown: "10s",
    },
  },

  stress: {
    stress: {
      executor: "ramping-vus",
      startVUs: 0,
      stages: [
        { duration: RAMP_UP, target: Math.floor(MAX_VUS * 0.2) },
        { duration: RAMP_UP, target: Math.floor(MAX_VUS * 0.4) },
        { duration: RAMP_UP, target: Math.floor(MAX_VUS * 0.6) },
        { duration: RAMP_UP, target: Math.floor(MAX_VUS * 0.8) },
        { duration: RAMP_UP, target: MAX_VUS },
        { duration: DURATION, target: MAX_VUS },
        { duration: RAMP_DOWN, target: 0 },
      ],
      gracefulRampDown: "30s",
    },
  },

  spike: {
    spike: {
      executor: "ramping-vus",
      startVUs: 0,
      stages: [
        { duration: "10s", target: 1 },
        { duration: "5s", target: MAX_VUS },
        { duration: DURATION, target: MAX_VUS },
        { duration: "5s", target: 1 },
        { duration: "10s", target: 0 },
      ],
      gracefulRampDown: "10s",
    },
  },

  soak: {
    soak: {
      executor: "ramping-vus",
      startVUs: 0,
      stages: [
        { duration: RAMP_UP, target: TARGET_VUS },
        { duration: SOAK_DURATION, target: TARGET_VUS },
        { duration: RAMP_DOWN, target: 0 },
      ],
      gracefulRampDown: "10s",
    },
  },
};

const selectedScenarios = scenarios[TEST_PROFILE];
if (!selectedScenarios) {
  throw new Error(
    `Unknown TEST_PROFILE: "${TEST_PROFILE}". Valid values: ${Object.keys(scenarios).join(", ")}`
  );
}

// ============================================================
// k6 options
// ============================================================

export const options = {
  scenarios: selectedScenarios,
  thresholds: {
    http_req_duration: [
      `p(95)<${THRESHOLD_P95}`,
      `p(99)<${THRESHOLD_P99}`,
    ],
    timeout_rate: [`rate<${THRESHOLD_TIMEOUT_RATE}`],
    error_rate: [`rate<${THRESHOLD_ERROR_RATE}`],
    http_req_failed: ["rate<0.05"],
  },
};

// ============================================================
// Setup: health check before test starts
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
  console.log(`=== k6 Load Test ===`);
  console.log(`Target: ${BASE_URL}${ENDPOINT}`);
  console.log(`Server language: ${healthBody.language}`);
  console.log(`Profile: ${TEST_PROFILE}`);
  console.log(`Target VUs: ${TARGET_VUS}`);
  if (HEAVY_CALC_N) {
    console.log(`HEAVY_CALC_N: ${HEAVY_CALC_N} (via query param)`);
  }
  console.log(`Timeout threshold (p95): ${THRESHOLD_P95}ms`);
  console.log(`====================`);

  return { language: healthBody.language };
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
    "response has durationMs": (r) => {
      if (r.status !== 200) return false;
      try {
        return JSON.parse(r.body).durationMs !== undefined;
      } catch (_e) {
        return false;
      }
    },
  });

  // Record custom metrics
  const isTimeout = res.timings.duration > parseInt(THRESHOLD_P95, 10);
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
      // ignore parse errors
    }
  }

  sleep(Math.random() + 0.5);
}

// ============================================================
// Summary: output to stdout + JSON file
// ============================================================

export function handleSummary(data) {
  const timestamp = new Date().toISOString().replace(/[:.]/g, "-");
  return {
    stdout: textSummary(data, { indent: " ", enableColors: true }),
    [`results/${TEST_PROFILE}_${timestamp}.json`]: JSON.stringify(data, null, 2),
  };
}
