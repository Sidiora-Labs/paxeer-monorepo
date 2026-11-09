"""Offline source and production JSON-parser proofs; no paxd CLI is substituted.

Bash is used for the explicitly required syntax check and its own argument
parser. Native paxd execution, cryptographic BFT verification and relocation
outside this task's paths remain the owner wiring requests in the repair record.
"""

import ast
import base64
from concurrent.futures import ThreadPoolExecutor
from copy import deepcopy
import io
import json
from pathlib import Path
import re
import shlex
import subprocess
import sys
import tomllib

import pytest


ROOT = Path(__file__).resolve().parents[3]
PAXEER = ROOT / "services/platform/hosted/paxeer"
RECORD = Path(__file__).with_name("repair_22_33_1320.kvx")
SCRIPTS = (
    "fork-rehearsal.sh", "gov-consensus-params.sh", "gov-lib.sh",
    "gov-upgrade.sh", "init-chain.sh",
)


def source(name):
    return (PAXEER / name).read_text(encoding="utf-8")


def embedded_python(name, function):
    shell = source(name)
    start = shell.index(function + "() {\n    python3 -c '\n")
    match = re.search(r"python3 -c '\n(.*?)\n' \"\$@\"", shell[start:], re.S)
    assert match, (name, function)
    program = match.group(1)
    ast.parse(program)
    return program


def production(name="gov-lib.sh", function="governance_json"):
    namespace = {"__name__": "qualification_import"}
    exec(compile(embedded_python(name, function), str(PAXEER / name), "exec"), namespace)
    return namespace


def shell_function(name, function):
    match = re.search(r"^" + re.escape(function) + r"\(\) \{\n(.*?)^\}", source(name), re.M | re.S)
    assert match, function
    return match.group(1)


@pytest.mark.parametrize("name", SCRIPTS)
def test_scoped_shell_scripts_parse(name):
    result = subprocess.run(["bash", "-n", str(PAXEER / name)], capture_output=True, text=True)
    assert result.returncode == 0, result.stderr


def test_relay_retirement_9_and_10_maintained_ownership_and_consumers():
    manifest = tomllib.loads(source("Cargo.toml"))
    assert manifest["package"]["name"] == "layerx-platform-paxeer-boundary"
    assert manifest["bin"] == [
        {"name": "paxeer-native-tool", "path": "src/native_main.rs"},
        {"name": "layerx-paxeer-boundary", "path": "src/main.rs"},
    ]
    assert manifest["lib"] == {"name": "paxeer_native", "path": "src/native.rs"}
    operator = manifest["package"]["metadata"]["paxd-operator"]
    assert operator == {
        "owner-domain": 8,
        "supported-root": "services/platform/hosted/node",
        "compatibility-root": "services/platform/hosted/paxeer",
        "rehearsal-profiles": ["merged-v7.0"],
        "relay-deployment": False,
        "relocation-record": "tools/qualification/audit_repairs/repair_22_33_1320.kvx",
    }
    for name in ("gov-consensus-params.sh", "gov-upgrade.sh", "fork-rehearsal.sh", "gov-lib.sh"):
        assert "Maintained owner: domain 8 paxd operators" in source(name)
        assert "../node/" + name in source(name)
    for name in ("gov-consensus-params.sh", "gov-upgrade.sh"):
        assert '. "$(dirname "${BASH_SOURCE[0]}")/gov-lib.sh"' in source(name)
    # The retained arithmetic remains real implementation, ready to move with its tests.
    for function in (
        "seconds_per_block", "halt_height", "projected_epoch", "submission_deadline",
        "voting_fits", "duration_ns", "seconds_from_pb", "ns_to_human",
    ):
        body = shell_function("gov-lib.sh", function)
        assert "awk " in body or "echo " in body or "submission_deadline " in body


def test_governance_proposal_association_exact_transaction_before_vote():
    shell = source("gov-consensus-params.sh")
    assert "q gov proposals" not in shell
    assert "paxd_query_optional proposals" not in shell
    statements = (
        'SUBMITTED_TX=$(run_tx "${SUBMIT[@]}")',
        'PROPOSAL_ID=$(governance_json proposal-id <<<"$SUBMITTED_TX")',
        'PROPOSAL=$(paxd_query proposal gov proposal "$PROPOSAL_ID")',
        'governance_json check-parameters "$PROPOSAL_ID" "$NEW_VALUE" <<<"$PROPOSAL"',
        'run_tx gov vote "$PROPOSAL_ID" yes',
        'wait_for_tally "$PROPOSAL_ID" "$WAIT_SECONDS"',
    )
    positions = [shell.index(statement) for statement in statements]
    assert positions == sorted(positions)
    assert 'changes: [{subspace: "baseapp", key: "TimeoutParams", value: $value}]' in shell
    # Check the real producer's event and proposal-value wire contracts as source facts.
    events = (ROOT / "sdk/x/gov/types/events.go").read_text()
    keeper = (ROOT / "sdk/x/gov/keeper/proposal.go").read_text()
    assert 'EventTypeSubmitProposal   = "submit_proposal"' in events
    assert 'AttributeKeyProposalID           = "proposal_id"' in events
    assert "types.EventTypeSubmitProposal," in keeper
    assert 'sdk.NewAttribute(types.AttributeKeyProposalID, fmt.Sprintf("%d", proposalID))' in keeper
    cli = (ROOT / "sdk/x/params/client/utils/utils.go").read_text()
    assert "proposal.NewParamChange(pcj.Subspace, pcj.Key, string(pcj.Value))" in cli
    assert "Value    string" in (ROOT / "sdk/x/params/types/proposal/params.pb.go").read_text()


def parameter_vector(proposal_id, commit):
    """Parser unit vectors, not transcripts or an emulation of paxd."""
    value = json.loads((PAXEER / "tests/fixtures/gov/consensus-params.json").read_text())["consensus_params"]["timeout"]
    value["commit"] = commit
    proposal = {
        "proposal_id": proposal_id,
        "content": {
            "@type": "/cosmos.params.v1beta1.ParameterChangeProposal",
            "changes": [{"subspace": "baseapp", "key": "TimeoutParams", "value": json.dumps(value)}],
        },
    }
    tx = {
        "code": 0, "height": "500",
        "logs": [{"events": [{"type": "submit_proposal", "attributes": [
            {"key": "proposal_id", "value": proposal_id},
        ]}]}],
    }
    return tx, proposal, value


def test_governance_proposal_association_concurrent_distinct_parser_inputs():
    parser = production()
    first = parameter_vector("71", "20000000")
    second = parameter_vector("72", "30000000")

    def associate(vector):
        tx, proposal, expected = vector
        proposal_id = parser["submitted_proposal_id"](tx)
        parser["check_parameter_proposal"](proposal, proposal_id, expected)
        return proposal_id

    with ThreadPoolExecutor(max_workers=2) as executor:
        assert list(executor.map(associate, [second, first])) == ["72", "71"]
    with pytest.raises(ValueError, match="id does not match"):
        parser["check_parameter_proposal"](second[1], "71", first[2])
    with pytest.raises(ValueError, match="parameter value"):
        parser["check_parameter_proposal"](first[1], "71", second[2])


@pytest.mark.parametrize("field,value", [("subspace", "staking"), ("key", "OtherParams"), ("value", "{}")])
def test_governance_proposal_association_rejects_changed_content(field, value):
    parser = production()
    _, proposal, expected = parameter_vector("71", "20000000")
    proposal["content"]["changes"][0][field] = value
    with pytest.raises(ValueError):
        parser["check_parameter_proposal"](proposal, "71", expected)


def test_governance_proposal_association_events_are_unambiguous_and_committed():
    parser = production()
    tx, _, _ = parameter_vector("71", "20000000")
    encoded = deepcopy(tx)
    attribute = encoded["logs"][0]["events"][0]["attributes"][0]
    for key in ("key", "value"):
        attribute[key] = base64.b64encode(attribute[key].encode()).decode()
    assert parser["submitted_proposal_id"](encoded) == "71"
    # Duplicate representations of the same event are fine; distinct IDs are unsafe.
    tx["events"] = deepcopy(tx["logs"][0]["events"])
    assert parser["submitted_proposal_id"](tx) == "71"
    tx["events"][0]["attributes"][0]["value"] = "72"
    with pytest.raises(ValueError, match="exactly one"):
        parser["submitted_proposal_id"](tx)
    for invalid in ({"code": 1, "height": "500"}, {"code": 0, "height": "0"}, {"height": "500"}):
        with pytest.raises(ValueError, match="not committed"):
            parser["submitted_proposal_id"](invalid)
    with pytest.raises(ValueError, match="exactly one"):
        parser["submitted_proposal_id"]({"code": 0, "height": "500", "events": []})


def test_governance_query_failure_only_documented_no_plan_is_absence():
    parser = production()
    producer = (ROOT / "sdk/x/upgrade/client/cli/query.go").read_text()
    assert 'if res.Plan == nil {\n\t\t\t\treturn fmt.Errorf("no upgrade scheduled")' in producer
    for message in ("no upgrade scheduled", "Error: no upgrade scheduled\n"):
        assert parser["query_output"](message, 1, "upgrade plan") == {}
    live_plan = (PAXEER / "tests/fixtures/gov/upgrade-plan.json").read_text()
    assert parser["query_output"](live_plan, 0, "upgrade plan") == json.loads(live_plan)
    for message, query in (
        ("no upgrade scheduled", "gov proposals"),
        ("Error: no upgrade scheduled\nconnection refused", "upgrade plan"),
        ("connection refused", "upgrade plan"),
        ("rpc error: code = Unavailable", "upgrade plan"),
        ("unknown command plan", "upgrade plan"),
    ):
        with pytest.raises(ValueError) as failure:
            parser["query_output"](message, 1, query)
        assert str(failure.value) == message
    for malformed in ("not json", "[]", "null", "{", "log line\n{}"):
        with pytest.raises(ValueError):
            parser["query_output"](malformed, 0, "upgrade plan")
    optional = shell_function("gov-lib.sh", "paxd_query_optional")
    assert '|| status=$?' in optional
    assert 'governance_json query "$status" "$*"' in optional
    assert "echo '{}'" not in optional
    assert '|| fail "cannot query the scheduled upgrade plan"' in shell_function("gov-upgrade.sh", "plan_scheduled")


def test_governance_query_failure_preserves_exit_and_diagnostics(monkeypatch, capsys):
    program = embedded_python("gov-lib.sh", "governance_json")
    monkeypatch.setattr(sys, "argv", ["governance_json", "query", "7", "upgrade plan"])
    monkeypatch.setattr(sys, "stdin", io.StringIO("connection refused: node is unreachable"))
    with pytest.raises(SystemExit) as failure:
        exec(compile(program, "gov-lib.sh", "exec"), {"__name__": "__main__"})
    assert failure.value.code == 7
    output = capsys.readouterr()
    assert output.out == ""
    assert "connection refused: node is unreachable" in output.err


def test_governance_shell_arguments_array_survives_bash_parse(tmp_path):
    values = ["operator [ab]* key", "hyperpax_125-1", "http://127.0.0.1:26657/path *", "1000000uhpx", "2000000", "test", str(tmp_path / "home [ab]* dir")]
    (tmp_path / "operator a key").write_text("glob must remain literal")
    command = r'''
set -euo pipefail
. "$1"
shift
KEY_NAME=$1 COSMOS_CHAIN_ID=$2 RPC=$3 FEES=$4 GAS=$5 KEYRING_BACKEND=$6 HOME_DIR=$7
build_tx_flags
printf '%s\0' "${TX_FLAGS[@]}"
printf '\n'
tx_flags
'''
    result = subprocess.run(
        ["bash", "--noprofile", "--norc", "-c", command, "argument-proof", str(PAXEER / "gov-lib.sh"), *values],
        cwd=tmp_path, capture_output=True, check=True,
    )
    array, displayed = result.stdout.split(b"\n", 1)
    actual = array.decode().split("\0")[:-1]
    expected = [
        "--from", values[0], "--chain-id", values[1], "--node", values[2],
        "--fees", values[3], "--gas", values[4], "--keyring-backend", values[5],
        "-b", "sync", "-y", "-o", "json", "--home", values[6],
    ]
    assert actual == expected
    assert shlex.split(displayed.decode()) == expected
    run = shell_function("gov-lib.sh", "run_tx")
    assert 'out=$("$PAXD_BIN" tx "$@" "${TX_FLAGS[@]}" 2>&1)' in run
    assert "$(tx_flags)" not in run
    assert 'printf \'%q \' "${TX_FLAGS[@]}"' in shell_function("gov-lib.sh", "tx_flags")


def test_relay_retirement_8_merged_profile_and_committed_finality():
    shell = source("fork-rehearsal.sh")
    assert 'PROFILE=""' in shell
    assert '[ "$PROFILE" = merged-v7.0 ] || fail' in shell
    assert '[ "$UPGRADE_NAME" = v7.0 ] || fail' in shell
    assert "MODULES=(sequencer)" in shell
    assert "VIEWS=" not in shell and "0x42cde4e8" not in shell
    assert "--view)" not in shell
    assert "--retired-call)" in shell
    assert 'rehearsal_json retired "${RETIRED_NAMES[$address]}" <<<"$RESPONSE"' in shell
    assert 'FINALIZED_HEADER=$(rpc_header finalized)' in shell
    assert '/commit?height=$PINNED_HEIGHT' in shell
    assert 'rehearsal_json committed "$COSMOS_CHAIN_ID" "$UPGRADE_HEIGHT"' in shell
    assert 'RECOVERED_HEADER=$(rpc_header "$PINNED_HEX")' in shell
    assert "committed header changed after reopening the node" in shell
    assert shell.index('FINALIZED_HEADER=$(rpc_header finalized)') < shell.index('start_node "$NEW_BIN" "$LOG_DIR/reopened-bin.log"') < shell.index('RECOVERED_HEADER=')


def test_relay_retirement_8_parser_rejects_generic_rpc_errors_and_success():
    parser = production("fork-rehearsal.sh", "rehearsal_json")
    for name in ("layerxCustody", "layerxAnchor"):
        refused = {"error": {"code": 3, "message": "execution reverted: LXP_MODULE_DISABLED: " + name + " retired"}}
        assert parser["retired_response"](refused, name) is None
    for response in (
        {"result": "0x"}, {"error": {"code": -32601, "message": "method not found"}},
        {"error": {"code": 3, "message": "execution reverted"}},
        {"error": {"code": 3, "message": "LXP_MODULE_DISABLED: layerxAnchor retired"}},
    ):
        with pytest.raises(ValueError):
            parser["retired_response"](response, "layerxCustody")


def test_relay_retirement_8_committed_header_checks_recorded_bft_fields():
    parser = production("fork-rehearsal.sh", "rehearsal_json")
    # Recorded BFT reply; the EVM projection here is a parser unit vector only.
    comet = json.loads((PAXEER / "tests/fixtures/repair_22_33_1324/commit.json").read_text())
    signed = comet["result"]["signed_header"]
    height = int(signed["header"]["height"])
    eth = {
        "number": hex(height), "hash": "0x" + signed["commit"]["block_id"]["hash"].lower(),
        "stateRoot": "0x" + signed["header"]["app_hash"].lower(), "receiptsRoot": "0x" + "ab" * 32,
    }
    assert parser["committed_header"](eth, comet, "hyperpax_125-1", height) == height
    for key, value in (("hash", "0x" + "00" * 32), ("stateRoot", "0x" + "00" * 32), ("receiptsRoot", "0x")):
        bad = dict(eth, **{key: value})
        with pytest.raises(ValueError):
            parser["committed_header"](bad, comet, "hyperpax_125-1", height)
    with pytest.raises(ValueError):
        parser["committed_header"](eth, comet, "another_125-1", height)
    with pytest.raises(ValueError):
        parser["committed_header"](eth, comet, "hyperpax_125-1", height + 1)
    for field, value in (("canonical", False), ("signed_header", {})):
        bad = deepcopy(comet)
        bad["result"][field] = value
        with pytest.raises((ValueError, KeyError)):
            parser["committed_header"](eth, bad, "hyperpax_125-1", height)


def test_relay_retirement_11_upgrade_halt_and_proposal_checks_preserved():
    shell = source("gov-upgrade.sh")
    for check in (
        '[ "$HALT_HEIGHT" -gt "$WC_HEIGHT" ] || fail',
        '[ "$PLAN_NAME" = "$UPGRADE_NAME" ] || fail',
        '"$VOTING_END" -gt $((HALT_EPOCH - MARGIN_SECONDS))',
        'voting_fits "$NOW_EPOCH" "$HALT_EPOCH" "$PATH_SECONDS" "$MARGIN_SECONDS"',
        'wait_for_tally "$PROPOSAL_ID" "$WAIT_SECONDS"',
        'poll_until 300 10 plan_scheduled || fail',
        '[ "$EXECUTE" -eq 0 ] || fail "--execute is refused with GOV_FIXTURE_DIR set"',
    ):
        assert check in shell


def test_relay_retirement_12_native_genesis_entrypoint_and_legacy_refusal():
    shell = source("init-chain.sh")
    assert 'exec paxeer-native-tool bootstrap --bootstrap-script "$SCRIPT_DIR/../node/bootstrap.sh" "$@"' in shell
    assert 'for name in "${!LAYERX_PAXEER_@}"' in shell
    assert "app_state.layerxcustody" not in shell
    assert "custody-genesis.py" not in shell
    for argument in ("--custody-genesis", "--deposit-root=secret-value", "--sequencer-authorizations"):
        result = subprocess.run(["bash", str(PAXEER / "init-chain.sh"), argument], capture_output=True, text=True)
        assert result.returncode == 2
        assert "relay genesis inputs are retired" in result.stderr
        assert "secret-value" not in result.stderr
    bootstrap = (ROOT / "services/platform/hosted/node/bootstrap.sh").read_text()
    assert 'exec python3 "$SCRIPT_DIR/native_runtime.py" bootstrap "$@"' in bootstrap
    runtime = (ROOT / "services/platform/hosted/node/native_runtime.py").read_text()
    assert "genesis['app_state']['bank']" in runtime
    assert "genesis['app_state']['lxphost']" in runtime
    assert "'LAYERX_CORE_FUNDING_SENDER_KEY_FILE': args.funding_sender_key_file" in runtime


def test_repair_record_maps_all_findings_and_out_of_scope_wiring():
    record = tomllib.loads(RECORD.read_text())
    expected = {
        "relay-retirement-8", "relay-retirement-9", "relay-retirement-10", "relay-retirement-11",
        "relay-retirement-12", "governance-proposal-association", "governance-query-failure", "governance-shell-arguments",
    }
    assert set(record["finding"]) == expected
    test_names = {node.name for node in ast.parse(Path(__file__).read_text()).body if isinstance(node, ast.FunctionDef)}
    for finding in record["finding"].values():
        assert finding["tests"]
        assert set(finding["tests"]) <= test_names
    repair = record["repair"]["22"]["33"]["1320"]
    assert repair["base_revision"] == "dc1290e182c7bb1dab76d275c6f31350538c0f93"
    assert repair["evidence_ledger"] == "/out/run/gates.jsonl"
    for path in (
        "services/platform/hosted/node/fork-rehearsal.sh",
        "services/platform/hosted/node/gov-lib.sh",
        "services/platform/hosted/node/gov-consensus-params.sh",
        "services/platform/hosted/node/gov-upgrade.sh", "docs/wiki/BetaCluster.md",
        "services/platform/hosted/tests/beta-images.sh",
    ):
        assert any(path in request and "domain" in request for request in repair["wiring_requests"])
    assert "not run" in repair["baseline"].lower()
    assert "review" in repair["proof_limits"].lower()
