import subprocess
import unittest
from pathlib import Path

import yaml


class CiLoadtestJobTest(unittest.TestCase):
    def test_root_workflow_runs_loadtest_module(self):
        root = Path(__file__).resolve().parents[3]
        with (root / ".github/workflows/ci.yml").open(encoding="utf-8") as handle:
            workflow = yaml.safe_load(handle)

        jobs = workflow["jobs"]
        job = jobs["loadtest"]
        self.assertEqual(job["runs-on"], jobs["c17"]["runs-on"])
        self.assertEqual(job["timeout-minutes"], 20)
        self.assertEqual(job["steps"][0], jobs["repository-audit"]["steps"][0])
        setup_go = [
            step
            for step in job["steps"]
            if step.get("uses", "").startswith("actions/setup-go@")
        ]
        self.assertEqual(len(setup_go), 1)
        self.assertEqual(
            setup_go[0]["with"]["go-version-file"], "tools/loadtest/go.mod"
        )
        self.assertEqual(
            [step["run"] for step in job["steps"] if "run" in step],
            [
                "go -C tools/loadtest build ./...",
                "go -C tools/loadtest test -count=1 ./...",
            ],
        )

        routing = subprocess.run(
            ["bash", "scripts/ci/runner-routing-check.sh"],
            cwd=root,
            capture_output=True,
            text=True,
            check=False,
        )
        self.assertEqual(routing.returncode, 0, routing.stdout + routing.stderr)


if __name__ == "__main__":
    unittest.main()
