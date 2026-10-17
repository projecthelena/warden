#!/usr/bin/env python3
"""Capture one external psql sampler for a bounded interval; close gzip on stop."""
import argparse
import datetime
import gzip
import json
import os
from pathlib import Path
import selectors
import signal
import subprocess
import time


def capture(sql, output, seconds):
    stop = False

    def request_stop(*_):
        nonlocal stop
        stop = True

    signal.signal(signal.SIGINT, request_stop)
    signal.signal(signal.SIGTERM, request_stop)
    samples = 0
    pending = b""
    started = time.monotonic()
    with gzip.open(output, "xt") as sink, open(str(output) + ".stderr", "x") as errors:
        process = subprocess.Popen(
            ["psql", "-XqAt", "-v", "ON_ERROR_STOP=1", "-f", str(sql)],
            stdout=subprocess.PIPE, stderr=errors,
        )
        selector = selectors.DefaultSelector()
        selector.register(process.stdout, selectors.EVENT_READ)
        try:
            while not stop and time.monotonic() - started < seconds:
                if not selector.select(timeout=max(0, min(1, seconds - (time.monotonic() - started)))):
                    continue
                chunk = os.read(process.stdout.fileno(), 65536)
                if not chunk:
                    raise RuntimeError("psql ended before capture deadline; inspect .stderr")
                pending += chunk
                while b"\n" in pending:
                    line, pending = pending.split(b"\n", 1)
                    if not line.startswith(b'{'):
                        continue  # psql watch headers are not samples.
                    record = json.loads(line)
                    record["received_at"] = datetime.datetime.now(datetime.timezone.utc).isoformat()
                    sink.write(json.dumps(record, separators=(",", ":")) + "\n")
                    sink.flush()
                    samples += 1
        finally:
            selector.close()
            process.terminate()
            try:
                process.wait(timeout=3)
            except subprocess.TimeoutExpired:
                process.kill()
                process.wait()
            process.stdout.close()
    if not samples:
        raise RuntimeError("capture produced no samples")
    print(f"Saved {samples} samples to {output}")


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("kind", choices=("activity", "stats"))
    parser.add_argument("output", type=Path)
    parser.add_argument("--seconds", type=int, default=600)
    args = parser.parse_args()
    if args.seconds < 1 or args.seconds > 72000:
        parser.error("seconds must be between 1 and 72000")
    capture(Path(__file__).with_name(args.kind + ".sql"), args.output, args.seconds)
