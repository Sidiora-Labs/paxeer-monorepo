"""Focused proofs for audit 22.30.171 findings 001, 002 and 003."""

import ast
from pathlib import Path
import re
import shutil

import pytest

import test_agent_codec_corpus as codec_checks
import test_disposition_budget_pull_removed as disposition_checks
import test_retired_make_rules as make_checks


ROOT = Path(__file__).resolve().parents[2]


def test_finding_001_retained_eip1559_producer_transport_consumer():
    codec_checks.test_retained_eip1559_producer_consumer_conformance()


def test_finding_001_preserves_corpus_rejections_and_budget_pull_exclusion():
    tree = ast.parse(Path(codec_checks.__file__).read_text())
    preparation, = [
        node for node in tree.body if isinstance(node, ast.FunctionDef)
        and node.name == "test_agent_vectors_prepare_codec_corpus_from_clean_checkout"
    ]
    strings = {
        node.value for node in ast.walk(preparation)
        if isinstance(node, ast.Constant) and isinstance(node.value, str)
    }
    assert {
        "repository_corpora_are_loaded_and_fully_accounted",
        "unsupported_corpus_version_is_build_breaking",
        "unknown_vector_class_names_the_missing_capability",
        "retired budget pull excluded",
        "unexpected undeclared replay activities: [196616, 196619]",
    } <= strings
    assert not any("activity::replay_registry_tests" in value for value in strings)
    assert any(
        isinstance(node, ast.Call) and isinstance(node.func, ast.Name)
        and node.func.id == "test_retained_eip1559_producer_consumer_conformance"
        for node in ast.walk(preparation)
    )


def test_finding_002_retained_protocol_and_budget_sources_are_clean():
    disposition_checks.test_no_queue_disposition_codec_case()
    disposition_checks.test_no_budget_pull_request_case()


@pytest.fixture
def protocol_checkout(tmp_path):
    shutil.copytree(ROOT / "tests/protocol", tmp_path / "tests/protocol")
    return tmp_path


@pytest.mark.parametrize(
    "symbol",
    [
        "two_class_disposition_case",
        "lxp_queue_disposition",
        "lxp_queue_disposition_encode",
        "LXP_QUEUE_DISPOSITION_ACCEPTED",
    ],
)
@pytest.mark.parametrize("relative", ["regression.c", "nested/regression.h"])
def test_finding_002_rejects_disposition_in_retained_sources(
    protocol_checkout, symbol, relative
):
    source = protocol_checkout / "tests/protocol" / relative
    source.parent.mkdir(parents=True, exist_ok=True)
    source.write_text(f"extern int {symbol};\n")
    with pytest.raises(AssertionError, match=re.escape(symbol)) as failure:
        disposition_checks.assert_no_queue_disposition(protocol_checkout)
    assert f"tests/protocol/{relative}" in str(failure.value)


@pytest.mark.parametrize("dangling_link", [False, True])
def test_finding_002_rejects_retired_suite_even_as_dangling_link(
    protocol_checkout, dangling_link
):
    retired = protocol_checkout / "tests/protocol/lxp_test_terminal_rejection.c"
    if dangling_link:
        retired.symlink_to("missing-terminal-suite.c")
    else:
        retired.write_text("int retired_terminal_suite;\n")
    with pytest.raises(AssertionError, match="Retired terminal rejection suite remains"):
        disposition_checks.assert_no_queue_disposition(protocol_checkout)


def test_finding_002_refuses_empty_protocol_inventory(tmp_path):
    (tmp_path / "tests/protocol").mkdir(parents=True)
    with pytest.raises(AssertionError, match="No protocol test sources found"):
        disposition_checks.assert_no_queue_disposition(tmp_path)


def test_finding_003_builds_then_executes_actual_differential_target():
    # Inspect the native CI entrypoint without invoking a subprocess toolchain
    # in the Python-only repair gate. Native execution belongs to that entrypoint.
    tree = ast.parse(Path(make_checks.__file__).read_text())
    function, = [
        node for node in tree.body if isinstance(node, ast.FunctionDef)
        and node.name == "test_parallel_differential_links_without_batch_wal"
    ]
    statements = [ast.dump(node) for node in function.body]
    required = ast.parse(
        'target = "build/tests/programs_parallel_differential"\n'
        'built = run_make(target)\n'
        'assert built.returncode == 0, built.stdout + built.stderr\n'
        'executed = subprocess.run(\n'
        '    [str(ROOT / target)], cwd=ROOT,\n'
        '    env={**os.environ, "LC_ALL": "C"},\n'
        '    capture_output=True, text=True, check=False,\n'
        ')\n'
        'output = executed.stdout + executed.stderr\n'
        'assert executed.returncode == 0, output\n'
    ).body
    positions = [statements.index(ast.dump(node)) for node in required]
    assert positions == sorted(positions)
    assert len(set(positions)) == len(required)
    assert ast.dump(function.body[0]) == ast.dump(
        ast.parse("test_parallel_differential_rule_omits_batch_wal()").body[0]
    )
    loop, = [node for node in function.body if isinstance(node, ast.For)]
    assert ast.literal_eval(loop.iter) == ("low-conflict", "all-conflicting")
    check, = loop.body
    assert isinstance(check, ast.Assert)
    assert isinstance(check.test, ast.Call)
    assert ast.unparse(check.test.func) == "re.search"
    assert ast.unparse(check.test.args[1]) == "output"
    assert ast.unparse(check.test.args[2]) == "re.M"
    assert isinstance(check.test.args[0], ast.JoinedStr)
    prefix, workload, suffix = check.test.args[0].values
    assert prefix.value == "^program batch differential workload="
    assert ast.unparse(workload.value) == "workload"
    assert suffix.value == (
        " activities=32 levels=[1-9][0-9]* workers=8 "
        "serial_ns=[0-9]+ parallel_ns=[0-9]+$"
    )
    producer = (ROOT / "programs/tests/differential/parallel.c").read_text()
    assert "DIFFERENTIAL_BATCH_SIZE = 32, DIFFERENTIAL_WORKERS = 8" in producer
    assert '"program batch differential workload=%s activities=%u levels=%zu workers=%u serial_ns=%llu parallel_ns=%llu\\n"' in producer
    assert '"low-conflict"' in producer and '"all-conflicting"' in producer
    makefile = (ROOT / "Makefile").read_text()
    rule = re.search(
        r"^\$\(BUILD_DIR\)/tests/programs_parallel_differential:.*?(?=\n\n)",
        makefile, re.M | re.S,
    )
    assert rule is not None
    assert "programs/tests/differential/parallel.c" in rule.group()
    assert "lxp_daemon_batch_wal" not in rule.group()
