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
    return EXPECTED_GROUP.replace(
        "${{ github.workflow }}", LANE_GROUP_PREFIXES.get(name, "${{ github.workflow }}")
    )


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
