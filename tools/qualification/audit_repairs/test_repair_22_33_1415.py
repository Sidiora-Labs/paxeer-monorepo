"""Offline source and production SPDX proofs for audit 22.30.79/039 and /033."""

import ast
import csv
from pathlib import Path
import re
import shlex
import tomllib

import pytest


ROOT = Path(__file__).resolve().parents[3]
SCRIPT = ROOT / "services/platform/tools/dependency-policy.sh"
RECORD = Path(__file__).with_name("repair_22_33_1415.kvx")


@pytest.fixture(scope="module")
def policy_source():
    return SCRIPT.read_text(encoding="utf-8")


@pytest.fixture(scope="module")
def spdx_policy(policy_source):
    match = re.search(
        r"^python3 - \"\$license_file\" <<'SPDX_POLICY'\n(.*?)^SPDX_POLICY$",
        policy_source,
        re.M | re.S,
    )
    assert match is not None
    namespace = {"__name__": "dependency_policy"}
    exec(compile(match[1], str(SCRIPT) + ":SPDX_POLICY", "exec"), namespace)
    return namespace


def test_039_standalone_retired_client_is_in_the_package_wide_ban(policy_source):
    # A standalone Cargo-metadata package has no ramp root or dependency edge.
    metadata = {
        "packages": [{
            "name": "layerx-paxeer-client",
            "id": "path+file:///recorded/sdk/platform/paxeer-client#layerx-paxeer-client@0.1.0",
            "version": "0.1.0",
            "source": None,
            "license": "MIT",
        }],
        "resolve": {"nodes": []},
        "version": 1,
    }
    match = re.search(r"^banned='([^']+)'$", policy_source, re.M)
    assert match is not None
    pattern = re.compile(match[1])
    assert [p["name"] for p in metadata["packages"] if pattern.search(p["name"])] == [
        "layerx-paxeer-client"
    ]
    assert metadata["resolve"]["nodes"] == []
    assert re.search(
        r'1\)\s+cat -- "\$1" >"\$metadata_file"\s+;;', policy_source
    )
    command = re.search(
        r'banned_packages=\$\((.*?)\)\nif \[ -n "\$banned_packages" \]; then\n'
        r'(.*?)\nfi', policy_source, re.S,
    )
    assert command is not None
    assert shlex.split(command[1].replace("\\\n", "")) == [
        "jq", "-r", "--arg", "banned", "$banned",
        '[.packages[].name | select(test($banned))] | join("\\n")',
        "$metadata_file",
    ]
    assert "forbidden boundary or global-initialisation crate" in command[2]
    assert '"$banned_packages" >&2' in command[2]
    assert re.search(r"^    exit 1$", command[2], re.M)
    assert policy_source.index("banned_packages=$(") < policy_source.index(
        "forbidden_ramp_dependencies=$("
    )


@pytest.mark.parametrize("name", [
    "bindgen", "sqlx-sqlite", "layerx-core", "layerx-protocol-core", "ctor", "inventory",
])
def test_039_existing_package_bans_remain(policy_source, name):
    pattern = re.search(r"^banned='([^']+)'$", policy_source, re.M)[1]
    assert re.search(pattern, name)


@pytest.mark.parametrize("name", [
    "layerx-paxeer-client-extra", "prefix-layerx-paxeer-client", "layerx-platform-sdk",
])
def test_039_boundary_ban_is_exact(policy_source, name):
    pattern = re.search(r"^banned='([^']+)'$", policy_source, re.M)[1]
    assert re.search(pattern, name) is None


def test_039_ramp_transitive_constraints_remain_guarded(policy_source):
    block = re.search(
        r"forbidden_ramp_dependencies=\$\(jq -r '(.*?)' \"\$metadata_file\"\)\n"
        r'if \[ -n "\$forbidden_ramp_dependencies" \]; then\n(.*?)\nfi',
        policy_source, re.S,
    )
    assert block is not None
    for fact in (
        "def reachable($nodes):", "{pending: ., seen: {}}",
        ".pending = .pending[1:]", ".seen[$id] = true",
        ".pending += $nodes[$id].dependencies", ".seen | keys[]",
        "INDEX(.packages[]; .id) as $packages", "INDEX(.resolve.nodes[]; .id) as $nodes",
        'select(.name == "layerx-ramp-toolkit" or .name == "layerx-reference-ramp")',
        "[.id] | reachable($nodes)", "$packages[.].name",
        'select(. == "layerx-paxeer-client" or . == "layerx-intents"',
        'or . == "rusqlite" or . == "libsqlite3-sys")',
    ):
        assert fact in block[1], fact
    assert "forbidden ramp dependency" in block[2]
    assert '"$forbidden_ramp_dependencies" >&2' in block[2]
    assert re.search(r"^    exit 1$", block[2], re.M)


def test_033_metadata_license_transport_and_failure_propagation(policy_source, spdx_policy):
    command = re.search(
        r"^jq -r '(.+)' \\\n    \"\$metadata_file\" >\"\$license_file\"$",
        policy_source, re.M,
    )
    assert command is not None
    assert command[1] == (
        '.packages[] | select(.source != null) | [.name, (.license // "")] | @tsv'
    )
    assert policy_source.startswith("#!/bin/sh\nset -eu\n")
    assert 'trap \'rm -f -- "$metadata_file" "$license_file"\' EXIT HUP INT TERM' in policy_source
    for manifest in (
        "services/platform/Cargo.toml", "tools/platform/Cargo.toml",
        "sdk/platform/generators/Cargo.toml",
    ):
        assert f'cargo metadata --manifest-path "$repo_root/{manifest}"' in policy_source
    assert policy_source.count("--locked --format-version 1") == 3
    code = re.search(r"<<'SPDX_POLICY'\n(.*?)^SPDX_POLICY$", policy_source, re.M | re.S)[1]
    tree = ast.parse(code)
    entry = tree.body[-1]
    assert isinstance(entry, ast.If)
    assert ast.unparse(entry.test) == "__name__ == '__main__'"
    assert ast.unparse(entry.body[0]) == "raise SystemExit(check_licenses(sys.argv[1]))"
    assert policy_source.index("SPDX_POLICY\n") < policy_source.index(
        'echo "platform dependency and unsafe policies passed"'
    )
    assert "allowlisted_license_token" not in policy_source
    assert spdx_policy["ALLOWED_LICENSES"] == {
        "Apache-2.0", "BSD-1-Clause", "BSD-2-Clause", "BSD-3-Clause", "CC0-1.0",
        "ISC", "MIT", "Unicode-3.0", "Zlib",
    }
    assert spdx_policy["ALLOWED_EXCEPTIONS"] == {"LLVM-exception"}


@pytest.mark.parametrize("expression,expected", [
    ("MIT", True),
    ("Apache-2.0", True),
    ("GPL-3.0-only", False),
    ("MIT OR GPL-3.0-only", True),
    ("MIT AND GPL-3.0-only", False),
    ("(MIT OR GPL-3.0-only) AND Apache-2.0", True),
    ("Apache-2.0 AND (GPL-3.0-only OR MIT)", True),
    ("(GPL-3.0-only OR MIT) AND (AGPL-3.0-only OR Apache-2.0)", True),
    ("(MIT OR GPL-3.0-only) AND (AGPL-3.0-only OR GPL-2.0-only)", False),
    ("MIT OR GPL-3.0-only AND AGPL-3.0-only", True),
    ("(MIT OR GPL-3.0-only) AND AGPL-3.0-only", False),
    ("GPL-3.0-only AND AGPL-3.0-only OR MIT", True),
    ("GPL-3.0-only AND (AGPL-3.0-only OR MIT)", False),
    ("((MIT OR (GPL-3.0-only AND ISC)) AND Apache-2.0)", True),
    ("Apache-2.0 WITH LLVM-exception", True),
    ("(GPL-3.0-only OR Apache-2.0 WITH LLVM-exception) AND MIT", True),
    ("Apache-2.0 WITH Classpath-exception-2.0", False),
    ("GPL-3.0-only WITH LLVM-exception", False),
    ("MIT OR GPL-3.0-only WITH Classpath-exception-2.0", True),
    ("LLVM-exception", False),
    ("LicenseRef-Private", False),
    ("MIT/Apache-2.0", True),
    ("(GPL-3.0-only/MIT) AND ISC", True),
    ("  (MIT OR GPL-3.0-only)AND(Apache-2.0)  ", True),
])
def test_033_grouped_spdx_choices_use_the_production_consumer(
    spdx_policy, tmp_path, capsys, expression, expected
):
    transport = tmp_path / "licenses.tsv"
    with transport.open("w", encoding="utf-8", newline="") as target:
        csv.writer(target, delimiter="\t", lineterminator="\n").writerow([
            "recorded-dependency", expression,
        ])
    assert spdx_policy["LicenseExpression"](expression).allowed() is expected
    assert spdx_policy["check_licenses"](transport) == (0 if expected else 1)
    output = capsys.readouterr()
    assert output.out == ""
    if expected:
        assert output.err == ""
    else:
        assert output.err == (
            "platform dependency policy: recorded-dependency uses non-allowlisted "
            f"license {expression}\n"
        )


@pytest.mark.parametrize("expression", [
    "MIT OR", "MIT AND", "MIT WITH", "()", "(MIT", "MIT)",
    "MIT OR ()", "MIT OR (Apache-2.0", "MIT OR Apache-2.0 WITH",
    "MIT OR OR GPL-3.0-only", "MIT AND AND Apache-2.0", "MIT Apache-2.0",
    "(MIT OR Apache-2.0) WITH LLVM-exception", "MIT WITH (LLVM-exception)",
    "MIT WITH LLVM-exception WITH LLVM-exception", "WITH LLVM-exception",
    "OR MIT", "MIT | GPL-3.0-only", "MIT || Apache-2.0", "MIT & Apache-2.0",
    '"MIT"', "MIT or Apache-2.0", "MIT OR Apache_2.0", "MIT/", "MIT OR ;",
])
def test_033_malformed_alternatives_fail_closed(spdx_policy, tmp_path, capsys, expression):
    transport = tmp_path / "licenses.tsv"
    transport.write_text(f"bad-dependency\t{expression}\n", encoding="utf-8")
    with pytest.raises(ValueError):
        spdx_policy["LicenseExpression"](expression).allowed()
    assert spdx_policy["check_licenses"](transport) == 1
    assert capsys.readouterr().err == (
        f"platform dependency policy: bad-dependency uses non-allowlisted license {expression}\n"
    )


@pytest.mark.parametrize("expression", ["", "   "])
def test_033_missing_license_fails_closed(spdx_policy, tmp_path, capsys, expression):
    transport = tmp_path / "licenses.tsv"
    transport.write_text(f"unlicensed-dependency\t{expression}\n", encoding="utf-8")
    assert spdx_policy["check_licenses"](transport) == 1
    assert capsys.readouterr().err == (
        "platform dependency policy: unlicensed-dependency has no SPDX license\n"
    )


def test_033_every_external_package_must_have_an_allowed_choice(spdx_policy, tmp_path, capsys):
    transport = tmp_path / "licenses.tsv"
    transport.write_text(
        "first\t(MIT OR GPL-3.0-only) AND Apache-2.0\n"
        "second\tGPL-3.0-only\n"
        "third\tMIT\n",
        encoding="utf-8",
    )
    assert spdx_policy["check_licenses"](transport) == 1
    assert capsys.readouterr().err == (
        "platform dependency policy: second uses non-allowlisted license GPL-3.0-only\n"
    )


def test_repair_record_maps_each_finding_to_executed_tests():
    record = tomllib.loads(RECORD.read_text(encoding="utf-8"))
    tests = {
        node.name for node in ast.parse(Path(__file__).read_text(encoding="utf-8")).body
        if isinstance(node, ast.FunctionDef) and node.name.startswith("test_")
    }
    assert set(record["finding"]) == {"033", "039"}
    for finding_id, finding in record["finding"].items():
        assert finding["audit"] == "22.30.79"
        assert finding["file"] == "services/platform/tools/dependency-policy.sh"
        assert finding["severity"] == "medium"
        assert finding["tests"]
        assert set(finding["tests"]) == {
            name for name in tests if name.startswith(f"test_{finding_id}_")
        }
    task = record["task"]["22"]["33"]["1415"]
    assert task["build_cmd"] == "bash -n services/platform/tools/dependency-policy.sh"
    assert task["verify_cmd"] == (
        "python3 -m pytest -q tools/qualification/audit_repairs/test_repair_22_33_1415.py"
    )
    assert task["evidence_ledger"] == "/out/run/gates.jsonl"
    assert task["wiring_requests"] == []
    assert "No pre-change verification was run" in task["baseline"]
    assert "not executed" in task["review_only"]
