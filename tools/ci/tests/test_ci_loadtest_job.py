import re
import shutil
import subprocess
import tempfile
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
        self.assertEqual(job["runs-on"], jobs["repository-audit"]["runs-on"])
        expression = job["runs-on"]
        routing_expression = re.fullmatch(
            r"\$\{\{ \(github\.event_name == 'pull_request' && "
            r"github\.event\.pull_request\.head\.repo\.full_name != github\.repository\) && "
            r"'(?P<public>ubuntu-24\.04)' \|\| vars\.CI_LINUX_RUNNER \|\| "
            r"'(?P<fallback>ubuntu-24\.04)' \}\}",
            expression,
        )
        self.assertIsNotNone(routing_expression, expression)
        for event, head, configured, expected in (
            ("pull_request", "contributor/fork", "private-linux", "ubuntu-24.04"),
            ("pull_request", "owner/repo", "private-linux", "private-linux"),
            ("push", "owner/repo", "private-linux", "private-linux"),
            ("push", "owner/repo", "", "ubuntu-24.04"),
        ):
            with self.subTest(event=event, head=head, configured=configured):
                runner = (
                    routing_expression["public"]
                    if event == "pull_request" and head != "owner/repo"
                    else configured or routing_expression["fallback"]
                )
                self.assertEqual(runner, expected)
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

    def test_routing_refuses_fixed_jobs_and_incomplete_runtime(self):
        root = Path(__file__).resolve().parents[3]
        with tempfile.TemporaryDirectory(prefix="routing-live-rules-") as directory:
            tree = Path(directory)
            (tree / "scripts/ci").mkdir(parents=True)
            shutil.copy2(
                root / "scripts/ci/runner-routing-check.sh",
                tree / "scripts/ci/runner-routing-check.sh",
            )
            shutil.copytree(root / ".github/workflows", tree / ".github/workflows")
            for filename, job, mutation in (
                ("explorer-images.yml", "explorer-backend-test", "fixed"),
                ("launchpad-test.yml", "launchpad-test", "fixed"),
                ("explorer-images.yml", "explorer-backend-test", "incomplete-runtime"),
                ("explorer-images.yml", "explorer-backend-test", "conditional-runtime"),
            ):
                with self.subTest(filename=filename, job=job, mutation=mutation):
                    workflow = tree / ".github/workflows" / filename
                    original = workflow.read_text(encoding="utf-8")
                    if mutation == "fixed":
                        changed, count = re.subn(
                            r"(?m)^    runs-on: .*", "    runs-on: ubuntu-24.04",
                            original, count=1,
                        )
                        self.assertEqual(count, 1)
                    elif mutation == "conditional-runtime":
                        changed = original.replace(
                            "      - name: Install and verify the container runtime\n",
                            "      - name: Install and verify the container runtime\n        if: false\n",
                        )
                    else:
                        changed = original.replace(
                            "          docker version --format '{{.Server.Version}}'\n", ""
                        )
                    self.assertNotEqual(original, changed)
                    workflow.write_text(changed, encoding="utf-8")
                    result = subprocess.run(
                        ["sh", "scripts/ci/runner-routing-check.sh"], cwd=tree,
                        capture_output=True, text=True, check=False, timeout=60,
                    )
                    workflow.write_text(original, encoding="utf-8")
                    self.assertEqual(result.returncode, 1, result.stdout + result.stderr)
                    self.assertIn(f"{filename}: job {job}:", result.stderr)
                    message = "fixed runner" if mutation == "fixed" else "needs a container runtime"
                    self.assertIn(message, result.stderr)


if __name__ == "__main__":
    unittest.main()
