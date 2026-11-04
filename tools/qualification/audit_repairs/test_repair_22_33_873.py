"""Source-contract and missing-input proofs for the real isolation gate."""

import ast
from pathlib import Path
import re
import shlex
import subprocess
import tomllib

import pytest


ROOT = Path(__file__).resolve().parents[3]
SCRIPT = ROOT / "tests/oracle/lx_oracle_adapter_isolation.sh"
RECORD = Path(__file__).with_name("repair_22_33_873.kvx")
DIRECTORIES = ("modules", "ledger", "state", "protocol")
FORBIDDEN = (
    "^(socket|connect|getaddrinfo|gethostbyname|send|recv|curl_easy_)"
)


def source():
    return SCRIPT.read_text(encoding="utf-8")


def run_isolation(root):
    return subprocess.run(
        ["bash", str(SCRIPT)],
        cwd=root,
        capture_output=True,
        text=True,
        timeout=10,
        check=False,
    )


def test_isolation_shell_syntax_and_retained_object_directories():
    parsed = subprocess.run(
        ["bash", "-n", str(SCRIPT)],
        capture_output=True,
        text=True,
        timeout=10,
        check=False,
    )
    assert parsed.returncode == 0, parsed.stderr
    text = source()
    assert text.startswith("#!/bin/bash\nset -euo pipefail\n")
    loop = re.search(r"^for directory in (.+); do$", text, re.MULTILINE)
    assert loop is not None
    assert shlex.split(loop.group(1)) == list(DIRECTORIES)
    assert 'object_directory="build/obj/src/$directory"' in text
    guard = re.search(
        r'if \[ ! -d "\$object_directory" \]; then\n(.*?)\n    fi',
        text,
        re.DOTALL,
    )
    assert guard is not None
    assert '"$object_directory" >&2' in guard.group(1)
    assert "exit 1" in guard.group(1)
    assert "continue" not in guard.group(1)
    assert guard.end() < text.index('for object in "$object_directory"/**/*.o; do')


def test_isolation_rejects_an_unbuilt_tree(tmp_path):
    result = run_isolation(tmp_path)
    assert result.returncode == 1
    assert result.stdout == ""
    assert result.stderr == "Missing built object directory: build/obj/src/modules\n"


@pytest.mark.parametrize("missing", DIRECTORIES)
def test_isolation_requires_each_declared_directory(tmp_path, missing):
    for directory in DIRECTORIES:
        if directory != missing:
            (tmp_path / "build/obj/src" / directory).mkdir(parents=True)
    result = run_isolation(tmp_path)
    assert result.returncode == 1
    assert result.stdout == ""
    assert result.stderr == (
        f"Missing built object directory: build/obj/src/{missing}\n"
    )


@pytest.mark.parametrize("non_objects", [False, True])
def test_isolation_rejects_zero_inspected_objects(tmp_path, non_objects):
    for directory in DIRECTORIES:
        path = tmp_path / "build/obj/src" / directory
        path.mkdir(parents=True)
        if non_objects:
            (path / "source.c").write_text("int retained;\n", encoding="utf-8")
            (path / "directory.o").mkdir()
            (path / ".nested").mkdir()
            (path / ".nested/README").write_text("No objects.\n", encoding="utf-8")
    result = run_isolation(tmp_path)
    assert result.returncode == 1
    assert result.stdout == ""
    assert result.stderr == "No built objects inspected for oracle adapter isolation\n"


def test_isolation_counts_only_successful_real_nm_inspections():
    text = source()
    assert "shopt -s globstar nullglob dotglob\n" in text
    start = text.index("inspected_objects=0\n")
    scan = text.index('for object in "$object_directory"/**/*.o; do')
    file_guard = text.index('if [ ! -f "$object" ]; then\n            continue\n        fi')
    inspect = text.index('undefined_symbols=$(nm -u "$object")\n')
    increment = text.index("inspected_objects=$((inspected_objects + 1))\n")
    consume = text.index('if printf \'%s\\n\' "$undefined_symbols" |')
    assert start < scan < file_guard < inspect < increment < consume
    assert text.count("nm -u") == 1
    assert text.count("inspected_objects=") == 2
    assert text.rstrip().endswith(
        'if [ "$inspected_objects" -eq 0 ]; then\n'
        "    printf 'No built objects inspected for oracle adapter isolation\\n' >&2\n"
        "    exit 1\nfi"
    )
    assert "|| true" not in text
    assert "set +e" not in text
    assert "2>/dev/null" not in text


def test_isolation_preserves_the_network_symbol_rejection():
    text = source()
    guarded_filter = (
        'if printf \'%s\\n\' "$undefined_symbols" |\n'
        "            awk '{print $NF}' |\n"
        f"            grep -E '{FORBIDDEN}' >/dev/null; then\n"
        "            exit 1\n"
        "        fi"
    )
    assert guarded_filter in text
    assert text.count("grep -E") == 1
    assert "grep -Eq" not in text
    pattern = re.compile(FORBIDDEN)
    for symbol in (
        "socket", "connect", "getaddrinfo", "gethostbyname", "send", "recv",
        "curl_easy_init", "curl_easy_perform",
    ):
        assert pattern.search(symbol)
    for symbol in ("malloc", "free", "memcpy", "lx_socket_count"):
        assert pattern.search(symbol) is None


def test_repair_record_maps_f006_to_proofs_and_evidence():
    record = tomllib.loads(RECORD.read_text(encoding="utf-8"))
    finding = record["finding"]["f006"]
    assert finding["file"] == str(SCRIPT.relative_to(ROOT))
    assert finding["audit"] == "22.30.183"
    assert finding["class"] == "test-gap"
    assert finding["severity"] == "medium"
    declared = {
        node.name
        for node in ast.parse(Path(__file__).read_text(encoding="utf-8")).body
        if isinstance(node, ast.FunctionDef) and node.name.startswith("test_")
    }
    assert set(finding["tests"]) == declared
    task = record["task"]["22"]["33"]["873"]
    assert task["wiring_requests"] == []
    assert task["evidence_ledger"] == "/out/run/gates.jsonl"
    assert task["verify_cmd"] == (
        "python3 -m pytest -q "
        "tools/qualification/audit_repairs/test_repair_22_33_873.py"
    )
    assert task["build_cmd"] == "bash -n tests/oracle/lx_oracle_adapter_isolation.sh"
    assert "same candidate revision" in task["evidence"]
    assert "No pre-change verification was run" in task["baseline"]
    assert "native object isolation" in task["review_only"]
