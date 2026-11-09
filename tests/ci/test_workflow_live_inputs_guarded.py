from pathlib import Path
import re

import yaml

from tools.ci.tests.workflow_graph import workflow_closure

from test_workflow_runs_on_routed import (
    ROUTED,
    assert_runner_routing,
    assert_workflow_runs_on_routed,
)


ROOT = Path(__file__).resolve().parents[2]
WORKFLOWS = ("platform.yml", "human.yml")
AUTOMATIC_EVENTS = {"push", "pull_request", "schedule"}
SCHEDULED_LIVE_JOBS = {
    "connected-middleware",
    "real-agent-framework-journeys",
    "real-ios-journey",
    "real-android-journey",
    "hosted-testnet-journey",
}
EXPRESSION = re.compile(r"\$\{\{(.*?)\}\}", re.DOTALL)
INPUT = re.compile(r"\b(?:secrets|vars)\.[A-Za-z_][A-Za-z_0-9]*\b")
PRESENCE = re.compile(r"\b(vars\.[A-Za-z_][A-Za-z_0-9]*)\s*!=\s*(?:''|\"\")")


def load_workflow(name):
    # BaseLoader preserves GitHub's `on` key instead of treating it as YAML 1.1 true.
    workflow = yaml.load(
        (ROOT / ".github/workflows" / name).read_text(encoding="utf-8"),
        Loader=yaml.BaseLoader,
    )
    assert workflow["jobs"], f"{name}: no jobs were inspected"
    return workflow


def input_references(value):
    if isinstance(value, dict):
        return set().union(*(input_references(item) for item in value.values()))
    if isinstance(value, list):
        return set().union(*(input_references(item) for item in value))
    if isinstance(value, str):
        return {
            reference
            for expression in EXPRESSION.findall(value)
            for reference in INPUT.findall(expression)
        }
    return set()


def job_inputs(workflow, job):
    # A guard must name an input consumed outside the guard itself. Include
    # workflow env because GitHub passes it to every job, including matrix jobs.
    body = {key: value for key, value in job.items() if key not in {"if", "steps"}}
    # The exact routed form has a hosted-runner fallback and its own shape check.
    if body.get("runs-on") == ROUTED:
        del body["runs-on"]
    inputs = input_references([workflow.get("env", {}), body])
    for step in job.get("steps", []):
        condition = step.get("if", "")
        step_inputs = input_references(step)
        if not can_run_without_inputs(condition):
            step_inputs -= set(PRESENCE.findall(condition))
        inputs |= step_inputs
    return inputs


def split_condition(expression, operator):
    parts = []
    depth = 0
    quote = None
    start = 0
    index = 0
    while index < len(expression):
        character = expression[index]
        if quote:
            if character == quote:
                if index + 1 < len(expression) and expression[index + 1] == quote:
                    index += 2
                    continue
                quote = None
        elif character in "'\"":
            quote = character
        elif character == "(":
            depth += 1
        elif character == ")":
            depth -= 1
        elif depth == 0 and expression.startswith(operator, index):
            parts.append(expression[start:index].strip())
            index += len(operator)
            start = index
            continue
        index += 1
    assert depth == 0 and quote is None, f"Unbalanced job condition: {expression}"
    parts.append(expression[start:].strip())
    return parts


def can_run_without_inputs(expression):
    expression = expression.strip()
    if expression.startswith("${{") and expression.endswith("}}"):
        expression = expression[3:-2].strip()
    for operator in ("||", "&&"):
        parts = split_condition(expression, operator)
        if len(parts) > 1:
            results = [can_run_without_inputs(part) for part in parts]
            return any(results) if operator == "||" else all(results)
    if expression.startswith("(") and expression.endswith(")"):
        return can_run_without_inputs(expression[1:-1])
    # Treat every other atom as potentially true, so an event, status function,
    # tag filter or unsupported expression cannot stand in for a presence guard.
    return PRESENCE.fullmatch(expression) is None


def assert_presence_guard(name, job):
    condition = job.get("if", "")
    assert PRESENCE.search(condition), f"{name}: missing job-level input guard"
    assert not can_run_without_inputs(condition), (
        f"{name}: job condition can bypass the input guard: {condition}"
    )


def automatic_jobs():
    for name, workflow in workflow_closure(ROOT, WORKFLOWS).items():
        triggers = workflow["on"]
        if isinstance(triggers, str):
            triggers = [triggers]
        if AUTOMATIC_EVENTS.intersection(triggers):
            for job_name, job in workflow["jobs"].items():
                yield f"{name}:{job_name}", workflow, job


def test_primary_platform_job_runs_unconditionally():
    workflow = load_workflow("platform.yml")
    job = workflow["jobs"]["build-lint-test"]
    assert "if" not in job
    registry_inputs = {
        "vars.LAYERX_TEN_LINE_REGISTRY_SNAPSHOT",
        "vars.LAYERX_TEN_LINE_REGISTRY_URL",
    }
    assert not registry_inputs.intersection(input_references(workflow.get("env", {})))
    assert not registry_inputs.intersection(input_references(job.get("env", {})))
    steps = {step["name"]: step for step in job["steps"]}
    benchmark = steps["Build the source-bound published quickstart benchmark"]
    presence = (
        "(vars.LAYERX_TEN_LINE_REGISTRY_SNAPSHOT != '' || "
        "vars.LAYERX_TEN_LINE_REGISTRY_URL != '')"
    )
    assert benchmark["if"] == presence
    assert set(PRESENCE.findall(benchmark["if"])) == registry_inputs
    assert not can_run_without_inputs(benchmark["if"])
    assert benchmark["env"] == {
        "LAYERX_TEN_LINE_REGISTRY_SNAPSHOT": "${{ vars.LAYERX_TEN_LINE_REGISTRY_SNAPSHOT }}",
        "LAYERX_TEN_LINE_REGISTRY_URL": "${{ vars.LAYERX_TEN_LINE_REGISTRY_URL }}",
    }
    assert input_references(benchmark) == registry_inputs
    upload = steps["Retain the actual published-package integration line counts"]
    assert upload["if"] == f"always() && {presence}"
    assert not can_run_without_inputs(upload["if"])
    middleware = steps[
        "Build the framework integrations and scan client bundles for secrets"
    ]
    assert "if" not in middleware
    assert not job_inputs(workflow, job)


def test_scheduled_platform_jobs_guard_live_inputs():
    workflow = load_workflow("platform.yml")
    assert workflow["on"]["schedule"], "Platform must retain its scheduled checks"
    assert SCHEDULED_LIVE_JOBS <= workflow["jobs"].keys()
    for name in sorted(SCHEDULED_LIVE_JOBS):
        job = workflow["jobs"][name]
        assert job_inputs(workflow, job), f"{name}: live inputs disappeared"
        assert_presence_guard(name, job)
        assert "github.event_name == 'schedule'" in job["if"]
        assert "github.event_name == 'workflow_dispatch'" in job["if"]


def test_every_automatic_job_guards_secrets():
    checked = set()
    for name, workflow, job in automatic_jobs():
        if job_inputs(workflow, job):
            checked.add(name)
            assert_presence_guard(name, job)
    assert {f"platform.yml:{name}" for name in SCHEDULED_LIVE_JOBS} <= checked
    assert "human.yml:browser-real-stack" in checked


def test_guard_names_an_input_the_job_reads():
    checked = set()
    for name, workflow, job in automatic_jobs():
        inputs = job_inputs(workflow, job)
        if inputs:
            guards = set(PRESENCE.findall(job.get("if", "")))
            assert guards, f"{name}: guard names no presence variable"
            assert guards <= inputs, (
                f"{name}: guard names inputs the job does not read: {guards - inputs}"
            )
            checked.add(name)
    assert checked, "No live job guards were inspected"


def test_live_input_workflows_keep_runner_routing():
    assert_runner_routing()
    workflows = workflow_closure(ROOT, WORKFLOWS)
    assert {"agent.yml", "programs-conformance.yml"} <= workflows.keys()
    for name, workflow in workflows.items():
        assert_workflow_runs_on_routed(name, workflow)
