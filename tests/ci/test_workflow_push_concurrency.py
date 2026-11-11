import ast
import re
from pathlib import Path

import yaml


ROOT = Path(__file__).resolve().parents[2]
WORKFLOWS = ROOT / ".github/workflows"
EXPECTED_GROUP = (
    "${{ github.workflow }}-${{ github.event_name }}-"
    "${{ github.event_name == 'push' && github.sha || github.ref }}"
)
LANE_GROUP_PREFIXES = {
    "agent.yml": "${{ github.workflow }}-agent-${{ inputs.lane }}",
    "programs-conformance.yml": "${{ github.workflow }}-programs-${{ inputs.scope }}",
}
TARGET_WORKFLOWS = {
    "agent.yml",
    "bridge-test.yml",
    "ci.yml",
    "codeql.yml",
    "docs-site.yml",
    "hosted-developer.yml",
    "human.yml",
    "interop-x402.yml",
    "launchpad-test.yml",
    "node-build.yml",
    "paxeer-docker-build.yml",
    "paxeer-ecr.yml",
    "paxeer-eth-blocktests.yml",
    "paxeer-forge-test.yml",
    "paxeer-golangci.yml",
    "paxeer-integration-test.yml",
    "paxeer-mock-balances-build.yml",
    "paxeer-proto-registry.yml",
    "platform.yml",
    "programs-conformance.yml",
    "scorecard.yml",
    "wallet-test.yml",
    "xweb-test.yml",
}
PULL_REQUEST_WORKFLOWS = TARGET_WORKFLOWS - {
    "paxeer-ecr.yml",
    "paxeer-proto-registry.yml",
    "scorecard.yml",
}


def load_workflows():
    paths = sorted(
        path
        for path in WORKFLOWS.rglob("*")
        if path.is_file() and path.suffix in {".yml", ".yaml"}
    )
    assert paths, "No workflow files found"
    return {
        path.relative_to(WORKFLOWS).as_posix(): yaml.load(
            path.read_text(encoding="utf-8"), Loader=yaml.BaseLoader
        )
        for path in paths
    }


def workflow_events(workflow):
    triggers = workflow["on"]
    return {triggers} if isinstance(triggers, str) else set(triggers)


def expected_group(name):
    if name == "paxeer-ecr.yml":
        return (
            "${{ github.workflow }}-${{ github.event_name }}-"
            "${{ github.event_name == 'push' && github.sha || "
            "github.event_name == 'workflow_dispatch' && "
            "format('{0}:{1}:{2}', github.ref, inputs.ref, inputs.tag) || github.ref }}"
        )
    return EXPECTED_GROUP.replace(
        "${{ github.workflow }}", LANE_GROUP_PREFIXES.get(name, "${{ github.workflow }}")
    )


def evaluate_concurrency_group(group, github, inputs):
    def evaluate(node):
        if isinstance(node, ast.Constant):
            return node.value
        if isinstance(node, ast.Name):
            return {"github": github, "inputs": inputs}[node.id]
        if isinstance(node, ast.Attribute):
            return evaluate(node.value).get(node.attr, "")
        if isinstance(node, ast.Compare):
            assert len(node.ops) == 1 and isinstance(node.ops[0], ast.Eq)
            return evaluate(node.left) == evaluate(node.comparators[0])
        if isinstance(node, ast.BoolOp):
            for value in node.values:
                result = evaluate(value)
                if isinstance(node.op, ast.And) and not result:
                    return result
                if isinstance(node.op, ast.Or) and result:
                    return result
            return result
        if isinstance(node, ast.Call):
            assert isinstance(node.func, ast.Name) and node.func.id == "format"
            assert not node.keywords
            template, *arguments = [evaluate(argument) for argument in node.args]
            return template.format(*arguments)
        raise AssertionError(f"Unsupported concurrency expression: {ast.dump(node)}")

    def interpolate(match):
        expression = match.group(1).strip().replace("&&", "and").replace("||", "or")
        return str(evaluate(ast.parse(expression, mode="eval").body))

    return re.sub(r"\$\{\{(.*?)\}\}", interpolate, group)


def test_ecr_dispatch_concurrency_preserves_request_identity():
    workflow = load_workflows()["paxeer-ecr.yml"]
    assert "workflow_dispatch" in workflow_events(workflow)
    concurrency = workflow["concurrency"]
    assert concurrency["cancel-in-progress"] == "true"
    assert concurrency["group"] == expected_group("paxeer-ecr.yml")
    github = {
        "workflow": workflow["name"],
        "event_name": "workflow_dispatch",
        "ref": "refs/heads/main",
        "sha": "a" * 40,
    }
    requests = [
        {"ref": "release/source", "tag": "candidate"},
        {"ref": "release/other", "tag": "candidate"},
        {"ref": "release/source", "tag": "other"},
        {"ref": "release/source"},
        {"ref": "release/other"},
        {"ref": "release/source", "tag": "release/source"},
        {"ref": "release/source-extra", "tag": "candidate"},
        {"ref": "release/source", "tag": "extra-candidate"},
    ]
    groups = [
        evaluate_concurrency_group(concurrency["group"], github, inputs)
        for inputs in requests
    ]
    assert len(set(groups)) == len(requests)
    for inputs, group in zip(requests, groups):
        assert evaluate_concurrency_group(concurrency["group"], github, inputs) == group
        assert evaluate_concurrency_group(
            concurrency["group"], dict(github, sha="b" * 40), inputs
        ) == group
    assert evaluate_concurrency_group(
        concurrency["group"], github, {"ref": "release/source", "tag": ""}
    ) == groups[3]

    push_groups = [
        evaluate_concurrency_group(
            concurrency["group"], dict(github, event_name="push", sha=sha), {}
        )
        for sha in ("a" * 40, "b" * 40)
    ]
    assert push_groups == [
        f"{workflow['name']}-push-{sha}" for sha in ("a" * 40, "b" * 40)
    ]
    assert push_groups[0] != push_groups[1]
    assert not set(push_groups).intersection(groups)

    for sha in ("a" * 40, "b" * 40):
        for ref in ("refs/pull/42/merge", "refs/pull/43/merge"):
            assert evaluate_concurrency_group(
                concurrency["group"],
                dict(github, event_name="pull_request", sha=sha, ref=ref),
                {},
            ) == f"{workflow['name']}-pull_request-{ref}"


def test_ecr_manual_run_concurrency_preserves_request_identity():
    test_ecr_dispatch_concurrency_preserves_request_identity()


def test_push_workflows_key_concurrency_on_sha():
    checked = set()
    incorrect = {}
    for name, workflow in load_workflows().items():
        if "push" not in workflow_events(workflow):
            continue
        concurrency = workflow.get("concurrency", {})
        if not isinstance(concurrency, dict):
            continue
        if concurrency.get("cancel-in-progress") != "true":
            continue
        checked.add(name)
        if concurrency.get("group") != expected_group(name):
            incorrect[name] = concurrency.get("group")
    assert TARGET_WORKFLOWS <= checked, (
        "Push triggers and cancellation must remain enabled",
        sorted(TARGET_WORKFLOWS - checked),
    )
    assert not incorrect, (
        "Push concurrency must use workflow, event and SHA, with ref otherwise",
        incorrect,
    )


def test_pull_request_concurrency_still_cancels():
    workflows = load_workflows()
    for name in sorted(PULL_REQUEST_WORKFLOWS):
        workflow = workflows[name]
        assert "pull_request" in workflow_events(workflow), name
        concurrency = workflow["concurrency"]
        assert concurrency["cancel-in-progress"] == "true", name
        assert concurrency["group"] == expected_group(name), (
            name,
            "Pull requests must share a ref-based group across commits",
        )


def test_codeql_concurrency_keyed_on_sha():
    workflow = load_workflows()["codeql.yml"]
    assert "push" in workflow_events(workflow)
    assert "main" in workflow["on"]["push"]["branches"]
    assert workflow["concurrency"]["cancel-in-progress"] == "true"
    assert workflow["concurrency"]["group"] == EXPECTED_GROUP


def test_called_lanes_have_distinct_concurrency_from_their_caller():
    workflows = load_workflows()
    caller = workflows["platform.yml"]
    groups = {caller["concurrency"]["group"]}
    for job_name in ("programs-acceptance", "agent-sanitizers", "agent-fuzz-corpus"):
        job = caller["jobs"][job_name]
        name = job["uses"].removeprefix("./.github/workflows/")
        group = workflows[name]["concurrency"]["group"]
        assert group == expected_group(name)
        for key, value in job["with"].items():
            assert f"${{{{ inputs.{key} }}}}" in group
            group = group.replace(f"${{{{ inputs.{key} }}}}", value)
        assert group not in groups, (job_name, group)
        groups.add(group)
