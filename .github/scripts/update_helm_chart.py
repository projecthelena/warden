#!/usr/bin/env python3

from __future__ import annotations

import argparse
import pathlib
import re


VERSION = r'(0|[1-9]\d*)\.(0|[1-9]\d*)\.(0|[1-9]\d*)'
RELEASE_TAG = re.compile(rf'^v{VERSION}$')
CHART_VERSION = re.compile(rf'(?m)^version:[ \t]*{VERSION}[ \t]*$')
APP_VERSION = re.compile(r'(?m)^appVersion:[ \t]*["\']?([^"\'\n]+?)["\']?[ \t]*$')
IMAGE = re.compile(r'(?m)^image:[ \t]*\n(?:[ \t]+[^\n]*\n|\n)*')
IMAGE_TAG = re.compile(r'(?m)^  tag:[ \t]*["\']?([^"\'\n]+?)["\']?[ \t]*$')


def update(chart_text: str, values_text: str, release_tag: str) -> tuple[str, str, str]:
    release = RELEASE_TAG.fullmatch(release_tag)
    if not release:
        raise ValueError(f'Stable release tag expected, got {release_tag!r}')
    chart_version = CHART_VERSION.search(chart_text)
    current_app = APP_VERSION.search(chart_text)
    image = IMAGE.search(values_text)
    image_tag = IMAGE_TAG.search(image.group()) if image else None
    if not chart_version:
        raise ValueError('Chart.yaml does not contain a semantic version')
    if not current_app or not re.fullmatch(VERSION, current_app[1]):
        raise ValueError('Chart.yaml does not contain a stable appVersion')
    if not image_tag:
        raise ValueError('values.yaml does not contain image.tag')

    target = tuple(map(int, release.groups()))
    current = tuple(map(int, current_app[1].split('.')))
    if target < current:
        raise ValueError(f'Refusing to downgrade Warden from {current_app[1]} to {release_tag}')
    version = '.'.join(chart_version.groups())
    if target == current and image_tag[1] == release_tag:
        return chart_text, values_text, version

    major, minor, patch = map(int, chart_version.groups())
    version = f'{major}.{minor}.{patch + 1}'
    chart_text = CHART_VERSION.sub(f'version: {version}', chart_text, count=1)
    chart_text = APP_VERSION.sub(f'appVersion: "{release_tag[1:]}"', chart_text, count=1)
    updated_image = IMAGE_TAG.sub(f'  tag: "{release_tag}"', image.group(), count=1)
    values_text = values_text[:image.start()] + updated_image + values_text[image.end():]
    return chart_text, values_text, version


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument('--chart', type=pathlib.Path, required=True)
    parser.add_argument('--values', type=pathlib.Path, required=True)
    parser.add_argument('--release', required=True)
    parser.add_argument('--github-output')
    args = parser.parse_args()
    old_chart = args.chart.read_text(encoding='utf-8')
    old_values = args.values.read_text(encoding='utf-8')
    chart, values, version = update(old_chart, old_values, args.release)
    changed = (chart, values) != (old_chart, old_values)
    if changed:
        args.chart.write_text(chart, encoding='utf-8')
        args.values.write_text(values, encoding='utf-8')
    if args.github_output:
        with open(args.github_output, 'a', encoding='utf-8') as output:
            output.write(f'chart_version={version}\nchanged={str(changed).lower()}\n')
    print(f'Warden chart {version}: {args.release} (changed={changed})')
    return 0


if __name__ == '__main__':
    raise SystemExit(main())
