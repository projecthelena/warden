# HTTP check diagnostics

New HTTP monitors created without custom request configuration use automatic retry, and the create-monitor form selects **Automatic** by default. This mode records timing evidence automatically; the user does not need to enable tracing separately. Existing monitors keep their stored retry policy. To collect traces for existing/manual-mode monitors too, set `HTTP_DIAGNOSTICS_ENABLED=true` and restart Warden; this global flag is disabled by default. Confirmation, cooldown and recovery rules continue to use the final local check result.

Open a monitor's **Checks** tab for a 24-hour summary of failed checks and recoveries, plus a paginated list of individual checks. Each row explains the observation and a next step; timing tables live under **Technical details**. These are Warden's synthetic requests, not application-user traffic. Counts include retained data only and indicate trace coverage, so missing historical traces are not counted as successful first attempts.

The authenticated `/api/monitors/{id}/checks?limit=20` endpoint accepts limits from 1 to 100 and `beforeId` for older stored checks. Pagination uses insertion IDs so new writes cannot shift older pages. `/api/monitors/{id}/checks/summary` aggregates the last 24 hours, counting each check once per first failed phase. MCP `get_monitor_checks` exposes pagination and the same summary; `get_monitor_events` includes event evidence.

Each check contains separate configured retry attempts and redirect hops. Phase records also identify transport attempts within a hop, including automatic retries on a stale connection. A hop records its hostname, connected remote IP when available, new/reused connection, HTTP status, failure phase and the observed DNS, TCP, TLS, request-write and first-response-byte timings. It stores no URL paths, queries, request bodies, credentials or request headers in the trace. Existing failed-response diagnostics retain their existing filtering rules.

Durations are milliseconds. Phase offsets are relative to the start of a hop. Connection acquisition encompasses DNS/TCP/TLS and pool waiting; it is not an additional network duration. Parallel connection attempts can overlap. Do not sum phases. Missing phases mean unobserved, not zero: a reused connection normally has no DNS/TCP/TLS phases. Incomplete phases report elapsed time until the trace snapshot. Callbacks arriving after that snapshot cannot change stored evidence.

Hop total ends when response headers arrive or the transport fails. Attempt total includes all its hops and any existing failed-body capture. The monitor's existing latency remains the final configured attempt's elapsed time; retry backoff is not included. Response wait ends at the first header byte, which can be an informational response. It includes network transit, proxies, load balancers and backend processing, and is **not server execution time**.

Traces are bounded to 11 hops per attempt and 64 phase records per hop. Truncated traces are marked. They are stored in the existing checks/events retention windows on both SQLite and PostgreSQL; enabling them increases storage and allocation costs. Compare throughput, database growth, queue depth and dropped checks with diagnostics off/on before enabling them on a large installation. Rollup timing diagnostics are controlled independently.

## Automatic retries

Automatic mode performs at most one additional attempt for GET/HEAD requests without a body. It considers temporary DNS/connection failures, early EOFs and HTTP 408/502/503/504 responses. It does not immediately repeat POST/PUT/DELETE, permanent DNS or certificate errors, 401/403/404/429/500 responses, blocked targets, or responses containing `Retry-After`. Respecting these distinctions avoids replaying application writes or amplifying persistent errors.

The retry waits a random 0.5–1.5 seconds and uses only the time remaining in the original check timeout (five seconds by default). If less than 100ms would remain, there is no immediate retry; the next scheduled check still runs normally. A first attempt that consumes the entire timeout therefore does not receive an extra full timeout. There is no retry loop beyond this single attempt. At most five of the 50 workers can perform automatic retries concurrently; checks that start while a queue is at least half full skip immediate retries. Capacity-limited decisions are recorded, and normal scheduled checks retain priority.

A successful retry makes the check successful, so it does not open an outage by itself. Both attempts remain in the history, and the 24-hour summary counts the recovery. If all eligible attempts fail, normal confirmation, outage and notification rules apply. Retries tolerate transient faults; they do not repair networking or application code.

API configuration uses `requestConfig.autoRetry=true`. Supplying custom request configuration preserves the chosen policy; `autoRetry` cannot be combined with a positive `retryCount`. The older manual `retryCount` mode (0–5, one-second delay, existing per-attempt timeout) remains available under advanced settings. Manual retries are explicit and are not restricted by the automatic-mode transient-error classifier. The global diagnostics flag controls additional tracing; automatic mode always preserves its own attempt evidence.

## Simultaneous failures

Broad outage alerts no longer infer that Warden's own Internet connection is broken solely from a percentage of monitors going down. Recent failures in the same five-minute outage cluster, across at least three distinct hostnames and in the same DNS/TCP/TLS phase, provide limited evidence of a possible connectivity problem. Distinct hostnames can still share a resolver, CDN, provider or backend.

If either worker or result queue is at least half full, the alert reports local queue pressure instead. Low queue depth does not establish healthy CPU, DNS or networking. This diagnosis does not change confirmation, cooldown, grouping, recovery or maintenance behavior and does not identify an ISP as the cause.

## Optional second probe

Build `go build -o warden-probe ./cmd/warden-probe` for the machine that will run the second probe. It is a small HTTP service, not an AI agent. Prepare a private JSON file mapping existing Warden monitor IDs to URLs:

```json
{
  "m-example": "https://example.com/health"
}
```

Provide the same independently generated secret (at least 32 characters) as `SECONDARY_PROBE_TOKEN` on both hosts. Start the probe with:

```sh
./warden-probe -targets /etc/warden-probe/targets.json
```

It listens on `127.0.0.1:9097` by default. Use an SSH tunnel or a TLS reverse proxy for access from Warden. Configure Warden with `SECONDARY_PROBE_URL=https://probe.example.com` (or a loopback HTTP tunnel URL) and `SECONDARY_PROBE_TOKEN`; use automatic mode or enable HTTP diagnostics for the monitors to corroborate. Warden refuses non-TLS remote endpoints and does not follow probe API redirects. Keep the token in a secret manager or restricted environment file, never in a URL or source control.

On failed HTTP checks, Warden requests corroboration by monitor ID. It never sends the monitor URL, headers, body or credentials to the probe. Unregistered IDs return no observation. The probe performs a GET with a two-second timeout and follows normal redirects; configure the exact desired URL independently. Authenticated application checks and custom methods are not reproduced, so a response only confirms HTTP reachability, even for a 401 or 503. It does not certify application health or override the local result.

Corroboration is best effort: at most one request starts per second, at most two can run concurrently, and the client timeout is three seconds. Busy, missing, stale, malformed or unreachable probe observations remain unavailable, not evidence of a target failure. Probe clocks must agree within 30 seconds. The probe server also limits concurrent target checks to two and preserves Warden's link-local address blocking.

Two PCs on the same home network can distinguish failures specific to one host or path. If both fail, they still share the gateway, resolver and Internet connection: this cannot distinguish an ISP outage from a destination problem. An off-network probe or matching service-side logs are needed for stronger attribution.
