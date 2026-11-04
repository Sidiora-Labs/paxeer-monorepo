"""Offline source qualification for the two oracle integration audit findings.

Only bash -n is executed out of process; no chain, CLI, jq or RPC is required.
The real Python runner evaluates the authored scenario verifiers in process.
"""

import ast
import importlib.util
from pathlib import Path
import re
import shlex
import subprocess
import tomllib

import pytest
import yaml


ROOT = Path(__file__).resolve().parents[3]
ORACLE = ROOT / "tests/integration/oracle_module"
RECORD = Path(__file__).with_name("repair_22_33_747.kvx")


def source(path):
    return (ROOT / path).read_text(encoding="utf-8")


def scenario(name):
    cases = yaml.safe_load((ORACLE / name).read_text(encoding="utf-8"))
    assert isinstance(cases, list) and len(cases) == 1
    case = cases[0]
    assert set(case) == {"name", "inputs", "verifiers"}
    assert case["inputs"] and case["verifiers"]
    return case


def captures(case):
    commands = {}
    for entry in case["inputs"]:
        assert set(entry) <= {"cmd", "env"}
        assert isinstance(entry["cmd"], str) and entry["cmd"].strip()
        if "env" in entry:
            assert entry["env"] not in commands
            commands[entry["env"]] = entry["cmd"]
    return commands


def runner():
    spec = importlib.util.spec_from_file_location(
        "oracle_integration_runner", ROOT / "tests/integration/scripts/runner.py"
    )
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module.TestRunner(docker=False)


def verifies(case, values):
    consumer = runner()
    return all(consumer.verify_result(values, check) for check in case["verifiers"])


@pytest.mark.parametrize("name", ["set_feeder_test.yaml", "verify_penalty_counts.yaml"])
def test_oracle_yaml_commands_have_valid_shell_syntax(name):
    case = scenario(name)
    captures(case)
    for entry in case["inputs"]:
        result = subprocess.run(
            ["bash", "-n"], input=entry["cmd"], text=True,
            capture_output=True, timeout=5, check=False,
        )
        assert result.returncode == 0, result.stderr
    for verifier in case["verifiers"]:
        assert verifier["type"] in {"eval", "regex"}
        if verifier["type"] == "eval":
            ast.parse(verifier["expr"], mode="eval")
        else:
            re.compile(verifier["expr"])


def test_oracle_checktx_only_reads_both_committed_delegations():
    case = scenario("set_feeder_test.yaml")
    commands = captures(case)
    names = list(commands)
    assert names.index("SET_FEEDER_ERROR_CODE") < names.index("COMMITTED_FEEDER_ADDR")
    assert names.index("COMMITTED_FEEDER_ADDR") < names.index("VOTE_START_HEIGHT")
    assert names.index("VOTE_START_HEIGHT") < names.index("NEW_FEEDER_VOTE_RESPONSE")
    assert names.index("NEW_FEEDER_VOTE_RESPONSE") < names.index("NEW_FEEDER_VOTE_ERROR_CODE")
    assert names.index("NEW_FEEDER_VOTE_ERROR_CODE") < names.index("RESTORE_FEEDER_ERROR_CODE")
    assert names.index("RESTORE_FEEDER_ERROR_CODE") < names.index("RESTORED_FEEDER_ADDR")
    for result, expected in (
        ("COMMITTED_FEEDER_ADDR", "NEW_FEEDER_ADDR"),
        ("RESTORED_FEEDER_ADDR", "OLD_FEEDER_ADDR"),
    ):
        command = commands[result]
        assert command.startswith("set -euo pipefail;")
        assert 'paxd q oracle feeder "$ADMIN_VAL_ADDR" --output json' in command
        assert 'jq -er .feeder_addr' in command
        assert f'test "$feeder" = "${expected}"' in command
        assert f'{result} == {expected}' in [v["expr"] for v in case["verifiers"]]
    assert "oracle set-feeder $NEW_FEEDER_ADDR" in commands["SET_FEEDER_ERROR_CODE"]
    assert "oracle set-feeder $OLD_FEEDER_ADDR" in commands["RESTORE_FEEDER_ERROR_CODE"]
    broadcast = shlex.split(commands["NEW_FEEDER_VOTE_RESPONSE"])
    assert broadcast[broadcast.index("aggregate-vote") + 1:] == [
        "1.5ueth", "$ADMIN_VAL_ADDR", "--from", "$NEW_FEEDER_ADDR", "--gas", "0",
        "-y", "--chain-id=pax", "--broadcast-mode=sync", "--output=json",
    ]
    assert shlex.split(commands["NEW_FEEDER_VOTE_ERROR_CODE"]) == [
        "bash", "tests/integration/oracle_module/verify_committed_vote.sh",
        "$NEW_FEEDER_VOTE_RESPONSE", "$VOTE_START_HEIGHT", "$ADMIN_VAL_ADDR",
        "$NEW_FEEDER_ADDR", "1.5ueth",
    ]


def test_oracle_checktx_only_matches_exact_transaction_and_execution_events():
    script = (ORACLE / "verify_committed_vote.sh").read_text(encoding="utf-8")
    result = subprocess.run(
        ["bash", "-n"], input=script, text=True,
        capture_output=True, timeout=5, check=False,
    )
    assert result.returncode == 0, result.stderr
    tokens = shlex.split(script, comments=True)
    assert script.startswith("#!/usr/bin/env bash\nset -euo pipefail\n")
    assert '.code | select(. == 0)' in tokens
    assert '.txhash | select(test("^[A-Fa-f0-9]{64}$")) | ascii_uppercase' in tokens
    for fact in (
        'deadline=$((SECONDS + 60))', 'while (( SECONDS < deadline )); do',
        'while (( height <= latest )); do', 'index=0',
        'base64 --decode | sha256sum', 'observed=${observed%% *}',
        'if [[ "${observed^^}" == "$txhash" ]]; then',
        'index=$((index + 1))', 'height=$((height + 1))',
        'http://localhost:26657/block?height=$height',
        'http://localhost:26657/block_results?height=$height',
        '--argjson index "$index"', 'sleep 0.5',
    ):
        assert fact in script, fact
    assert script.count('curl --fail --silent --show-error --max-time 5') == 2
    programs = [token for token in tokens if '.txs_results[$index]' in token]
    assert len(programs) == 1
    program = programs[0]
    for fact in (
        '.result | select(.height == $height) | .txs_results[$index]',
        '(.code == 0 or (has("code") | not)) and',
        'any(.events[]; .type == "aggregate_vote" and',
        '(.key | @base64d) == "voter" and (.value | @base64d) == $validator',
        '(.key | @base64d) == "exchange_rates" and (.value | @base64d) == $rates',
        'any(.events[]; .type == "message" and',
        '(.key | @base64d) == "module" and (.value | @base64d) == "oracle"',
        '(.key | @base64d) == "sender" and (.value | @base64d) == $feeder',
    ):
        assert fact in program, fact
    assert script.index("' <<< \"$results\" >/dev/null") < script.index("printf '%s\\n' \"$code\"")
    assert script.rstrip().endswith('exit 1')
    assert '|| true' not in script and 'paxd q tx' not in script


def test_oracle_checktx_only_producer_transport_and_consumer_source_contract():
    producer = source("modules/oracle/keeper/msg_server.go")
    assert 'ms.ValidateFeeder(ctx, feederAddr, valAddr)' in producer
    assert 'ms.SetAggregateExchangeRateVote(ctx, valAddr,' in producer
    for fact in (
        'types.EventTypeAggregateVote', 'types.AttributeKeyVoter, msg.Validator',
        'types.AttributeKeyExchangeRates, msg.ExchangeRates',
        'sdk.AttributeKeyModule, types.AttributeValueCategory',
        'sdk.AttributeKeySender, msg.Feeder',
    ):
        assert fact in producer, fact
    events = source("modules/oracle/types/events.go")
    for name, value in (
        ("EventTypeAggregateVote", "aggregate_vote"),
        ("AttributeKeyVoter", "voter"), ("AttributeKeyExchangeRates", "exchange_rates"),
    ):
        assert re.search(rf'{name}\s*=\s*"{value}"', events)
    assert 'AttributeValueCategory = ModuleName' in events
    assert 'ModuleName = "oracle"' in source("modules/oracle/types/keys.go")
    assert 'func (tx Tx) Hash() TxHash { return TxHash(crypto.Checksum(tx)) }' in source("consensus/types/tx.go")
    transport = source("consensus/internal/rpc/core/blocks.go")
    assert 'env.StateStore.LoadFinalizeBlockResponses(height)' in transport
    assert 'TxsResults:            results.TxResults,' in transport
    schema = source("consensus/abci/types/types.pb.go")
    execution = schema.split('type ExecTxResult struct {', 1)[1].split('\n}', 1)[0]
    assert re.search(r'Code\s+uint32\s+`[^`]*json:"code,omitempty"', execution)
    attributes = schema.split('type EventAttribute struct {', 1)[1].split('\n}', 1)[0]
    assert re.search(r'Key\s+\[\]byte', attributes)
    assert re.search(r'Value\s+\[\]byte', attributes)
    codec = source("consensus/libs/json/encoder.go")
    assert 'Encode byte slices as base64' in codec
    assert not re.search(r'func \([^)]*ExecTxResult\) MarshalJSON', source("consensus/abci/types/types.go"))
    feeder_query = source("modules/oracle/keeper/querier.go")
    assert 'FeederAddr: q.GetFeederDelegation(ctx, valAddr).String()' in feeder_query


def test_oracle_zero_window_validates_parameters_before_selection():
    commands = captures(scenario("verify_penalty_counts.yaml"))
    names = list(commands)
    assert names.index("ORACLE_PARAMS") < names.index("ORACLE_SLASH_WINDOW")
    assert names.index("ORACLE_SLASH_WINDOW") < names.index("BLOCK_HEIGHT")
    assert names.index("VOTE_PERIOD") < names.index("BLOCK_HEIGHT")
    assert '--height "$START_HEIGHT"' in commands["ORACLE_PARAMS"]
    for key, field in (("ORACLE_SLASH_WINDOW", ".slash_window"), ("VOTE_PERIOD", ".vote_period")):
        assert field in commands[key]
        assert '^ [1-9]' not in commands[key]
        assert '^[1-9][0-9]*$' in commands[key]
        assert 'jq -er' in commands[key]
    selection = commands["BLOCK_HEIGHT"]
    first_division = selection.index('height=$(( ')
    for guard in (
        '[[ "$ORACLE_SLASH_WINDOW" =~ ^[1-9][0-9]*$ ]] || exit 1',
        '[[ "$VOTE_PERIOD" =~ ^[1-9][0-9]*$ ]] || exit 1',
        '[[ "$START_HEIGHT" =~ ^[1-9][0-9]*$ ]] || exit 1',
        '(( ORACLE_SLASH_WINDOW > VOTE_PERIOD )) ||',
        '(( ORACLE_SLASH_WINDOW % VOTE_PERIOD == 0 )) || exit 1',
    ):
        assert selection.index(guard) < first_division
    assert 'height=$(( ((START_HEIGHT + 1) / VOTE_PERIOD + 1) * VOTE_PERIOD - 1 ))' in selection
    assert 'if (( (height + 1) % ORACLE_SLASH_WINDOW == 0 )); then\n  height=$((height + VOTE_PERIOD))' in selection


def test_oracle_zero_window_waits_and_pins_params_and_counts():
    commands = captures(scenario("verify_penalty_counts.yaml"))
    names = list(commands)
    assert names.index("BLOCK_HEIGHT") < names.index("OBSERVED_HEIGHT")
    assert names.index("OBSERVED_HEIGHT") < names.index("PARAMETERS_MATCH")
    assert names.index("PARAMETERS_MATCH") < names.index("WINDOW_PROGRESS")
    assert names.index("EXPECTED_COUNTS") < names.index("PENALTY_JSON")
    wait = commands["OBSERVED_HEIGHT"]
    for fact in (
        'for ((attempt = 0; attempt < 60; attempt++)); do',
        'paxd status | jq -er', 'if (( latest >= BLOCK_HEIGHT )); then',
        'sleep 0.5', 'exit 1',
    ):
        assert fact in wait
    assert wait.rstrip().endswith('exit 1')
    assert '--height "$BLOCK_HEIGHT"' in commands["PARAMETERS_MATCH"]
    assert '.params.slash_window == \\$window and .params.vote_period == \\$period' in commands["PARAMETERS_MATCH"]
    assert 'paxd q oracle vote-penalty-counter "$ADMIN_VAL_ADDR" --height "$BLOCK_HEIGHT"' in commands["PENALTY_JSON"]
    assert 'paxd q block' not in '\n'.join(commands.values())
    assert '$(( (BLOCK_HEIGHT + 1) % ORACLE_SLASH_WINDOW ))' in commands["WINDOW_PROGRESS"]
    assert 'expected=$(( WINDOW_PROGRESS / VOTE_PERIOD )); (( expected > 0 ));' in commands["EXPECTED_COUNTS"]
    period = source("modules/oracle/utils/period.go")
    assert 'ctx.BlockHeight())+1)%blocksPerPeriod == 0' in period
    abci = source("modules/oracle/abci.go")
    assert 'utils.IsPeriodLastBlock(ctx, params.VotePeriod)' in abci
    assert 'utils.IsPeriodLastBlock(ctx, params.SlashWindow)' in abci
    assert 'k.SlashAndResetCounters(ctx)' in abci
    assert 'k.DeleteVotePenaltyCounter(ctx, operator)' in source("modules/oracle/keeper/slash.go")


def test_oracle_zero_window_guards_denominator_and_requires_exact_total():
    case = scenario("verify_penalty_counts.yaml")
    commands = captures(case)
    for name, field in (
        ("SUCCESS_COUNT", ".success_count"), ("ABSTAIN_COUNT", ".abstain_count"),
        ("MISS_COUNT", ".miss_count"),
    ):
        assert field in commands[name]
        assert '^(0|[1-9][0-9]*)$' in commands[name]
        assert 'jq -er' in commands[name]
    assert 'SUCCESS_COUNT + ABSTAIN_COUNT + MISS_COUNT' in commands["TOTAL_COUNT"]
    percent = commands["SUCCESS_PERCENT"]
    assert percent.index('(( TOTAL_COUNT > 0 )) ||') < percent.index('SUCCESS_COUNT * 100 / TOTAL_COUNT')
    assert 'echo "Empty vote observation window" >&2; exit 1;' in percent
    assert not any('expr ' in command for command in commands.values())
    assert [check["expr"] for check in case["verifiers"]] == [
        'OBSERVED_HEIGHT >= BLOCK_HEIGHT', '^true$', 'TOTAL_COUNT > 0',
        'TOTAL_COUNT == EXPECTED_COUNTS', 'SUCCESS_PERCENT < 5',
    ]


@pytest.mark.parametrize("key,bad", [
    ("SET_FEEDER_ERROR_CODE", "5"), ("COMMITTED_FEEDER_ADDR", "wrong"),
    ("NEW_FEEDER_VOTE_ERROR_CODE", "5"), ("RESTORE_FEEDER_ERROR_CODE", "5"),
    ("RESTORED_FEEDER_ADDR", "wrong"),
])
def test_oracle_checktx_only_real_verifiers_reject_failed_effects(key, bad):
    case = scenario("set_feeder_test.yaml")
    values = {
        "SET_FEEDER_ERROR_CODE": "0", "COMMITTED_FEEDER_ADDR": "new",
        "NEW_FEEDER_ADDR": "new", "NEW_FEEDER_VOTE_ERROR_CODE": "0",
        "RESTORE_FEEDER_ERROR_CODE": "0", "RESTORED_FEEDER_ADDR": "old",
        "OLD_FEEDER_ADDR": "old",
    }
    assert verifies(case, values)
    values[key] = bad
    assert not verifies(case, values)


@pytest.mark.parametrize("changes", [
    {"TOTAL_COUNT": "0", "EXPECTED_COUNTS": "0", "SUCCESS_PERCENT": "0"},
    {"TOTAL_COUNT": "41"}, {"SUCCESS_PERCENT": "5"},
    {"OBSERVED_HEIGHT": "78"}, {"PARAMETERS_MATCH": "false"},
])
def test_oracle_zero_window_real_verifiers_reject_invalid_observations(changes):
    case = scenario("verify_penalty_counts.yaml")
    values = {
        "OBSERVED_HEIGHT": "80", "BLOCK_HEIGHT": "79", "PARAMETERS_MATCH": "true",
        "TOTAL_COUNT": "40", "EXPECTED_COUNTS": "40", "SUCCESS_PERCENT": "0",
    }
    assert verifies(case, values)
    values.update(changes)
    assert not verifies(case, values)


def test_oracle_repair_record_maps_both_findings_and_evidence_limits():
    record = tomllib.loads(RECORD.read_text(encoding="utf-8"))
    task = record["task"]["22"]["33"]["747"]
    assert task["base_revision"] == "76c050242a1edcad6212aa52fac15670ecb5612d"
    assert task["build_cmd"] == "python3 --version"
    assert task["verify_cmd"] == "python3 -m pytest -q tools/qualification/audit_repairs/test_repair_22_33_747.py"
    assert task["evidence_ledger"] == "/out/run/gates.jsonl"
    assert "same candidate revision" in task["evidence"]
    assert "No baseline test was run" in task["baseline"]
    assert "not executed" in task["review_only"]
    assert task["wiring_requests"] == []
    audit = tomllib.loads(source("spec/layerx-merge/programme/audit/records/22.30.172.kvx"))
    declared = {
        node.name for node in ast.parse(Path(__file__).read_text(encoding="utf-8")).body
        if isinstance(node, ast.FunctionDef) and node.name.startswith("test_")
    }
    for finding_id in ("oracle-checktx-only", "oracle-zero-window"):
        finding = record["finding"][finding_id]
        original = audit["finding"][finding_id]
        assert finding["audit"] == "22.30.172"
        for key in ("file", "class", "severity"):
            assert finding[key] == original[key]
        assert finding["tests"]
        assert set(finding["tests"]) <= declared
        assert finding["proof_kind"] == "source facts and real in-process verifier execution"
