"""Resolve the release gates from their platform callers to their owning jobs."""

from pathlib import Path

import yaml


RELEASE_GATE_CALLS = {
    "programs-acceptance": (
        "programs-conformance.yml",
        "scope",
        "acceptance",
        "acceptance",
    ),
    "agent-sanitizers": ("agent.yml", "lane", "sanitizers", "adversarial"),
    "agent-fuzz-corpus": ("agent.yml", "lane", "fuzz-corpus", "fuzz-corpus"),
}


def load_workflow(root, name):
    return yaml.load(
        (root / ".github/workflows" / name).read_text(encoding="utf-8"),
        Loader=yaml.BaseLoader,
    )


def release_gate_job(root, name):
    filename, input_name, value, job_name = RELEASE_GATE_CALLS[name]
    caller = load_workflow(root, "platform.yml")["jobs"][name]
    assert caller["uses"] == f"./.github/workflows/{filename}"
    assert caller["with"] == {input_name: value}
    assert "steps" not in caller
    assert "runs-on" not in caller
    assert "continue-on-error" not in caller
    workflow = load_workflow(root, filename)
    assert workflow["on"]["workflow_call"]["inputs"][input_name]["type"] == "string"
    job = workflow["jobs"][job_name]
    assert job["steps"], (filename, job_name)
    return workflow, job


def workflow_closure(root, names):
    """Include local callees so source checks also cover the code running lanes."""
    pending = list(names)
    workflows = {}
    while pending:
        name = pending.pop()
        if name in workflows:
            continue
        workflow = load_workflow(root, name)
        workflows[name] = workflow
        for job in workflow["jobs"].values():
            uses = job.get("uses", "")
            if not uses:
                continue
            assert uses.startswith("./.github/workflows/"), uses
            callee = uses.removeprefix("./.github/workflows/")
            assert Path(callee).name == callee, uses
            pending.append(callee)
    return workflows
