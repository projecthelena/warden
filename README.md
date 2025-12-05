# Cluster Uptime

A distributed monitoring dashboard that checks the uptime of a target API. It consists of a Go backend (performing the checks) and a React frontend (displaying the status).

## Features
- Periodical HTTP checks (Uptime monitoring).
- Latency tracking.
- Simple, real-time dashboard.

## Getting Started

### Prerequisites
- Go 1.21+
- Node.js 18+

### Dev Mode
1. **Backend**:
   ```bash
   make dev-backend
   # Defaults to checking google.com every 10s
   ```

2. **Frontend**:
   ```bash
   make dev-frontend
   # Opens at http://localhost:5173
   ```

### Production Build
```bash
make build
./bin/clusteruptime
```

## Configuration
- `LISTEN_ADDR`: Address to listen on (default `:9090`).
- `TARGET_URL`: The URL to monitor.
- `CHECK_INTERVAL`: Check interval (e.g. `10s`, `1m`).