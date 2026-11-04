"""Source wiring and in-process validation for the three authz audit findings."""

import ast
import importlib.util
import json
from pathlib import Path
import shlex
import tomllib

import pytest


ROOT = Path(__file__).resolve().parents[3]
AUTHZ = ROOT / "tests/integration/authz_module"
RECORD = ROOT / "tools/qualification/audit_repairs/repair_22_33_738.kvx"
HELPER = "tests/integration/authz_module/authz_evidence.py"
TXHASH = "AB" * 32
DELEGATOR = "pax1delegator"
VALIDATOR = "paxvaloper1validator"


def load_module(name, path):
    spec = importlib.util.spec_from_file_location(name, path)
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


EVIDENCE = load_module("authz_evidence_738", AUTHZ / "authz_evidence.py")
RUNNER = load_module("integration_runner_738", ROOT / "tests/integration/scripts/runner.py")


def suite(filename):
    cases = RUNNER.TestRunner().load_yaml_file(AUTHZ / filename)
    assert len(cases) == 1
    return cases[0]


def captured(case, name):
    matches = [step for step in case["inputs"] if step.get("env") == name]
    assert len(matches) == 1
    return matches[0]


def require_verifier(case, expression, accepted, rejected):
    matches = [
        verifier for verifier in case["verifiers"]
        if verifier["type"] == "eval" and verifier["expr"] == expression
    ]
    assert len(matches) == 1
    runner = RUNNER.TestRunner()
    assert runner.verify_result(accepted, matches[0]) is True
    for values in rejected:
        assert runner.verify_result(values, matches[0]) is False


def result_json(execution_code, execution_reason, **changes):
    value = {
        "txhash": TXHASH, "height": "42", "code": execution_code,
        "codespace": "sdk" if execution_code else "", "raw_log": execution_reason,
    }
    value.update(changes)
    return json.dumps(value)


def test_authz_send_rejection():
    case = suite("send_authorization_test.yaml")
    step = captured(case, "AUTHZ_2_ERROR_CODE")
    assert shlex.split(step["cmd"]) == [
        "python3", HELPER, "exec", "$GRANTEE_ADDR", "send_tx.json", "send-limit",
    ]
    assert case["inputs"].index(captured(case, "AUTHZ_1_ERROR_CODE")) < case["inputs"].index(step)
    assert case["inputs"].index(step) < case["inputs"].index(captured(case, "OTHER_ACC_BALANCE"))
    require_verifier(case, "AUTHZ_1_ERROR_CODE == 0", {"AUTHZ_1_ERROR_CODE": "0"}, [{"AUTHZ_1_ERROR_CODE": "5"}])
    require_verifier(
        case, "AUTHZ_2_ERROR_CODE == 5", {"AUTHZ_2_ERROR_CODE": "5"},
        [{"AUTHZ_2_ERROR_CODE": str(code)} for code in (0, 4, 11, 13, 32)],
    )
    require_verifier(
        case, "OTHER_ACC_BALANCE == 6000000", {"OTHER_ACC_BALANCE": "6000000"},
        [{"OTHER_ACC_BALANCE": str(amount)} for amount in (0, 12000000)],
    )
    assert '--spend-limit 10pax' in case["inputs"][4]["cmd"]
    assert '6pax --generate-only > send_tx.json' in case["inputs"][5]["cmd"]


def test_authz_unbond_rejection():
    case = suite("staking_authorization_test.yaml")
    denied = captured(case, "AUTHZ_2_ERROR_CODE")
    assert shlex.split(denied["cmd"]) == [
        "python3", HELPER, "exec", "$GRANTEE_ADDR", "unbond_tx.json", "unbond-denied",
    ]
    grants = [step for step in case["inputs"] if 'authz grant $GRANTEE_ADDR unbond ' in step["cmd"]]
    assert len(grants) == 1
    ordered = [
        captured(case, "AUTHZ_1_ERROR_CODE"), denied, captured(case, "STAKING_AMOUNT"),
        grants[0], captured(case, "AUTHZ_3_ERROR_CODE"), captured(case, "STAKING_AMOUNT_AFTER_UNBOND"),
    ]
    indices = [case["inputs"].index(step) for step in ordered]
    assert indices == sorted(set(indices))
    require_verifier(case, "AUTHZ_1_ERROR_CODE == 0", {"AUTHZ_1_ERROR_CODE": "0"}, [{"AUTHZ_1_ERROR_CODE": "4"}])
    require_verifier(
        case, "AUTHZ_2_ERROR_CODE == 4", {"AUTHZ_2_ERROR_CODE": "4"},
        [{"AUTHZ_2_ERROR_CODE": str(code)} for code in (0, 5, 11, 13, 32)],
    )
    require_verifier(case, "STAKING_AMOUNT == 1000000", {"STAKING_AMOUNT": "1000000"}, [{"STAKING_AMOUNT": "0"}])


def test_authz_unbond_query():
    case = suite("staking_authorization_test.yaml")
    assert shlex.split(captured(case, "AUTHZ_3_ERROR_CODE")["cmd"]) == [
        "python3", HELPER, "exec", "$GRANTEE_ADDR", "unbond_tx.json", "unbond-success",
    ]
    assert shlex.split(captured(case, "STAKING_AMOUNT_AFTER_UNBOND")["cmd"]) == [
        "python3", HELPER, "delegation", "$ADMIN_ADDR", "$NODE_ADMIN_VAL_ADDR",
    ]
    require_verifier(case, "AUTHZ_3_ERROR_CODE == 0", {"AUTHZ_3_ERROR_CODE": "0"}, [{"AUTHZ_3_ERROR_CODE": "4"}])
    require_verifier(
        case, "STAKING_AMOUNT_AFTER_UNBOND == 0", {"STAKING_AMOUNT_AFTER_UNBOND": "0"},
        [{"STAKING_AMOUNT_AFTER_UNBOND": str(value)} for value in (1, 1000000, "null", "")],
    )


@pytest.mark.parametrize("code,reason", [
    (5, "requested amount is more than spend limit: insufficient funds"),
    (4, "authorization not found: unauthorized"),
    (0, ""),
])
def test_committed_results_require_height_hash_and_intended_execution(code, reason):
    expected_reason = {5: EVIDENCE.SEND_LIMIT_REASON, 4: EVIDENCE.UNBOND_AUTH_REASON, 0: ""}[code]
    assert EVIDENCE.committed_code(result_json(code, reason), TXHASH, code, expected_reason) == code
    assert EVIDENCE.committed_code(result_json(code, reason, height=42, txhash=TXHASH.lower()), TXHASH, code, expected_reason) == code
    invalid = [
        {"height": "0"}, {"height": 0}, {"height": -1}, {"height": True},
        {"height": None}, {"height": "garbage"}, {"height": 1.5},
        {"txhash": "CD" * 32}, {"txhash": None}, {"code": None},
        {"code": str(code)}, {"code": False}, {"code": (code + 1) % 6},
    ]
    if code:
        invalid.extend([
            {"codespace": "bank"}, {"codespace": ""}, {"codespace": None},
            {"raw_log": "insufficient fees"}, {"raw_log": "account sequence mismatch"},
            {"raw_log": ""}, {"raw_log": None},
        ])
    for changes in invalid:
        with pytest.raises(ValueError):
            EVIDENCE.committed_code(result_json(code, reason, **changes), TXHASH, code, expected_reason)
    for raw in ("", "not JSON", "null", "[]", "{}"):
        with pytest.raises(ValueError):
            EVIDENCE.committed_code(raw, TXHASH, code, expected_reason)


def test_checktx_and_missing_fields_cannot_supply_execution_evidence():
    admitted = json.dumps({"code": 0, "txhash": TXHASH, "height": "0"})
    assert EVIDENCE.submission_hash(admitted) == TXHASH
    with pytest.raises(ValueError):
        EVIDENCE.committed_code(admitted, TXHASH, 0, "")
    for changes in (
        {"code": 4}, {"code": 5}, {"code": 13}, {"code": 32}, {"code": None},
        {"code": False}, {"code": "0"}, {"txhash": ""}, {"txhash": "xyz"},
        {"txhash": "A" * 63}, {"txhash": "G" * 64}, {"txhash": None},
    ):
        value = {"code": 0, "txhash": TXHASH}
        value.update(changes)
        with pytest.raises(ValueError):
            EVIDENCE.submission_hash(json.dumps(value))
    for field in ("code", "txhash", "height"):
        value = json.loads(result_json(0, ""))
        del value[field]
        with pytest.raises(ValueError):
            EVIDENCE.committed_code(json.dumps(value), TXHASH, 0, "")
    for raw in ("", "{}", "[]", "null", "malformed"):
        with pytest.raises(ValueError):
            EVIDENCE.submission_hash(raw)


def missing_delegation_error():
    return f"delegation with delegator {DELEGATOR} not found for validator {VALIDATOR}"


def delegation_json(amount="1000000", **changes):
    value = {
        "delegation": {"delegator_address": DELEGATOR, "validator_address": VALIDATOR, "shares": "1000000"},
        "balance": {"denom": "uhpx", "amount": amount},
    }
    value.update(changes)
    return json.dumps(value)


def test_delegation_only_specific_not_found_becomes_zero():
    prefix = "rpc error: code = NotFound desc = "
    for stderr in (
        f"Error: {prefix}{missing_delegation_error()}\n",
        f"Error: {prefix}{prefix}{missing_delegation_error()}: key not found\n",
    ):
        assert EVIDENCE.delegation_amount(1, "", stderr, DELEGATOR, VALIDATOR) == 0
        for returncode, stdout in ((0, ""), (2, ""), (-9, ""), (1, "malformed")):
            with pytest.raises(ValueError):
                EVIDENCE.delegation_amount(returncode, stdout, stderr, DELEGATOR, VALIDATOR)
    for stderr in (
        "", "Error: connection refused", "Error: context deadline exceeded",
        "Error: rpc error: code = Unavailable desc = connection closed",
        "Error: rpc error: code = NotFound desc = transaction not found",
        "Error: no delegation for (address, validator) tuple",
        f"Error: {prefix}{missing_delegation_error().replace(DELEGATOR, 'someone-else')}",
        f"Error: {prefix}{missing_delegation_error().replace(VALIDATOR, 'another-validator')}",
        f"Error: {prefix}{missing_delegation_error()}\nError: connection refused",
        f"Error: rpc error: code = Internal desc = {missing_delegation_error()}",
    ):
        with pytest.raises(ValueError):
            EVIDENCE.delegation_amount(1, "", stderr, DELEGATOR, VALIDATOR)


def test_delegation_success_requires_valid_balance_and_pair():
    for amount in ("0", "1", "1000000"):
        assert EVIDENCE.delegation_amount(0, delegation_json(amount), "", DELEGATOR, VALIDATOR) == int(amount)
    invalid = ["", "null", "{}", "[]", "not JSON"]
    invalid.extend(delegation_json(amount) for amount in (None, 0, True, -1, "-1", "null", "", "1.5"))
    invalid.extend([
        delegation_json(balance=None), delegation_json(balance={"amount": "0"}),
        delegation_json(balance={"denom": "pax", "amount": "0"}),
        delegation_json(balance={"denom": "uhpx"}), delegation_json(delegation={}),
        delegation_json(delegation={"delegator_address": DELEGATOR, "validator_address": "other"}),
        delegation_json(delegation={"delegator_address": "other", "validator_address": VALIDATOR}),
    ])
    for raw in invalid:
        with pytest.raises(ValueError):
            EVIDENCE.delegation_amount(0, raw, "", DELEGATOR, VALIDATOR)


def test_real_transport_and_cli_source_wiring():
    source = (AUTHZ / "authz_evidence.py").read_text()
    tree = ast.parse(source)
    functions = {node.name: node for node in tree.body if isinstance(node, ast.FunctionDef)}
    execution = ast.get_source_segment(source, functions["execute"])
    assert '"send-limit": (5, SEND_LIMIT_REASON)' in execution
    assert '"unbond-denied": (4, UNBOND_AUTH_REASON)' in execution
    assert '"unbond-success": (0, "")' in execution
    assert '"paxd", "tx", "authz", "exec", transaction, "--from", grantee' in execution
    assert '"--fees", "2000uhpx", "--chain-id", "pax", "-y"' in execution
    assert '"--broadcast-mode=sync", "--output=json"' in execution
    assert 'input="12345678\\n", text=True, capture_output=True, check=True, timeout=30' in execution
    assert execution.index('txhash = submission_hash(submitted.stdout)') < execution.index('["paxd", "q", "tx", txhash, "--output=json"]')
    assert 'if result.returncode == 0:\n            return committed_code(result.stdout, txhash, expected_code, expected_reason)' in execution
    assert 'deadline = time.monotonic() + 30' in execution
    assert 'while time.monotonic() < deadline:' in execution
    assert 'raise TimeoutError(' in execution
    query = ast.get_source_segment(source, functions["query_delegation"])
    assert '["paxd", "q", "staking", "delegation", delegator, validator, "--output=json"]' in query
    assert 'result.returncode, result.stdout, result.stderr, delegator, validator' in query
    main = ast.get_source_segment(source, functions["main"])
    assert 'value = execute(args.grantee, args.transaction, args.expectation)' in main
    assert 'value = query_delegation(args.delegator, args.validator)' in main
    assert 'return 1' in main and 'print(value)' in main


def test_sdk_producer_transport_and_result_contracts():
    def source(path):
        return (ROOT / path).read_text()

    errors = source("sdk/types/errors/errors.go")
    assert 'ErrUnauthorized = Register(RootCodespace, 4, "unauthorized")' in errors
    assert 'ErrInsufficientFunds = Register(RootCodespace, 5, "insufficient funds")' in errors
    assert 'RootCodespace = "sdk"' in errors
    assert 'sdkerrors.ErrInsufficientFunds.Wrapf("requested amount is more than spend limit")' in source("sdk/x/bank/types/send_authorization.go")
    assert 'sdkerrors.ErrUnauthorized.Wrap("authorization not found")' in source("sdk/x/authz/keeper/keeper.go")
    result = source("sdk/types/result.go")
    for fact in ("TxHash:    res.Hash.String()", "Height:    res.Height", "Codespace: res.TxResult.Codespace", "Code:      res.TxResult.Code", "RawLog:    res.TxResult.Log"):
        assert fact in result
    assert 'node.Tx(ctx, hash, true)' in source("sdk/x/auth/tx/query.go")
    assert '"delegation with delegator %s not found for validator %s"' in source("sdk/x/staking/keeper/grpc_query.go")
    assert 'return clientCtx.PrintProto(res.DelegationResponse)' in source("sdk/x/staking/client/cli/query.go")
    assert 'sdkerrors.Wrap(sdkerrors.ErrKeyNotFound, err.Error())' in source("sdk/baseapp/abci.go")
    assert 'return status.Error(codes.NotFound, resp.Log)' in source("sdk/client/query.go")
    assert 'fmt.Fprintln(os.Stderr, "Error:", err)' in source("daemon/paxd/main.go")


def test_repair_record_maps_every_finding_and_evidence_limit():
    record = tomllib.loads(RECORD.read_text())
    assert record["repair"]["task"] == "22.33.738"
    assert record["repair"]["base_revision"] == "76c050242a1edcad6212aa52fac15670ecb5612d"
    assert record["repair"]["build_cmd"] == "python3 --version"
    assert record["repair"]["verify_cmd"] == "python3 -m pytest -q tools/qualification/audit_repairs/test_repair_22_33_738.py"
    assert record["repair"]["evidence_ledger"] == "/out/run/gates.jsonl"
    assert "No baseline test was run" in record["repair"]["baseline"]
    assert "not executed" in record["repair"]["review_only"]
    assert "same candidate revision" in record["repair"]["evidence"]
    expected = {"authz-send-rejection", "authz-unbond-rejection", "authz-unbond-query"}
    assert set(record["finding"]) == expected
    declared = {node.name for node in ast.parse(Path(__file__).read_text()).body if isinstance(node, ast.FunctionDef)}
    audit = tomllib.loads((ROOT / "spec/layerx-merge/programme/audit/records/22.30.172.kvx").read_text())
    for finding in expected:
        repair = record["finding"][finding]
        assert repair["audit"] == "22.30.172"
        assert repair["file"] == audit["finding"][finding]["file"]
        assert repair["tests"] and set(repair["tests"]) <= declared
    assert record["wiring"]["requests"] == []
