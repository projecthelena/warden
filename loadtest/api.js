import http from "k6/http";
import exec from "k6/execution";
import { check, fail, sleep } from "k6";

const baseURL = (__ENV.WARDEN_URL || "http://localhost:9096").replace(/\/$/, "");
const statusSlug = __ENV.STATUS_SLUG || "";
const configuredGroupID = __ENV.GROUP_ID || "";
const pageSize = Number(__ENV.PAGE_SIZE || 25);
const searchTerm = __ENV.SEARCH_TERM || "Load Monitor 00500";
const profile = __ENV.TEST_PROFILE || "all";

function arrivalScenario(execName, prefix, defaultRate) {
  return {
    executor: "ramping-arrival-rate",
    exec: execName,
    startRate: Number(__ENV[`${prefix}_START_RPS`] || 1),
    timeUnit: "1s",
    preAllocatedVUs: Number(__ENV[`${prefix}_PREALLOCATED_VUS`] || 10),
    maxVUs: Number(__ENV[`${prefix}_MAX_VUS`] || 100),
    stages: [
      { target: Number(__ENV[`${prefix}_PEAK_RPS`] || defaultRate), duration: __ENV.RAMP_DURATION || "5m" },
      { target: Number(__ENV[`${prefix}_PEAK_RPS`] || defaultRate), duration: __ENV.HOLD_DURATION || "10m" },
      { target: 0, duration: __ENV.COOLDOWN_DURATION || "2m" },
    ],
  };
}

const scenarios = {};
if (profile === "all" || profile === "public") scenarios.public_status = arrivalScenario("publicStatus", "PUBLIC", 5);
if (profile === "all" || profile === "operator") scenarios.operator_dashboard = arrivalScenario("operatorDashboard", "OPERATOR", 1);

const thresholds = {
  http_req_failed: ["rate<0.01"],
  "http_req_failed{journey:public}": ["rate<0.01"],
  "http_req_failed{journey:operator}": ["rate<0.01"],
  "http_req_duration{journey:public}": ["p(95)<1000", "p(99)<2500"],
  "http_req_duration{journey:operator}": ["p(95)<1000", "p(99)<2500"],
  checks: ["rate>0.99"],
  dropped_iterations: ["count==0"],
};

const endpointNames = [
  "public overview",
  "public group page",
  "operator overview",
  "operator monitor list",
  "operator pagination",
  "operator search",
  "operator filter",
  "monitor uptime",
  "monitor latency",
  "monitor events",
];
endpointNames.forEach((name) => {
  thresholds[`http_req_failed{name:${name}}`] = ["rate<0.01"];
  thresholds[`http_req_duration{name:${name}}`] = ["p(95)<1000", "p(99)<2500"];
});

export const options = {
  scenarios,
  thresholds,
  summaryTrendStats: ["avg", "min", "med", "p(90)", "p(95)", "p(99)", "max"],
};

function parseJSON(response, label) {
  try {
    return response.json();
  } catch (_) {
    check(response, { [`${label} returns JSON`]: () => false });
    return null;
  }
}

function isOK(response, label) {
  return check(response, { [`${label} returns 200`]: (r) => r.status === 200 });
}

function groupFromOverview(payload) {
  const groups = payload && Array.isArray(payload.groups) ? payload.groups : [];
  if (configuredGroupID) return groups.find((group) => group.id === configuredGroupID);
  return groups.find((group) => Number(group.monitorCount || 0) > 0);
}

function queryString(values) {
  return Object.keys(values)
    .filter((key) => values[key] !== undefined && values[key] !== "")
    .map((key) => `${encodeURIComponent(key)}=${encodeURIComponent(String(values[key]))}`)
    .join("&");
}

function publicDetailPath(groupID, extra = {}) {
  const params = queryString({
    group: groupID,
    page: String(extra.page || 1),
    page_size: String(pageSize),
    status: extra.status || "all",
    search: extra.search,
  });
  return `/api/s/${encodeURIComponent(statusSlug)}?${params}`;
}

function operatorListPath(groupID, extra = {}) {
  const params = queryString({
    page: String(extra.page || 1),
    page_size: String(pageSize),
    status: extra.status || "all",
    search: extra.search,
  });
  return `/api/groups/${encodeURIComponent(groupID)}/monitors?${params}`;
}

export function setup() {
  const data = {};
  if (profile === "all" || profile === "operator") {
    if (!__ENV.WARDEN_USERNAME || !__ENV.WARDEN_PASSWORD) {
      fail("WARDEN_USERNAME and WARDEN_PASSWORD are required for the operator scenario");
    }
    const login = http.post(
      `${baseURL}/api/auth/login`,
      JSON.stringify({ username: __ENV.WARDEN_USERNAME, password: __ENV.WARDEN_PASSWORD }),
      { headers: { "Content-Type": "application/json" }, tags: { name: "operator login", journey: "setup" } },
    );
    if (login.status !== 200) fail(`operator login failed with status ${login.status}`);
    const setCookie = login.headers["Set-Cookie"];
    if (!setCookie) fail("operator login did not return a session cookie");
    data.cookie = setCookie.split(";")[0];
  }
  return data;
}

export function publicStatus() {
  if (!statusSlug) fail("STATUS_SLUG is required for the public scenario");
  const overview = http.get(`${baseURL}/api/s/${encodeURIComponent(statusSlug)}`, {
    tags: { name: "public overview", journey: "public" },
  });
  if (!isOK(overview, "public overview")) return;

  const group = groupFromOverview(parseJSON(overview, "public overview"));
  if (!check(group, { "public overview contains the load group": (value) => Boolean(value) })) return;

  const detail = http.get(`${baseURL}${publicDetailPath(group.id)}`, {
    tags: { name: "public group page", journey: "public" },
  });
  isOK(detail, "public group page");
  check(parseJSON(detail, "public group page"), {
    "public group page is bounded": (body) =>
      body && body.pagination && body.pagination.pageSize === pageSize && body.pagination.total >= pageSize,
  });
  sleep(0.1);
}

export function operatorDashboard(data) {
  const auth = { headers: { Cookie: data.cookie } };
  const overview = http.get(`${baseURL}/api/overview`, {
    ...auth,
    tags: { name: "operator overview", journey: "operator" },
  });
  if (!isOK(overview, "operator overview")) return;

  const group = groupFromOverview(parseJSON(overview, "operator overview"));
  if (!check(group, { "operator overview contains the load group": (value) => Boolean(value) })) return;

  const firstPage = http.get(`${baseURL}${operatorListPath(group.id)}`, {
    ...auth,
    tags: { name: "operator monitor list", journey: "operator" },
  });
  if (!isOK(firstPage, "operator monitor list")) return;

  const listPayload = parseJSON(firstPage, "operator monitor list");
  const pagination = listPayload && listPayload.pagination;
  const totalPages = pagination ? pagination.totalPages : 0;
  const page = totalPages > 0 ? (exec.scenario.iterationInTest % totalPages) + 1 : 1;
  const filters = ["operational", "issues", "paused"];
  const status = filters[exec.scenario.iterationInTest % filters.length];

  const navigation = http.batch([
    ["GET", `${baseURL}${operatorListPath(group.id, { page })}`, null,
      { ...auth, tags: { name: "operator pagination", journey: "operator" } }],
    ["GET", `${baseURL}${operatorListPath(group.id, { search: searchTerm })}`, null,
      { ...auth, tags: { name: "operator search", journey: "operator" } }],
    ["GET", `${baseURL}${operatorListPath(group.id, { status })}`, null,
      { ...auth, tags: { name: "operator filter", journey: "operator" } }],
  ]);
  check(navigation, {
    "pagination search and filter return 200": (responses) => responses.every((response) => response.status === 200),
  });

  const pagePayload = parseJSON(navigation[0], "operator pagination");
  const searchPayload = parseJSON(navigation[1], "operator search");
  const filterPayload = parseJSON(navigation[2], "operator filter");
  check(pagePayload, {
    "pagination returns the requested bounded page": (body) =>
      body && body.pagination && body.pagination.page === page &&
      body.pagination.pageSize === pageSize && body.group.monitors.length <= pageSize,
  });
  check(searchPayload, {
    "search finds only matching monitors": (body) => {
      if (!body || !body.group || !Array.isArray(body.group.monitors) || body.group.monitors.length === 0) return false;
      const needle = searchTerm.toLowerCase();
      return body.group.monitors.every((item) =>
        [item.name, item.url, item.id].some((value) => String(value || "").toLowerCase().includes(needle)),
      );
    },
  });
  check(filterPayload, {
    "status filter returns only matching monitors": (body) => {
      if (!body || !body.group || !Array.isArray(body.group.monitors)) return false;
      return body.group.monitors.every((item) =>
        status === "operational" ? item.status === "up" :
          status === "issues" ? item.status === "down" || item.status === "degraded" : item.status === "paused",
      );
    },
  });

  const monitors = pagePayload && pagePayload.group && Array.isArray(pagePayload.group.monitors)
    ? pagePayload.group.monitors
    : [];
  const monitor = monitors[exec.scenario.iterationInTest % Math.max(monitors.length, 1)];
  if (!check(monitor, { "operator page contains a monitor": (value) => Boolean(value && value.id) })) return;

  const monitorID = encodeURIComponent(monitor.id);
  const detail = http.batch([
    ["GET", `${baseURL}/api/monitors/${monitorID}/uptime`, null,
      { ...auth, tags: { name: "monitor uptime", journey: "operator" } }],
    ["GET", `${baseURL}/api/monitors/${monitorID}/latency?range=1h`, null,
      { ...auth, tags: { name: "monitor latency", journey: "operator" } }],
    ["GET", `${baseURL}/api/monitors/${monitorID}/events?limit=100`, null,
      { ...auth, tags: { name: "monitor events", journey: "operator" } }],
  ]);
  check(detail, {
    "monitor detail APIs return 200": (responses) => responses.every((response) => response.status === 200),
  });
  const uptime = parseJSON(detail[0], "monitor uptime");
  const latency = parseJSON(detail[1], "monitor latency");
  const events = parseJSON(detail[2], "monitor events");
  check(uptime, {
    "monitor uptime contains all windows": (body) =>
      body && body.windows && body.windows.last24Hours && body.windows.last7Days && body.windows.last30Days,
  });
  check(latency, { "monitor latency is a series": (body) => Array.isArray(body) });
  check(events, { "monitor events is a list": (body) => Array.isArray(body) });
  sleep(0.2);
}

export function handleSummary(data) {
  const resultFile = __ENV.RESULT_FILE || "loadtest/results/summary.json";
  return { [resultFile]: JSON.stringify(data, null, 2) };
}
