"""Guard the real Wallet Go workflow prerequisites and immutable action pins."""

from pathlib import Path
import re
import shlex
import tomllib

import pytest
import yaml


ROOT = Path(__file__).resolve().parents[3]
WORKFLOW = ROOT / ".github/workflows/wallet-go-test.yml"
CHECKOUT = "actions/checkout@3d3c42e5aac5ba805825da76410c181273ba90b1"
SETUP_GO = "actions/setup-go@b7ad1dad31e06c5925ef5d2fc7ad053ef454303e"
SETUP_NODE = "actions/setup-node@820762786026740c76f36085b0efc47a31fe5020"


def workflow_jobs():
    return yaml.safe_load(WORKFLOW.read_text(encoding="utf-8"))["jobs"]


def test_ceremony_installs_node_and_pinned_pnpm_before_go_tests():
    job = workflow_jobs()["ceremony"]
    steps = job["steps"]
    node_steps = [(i, step) for i, step in enumerate(steps) if step.get("uses") == SETUP_NODE]
    pnpm_steps = [
        (i, step)
        for i, step in enumerate(steps)
        if shlex.split(step.get("run", "")) == ["npm", "install", "--global", "pnpm@10.34.9+sha512.f49b5a8ea0fdedb44bb3c0c1d6575092a53c2950d95b7ad15df11f17974877a2de3a01c07e934e9b28ce716e51c09a94962c7306d09be2e1e4b18a8c8762fd32"]
    ]
    test_steps = [(i, step) for i, step in enumerate(steps) if step.get("name") == "Test ceremony"]
    assert len(node_steps) == len(pnpm_steps) == len(test_steps) == 1
    node_index, node = node_steps[0]
    pnpm_index, pnpm = pnpm_steps[0]
    test_index, test = test_steps[0]
    assert node_index < pnpm_index < test_index
    assert str(node["with"]["node-version"]) == "22"
    assert shlex.split(test["run"]) == ["go", "test", "./..."]
    assert job["defaults"]["run"]["working-directory"] == "services/wallet/ceremony"
    for step in (node, pnpm, test):
        assert "if" not in step
        assert step.get("continue-on-error", False) is False


@pytest.mark.parametrize("name", ["attestor", "ceremony"])
def test_both_jobs_keep_action_commits_and_checkout_credentials_disabled(name):
    job = workflow_jobs()[name]
    steps = job["steps"]
    actions = [step["uses"] for step in steps if "uses" in step]
    expected = [CHECKOUT, SETUP_GO]
    if name in ("attestor", "ceremony"):
        expected.append(SETUP_NODE)
    assert actions == expected
    for action in actions:
        assert re.fullmatch(r"actions/[a-z-]+@[0-9a-f]{40}", action)
    checkout = next(step for step in steps if step.get("uses") == CHECKOUT)
    assert checkout["with"]["persist-credentials"] is False
    setup_go = next(step for step in steps if step.get("uses") == SETUP_GO)
    assert setup_go["with"] == {
        "go-version-file": f"services/wallet/{name}/go.mod",
        "cache-dependency-path": f"services/wallet/{name}/go.sum",
    }
    assert job["defaults"]["run"]["working-directory"] == f"services/wallet/{name}"
    command = ("sh ../../../tools/runtime/run-with-clock.sh go test -count=1 ./..."
               if name == "attestor" else "go test ./...")
    go_tests = [step for step in steps if step.get("run") == command]
    assert len(go_tests) == 1
    assert go_tests[0].get("continue-on-error", False) is False
    assert "if" not in go_tests[0]


def test_repair_record_names_causes_main_proof_and_private_revision_evidence():
    record = tomllib.loads(
        (ROOT / "services/wallet/attestor/repair_22_31_506.kvx").read_text(encoding="utf-8")
    )
    assert record["repair"]["gotcha"] == 3052
    assert record["repair"]["decision"] == 2550
    assert record["snapshot"]["test"] == "TestSnapshotFollowsAddShare"
    assert record["describe"]["test"] == "TestDescribeReturnsPublicMaterialToTheOperator"
    assert "import receipt" in record["snapshot"]["cause"]
    assert "refresh" in record["describe"]["cause"]
    assert record["main_proof"]["workflow"] == ".github/workflows/wallet-go-test.yml"
    assert record["main_proof"]["event"] == "push"
    assert record["main_proof"]["branch"] == "main"
    assert record["main_proof"]["status"] == "pending first post-repair run on main"
    assert record["evidence"]["ledger"] == "/out/run/gates.jsonl"
    assert record["evidence"]["revision_source"] == "passing hf-done revision, identical to the hf-build revision"
