#!/usr/bin/env python3

import importlib.util
import pathlib
import sys
import unittest


SCRIPT = pathlib.Path(__file__).with_name("update_helm_chart.py")
SPEC = importlib.util.spec_from_file_location("update_helm_chart", SCRIPT)
update_helm_chart = importlib.util.module_from_spec(SPEC)
sys.modules[SPEC.name] = update_helm_chart
SPEC.loader.exec_module(update_helm_chart)


class UpdateHelmChartTests(unittest.TestCase):
    def test_updates_chart_and_image_versions(self) -> None:
        chart, values, version = update_helm_chart.update(
            'apiVersion: v2\nversion: 0.2.4\nappVersion: "0.7.0"\n',
            'image:\n  repository: ghcr.io/projecthelena/warden\n  tag: latest\n  pullPolicy: Always\n',
            "v0.8.0",
        )

        self.assertEqual(version, "0.2.5")
        self.assertIn("version: 0.2.5", chart)
        self.assertIn('appVersion: "0.8.0"', chart)
        self.assertIn('tag: "v0.8.0"', values)

    def test_rejects_prerelease(self) -> None:
        with self.assertRaisesRegex(ValueError, "Stable release tag expected"):
            update_helm_chart.update(
                'version: 0.2.4\nappVersion: "0.7.0"\n',
                "image:\n  tag: latest\n",
                "v0.8.0-rc.1",
            )


if __name__ == "__main__":
    unittest.main()
