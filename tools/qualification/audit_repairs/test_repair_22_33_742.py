"""Offline source proof for state-store-fixed-fixtures; no node or toolchain."""

import ast
import importlib.util
from pathlib import Path
import re
import shlex
import tomllib

import pytest
import yaml


ROOT = Path(__file__).resolve().parents[3]
SOURCE = ROOT / "tests/integration/db/state_store_test.yaml"
RECORD = Path(__file__).with_name("repair_22_33_742.kvx")
STAGES = ("FIRST_SET", "SECOND_SET", "THIRD_SET")
CASES = yaml.safe_load(SOURCE.read_text(encoding="utf-8"))


def inputs(case):
    return {item["env"]: item["cmd"] for item in case["inputs"]}


def expressions(case):
    assert all(item["type"] == "eval" for item in case["verifiers"])
    return [item["expr"] for item in case["verifiers"]]


def query(command):
    """Parse the actual shell pipeline without executing external programs."""
    assert command.startswith("set -o pipefail; paxd q ")
    cli, jq = command.removeprefix("set -o pipefail; ").split("| jq ", 1)
    return shlex.split(cli), shlex.split(jq)


def option(argv, name):
    assert argv.count(name) == 1
    return argv[argv.index(name) + 1]


def runner():
    spec = importlib.util.spec_from_file_location(
        "state_store_integration_runner",
        ROOT / "tests/integration/scripts/runner.py",
    )
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module.TestRunner(docker=False)


def test_state_store_yaml_captures_every_input_before_consuming_it():
    assert [case["name"] for case in CASES] == [
        "Test state store iteration through wasm list code",
        "Test state store historical data checking specific wasm codes",
        "Test state store iteration through tokenfactory denoms",
    ]
    for case in CASES:
        captured = set()
        for item in case["inputs"]:
            assert set(item) == {"cmd", "env"}
            command, name = item["cmd"], item["env"]
            assert name not in captured
            # jq's lowercase $creator is an explicitly bound jq argument.
            references = set(re.findall(r"\$([A-Z][A-Z_]+)", command))
            references.update(re.findall(r"\b(BEGINNING_LIST_CODE_LENGTH)\b", command))
            assert references <= captured, (name, references - captured)
            assert "|| true" not in command and "//" not in command
            captured.add(name)
        used = set()
        for expression in expressions(case):
            tree = ast.parse(expression, mode="eval")
            names = {node.id for node in ast.walk(tree) if isinstance(node, ast.Name)}
            assert names <= captured
            used.update(names)
        # Every captured value feeds a later command or an actual assertion.
        all_commands = "\n".join(inputs(case).values())
        for name in captured - used:
            assert name in all_commands


def test_wasm_totals_use_the_recorded_baseline_and_both_iteration_orders():
    commands = inputs(CASES[0])
    expected = [
        "BEGINNING_LIST_CODE_LENGTH_REVERSE == BEGINNING_LIST_CODE_LENGTH"
    ]
    for stage in ("BEGINNING",) + STAGES:
        assert commands[f"{stage}_BLOCK_HEIGHT"] == (
            f"tail -1 tests/integration/contracts/wasm_{stage.lower()}_block_height.txt"
        )
        for suffix in ("", "_REVERSE"):
            argv, jq = query(commands[f"{stage}_LIST_CODE_LENGTH{suffix}"])
            assert argv[:4] == ["paxd", "q", "wasm", "list-code"]
            assert ("--reverse" in argv) == bool(suffix)
            assert "--count-total" in argv
            assert option(argv, "--limit") == "1"
            assert option(argv, "--height") == f"${stage}_BLOCK_HEIGHT"
            assert option(argv, "--output") == "json"
            assert jq == ["-er", ".pagination.total | tonumber"]
            if stage != "BEGINNING":
                total = (STAGES.index(stage) + 1) * 10
                expected.append(
                    f"{stage}_LIST_CODE_LENGTH{suffix} - BEGINNING_LIST_CODE_LENGTH == {total}"
                )
    assert expressions(CASES[0]) == expected
    assert "--limit 400" not in SOURCE.read_text(encoding="utf-8")


@pytest.mark.parametrize("baseline", [0, 3, 9, 401, 1201])
def test_real_verifier_accepts_count_deltas_and_rejects_missing_or_extra_codes(baseline):
    consumer = runner()
    case = CASES[0]
    values = {
        "BEGINNING_LIST_CODE_LENGTH": str(baseline),
        "BEGINNING_LIST_CODE_LENGTH_REVERSE": str(baseline),
    }
    for index, stage in enumerate(STAGES, 1):
        for suffix in ("", "_REVERSE"):
            values[f"{stage}_LIST_CODE_LENGTH{suffix}"] = str(baseline + index * 10)
    assert all(consumer.verify_result(values, item) for item in case["verifiers"])
    for name in values:
        if name == "BEGINNING_LIST_CODE_LENGTH":
            continue
        original = values[name]
        for delta in (-1, 1):
            values[name] = str(int(original) + delta)
            assert not all(
                consumer.verify_result(values, item) for item in case["verifiers"]
            ), (name, delta)
        values[name] = original


def test_fixture_ids_and_creators_are_captured_and_compared_across_heights():
    commands = inputs(CASES[1])
    assert commands["CREATOR_ID"] == (
        "tail -1 tests/integration/contracts/wasm_creator_id.txt"
    )
    assert commands["BEGINNING_BLOCK_HEIGHT"] == (
        "tail -1 tests/integration/contracts/wasm_beginning_block_height.txt"
    )
    baseline_argv, baseline_jq = query(commands["BEGINNING_LIST_CODE_LENGTH"])
    assert option(baseline_argv, "--height") == "$BEGINNING_BLOCK_HEIGHT"
    assert "--count-total" in baseline_argv
    assert baseline_jq == ["-er", ".pagination.total | tonumber"]
    expected = []
    for at_index, at in enumerate(STAGES):
        assert commands[f"{at}_BLOCK_HEIGHT"] == (
            f"tail -1 tests/integration/contracts/wasm_{at.lower()}_block_height.txt"
        )
        for batch_index, batch in enumerate(STAGES[:at_index + 1]):
            canonical = batch + "_CODES"
            for reverse in (False, True):
                name = canonical + (f"_AT_{at}" if at != batch else "")
                name += "_REVERSE" if reverse else ""
                argv, jq = query(commands[name])
                assert argv[:4] == ["paxd", "q", "wasm", "list-code"]
                assert ("--reverse" in argv) == reverse
                if reverse:
                    offset = str((at_index - batch_index) * 10)
                elif batch_index == 0:
                    offset = "$BEGINNING_LIST_CODE_LENGTH"
                else:
                    offset = f"$((BEGINNING_LIST_CODE_LENGTH + {batch_index * 10}))"
                assert option(argv, "--offset") == offset
                assert option(argv, "--height") == f"${at}_BLOCK_HEIGHT"
                assert option(argv, "--limit") == "10"
                assert option(argv, "--output") == "json"
                assert jq[:4] == ["-ce", "--arg", "creator", "$CREATOR_ID"]
                assert len(jq) == 5
                assert jq[4] == (
                    '.code_infos | if length == 10 and all(.[]; '
                    '(.code_id | test("^[1-9][0-9]*$")) and .creator == $creator) '
                    'then sort_by(.code_id | tonumber) '
                    'else error("expected ten fixture codes from recorded creator") end'
                )
                if name != canonical:
                    expected.append(f"{name} == {canonical}")
    assert expressions(CASES[1]) == expected
    assert len(expected) == 9
    assert len([name for name in commands if "_CODES" in name]) == 12
    assert ".code_infos[3]" not in SOURCE.read_text(encoding="utf-8")


def test_tokenfactory_uses_baseline_deltas_and_comments_match_ten_item_batches():
    commands = inputs(CASES[2])
    for stage in ("BEGINNING",) + STAGES:
        assert commands[f"{stage}_BLOCK_HEIGHT"] == (
            f"tail -1 tests/integration/contracts/tfk_{stage.lower()}_block_height.txt"
        )
        argv, jq = query(commands[f"{stage}_TFK_DENOMS"])
        assert argv[:5] == [
            "paxd", "q", "tokenfactory", "denoms-from-creator", "$CREATOR_ID"
        ]
        assert option(argv, "--height") == f"${stage}_BLOCK_HEIGHT"
        assert option(argv, "--limit") == "1"
        assert "--count-total" in argv
        assert jq == ["-er", ".pagination.total | tonumber"]
    assert expressions(CASES[2]) == [
        f"{stage}_TFK_DENOMS - BEGINNING_TFK_DENOMS == {index * 10}"
        for index, stage in enumerate(STAGES, 1)
    ]
    source = SOURCE.read_text(encoding="utf-8")
    comments = "\n".join(line for line in source.splitlines() if line.lstrip().startswith("#"))
    assert not re.search(r"\b(?:100|200|300)\b", comments)
    assert "ten" in comments.lower()
    consumer = runner()
    for baseline in (0, 7, 401):
        values = {"BEGINNING_TFK_DENOMS": str(baseline)}
        values.update({
            f"{stage}_TFK_DENOMS": str(baseline + index * 10)
            for index, stage in enumerate(STAGES, 1)
        })
        assert all(consumer.verify_result(values, item) for item in CASES[2]["verifiers"])
        for stage in STAGES:
            name = f"{stage}_TFK_DENOMS"
            values[name] = str(int(values[name]) - 1)
            assert not all(
                consumer.verify_result(values, item) for item in CASES[2]["verifiers"]
            )
            values[name] = str(int(values[name]) + 1)


def test_source_proof_connects_fixture_producers_cli_pagination_and_consumers():
    for kind, script in (
        ("wasm", "deploy_wasm_contracts.sh"),
        ("tfk", "create_tokenfactory_denoms.sh"),
    ):
        producer = (ROOT / "tests/integration/contracts" / script).read_text(encoding="utf-8")
        assert re.findall(r"for i in \{(\d+)\.\.(\d+)\}", producer) == [
            ("1", "10"), ("11", "20"), ("21", "30")
        ]
        for stage in ("beginning", "first_set", "second_set", "third_set"):
            assert f'{kind}_{stage}_block_height.txt' in producer
        assert f'{kind}_creator_id.txt' in producer
    cli = (ROOT / "wasm/x/wasm/client/cli/query.go").read_text(encoding="utf-8")
    assert 'Use:     "list-code"' in cli
    assert 'flags.AddPaginationFlagsToCmd(cmd, "list codes")' in cli
    keeper = (ROOT / "wasm/x/wasm/keeper/querier.go").read_text(encoding="utf-8")
    assert "query.FilteredPaginate(prefixStore, req.Pagination" in keeper
    assert "CodeID:                binary.BigEndian.Uint64(key)" in keeper
    assert "Creator:               c.Creator" in keeper
    assert "CodeInfos: r, Pagination: pageRes" in keeper
    token_cli = (ROOT / "modules/tokenfactory/client/cli/query.go").read_text(encoding="utf-8")
    assert "Pagination: pageReq" in token_cli
    assert 'flags.AddPaginationFlagsToCmd(cmd, "denoms-from-creator")' in token_cli
    go_consumer = (ROOT / "tests/integration/runner/runner.go").read_text(encoding="utf-8")
    assert "envMap[inp.Env] = out" in go_consumer
    assert "verify(v, envMap)" in go_consumer
    assert '"db/state_store_test.yaml"' in (
        ROOT / "tests/integration/runner/suites.go"
    ).read_text(encoding="utf-8")


def test_repair_record_maps_finding_to_executable_proofs_and_limits():
    record = tomllib.loads(RECORD.read_text(encoding="utf-8"))
    finding = record["finding"]["state-store-fixed-fixtures"]
    assert finding["audit"] == "22.30.172"
    assert finding["file"] == "tests/integration/db/state_store_test.yaml"
    assert finding["tests"]
    for name in finding["tests"]:
        assert name in globals() and callable(globals()[name])
    task = record["task"]["22"]["33"]["742"]
    assert task["base_revision"] == "76c050242a1edcad6212aa52fac15670ecb5612d"
    assert task["evidence_ledger"] == "/out/run/gates.jsonl"
    assert task["review_only"] and task["baseline"]
    assert task["wiring_request"] == []
