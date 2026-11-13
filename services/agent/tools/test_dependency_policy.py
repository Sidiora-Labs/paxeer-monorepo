#!/usr/bin/env python3
"""Behavior checks for the agent dependency policy's shipped closure."""

import json
import subprocess
import tempfile
import unittest
from pathlib import Path


ROOT = Path(__file__).resolve().parents[3]
POLICY = ROOT / "services/agent/tools/dependency-policy.sh"


def metadata_for(edges):
    names = {"member", "helper", "rusqlite"}
    packages = [
        {"id": name, "name": name, "source": None, "license": "MIT"}
        for name in sorted(names)
    ]
    nodes = [
        {
            "id": name,
            "deps": [
                {"pkg": child, "name": child, "dep_kinds": [{"kind": kind, "target": None}]}
                for parent, child, kind in edges
                if parent == name
            ],
        }
        for name in sorted(names)
    ]
    return {"packages": packages, "workspace_members": ["member"], "resolve": {"nodes": nodes}}


class DependencyPolicyTests(unittest.TestCase):
    def run_fixture(self, edges):
        with tempfile.TemporaryDirectory() as directory:
            path = Path(directory) / "metadata.json"
            path.write_text(json.dumps(metadata_for(edges)), encoding="utf-8")
            return subprocess.run(
                ["sh", str(POLICY), str(path)],
                cwd=ROOT,
                capture_output=True,
                text=True,
                check=False,
            )

    def test_member_development_edge_is_excluded(self):
        result = self.run_fixture([("member", "rusqlite", "dev")])
        self.assertEqual(result.returncode, 0, result.stderr)

    def test_member_ordinary_edge_is_refused_with_path(self):
        result = self.run_fixture([("member", "rusqlite", None)])
        self.assertNotEqual(result.returncode, 0)
        self.assertIn("member -> rusqlite", result.stderr)

    def test_member_build_edge_is_refused_with_path(self):
        result = self.run_fixture([("member", "rusqlite", "build")])
        self.assertNotEqual(result.returncode, 0)
        self.assertIn("member -> rusqlite", result.stderr)

    def test_transitive_ordinary_edge_is_refused_with_path(self):
        result = self.run_fixture(
            [("member", "helper", None), ("helper", "rusqlite", None)]
        )
        self.assertNotEqual(result.returncode, 0)
        self.assertIn("member -> helper -> rusqlite", result.stderr)

    def test_dependency_development_edge_is_excluded(self):
        result = self.run_fixture(
            [("member", "helper", None), ("helper", "rusqlite", "dev")]
        )
        self.assertEqual(result.returncode, 0, result.stderr)

    def test_repository_metadata_passes(self):
        result = subprocess.run(
            ["sh", str(POLICY)],
            cwd=ROOT,
            capture_output=True,
            text=True,
            check=False,
        )
        self.assertEqual(result.returncode, 0, result.stderr)


if __name__ == "__main__":
    unittest.main()
