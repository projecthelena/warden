# Monitor Types

A monitor runs one of four checks against its target. The check type is picked when the
monitor is created and can be changed later from the monitor's settings.

Every type shares the same machinery around the check itself — confirmation thresholds,
notification cooldowns, flap detection, incidents, the daily digest and retention all
work the same way regardless of what is being probed.

| Type   | Target format             | Up when                                        |
| ------ | ------------------------- | ---------------------------------------------- |
| `http` | `https://example.com/health` | the response status code is accepted        |
| `tcp`  | `db.example.com:5432`     | a TCP connection to that port completes        |
| `ping` | `192.168.1.1`             | an ICMP echo request gets a reply              |
| `dns`  | `example.com`             | the lookup returns at least one record         |

Monitors created before check types existed are `http`, and keep behaving exactly as
they did.

Hostnames may contain underscores. They are not legal in a strict reading of DNS, but
Docker's embedded resolver serves compose service names that use them, so `db_primary`
is a target worth accepting.

## Shared options

Two settings apply to every type:

- **Timeout** — how long to wait for the target. Defaults to 5 seconds.
- **Retry on failure** — how many extra attempts to make before recording a failure,
  one second apart. Defaults to no retry.

A target that can never work — a malformed address, or an ICMP socket the process is not
allowed to open — is not retried, since retrying it only delays the result.

## HTTP

The original check, unchanged. Request method, custom headers, request body, accepted
status codes and redirect policy all live under **Request Configuration**. TLS
certificate expiry is captured here, which is what drives the SSL expiry warnings — the
other types have no certificate to report.

## TCP

Opens a TCP connection and closes it immediately. Nothing is written to the connection:
a service that accepts is a service that is listening. Use it for databases, message
brokers, SSH, or anything else that speaks a protocol Warden doesn't need to understand.

The target must be `host:port`. IPv6 literals are bracketed: `[2001:db8::1]:5432`.

## Ping (ICMP)

Sends one ICMP echo request and waits for the matching reply. The target is a bare
hostname or IP — no scheme, no port.

**ICMP needs permission that ordinary sockets don't.** Warden asks for an unprivileged
ICMP datagram socket first and falls back to a raw socket. If neither is available the
monitor reports `icmp socket unavailable` instead of pretending the host is down.

| Environment       | What you need                                                       |
| ----------------- | ------------------------------------------------------------------- |
| Docker            | Works out of the box — Docker sets `net.ipv4.ping_group_range` to allow unprivileged ICMP. |
| Kubernetes        | Allow the sysctl on the pod, or grant `CAP_NET_RAW`. See below.      |
| Bare metal, root  | Works — the raw socket fallback applies.                             |
| Bare metal, non-root | Set `net.ipv4.ping_group_range` to cover the service user's group, or grant the binary `CAP_NET_RAW`. |

For Kubernetes, the sysctl route keeps the container unprivileged:

```yaml
spec:
  securityContext:
    sysctls:
      - name: net.ipv4.ping_group_range
        value: "0 2147483647"
```

`net.ipv4.ping_group_range` is a namespaced but unsafe sysctl, so the kubelet must be
started with `--allowed-unsafe-sysctls=net.ipv4.ping_group_range`. If that isn't an
option, grant the capability instead:

```yaml
spec:
  containers:
    - name: warden
      securityContext:
        capabilities:
          add: ["NET_RAW"]
```

On a bare-metal host, the sysctl equivalent is:

```bash
sysctl -w net.ipv4.ping_group_range="0 2147483647"   # or the group Warden runs as
```

Or, to grant the binary the capability once:

```bash
setcap cap_net_raw+ep /usr/local/bin/warden
```

## DNS

Resolves the target name and reports it up when the lookup returns at least one record.
An empty answer counts as down — the name resolved but serves nothing, which is the
failure a DNS monitor exists to catch.

The target must be a name, not an IP address. Looking up an IP literal returns it
without a query ever leaving the process, so such a monitor would report up forever;
Warden rejects it rather than hand you a check that cannot fail.

Two options live under **DNS Configuration**:

- **Record type** — `A` (default), `AAAA`, `CNAME`, `MX`, `NS` or `TXT`.
- **Resolver** — the nameserver to query, as `host` or `host:port` (port 53 is assumed).
  Leave it empty to use the system resolver. Pointing this at your own nameserver is how
  you monitor the nameserver itself rather than whoever happens to answer.

## API

`type` is a field on the monitor payload, alongside `url`:

```bash
curl -X POST http://localhost:9090/api/monitors \
  -H 'Content-Type: application/json' \
  -d '{
    "name": "Postgres",
    "type": "tcp",
    "url": "db.internal:5432",
    "groupId": "g-default",
    "interval": 60
  }'
```

DNS options go in `requestConfig`, which is also where the shared timeout and retry
count live:

```json
{
  "name": "Zone",
  "type": "dns",
  "url": "example.com",
  "groupId": "g-default",
  "interval": 300,
  "requestConfig": { "dnsRecordType": "MX", "dnsResolver": "1.1.1.1" }
}
```

Omitting `type` on a `PUT` keeps the type the monitor already has, so a client that only
renames a monitor can't accidentally turn it into an HTTP check. Targets are validated
against the type they belong to: a URL is rejected for a `ping` monitor, and a bare
hostname is rejected for an `http` one.
