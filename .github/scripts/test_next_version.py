#!/usr/bin/env python3

import importlib.util
import pathlib
import sys
import unittest


SCRIPT = pathlib.Path(__file__).with_name("next_version.py")
SPEC = importlib.util.spec_from_file_location("next_version", SCRIPT)
next_version = importlib.util.module_from_spec(SPEC)
assert SPEC.loader
sys.modules[SPEC.name] = next_version
SPEC.loader.exec_module(next_version)


class NextVersionTests(unittest.TestCase):
    def test_patch_release(self):
        self.assertEqual(
            next_version.next_tag("stable", ["v1.2.3"], [("fix: repair checks", "")]),
            ("v1.2.3", "patch", "v1.2.4"),
        )

    def test_feature_release(self):
        commits = [("fix: repair checks", ""), ("feat(api): add filters", "")]
        self.assertEqual(next_version.next_tag("stable", ["v1.2.3"], commits)[2], "v1.3.0")

    def test_breaking_release(self):
        commits = [("feat(api)!: remove old endpoint", "")]
        self.assertEqual(next_version.next_tag("stable", ["v1.2.3"], commits)[2], "v2.0.0")

    def test_breaking_change_in_body(self):
        commits = [("refactor: simplify API", "BREAKING CHANGE: old clients must migrate")]
        self.assertEqual(next_version.next_tag("stable", ["v1.2.3"], commits)[2], "v2.0.0")

    def test_first_and_following_release_candidate(self):
        commits = [("feat: add filters", "")]
        self.assertEqual(next_version.next_tag("rc", ["v1.2.3"], commits)[2], "v1.3.0-rc.1")
        tags = ["v1.2.3", "v1.3.0-rc.1", "v1.3.0-rc.2"]
        self.assertEqual(next_version.next_tag("rc", tags, commits)[2], "v1.3.0-rc.3")

    def test_stable_promotes_release_candidate_version(self):
        commits = [("feat: add filters", "")]
        tags = ["v1.2.3", "v1.3.0-rc.1"]
        self.assertEqual(next_version.next_tag("stable", tags, commits)[2], "v1.3.0")

    def test_non_releasable_commits_stop_release(self):
        with self.assertRaisesRegex(ValueError, "No releasable"):
            next_version.next_tag("stable", ["v1.2.3"], [("docs: update README", "")])


if __name__ == "__main__":
    unittest.main()
