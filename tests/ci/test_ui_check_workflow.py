"""Keep the UI render check connected to its lockfile and test script."""

import json
from pathlib import Path
import shlex

import yaml


ROOT = Path(__file__).resolve().parents[2]
WORKFLOW = ".github/workflows/ui-check-test.yml"
PACKAGE_DIR = ROOT / "tools/ui-check"


def ui_check_workflow():
    # BaseLoader preserves GitHub's `on` key instead of treating it as YAML 1.1 true.
    return yaml.load((ROOT / WORKFLOW).read_text(encoding="utf-8"), Loader=yaml.BaseLoader)


def command_step(steps, command):
    matches = [
        step for step in steps
        if shlex.split(step.get("run", "")) == shlex.split(command)
        and step.get("working-directory") == "tools/ui-check"
    ]
    assert len(matches) == 1, (command, matches)
    return matches[0]


def test_ui_check_workflow_runs_the_render_check_suite():
    workflow = ui_check_workflow()
    triggers = workflow["on"]
    assert {"pull_request", "push", "workflow_dispatch"} <= triggers.keys()
    assert triggers["push"]["branches"] == ["main"]
    required_paths = {
        "tools/ui-check/**",
        WORKFLOW,
    }
    for event in ("pull_request", "push"):
        assert required_paths <= set(triggers[event]["paths"]), event

    assert workflow["permissions"] == {"contents": "read"}
    assert set(workflow["jobs"]) == {"test"}
    job = workflow["jobs"]["test"]
    assert "ubuntu-24.04" in job["runs-on"]
    assert job["timeout-minutes"] == "30"
    assert not {"if", "needs", "continue-on-error"} & job.keys()

    steps = job["steps"]
    checkout = next(
        step for step in steps if step.get("uses", "").startswith("actions/checkout@")
    )
    node = next(
        step for step in steps if step.get("uses", "").startswith("actions/setup-node@")
    )
    assert checkout["with"]["persist-credentials"] == "false"
    assert node["with"] == {"node-version": "22", "package-manager-cache": "false"}

    install = command_step(steps, "npm ci")
    browser = command_step(steps, "npx playwright install --with-deps chromium")
    test = command_step(steps, "npm test")
    assert steps.index(checkout) < steps.index(node) < steps.index(install)
    assert steps.index(install) < steps.index(browser) < steps.index(test)

    package = json.loads((PACKAGE_DIR / "package.json").read_text(encoding="utf-8"))
    assert package["scripts"]["test"] == "node --test test/*.test.mjs"
    assert (PACKAGE_DIR / "package-lock.json").is_file()
