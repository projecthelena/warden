import http from "k6/http";
import { check, sleep } from "k6";

const baseURL = (__ENV.WARDEN_URL || "http://localhost:9096").replace(
  /\/$/,
  "",
);
const statusSlug = __ENV.STATUS_SLUG || "";

export const options = {
  scenarios: {
    public_status: {
      executor: "ramping-arrival-rate",
      startRate: Number(__ENV.START_RPS || 1),
      timeUnit: "1s",
      preAllocatedVUs: Number(__ENV.PREALLOCATED_VUS || 20),
      maxVUs: Number(__ENV.MAX_VUS || 200),
      stages: [
        {
          target: Number(__ENV.PEAK_RPS || 50),
          duration: __ENV.RAMP_DURATION || "5m",
        },
        {
          target: Number(__ENV.PEAK_RPS || 50),
          duration: __ENV.HOLD_DURATION || "10m",
        },
        { target: 0, duration: __ENV.COOLDOWN_DURATION || "2m" },
      ],
    },
  },
  thresholds: {
    http_req_failed: ["rate<0.01"],
    http_req_duration: ["p(95)<1000", "p(99)<2500"],
    dropped_iterations: ["count==0"],
  },
};

export default function () {
  const path = statusSlug ? `/api/s/${statusSlug}` : "/healthz";
  const response = http.get(`${baseURL}${path}`, { tags: { name: path } });
  check(response, {
    "status is successful": (r) => r.status >= 200 && r.status < 400,
  });
  sleep(0.05);
}

export function handleSummary(data) {
  const resultFile = __ENV.RESULT_FILE || "loadtest/results/summary.json";
  return {
    [resultFile]: JSON.stringify(data, null, 2),
  };
}
