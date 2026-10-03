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

    def test_retry_is_a_noop(self):
        chart = 'version: 0.3.1\nappVersion: "0.8.0"\n'
        values = 'image:\n  tag: "v0.8.0"\n'
        self.assertEqual(update_helm_chart.update(chart, values, 'v0.8.0'),
                         (chart, values, '0.3.1'))

    def test_rejects_downgrade(self):
        with self.assertRaisesRegex(ValueError, 'Refusing to downgrade'):
            update_helm_chart.update('version: 0.3.1\nappVersion: "0.9.0"\n',
                                     'image:\n  tag: v0.9.0\n', 'v0.8.0')

    def test_repairs_image_tag_with_new_chart_version(self):
        chart, values, version = update_helm_chart.update(
            'version: 0.3.1\nappVersion: "0.8.0"\n',
            'image:\n  tag: latest\n', 'v0.8.0')
        self.assertEqual(version, '0.3.2')
        self.assertIn('tag: "v0.8.0"', values)

    def test_preserves_other_images_and_blank_lines(self):
        values = ('image:\n  repository: ghcr.io/projecthelena/warden\n'
                  '  tag: latest\n  pullPolicy: IfNotPresent\n\n'
                  'database:\n  image:\n    tag: "18"\n')
        chart = 'version: 0.3.1\n\nappVersion: "0.8.0"\n\nannotations:\n  example: value\n'
        updated, result, version = update_helm_chart.update(chart, values, 'v0.10.0')
        self.assertEqual(result, values.replace('tag: latest', 'tag: "v0.10.0"'))
        self.assertEqual(updated, chart.replace('0.3.1', '0.3.2').replace('0.8.0', '0.10.0'))

    def test_missing_warden_image_does_not_change_postgres(self):
        with self.assertRaisesRegex(ValueError, 'image.tag'):
            update_helm_chart.update('version: 0.3.1\nappVersion: "0.8.0"\n',
                'image:\n  repository: warden\ndatabase:\n  tag: latest\n', 'v0.9.0')

    def test_rejects_invalid_tags(self):
        for tag in ['0.8.0', 'v01.2.3', 'v0.8.0+build', 'v0.8.0\n']:
            with self.subTest(tag=tag), self.assertRaises(ValueError):
                update_helm_chart.update('version: 0.3.1\nappVersion: "0.8.0"\n',
                                         'image:\n  tag: latest\n', tag)


if __name__ == "__main__":
    unittest.main()
