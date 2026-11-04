"""Offline documentation proof for audit 22.30.172/template-eval-grammar.

Execute the real Python YAML loader/verifier and inspect the Go consumer source
and committed evaluator cases; do not claim to execute the Go toolchain.
"""

import ast
import importlib.util
from pathlib import Path
import re
import tomllib

import pytest


ROOT = Path(__file__).resolve().parents[3]
TEMPLATE = ROOT / "tests/integration/template/template_test.yaml"
RECORD = Path(__file__).with_name("repair_22_33_759.kvx")
EXAMPLES = {
    "1 == 1 or 2 == 3 and 4 == 5": True,
    "1 == 2 and 2 == 3 or 4 == 4": True,
    "1 == 2 or 2 == 2 and 3 == 4": False,
    "2 + 3 * 4 == 14": True,
    "2 + 3 * 4 == 20": False,
    "8 - 3 - 2 == 3": True,
    "3 / 2 == 1.5": True,
}


def source(path):
    return (ROOT / path).read_text(encoding="utf-8")


@pytest.fixture
def runner():
    spec = importlib.util.spec_from_file_location(
        "template_repair_runner", ROOT / "tests/integration/scripts/runner.py"
    )
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module.TestRunner()


def test_template_eval_grammar_guidance_and_yaml_schema(runner):
    text = TEMPLATE.read_text(encoding="utf-8")
    assert "valid python eval" not in text.lower()
    for fact in (
        "Python scripts/runner.py and Go runner/runner.go",
        "operand ==, !=, <, <=, > or >= operand",
        "Separate every token with spaces",
        "canonical decimal numbers (no leading zeroes except 0)",
        "Use numeric captures for arithmetic; use nonnumeric strings for text checks",
        "Arithmetic supports +, -, *, /; * and / bind before + and -",
        "Division produces a floating-point value; never divide by zero",
        "and binds before or",
        "associate left to right",
        "Keep all names bound and all branches valid",
        "Go OR can accept a true",
        "Python raises on that error",
        "Python's AST evaluator also supports not, //, %, parentheses and chained",
        "outside the shared grammar",
        "Neither runner supports",
        "general Python calls or indexing",
        "Avoid float equality due to rounding",
    ):
        assert fact in text, fact
    assert runner.load_yaml_file(TEMPLATE) == [{
        "name": "<Replace with test description>",
        "inputs": [
            {
                "cmd": "<Replace with bash command>",
                "env": "<Add if you want to store the output as an env variable>",
                "node": "<Optional, default is pax-node-0>",
            },
            {"cmd": "<Replace with bash command>", "env": "RESULT"},
        ],
        "verifiers": [
            {"type": "eval", "expr": "RESULT == 4"},
            {"type": "regex", "result": "RESULT", "expr": "<Replace with regular expression>"},
        ],
    }]


@pytest.mark.parametrize("expression, expected", EXAMPLES.items())
def test_template_boolean_arithmetic_examples_match_committed_go_cases(
    runner, expression, expected
):
    examples = re.findall(
        r"^    # Shared eval example: (.+)$", TEMPLATE.read_text(encoding="utf-8"), re.M
    )
    assert examples == list(EXAMPLES)
    assert runner.verify_result({}, {"type": "eval", "expr": expression}) is expected
    go_cases = source("tests/integration/runner/eval_test.go")
    assert "func TestEvalExprPrecedence(t *testing.T)" in go_cases
    assert '{"' + expression + '", ' + str(expected).lower() + '}' in go_cases
    assert "evalExpr(tt.expr," in go_cases
    assert "got != tt.want" in go_cases


@pytest.mark.parametrize("capture, expected", [("4", True), ("3", False), ("5", False)])
def test_template_capture_reaches_real_yaml_eval_consumer(runner, capture, expected):
    case = runner.load_yaml_file(TEMPLATE)[0]
    capture_name = case["inputs"][-1]["env"]
    assert capture_name == "RESULT"
    assert runner.verify_result({capture_name: capture}, case["verifiers"][0]) is expected
    expression = re.search(
        r'# Captured example \(RESULT is trimmed stdout "4"\): (.+)',
        TEMPLATE.read_text(encoding="utf-8"),
    ).group(1)
    assert expression == "RESULT >= 1 and RESULT <= 4"
    assert runner.verify_result(
        {capture_name: capture}, {"type": "eval", "expr": expression}
    ) is (capture in {"3", "4"})


def test_template_common_grammar_is_wired_to_go_consumer_source():
    go = source("tests/integration/runner/runner.go")
    for fact in (
        '`yaml:"verifiers"`', '`yaml:"expr"`', '`yaml:"env,omitempty"`',
        "yaml.Unmarshal(data, &cases)", "runCase(t, tc, o)",
        "envMap[inp.Env] = out", "verify(v, envMap)",
        "evalExpr(v.Expr, envMap)", "exprTokens(expr)",
        "validateExprTokens(tokens)", "evalBoolTokens(tokens, envMap)",
        'case "==", "!=", ">", "<", ">=", "<=":',
        'for _, op := range []string{"or", "and"}',
        "v, ok := envMap[t.text]", "evalArith(resolve(tokens[:i]))",
        'case "*", "/":', 'case "+", "-":',
        'op != "/"', "x /= y", 'operand.asFloat() == 0',
        "cmpValues(lhs, rhs, token.text)",
    ):
        assert fact in go, fact


def test_template_repair_record_maps_finding_and_evidence():
    record = tomllib.loads(RECORD.read_text(encoding="utf-8"))
    finding = record["finding"]["template-eval-grammar"]
    assert finding["audit"] == "22.30.172"
    assert finding["file"] == "tests/integration/template/template_test.yaml"
    assert finding["class"] == "doc-drift"
    declared = {
        node.name for node in ast.parse(Path(__file__).read_text(encoding="utf-8")).body
        if isinstance(node, ast.FunctionDef) and node.name.startswith("test_")
    }
    assert set(finding["tests"]) == declared - {
        "test_template_repair_record_maps_finding_and_evidence"
    }
    task = record["task"]["22"]["33"]["759"]
    assert task["base_revision"] == "76c050242a1edcad6212aa52fac15670ecb5612d"
    assert task["build_cmd"] == "python3 --version"
    assert task["verify_cmd"] == (
        "python3 -m pytest -q tools/qualification/audit_repairs/test_repair_22_33_759.py"
    )
    assert task["evidence_ledger"] == "/out/run/gates.jsonl"
    assert task["wiring_requests"] == []

