#!/usr/bin/env python3

from __future__ import annotations

import argparse
import pathlib
import re


RELEASE_TAG = re.compile(r"^v?(\d+)\.(\d+)\.(\d+)$")
CHART_VERSION = re.compile(r"(?m)^version:\s*(\d+)\.(\d+)\.(\d+)\s*$")
APP_VERSION = re.compile(r'(?m)^appVersion:\s*["\']?[^"\'\n]+["\']?\s*$')
IMAGE_TAG = re.compile(r'(?m)^(image:\s*\n(?:^[ \t]+[^\n]*\n)*?^[ \t]+tag:)\s*["\']?[^"\'\n]+["\']?\s*$')


def update(chart_text: str, values_text: str, release_tag: str) -> tuple[str, str, str]:
    release = RELEASE_TAG.fullmatch(release_tag)
    if not release:
        raise ValueError(f"Stable release tag expected, got {release_tag!r}")

    chart_version = CHART_VERSION.search(chart_text)
    if not chart_version:
        raise ValueError("Chart.yaml does not contain a semantic version")
    if not APP_VERSION.search(chart_text):
        raise ValueError("Chart.yaml does not contain appVersion")
    if not IMAGE_TAG.search(values_text):
        raise ValueError("values.yaml does not contain image.tag")

    major, minor, patch = map(int, chart_version.groups())
    next_chart_version = f"{major}.{minor}.{patch + 1}"
    app_version = ".".join(release.groups())

    chart_text = CHART_VERSION.sub(f"version: {next_chart_version}", chart_text, count=1)
    chart_text = APP_VERSION.sub(f'appVersion: "{app_version}"', chart_text, count=1)
    values_text = IMAGE_TAG.sub(rf'\1 "{release_tag}"', values_text, count=1)
    return chart_text, values_text, next_chart_version


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--chart", type=pathlib.Path, required=True)
    parser.add_argument("--values", type=pathlib.Path, required=True)
    parser.add_argument("--release", required=True)
    parser.add_argument("--github-output")
    args = parser.parse_args()

    chart_text, values_text, chart_version = update(
        args.chart.read_text(encoding="utf-8"),
        args.values.read_text(encoding="utf-8"),
        args.release,
    )
    args.chart.write_text(chart_text, encoding="utf-8")
    args.values.write_text(values_text, encoding="utf-8")

    if args.github_output:
        with open(args.github_output, "a", encoding="utf-8") as output:
            output.write(f"chart_version={chart_version}\n")
    print(f"Warden chart {chart_version} now deploys {args.release}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
