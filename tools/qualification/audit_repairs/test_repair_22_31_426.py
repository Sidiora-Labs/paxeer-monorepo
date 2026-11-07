"""Bind the real Makefile caller and shell adapter to the real source consumer.

The embedded Python producer and consumer run in-process against a committed
spec fixture. Bash supplies syntax parsing only; no build or qualification
target, external service, or source-monitor subprocess runs in this gate.
"""

import ast
import json
from pathlib import Path
import re
import shlex
import subprocess
import sys
import tomllib
from types import SimpleNamespace

import pytest


ROOT = Path(__file__).resolve().parents[3]
HERE = Path(__file__).parent
SCRIPT = ROOT / "scripts/ci/beta-qualify.sh"
CONSUMER = ROOT / "scripts/ci/beta-ledger-check.sh"
FIXTURE = HERE / "focused_tasks_22_31_426.kvx"
RECORD = HERE / "repair_22_31_426.kvx"


def embedded_python(path, delimiter):
    text = path.read_text()
    return text.split("<<'" + delimiter + "'\n", 1)[1].split(
        "\n" + delimiter + "\n", 1
    )[0]


@pytest.fixture
def producer():
    namespace = {"__name__": "focused_task_interface"}
    exec(compile(embedded_python(SCRIPT, "PY_TASK_COMMAND"), str(SCRIPT), "exec"), namespace)
    return namespace["focused_task_command"]


@pytest.fixture
def consumer():
    # Load the actual embedded source consumer, including its argparse setup.
    # Its CLI entrypoint starts at `arguments`; do not run a source monitor or
    # execute a task as part of testing the command-binding interface.
    source = embedded_python(CONSUMER, "PY_SOURCE")
    source = source.split("arguments = sys.argv[1:]", 1)[0]
    namespace = {"__name__": "source_evidence_interface"}
    exec(compile(source, str(CONSUMER), "exec"), namespace)
    return namespace


def make_caller():
    source = (ROOT / "Makefile").read_text()
    match = re.search(r"^beta-qualify-focused:\n((?:\t[^\n]*\n)+)", source, re.M)
    assert match, "the production Makefile must declare beta-qualify-focused"
    body = match[1].replace("\\\n", " ")
    return source, body


def parse_source_run(consumer, argv):
    source = embedded_python(CONSUMER, "PY_SOURCE")
    transport = source.split("arguments = sys.argv[1:]", 1)[1].split("\ntry:\n", 1)[0]
    saved = sys.argv
    try:
        sys.argv = [str(CONSUMER), *argv]
        exec(compile("arguments = sys.argv[1:]" + transport, str(CONSUMER), "exec"), consumer)
        return consumer["args"]
    finally:
        sys.argv = saved


def test_finding_1_makefile_supplies_the_task_bound_cli():
    source, body = make_caller()
    for key, value in {
        "BETA_SPEC": "spec/layerx-merge/spec.kvx",
        "BETA_TASK": "",
        "BETA_LEDGER": "",
        "BETA_EVIDENCE_ROOT": "",
    }.items():
        default = " " + value if value else ""
        assert re.search(r"^" + key + r" \?=" + re.escape(default) + r"$", source, re.M)
    assert body.count("scripts/ci/beta-qualify.sh") == 2
    selection, invocation = body.split(";", 2)[1:]
    assert shlex.split(selection.strip()) == [
        "command=$$(scripts/ci/beta-qualify.sh", "--print-command",
        "$(BETA_SPEC)", "$(BETA_TASK))",
    ]
    assert body.lstrip().startswith("@set -eu;")
    assert shlex.split(invocation) == [
        "scripts/ci/beta-qualify.sh", "--spec", "$(BETA_SPEC)",
        "--task", "$(BETA_TASK)", "--ledger", "$(BETA_LEDGER)",
        "--evidence-root", "$(BETA_EVIDENCE_ROOT)", "--", "$$command",
    ]


def test_finding_1_shell_parses_and_guards_the_source_run():
    for script in (SCRIPT, CONSUMER):
        result = subprocess.run(
            ["bash", "-n", str(script)], capture_output=True, text=True, timeout=10
        )
        assert result.returncode == 0, result.stderr
    source = SCRIPT.read_text()
    assert "set -euo pipefail\numask 077" in source
    guard = source.index('[ -n "$spec" ] && [ -n "$ledger" ]')
    dispatch = source.index('exec bash "$root/scripts/ci/beta-ledger-check.sh"')
    assert guard < source.index("return 2", guard) < dispatch
    assert '[ -n "$directory" ] && [ -n "$task" ] && [ "$#" -gt 0 ]' in source
    assert shlex.split(source[dispatch:source.index("\n}", dispatch)].replace("\\\n", " ")) == [
        "exec", "bash", "$root/scripts/ci/beta-ledger-check.sh", "--source-run",
        "--root", "$root", "--evidence-root", "$directory", "--spec", "$spec",
        "--task", "$task", "--ledger", "$ledger", "--timeout", "$timeout",
        "--evidence-kind", "$kind", "--", "$@",
    ]
    assert 'if [ "${1:-}" = --print-command ]; then\n    shift\n    beta_task_command "$@"' in source
    assert 'else\n    beta_qualify_focused "$@"\nfi' in source
    adapter = CONSUMER.read_text()
    assert 'operation=${1#--source-}\n        shift\n        source_evidence "$operation" "$@"' in adapter


@pytest.mark.parametrize("task", ["426.1", "426.2"])
def test_finding_1_exact_producer_command_reaches_source_run(producer, consumer, tmp_path, task):
    command = producer(FIXTURE, task)
    private = tmp_path / "private evidence"
    private.mkdir(mode=0o700)
    ledger = private / (task + ".kvx")
    # Drive the production argparse transport and recipe reader, preserving
    # the complete command as one argument after --, as the Makefile does.
    argv = ["run", "--root", str(ROOT), "--spec", str(FIXTURE), "--task", task,
            "--ledger", str(ledger), "--evidence-root", str(private), "--", command]
    parsed = parse_source_run(consumer, argv)
    assert parsed.operation == "run"
    assert parsed.task == task
    assert parsed.spec == str(FIXTURE)
    assert parsed.ledger == str(ledger)
    assert parsed.evidence_root == str(private)
    assert parsed.commands == [command]
    assert consumer["private_root"](parsed.evidence_root, ROOT) == private
    expected, reqs = consumer["recipe"](parsed.spec, parsed.task)
    assert parsed.commands == expected
    assert reqs == ["22998.1"]
    task_table = tomllib.loads(FIXTURE.read_text())["task"]
    for part in task.split("."):
        task_table = task_table[part]
    assert command == task_table["verify_cmd"]
    assert re.fullmatch(r"make(?: [A-Za-z0-9_-]+)+", command)
    makefile = (ROOT / "Makefile").read_text()
    for target in command.split()[1:]:
        assert re.search(r"^" + re.escape(target) + r":", makefile, re.M)
    # The real run entry must accept that command binding. An intentionally
    # external ledger then fails the next guard before git/process inspection.
    parsed.ledger = str(tmp_path / "outside.kvx")
    with pytest.raises(consumer["EvidenceError"], match="ledger must be inside private evidence root"):
        consumer["run"](parsed, ROOT, private)


def test_finding_1_further_commands_have_separate_task_bindings(producer, consumer):
    first = producer(FIXTURE, "426.1")
    second = producer(FIXTURE, "426.2")
    assert first != second
    for task, command in (("426.1", first), ("426.2", second)):
        expected, reqs = consumer["recipe"](FIXTURE, task)
        assert expected == [command]
        assert reqs == ["22998.1"]
    for commands in ([first, second], [second], [first + " " + second]):
        args = SimpleNamespace(spec=str(FIXTURE), task="426.1", commands=commands)
        with pytest.raises(consumer["EvidenceError"], match="focused commands differ from selected task spec"):
            consumer["run"](args, ROOT, HERE)


@pytest.mark.parametrize("task, message", [
    ("", "one explicit task ID"),
    ("426.1 426.2", "one explicit task ID"),
    ("426.99", "task absent"),
    ("426.0", "not pending or in progress"),
    ("426.3", "unfinished dependency"),
    ("426.4", "unfinished dependency"),
])
def test_finding_1_ineligible_selection_stops_before_execution(producer, task, message):
    with pytest.raises(ValueError, match=message):
        producer(FIXTURE, task)


@pytest.mark.parametrize("command", [
    "make test-result; make test-protocol", "make  test-result", "make test-result\n",
    "python3 --version", "make -f elsewhere", "make --help", "make -n test-result",
    "make test-result && make test-protocol",
    "make $(touch sentinel)", "", None, [],
])
def test_finding_1_unsupported_commands_fail_without_rewriting(producer, tmp_path, command):
    path = tmp_path / "spec.kvx"
    path.write_text(
        '[task.426.1]\nstatus = "pending"\nrequires = []\nverify_cmd = '
        + json.dumps(command) + "\n"
    )
    with pytest.raises(ValueError, match="exact plain make verify_cmd"):
        producer(path, "426.1")


@pytest.mark.parametrize("suffix", [
    '\n[task.426.1]\nstatus = "pending"\n',
    '\nverify_cmd = "make test-protocol"\n',
])
def test_finding_1_ambiguous_specs_are_rejected(producer, tmp_path, suffix):
    path = tmp_path / "ambiguous.kvx"
    path.write_text(
        '[task.426.1]\nstatus = "pending"\nrequires = []\n'
        'verify_cmd = "make test-result"\n' + suffix
    )
    with pytest.raises(ValueError, match="duplicate spec"):
        producer(path, "426.1")


def test_finding_1_command_cli_emits_only_the_exact_command(capsys):
    saved = sys.argv
    try:
        sys.argv = [str(SCRIPT), str(FIXTURE), "426.2"]
        exec(compile(embedded_python(SCRIPT, "PY_TASK_COMMAND"), str(SCRIPT), "exec"), {
            "__name__": "__main__",
        })
    finally:
        sys.argv = saved
    output = capsys.readouterr()
    assert output.out == "make test-result test-protocol\n"
    assert output.err == ""


@pytest.mark.parametrize("arguments", [[], [str(FIXTURE)], [str(FIXTURE), "426.3"]])
def test_finding_1_command_cli_fails_closed(capsys, arguments):
    saved = sys.argv
    try:
        sys.argv = [str(SCRIPT), *arguments]
        with pytest.raises(SystemExit) as result:
            exec(compile(embedded_python(SCRIPT, "PY_TASK_COMMAND"), str(SCRIPT), "exec"), {
                "__name__": "__main__",
            })
        assert result.value.code == 2
    finally:
        sys.argv = saved
    output = capsys.readouterr()
    assert output.out == ""
    assert output.err.startswith("beta-qualify: ")


def test_finding_1_record_maps_the_finding_to_declared_proofs():
    record = tomllib.loads(RECORD.read_text())
    finding = record["finding"]["1"]
    assert finding["file"] == "Makefile"
    assert finding["audit"] == "22.30.176"
    functions = {
        node.name for node in ast.parse(Path(__file__).read_text()).body
        if isinstance(node, ast.FunctionDef) and node.name.startswith("test_")
    }
    assert set(finding["tests"]) == functions
    assert record["task"]["22"]["31"]["426"]["wiring_requests"] == []
