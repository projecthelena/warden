#!/usr/bin/env python3

from __future__ import annotations

import argparse
import re
import subprocess
import sys
from dataclasses import dataclass


STABLE_TAG = re.compile(r"^v(\d+)\.(\d+)\.(\d+)$")
RC_TAG = re.compile(r"^v(\d+)\.(\d+)\.(\d+)-rc\.(\d+)$")
COMMIT = re.compile(r"^(?P<type>[a-z]+)(?:\([^)]*\))?(?P<breaking>!)?: .+")


@dataclass(frozen=True, order=True)
class Version:
    major: int
    minor: int
    patch: int

    def tag(self) -> str:
        return f"v{self.major}.{self.minor}.{self.patch}"


def git(*args: str) -> str:
    return subprocess.check_output(["git", *args], text=True).strip()


def tags() -> list[str]:
    output = git("tag", "--list")
    return output.splitlines() if output else []


def latest_stable(all_tags: list[str]) -> Version:
    versions = [Version(*map(int, match.groups())) for tag in all_tags if (match := STABLE_TAG.fullmatch(tag))]
    return max(versions, default=Version(0, 0, 0))


def commits_since(tag: str, tag_exists: bool) -> list[tuple[str, str]]:
    revision = f"{tag}..HEAD" if tag_exists else "HEAD"
    output = git("log", revision, "--no-merges", "--format=%s%x1f%b%x1e")
    commits = []
    for record in output.split("\x1e"):
        if not record.strip():
            continue
        subject, _, body = record.strip().partition("\x1f")
        commits.append((subject, body))
    return commits


def required_bump(commits: list[tuple[str, str]]) -> str | None:
    bump = None
    for subject, body in commits:
        match = COMMIT.fullmatch(subject)
        if not match:
            continue
        if match.group("breaking") or re.search(r"^BREAKING CHANGE:", body, re.MULTILINE):
            return "major"
        if match.group("type") == "feat":
            bump = "minor"
        elif match.group("type") in {"fix", "perf"} and bump is None:
            bump = "patch"
    return bump


def bump(version: Version, kind: str) -> Version:
    if kind == "major":
        return Version(version.major + 1, 0, 0)
    if kind == "minor":
        return Version(version.major, version.minor + 1, 0)
    return Version(version.major, version.minor, version.patch + 1)


def next_tag(channel: str, all_tags: list[str], commits: list[tuple[str, str]]) -> tuple[str, str, str]:
    current = latest_stable(all_tags)
    kind = required_bump(commits)
    if kind is None:
        raise ValueError("No releasable Conventional Commit found since the last stable release")

    target = bump(current, kind)
    if channel == "stable":
        result = target.tag()
    else:
        rc_numbers = [
            int(match.group(4))
            for tag in all_tags
            if (match := RC_TAG.fullmatch(tag))
            and Version(*map(int, match.groups()[:3])) == target
        ]
        result = f"{target.tag()}-rc.{max(rc_numbers, default=0) + 1}"

    if result in all_tags:
        raise ValueError(f"Tag {result} already exists")
    return current.tag(), kind, result


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--channel", choices=("stable", "rc"), required=True)
    parser.add_argument("--github-output")
    args = parser.parse_args()

    all_tags = tags()
    current = latest_stable(all_tags)
    commits = commits_since(current.tag(), current != Version(0, 0, 0))
    try:
        stable, kind, result = next_tag(args.channel, all_tags, commits)
    except ValueError as error:
        print(f"error: {error}", file=sys.stderr)
        return 1

    print(f"Current stable: {stable}")
    print(f"Semantic bump: {kind}")
    print(f"Next version: {result}")
    if args.github_output:
        with open(args.github_output, "a", encoding="utf-8") as output:
            output.write(f"stable={stable}\nbump={kind}\ntag={result}\n")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
