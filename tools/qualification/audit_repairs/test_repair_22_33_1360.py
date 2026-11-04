"""Source proofs for the production module runner; Bash parsing is the build gate.

No Go runner or substitute producer is executed. These assertions check the real
discovery -> test invocation -> profile validation -> publication control flow.
"""

import ast
from pathlib import Path
import re
import shlex
import tomllib


ROOT = Path(__file__).resolve().parents[3]
SCRIPT = ROOT / "sdk/scripts/module-tests.sh"
RECORD = Path(__file__).with_name("repair_22_33_1360.kvx")
FUNCTION = re.compile(r"^([a-z_]+)\(\) \{\n(.*?)^\}\n", re.MULTILINE | re.DOTALL)


def commands(source):
    """Tokenize shell quoting and continuations without starting a toolchain."""
    result = []
    for line in source.replace("\\\n", "").splitlines():
        lexer = shlex.shlex(line, posix=True, punctuation_chars=";&|()<>")
        lexer.whitespace_split = True
        tokens = list(lexer)
        if tokens:
            result.append(tokens)
    return result


def script_parts():
    source = SCRIPT.read_text(encoding="utf-8")
    functions = dict(FUNCTION.findall(source))
    assert set(functions) == {"execute_mod_tests", "append_coverage_profile"}
    return source, functions, FUNCTION.sub("", source)


def test_module_dependent_tests_always_discovers_and_runs_all_nested_modules():
    source, functions, top = script_parts()
    top_commands = commands(top)
    discovery = [
        "find", ".", "-name", "go.mod", "-not", "-path", "./go.mod",
        "-print0", ">", "$module_list",
    ]
    loop = ["while", "IFS=", "read", "-r", "-d", "", "go_mod", ";", "do"]
    invocation = ["if", "execute_mod_tests", "$go_mod", ";", "then"]
    assert discovery in top_commands
    assert loop in top_commands
    assert invocation in top_commands
    assert ["done", "<", "$module_list"] in top_commands
    assert top_commands.index(discovery) < top_commands.index(loop)
    assert top_commands.index(loop) < top_commands.index(invocation)
    executable = commands(source)
    words = [word for command in executable for word in command]
    assert "GIT_DIFF" not in source
    assert not {"grep", "continue", "break"}.intersection(words)
    assert "return" not in [
        word for command in commands(functions["execute_mod_tests"])
        for word in command
    ]
    assert sum(command[:1] == ["find"] for command in executable) == 1
    assert sum(
        command[:2] == ["if", "execute_mod_tests"] for command in executable
    ) == 1


def test_module_test_command_preserves_all_flags_and_guards_working_directory():
    _, functions, _ = script_parts()
    runner = commands(functions["execute_mod_tests"])
    assert ["local", "go_mod=$1"] in runner
    assert 'mod_dir=$(dirname "$go_mod")' in functions["execute_mod_tests"]
    guarded_cd = ["cd", "--", "$mod_dir", "||", "exit", "1"]
    go_test = [
        "go", "test", "-mod=readonly", "-timeout", "30m",
        "-coverprofile=$module_profile", "-covermode=atomic",
        "-tags=norace ledger test_ledger_mock", "./...",
    ]
    assert runner[-4:] == [["("], guarded_cd, go_test, [")"]]
    assert sum(command[:2] == ["go", "test"] for command in runner) == 1
    assert not {"-run", "-skip", "-short"}.intersection(go_test)


def test_module_coverage_profile_initializes_one_fresh_staged_header():
    source, _, top = script_parts()
    top_commands = commands(top)
    assert ["set", "-euo", "pipefail"] in top_commands
    assert 'work_dir=$(mktemp -d "$root_dir/.module-tests.XXXXXX")' in top
    for assignment in (
        'coverage_file="$root_dir/coverage-go-submod-profile.out"',
        'module_profile="$work_dir/module.out"',
        'aggregate_profile="$work_dir/coverage.out"',
        'module_list="$work_dir/modules"',
    ):
        assert assignment in top
    initialize = ["printf", "mode: atomic\\n", ">", "$aggregate_profile"]
    assert top_commands.count(initialize) == 1
    loop = ["while", "IFS=", "read", "-r", "-d", "", "go_mod", ";", "do"]
    assert top_commands.index(initialize) < top_commands.index(loop)
    assert ["trap", 'rm -rf -- "$work_dir"', "EXIT"] in top_commands
    profile_writers = [
        command for command in commands(source)
        if "$aggregate_profile" in command and {">", ">>"}.intersection(command)
    ]
    assert profile_writers == [
        ["tail", "-n", "+2", "--", "$module_profile", ">>",
         "$aggregate_profile", "||", "return", "1"],
        initialize,
    ]


def test_module_coverage_profile_rejects_missing_and_mismatched_headers():
    _, functions, _ = script_parts()
    append = commands(functions["append_coverage_profile"])
    assert append == [
        ["local", "mode"],
        ["if", "!", "IFS=", "read", "-r", "mode", "<", "$module_profile",
         ";", "then"],
        ["echo", "Missing or incomplete coverage header: $module_profile", ">&", "2"],
        ["return", "1"],
        ["fi"],
        ["if", "[[", "$mode", "!=", "mode: atomic", "]]", ";", "then"],
        ["echo", "Unexpected coverage mode: $mode (expected mode: atomic)", ">&", "2"],
        ["return", "1"],
        ["fi"],
        ["tail", "-n", "+2", "--", "$module_profile", ">>",
         "$aggregate_profile", "||", "return", "1"],
    ]


def test_module_coverage_profile_test_and_append_failures_prevent_publication():
    source, _, top = script_parts()
    top_commands = commands(top)
    start = top_commands.index(["return_val=0"])
    assert top_commands[start:] == [
        ["return_val=0"],
        ["while", "IFS=", "read", "-r", "-d", "", "go_mod", ";", "do"],
        ["rm", "-f", "--", "$module_profile"],
        ["if", "execute_mod_tests", "$go_mod", ";", "then"],
        ["if", "!", "append_coverage_profile", ";", "then"],
        ["return_val=2"],
        ["fi"],
        ["else"],
        ["return_val=2"],
        ["fi"],
        ["done", "<", "$module_list"],
        ["if", "[[", "$return_val", "-eq", "0", "]]", ";", "then"],
        ["mv", "--", "$aggregate_profile", "$coverage_file"],
        ["fi"],
        ["exit", "$return_val"],
    ]
    output_users = [command for command in commands(source) if "$coverage_file" in command]
    assert output_users == [["mv", "--", "$aggregate_profile", "$coverage_file"]]


def test_module_runner_documentation_matches_selection_and_publication():
    document = (ROOT / "sdk/scripts/README.md").read_text(encoding="utf-8")
    row = next(line for line in document.splitlines() if "| `module-tests.sh` |" in line)
    for fact in (
        "every nested Go module", "other than `./go.mod`", "regardless of `GIT_DIFF`",
        "dependent modules are tested too", "fresh `coverage-go-submod-profile.out`",
        "one atomic-mode header", "only after all module tests and coverage-mode checks succeed",
        "a failed run preserves the previous profile",
    ):
        assert fact in row


def test_repair_record_maps_both_findings_to_declared_source_proofs():
    record = tomllib.loads(RECORD.read_text(encoding="utf-8"))
    repair = record["repair"]
    assert repair["task"] == "22.33.1360"
    assert repair["requirement"] == "22998.1"
    assert repair["input_revision"] == "04c1e9407e75f7ac92c260452f2232ab07bf3ef1"
    assert repair["scope"] == [
        "sdk/scripts", "tools/qualification/audit_repairs/repair_22_33_1360.kvx",
        "tools/qualification/audit_repairs/test_repair_22_33_1360.py",
    ]
    assert repair["wiring_requests"] == []
    assert "No pre-change verification was run" in repair["baseline"]
    assert "not executed" in repair["review_only"]
    assert repair["build_cmd"] == "bash -n sdk/scripts/module-tests.sh"
    assert repair["verify_cmd"] == (
        "python3 -m pytest -q tools/qualification/audit_repairs/test_repair_22_33_1360.py"
    )
    assert repair["evidence_ledger"] == "/out/run/gates.jsonl"
    assert "same candidate revision" in repair["evidence"]
    audit = tomllib.loads((ROOT / (
        "spec/layerx-merge/programme/audit/records/22.30.74.kvx"
    )).read_text(encoding="utf-8"))
    assert set(record["finding"]) == {"module-dependent-tests", "module-coverage-profile"}
    proved = set()
    for finding_id, finding in record["finding"].items():
        original = audit["finding"][finding_id]
        for key in ("file", "class", "severity", "owner_domain", "expected"):
            assert finding[key] == original[key]
        assert finding["audit"] == "22.30.74"
        assert finding["tests"]
        for name in finding["tests"]:
            assert callable(globals()[name])
        proved.update(finding["tests"])
    declared = {
        node.name for node in ast.parse(Path(__file__).read_text(encoding="utf-8")).body
        if isinstance(node, ast.FunctionDef) and node.name.startswith("test_")
    }
    assert proved == declared - {
        "test_repair_record_maps_both_findings_to_declared_source_proofs"
    }
