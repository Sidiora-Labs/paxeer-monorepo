"""Offline source contracts for the actual governance YAML and runner consumers.

No shell, node, compiler, Docker daemon or network is executed by this gate.
"""

import ast
import importlib.util
import json
from pathlib import Path
import shlex
import tomllib

import pytest
import yaml


ROOT = Path(__file__).resolve().parents[3]
GOV = ROOT / "tests/integration/gov_module"
RECORD = Path(__file__).with_name("repair_22_33_746.kvx")
RUNNER_SPEC = importlib.util.spec_from_file_location(
    "governance_repair_runner", ROOT / "tests/integration/scripts/runner.py"
)
RUNNER_MODULE = importlib.util.module_from_spec(RUNNER_SPEC)
RUNNER_SPEC.loader.exec_module(RUNNER_MODULE)
RUNNER = RUNNER_MODULE.TestRunner()


def cases():
    return RUNNER.load_yaml_file(GOV / "gov_proposal_test.yaml") + RUNNER.load_yaml_file(
        GOV / "staking_proposal_test.yaml"
    )


def captured(case, name):
    matches = [step for step in case["inputs"] if step.get("env") == name]
    assert matches, name
    return matches[-1]


def expressions(case):
    assert all(verifier["type"] == "eval" for verifier in case["verifiers"])
    return [verifier["expr"] for verifier in case["verifiers"]]


@pytest.mark.parametrize("index", range(4))
def test_proposal_poll_has_deadline_and_checks_its_committed_final_state(index):
    case = cases()[index]
    steps = case["inputs"]
    polls = [step for step in steps if step.get("env") == "POST_EXECUTION_HEIGHT"]
    assert len(polls) == 1
    poll = polls[0]
    script = poll["cmd"]
    assert shlex.split(script, comments=True)
    lines = [line.strip() for line in script.splitlines()]
    assert lines[:2] == ["set -euo pipefail", "deadline=$((SECONDS + 90))"]
    assert lines[-3:] == [
        "done",
        'echo "Deadline exceeded for proposal $PROPOSAL_ID" >&2',
        "exit 1",
    ]
    guard = script.split("query() {", 1)[1].split("}", 1)[0]
    assert "remaining=$((deadline - SECONDS))" in guard
    assert "if (( remaining <= 0 )); then" in guard
    assert guard.index("exit 1") < guard.index("timeout")
    assert 'timeout "${remaining}s" paxd "$@"' in script
    assert "while (( SECONDS < deadline )); do" in lines
    assert "height=$(query status | jq -er '.SyncInfo.latest_block_height | tonumber | select(. > 0)')" in lines
    assert 'status=$(query q gov proposal "$PROPOSAL_ID" --height "$height" --output json | jq -er .status)' in lines
    expected = "PROPOSAL_STATUS_REJECTED" if index == 2 else "PROPOSAL_STATUS_PASSED"
    assert f'{expected}) printf \'%s\\n\' "$height"; exit 0 ;;' in lines
    wrong = (
        "PROPOSAL_STATUS_PASSED|PROPOSAL_STATUS_FAILED"
        if index == 2
        else "PROPOSAL_STATUS_REJECTED|PROPOSAL_STATUS_FAILED"
    )
    assert f"{wrong})" in lines
    failure = script.split(f"{wrong})", 1)[1].split(";;", 1)[0]
    assert 'echo "Proposal $PROPOSAL_ID reached unexpected $status" >&2' in failure
    assert "exit 1" in failure
    assert "PROPOSAL_STATUS_DEPOSIT_PERIOD|PROPOSAL_STATUS_VOTING_PERIOD) ;;" in lines
    assert '*) echo "Unknown proposal status: $status" >&2; exit 1 ;;' in lines
    assert script.count("exit 0") == 1
    assert [line for line in lines if line.startswith("sleep ")] == ["sleep 1"]
    assert script.index("sleep 1") > script.index("esac")
    assert "|| true" not in script and "2>/dev/null" not in script
    assert all(not step["cmd"].strip().startswith("sleep ") for step in steps)
    assert f'PROPOSAL_STATUS == "{expected}"' in expressions(case)
    submit = captured(case, "PROPOSAL_ID")
    assert "submit_gov_proposal admin" in submit["cmd"]
    assert steps.index(submit) < steps.index(poll)
    votes = [step for step in steps if "gov vote $PROPOSAL_ID yes" in step["cmd"]]
    assert [step["node"] for step in votes] == [
        f"pax-node-{i}" for i in range([2, 4, 1, 4][index])
    ]
    assert all(steps.index(vote) < steps.index(poll) for vote in votes)
    assert poll.get("node", "pax-node-0") == "pax-node-0"
    assert all(step.get("expected_status", 0) == 0 for step in steps)


@pytest.mark.parametrize("index", range(4))
def test_post_execution_queries_use_the_poll_height_through_real_runner(index):
    case = cases()[index]
    poll = captured(case, "POST_EXECUTION_HEIGHT")
    steps = case["inputs"]
    following = steps[steps.index(poll) + 1:]
    assert following
    assert following[0] == captured(case, "PROPOSAL_STATUS")
    for step in following:
        tokens = shlex.split(step["cmd"])
        assert tokens[tokens.index("--height") + 1] == "$POST_EXECUTION_HEIGHT"
        assert tokens[tokens.index("--output") + 1] == "json"
        # Exercise the real transport builder without launching an external peer.
        argv = RUNNER._command_argv(
            step["cmd"], True, "pax-node-0",
            {"PROPOSAL_ID": "71", "POST_EXECUTION_HEIGHT": "1042"},
        )
        assert argv[:7] == [
            "docker", "exec", "-e", "PROPOSAL_ID=71", "-e",
            "POST_EXECUTION_HEIGHT=1042", "--",
        ]
        assert argv[7:10] == ["pax-node-0", "/bin/bash", "-c"]
        assert argv[-1].endswith(" && " + step["cmd"])
    assert 'q gov proposal "$PROPOSAL_ID"' in following[0]["cmd"]
    go_runner = (ROOT / "tests/integration/runner/runner.go").read_text()
    assert 'args = append(args, "-e", k+"="+v)' in go_runner
    assert 'opts.Shell, "-c", "export PATH=$PATH:"+opts.ExtraPath+" && "+inp.Cmd' in go_runner


def test_governance_supply_delta_uses_own_baseline_and_full_deposit():
    case = cases()[2]
    steps = case["inputs"]
    before_height = captured(case, "PRE_PROPOSAL_HEIGHT")
    before = captured(case, "TOTAL_SUPPLY_BEFORE_BURN")
    submit = captured(case, "PROPOSAL_ID")
    poll = captured(case, "POST_EXECUTION_HEIGHT")
    after = captured(case, "TOTAL_SUPPLY_AFTER_BURN")
    assert steps.index(before_height) < steps.index(before) < steps.index(submit)
    assert steps.index(submit) < steps.index(poll) < steps.index(after)
    assert shlex.split(before_height["cmd"])[-1] == (
        ".SyncInfo.latest_block_height | tonumber | select(. > 0)"
    )
    for step, height in [(before, "$PRE_PROPOSAL_HEIGHT"), (after, "$POST_EXECUTION_HEIGHT")]:
        tokens = shlex.split(step["cmd"])
        assert tokens[:4] == ["set", "-euo", "pipefail;", "paxd"]
        assert tokens[4:7] == ["q", "bank", "total"]
        assert tokens[tokens.index("--denom") + 1] == "uhpx"
        assert tokens[tokens.index("--height") + 1] == height
        assert tokens[-3:] == ["jq", "-er", ".amount"]
    proposal = json.loads((GOV / "proposal/expedited_proposal.json").read_text())
    assert proposal["deposit"] == "10000000uhpx"
    deposits = [step for step in steps if "gov deposit $PROPOSAL_ID" in step["cmd"]]
    assert len(deposits) == 1
    assert "gov deposit $PROPOSAL_ID 10000000uhpx" in deposits[0]["cmd"]
    assert steps.index(submit) < steps.index(deposits[0]) < steps.index(poll)
    assert "POST_EXECUTION_HEIGHT > PRE_PROPOSAL_HEIGHT" in expressions(case)
    assert "TOTAL_SUPPLY_BEFORE_BURN - TOTAL_SUPPLY_AFTER_BURN == 20000000" in expressions(case)
    assert "50000000003333" not in yaml.safe_dump(case)


@pytest.mark.parametrize("baseline", [5000000000333333333333, 10**40 + 13])
def test_governance_supply_verifier_rejects_wrong_delta_state_and_height(baseline):
    case = cases()[2]
    values = {
        "TOTAL_SUPPLY_BEFORE_BURN": str(baseline),
        "TOTAL_SUPPLY_AFTER_BURN": str(baseline - 20000000),
        "PROPOSAL_STATUS": "PROPOSAL_STATUS_REJECTED",
        "PRE_PROPOSAL_HEIGHT": "1000",
        "POST_EXECUTION_HEIGHT": "1042",
    }
    assert all(RUNNER.verify_result(values, verifier) for verifier in case["verifiers"])
    for difference in [0, 19999999, 20000001, -20000000]:
        wrong = dict(values, TOTAL_SUPPLY_AFTER_BURN=str(baseline - difference))
        assert not all(RUNNER.verify_result(wrong, v) for v in case["verifiers"])
    for status in ["PROPOSAL_STATUS_PASSED", "PROPOSAL_STATUS_VOTING_PERIOD", "PROPOSAL_STATUS_FAILED"]:
        wrong = dict(values, PROPOSAL_STATUS=status)
        assert not all(RUNNER.verify_result(wrong, v) for v in case["verifiers"])
    for height in ["999", "1000"]:
        wrong = dict(values, POST_EXECUTION_HEIGHT=height)
        assert not all(RUNNER.verify_result(wrong, v) for v in case["verifiers"])


def test_staking_retains_both_parameter_consumers_and_exact_expected_values():
    case = cases()[3]
    proposal = json.loads((GOV / "proposal/staking_proposal.json").read_text())
    assert proposal["changes"] == [
        {"subspace": "staking", "key": "UnbondingTime", "value": "1814400000000000"}
    ]
    assert "q params subspace staking UnbondingTime" in captured(case, "NEW_PARAM")["cmd"]
    assert "q staking params" in captured(case, "NEW_PARAM_FROM_STAKING")["cmd"]
    assert expressions(case) == [
        'PROPOSAL_STATUS == "PROPOSAL_STATUS_PASSED"',
        "NEW_PARAM == 1814400000000000",
        'NEW_PARAM_FROM_STAKING == "1814400s"',
    ]
    values = {
        "PROPOSAL_STATUS": "PROPOSAL_STATUS_PASSED",
        "NEW_PARAM": proposal["changes"][0]["value"],
        "NEW_PARAM_FROM_STAKING": "1814400s",
    }
    assert all(RUNNER.verify_result(values, verifier) for verifier in case["verifiers"])
    for key, value in [
        ("PROPOSAL_STATUS", "PROPOSAL_STATUS_VOTING_PERIOD"),
        ("NEW_PARAM", "10000000000"),
        ("NEW_PARAM_FROM_STAKING", "10s"),
    ]:
        assert not all(
            RUNNER.verify_result(dict(values, **{key: value}), verifier)
            for verifier in case["verifiers"]
        )
    assert "NEW_TALLY_PARAM == 0.450000000000000000" in expressions(cases()[0])
    assert 'NEW_ABCI_PARAM == "true"' in expressions(cases()[0])
    assert "NEW_PARAM == 0.750000000000000000" in expressions(cases()[1])


def test_producer_cli_and_committed_handler_source_match_yaml_consumers():
    helper = (ROOT / "tests/integration/utils/_tx_helpers.sh").read_text()
    assert 'find_proposal_by_title "$title" "$max_before" "$txhash"' in helper
    assert 'if [ "$observed" = "$title" ]; then echo "$id"; return 0; fi' in helper
    cli = (ROOT / "sdk/x/gov/client/cli/query.go").read_text()
    assert "&types.QueryProposalRequest{ProposalId: proposalID}" in cli
    assert "return clientCtx.PrintProto(&res.Proposal)" in cli
    bank = (ROOT / "sdk/x/bank/client/cli/query.go").read_text()
    assert "&types.QuerySupplyOfRequest{Denom: denom}" in bank
    assert "return clientCtx.PrintProto(&res.Amount)" in bank
    handler = (ROOT / "sdk/x/gov/abci.go").read_text()
    assert handler.index("err := handler(cacheCtx, proposal.GetContent())") < handler.index(
        "proposal.Status = types.StatusPassed"
    ) < handler.index("writeCache()") < handler.index("keeper.SetProposal(ctx, proposal)")
    assert "proposal.Status = types.StatusFailed" in handler
    assert "proposal.IsExpedited = false" in handler
    assert "proposal.VotingEndTime = proposal.VotingStartTime.Add(votingParams.VotingPeriod)" in handler
    assert "proposal.Status = types.StatusRejected" in handler
    assert "keeper.DeleteDeposits(ctx, proposal.ProposalId)" in handler
    deposit = (ROOT / "sdk/x/gov/keeper/deposit.go").read_text()
    assert "keeper.bankKeeper.BurnCoins(ctx, types.ModuleName, deposit.Amount)" in deposit


def test_record_names_each_finding_proof_and_the_private_evidence_contract():
    record = tomllib.loads(RECORD.read_text())
    assert record["task"]["id"] == "22.33.746"
    assert record["task"]["verify_cmd"] == (
        "python3 -m pytest -q tools/qualification/audit_repairs/test_repair_22_33_746.py"
    )
    assert record["task"]["evidence_ledger"] == "/out/run/gates.jsonl"
    assert "not run" in record["task"]["baseline"]
    assert "not executed" in record["task"]["review_only"]
    declared = {
        node.name for node in ast.parse(Path(__file__).read_text()).body
        if isinstance(node, ast.FunctionDef) and node.name.startswith("test_")
    }
    assert set(record["finding"]) == {
        "governance-fixed-time-supply", "staking-proposal-fixed-sleep"
    }
    for finding, filename in [
        ("governance-fixed-time-supply", "gov_proposal_test.yaml"),
        ("staking-proposal-fixed-sleep", "staking_proposal_test.yaml"),
    ]:
        entry = record["finding"][finding]
        assert entry["file"] == f"tests/integration/gov_module/{filename}"
        assert entry["audit"] == "22.30.172"
        assert entry["tests"] and set(entry["tests"]) <= declared
        assert entry["wiring_requests"] == []
