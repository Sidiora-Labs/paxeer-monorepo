import json
from pathlib import Path

import yaml


def test_bridge_workflow_runs_app_job():
    root = Path(__file__).resolve().parents[3]
    with (root / ".github/workflows/bridge-test.yml").open(
        encoding="utf-8"
    ) as handle:
        workflow = yaml.safe_load(handle)

    triggers = workflow[True]
    for event in ("pull_request", "push"):
        assert "apps/bridge/**" in triggers[event]["paths"]

    job = workflow["jobs"]["bridge-app"]
    assert "if" not in job
    assert not job.get("continue-on-error", False)
    steps = job["steps"]
    assert steps[0]["uses"].startswith("actions/checkout@")
    setup_node = next(
        index
        for index, step in enumerate(steps)
        if step.get("uses", "").startswith("actions/setup-node@")
    )
    with (root / "apps/bridge/package.json").open(encoding="utf-8") as handle:
        package_manager = json.load(handle)["packageManager"]

    run_steps = [step for step in steps if "run" in step]
    assert [step["run"] for step in run_steps] == [
        f"npm install --global {package_manager}",
        "pnpm install --frozen-lockfile",
        "pnpm typecheck",
        "pnpm test",
    ]
    assert setup_node < steps.index(run_steps[0])
    default_directory = job["defaults"]["run"]["working-directory"]
    for step in run_steps:
        assert step.get("working-directory", default_directory) == "apps/bridge"
        assert "if" not in step
        assert not step.get("continue-on-error", False)
