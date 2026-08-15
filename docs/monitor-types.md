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

## Ping permissions

If a ping monitor says Warden is not allowed to send ICMP packets, this section is the
fix. Sending a ping is not something an ordinary program is allowed to do; the operating
system has to be told to permit it. Nothing here is specific to Warden, it is the same
permission the `ping` command itself needs.

Find the line that matches how you run Warden.

**Docker**

Usually this already works, because Docker permits pings by default. If it does not,
your daemon or your compose file has turned it off. Turn it back on for the container:

```bash
docker run --sysctl net.ipv4.ping_group_range="0 2147483647" ...
```

In `docker-compose.yml`:

```yaml
services:
  warden:
    sysctls:
      - net.ipv4.ping_group_range=0 2147483647
```

**Kubernetes**

Kubernetes does not permit pings by default. Add the sysctl to the pod:

```yaml
spec:
  securityContext:
    sysctls:
      - name: net.ipv4.ping_group_range
        value: "0 2147483647"
```

Kubernetes treats this sysctl as unsafe, so the kubelet has to be started with
`--allowed-unsafe-sysctls=net.ipv4.ping_group_range`. If you cannot change the kubelet,
grant the capability instead:

```yaml
spec:
  containers:
    - name: warden
      securityContext:
        capabilities:
          add: ["NET_RAW"]
```

**Running the binary directly, as root**

Already works, nothing to do.

**Running the binary directly, as a normal user**

Either permit pings for every user on the host:

```bash
sysctl -w net.ipv4.ping_group_range="0 2147483647"
```

Or grant the permission to the Warden binary alone, which is narrower:

```bash
setcap cap_net_raw+ep /usr/local/bin/warden
```

To make the sysctl survive a reboot, put it in `/etc/sysctl.d/`.

**macOS**

Already works, nothing to do.

### Why it needs this

Warden asks the system for an unprivileged ICMP socket first, which needs no special
privileges but does need `net.ipv4.ping_group_range` to cover the group it runs as. If
that is refused it falls back to a raw socket, which needs root or `CAP_NET_RAW`. When
both are refused it says so, rather than reporting the target as down. A monitor that
cannot run its check has not learned anything about the target.

Only ping is affected. HTTP, TCP and DNS monitors need no special permissions.

## Targets Warden refuses

Checks cannot connect to link-local addresses (`169.254.0.0/16`, `fe80::/10`). That range is where cloud providers serve instance metadata, including credentials, and a monitor is not a reason to read it.

It matters because a failed check stores what the target answered, and anyone who can read the incident can read that. Without the block, an operator who can create monitors could point one at the metadata service, set `acceptedStatusCodes` to something it never returns so every response counts as a failure, and read the host's cloud credentials out of the drill-down.

Private ranges stay reachable: monitoring an internal network is the point. The check runs on the resolved address, so a hostname pointing at the metadata service is refused too.

## DNS

Resolves the target name and reports it up when the lookup returns at least one record.
An empty answer counts as down — the name resolved but serves nothing, which is the
failure a DNS monitor exists to catch.

The target must be a name, not an IP address. Looking up an IP literal returns it
without a query ever leaving the process, so such a monitor would report up forever;
Warden rejects it rather than hand you a check that cannot fail.

Two options live under **DNS Configuration**:

- **Record type** — `A` (default), `AAAA`, `MX`, `NS` or `TXT`.

  CNAME is not offered. Looking one up reaches DNS parsing that panics on a malformed
  SVCB or HTTPS record ([CVE-2026-46600](https://pkg.go.dev/vuln/GO-2026-5942), fixed in
  Go 1.26.6), and a panic in a check worker stops the whole process, so a hostile
  resolver could take your monitoring down. It also barely checked anything: a name with
  no CNAME resolves to itself and reports up.
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
