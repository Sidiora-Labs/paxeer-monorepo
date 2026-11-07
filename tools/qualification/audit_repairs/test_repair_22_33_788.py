"""Offline production-source proofs for audit 22.30.176 findings 7, 6, 5, 23, 22.

Bash is used only for the required syntax parse. No compiler, Make, Cargo,
Binaryen, node process, deployment, release or external service runs here.
"""

import ast
import hashlib
import json
from pathlib import Path
import re
import shlex
import subprocess
import tomllib

import pytest


ROOT = Path(__file__).resolve().parents[3]
MAKEFILE = (ROOT / "Makefile").read_text(encoding="utf-8")
RELEASE_SCRIPT = ROOT / "tools/release/build-paxd.sh"
RECORD = Path(__file__).with_name("repair_22_33_788.kvx")


def read(path):
    return (ROOT / path).read_text(encoding="utf-8")


def make_rule(target):
    """Read a real target's prerequisites and logical recipe lines without Make."""
    lines = MAKEFILE.replace("\\\n", " ").splitlines()
    headers = [i for i, line in enumerate(lines) if line.startswith(target + ":")]
    assert len(headers) == 1, target
    index = headers[0]
    dependencies = lines[index].split(":", 1)[1].split()
    commands = []
    for line in lines[index + 1:]:
        if not line.startswith("\t"):
            break
        commands.append(line.strip().removeprefix("@"))
    assert commands, target
    return dependencies, commands


def test_finding_7_guest_size_profile_and_required_binaryen_pass():
    dependencies, commands = make_rule("paxai-host-boundary-build")
    assert dependencies == ["$(BUILD_DIR)/tests/paxai_host_boundary"]
    assert commands[0] == "rm -f $(PAXAI_HOST_BOUNDARY_DIR)/manifest"
    tool_guard = commands[2]
    assert tool_guard.startswith("command -v wasm-opt >/dev/null 2>&1 || {")
    assert "wasm-opt is required" in tool_guard and "exit 1; }" in tool_guard
    assert "|| true" not in tool_guard
    cargo = shlex.split(commands[3])
    assert cargo == [
        "cd", "programs", "&&", "PAXAI_CHAIN_DOMAIN=$(PAXAI_CHAIN_DOMAIN)",
        "$(PROGRAMS_CARGO)", "build", "--locked", "--release", "--config",
        'profile.release.opt-level="z"', "--target", "wasm32-unknown-unknown",
        "-p", "layerx-programs-ai-market",
    ]
    # The explicit override must beat the workspace's throughput profile.
    workspace = tomllib.loads(read("programs/Cargo.toml"))
    assert workspace["profile"]["release"]["opt-level"] == 3
    optimizer = shlex.split(commands[5])
    assert optimizer == [
        "wasm-opt", "-Oz", "--strip-debug", "--strip-producers",
        "$(if", "$(CARGO_TARGET_DIR),$(CARGO_TARGET_DIR),programs/target)/"
        "wasm32-unknown-unknown/release/layerx_programs_ai_market.wasm",
        "-o", "$(PAXAI_HOST_BOUNDARY_DIR)/ai_market.wasm",
    ]
    assert not any(command.startswith("cp ") for command in commands)


def test_finding_7_final_bytes_are_bounded_deployed_and_then_published():
    _, commands = make_rule("paxai-host-boundary-build")
    final_wasm = "$(PAXAI_HOST_BOUNDARY_DIR)/ai_market.wasm"
    guard = commands[6]
    assert guard.startswith(f'test "$$(wc -c < {final_wasm})" -le 524184 || {{')
    assert "exit 1; }" in guard and "|| true" not in guard
    assert 524288 - 104 == 524184
    assert commands[7] == (
        "$(RUN_PREFIX) $(BUILD_DIR)/tests/paxai_host_boundary "
        + final_wasm + " < /dev/null"
    )
    assert commands[8] == (
        "printf '%s\\n%s\\n%s\\n' "
        "'$(abspath $(PAXAI_HOST_BOUNDARY_DIR)/ai_market.wasm)' "
        "'$(abspath $(BUILD_DIR)/tests/paxai_host_boundary)' "
        "'$(PAXAI_CHAIN_DOMAIN)' > $(PAXAI_HOST_BOUNDARY_DIR)/manifest"
    )
    assert len(commands) == 9
    expected = hashlib.sha256(b"PAXAI/chain-domain/v1" + (125).to_bytes(8, "big")).hexdigest()
    assert re.search(r"^PAXAI_CHAIN_DOMAIN \?= " + expected + r"$", MAKEFILE, re.M)
    assert commands[1] == (
        "printf '%s\\n' '$(PAXAI_CHAIN_DOMAIN)' | LC_ALL=C grep -Eq '^[0-9a-f]{64}$$'"
    )
    assert MAKEFILE.count("$(PAXAI_CHAIN_DOMAIN)") == 3
    # Trace the unchanged consumers of the same manifest and final artifact.
    consumer = read("programs/crates/layerx-programs-ai-market/tests/host_boundary.rs")
    for fact in (
        "let [wasm, binary, chain] = lines[..]", "wasm: wasm.to_owned()",
        "harness: binary.to_owned()", "chain: fixed(&unhex(chain)?)?",
        "Command::new(&m.harness)", ".arg(&m.wasm)",
        "assert_eq!(s.header.deployment_chain_domain, n.chain)",
    ):
        assert fact in consumer
    harness = read("tests/programs/test_ai_host_boundary.c")
    for fact in (
        'FILE *artifact = fopen(path, "rb")',
        "fread(payload+104U, 1U, length, artifact) != length",
        "lxp_hash_sha256(payload+104U, length, payload+68U)",
        "execute(f, 0U, LX_PROGRAMS_DEPLOY, payload, length+104U)",
        "execute(f, 0U, LX_PROGRAMS_UPGRADE, upgrade, length+106U)",
        "deploy(f, argv[1]) != 0", "native bring-up failed", "return 2;",
    ):
        assert fact in harness
    assert "LXP_MAX_ACTIVITY_BYTES-106U" in harness


@pytest.mark.parametrize("binary,source", [
    ("lxp_test_blob_admission", "test_blob_admission.c"),
    ("lxp_test_paxai_native_gaps", "test_paxai_native_gaps.c"),
])
def test_finding_6_retained_kernel_linkage_preserves_consumers(binary, source):
    dependencies, commands = make_rule("$(BUILD_DIR)/tests/" + binary)
    assert dependencies == [
        "tests/programs/" + source, "$(LIBRARY)", "$(PROGRAMS_RUNTIME_LIB)",
        "|", "programs-build",
    ]
    assert commands[0] == "mkdir -p $(@D)"
    assert shlex.split(commands[1]) == [
        "$(CC)", "$(CPPFLAGS)", "$(CFLAGS)", "$<", "$(LIBRARY)",
        "$(PROGRAMS_RUNTIME_LIB)", "$(EXTRA_LDFLAGS)", "$(PROGRAMS_NATIVE_LDLIBS)",
        "-lcrypto", "-lsqlite3", "-pthread", "-ldl", "-lm", "-o", "$@",
    ]
    assert len(commands) == 2
    assert "LAYERXD_SOURCES" not in MAKEFILE and "-Icmd/layerxd" not in MAKEFILE
    code = read("tests/programs/" + source)
    assert "lxp_daemon" not in code and '"runtime.h"' not in code
    includes = re.findall(r'^#include "([^"]+)"', code, re.M)
    assert includes
    for header in includes:
        assert (ROOT / "include" / header).is_file(), header
    if source == "test_blob_admission.c":
        for fact in (
            "lxp_module_ctx_prepare_commit(&fx.ctx)",
            'CHECK(status == LXP_ERR_ARENA_EXHAUSTED, "native_prepare_refuses_addition")',
            'CHECK(fx.kernel.blob_count == committed_blobs, "nothing_deleted")',
            'return pass("full_live_store_refused")',
        ):
            assert fact in code
    else:
        for fact in (
            "CHECK(status == LXP_ERR_AUTH_SCOPE, name)",
            "primary->receipt.result_code == LXP_ERR_TRUNCATED",
            "primary->receipt.result_code == LXP_ERR_NOT_YET_VALID",
            "case_module5_retirement()", "case_module_pin()",
        ):
            assert fact in code


def test_finding_5_orphan_archive_build_surface_is_removed():
    assert "layerx-archive-codec" not in MAKEFILE
    assert "ARCHIVE_CODEC_" not in MAKEFILE
    assert not (ROOT / "cmd/layerx-archive-codec").exists()
    for retained in ("layerx-module-registry:", "test-module-registry:", "paxd-build:"):
        assert retained in MAKEFILE
    assert "include tools/paxeer-x/build/104.35.11.mk" in MAKEFILE
    assert "include tools/paxeer-x/build/6.6.mk" in MAKEFILE
    assert "include tools/paxeer-x/build/proof.mk" in MAKEFILE


def test_finding_23_readme_matches_embedded_network_and_precompile_producer():
    readme = read("README.md")
    for fact in (
        "one BFT network with EVM chain ID **125** (`0x7d`)",
        "LayerX kernel is embedded in the node", "proposes blocks inside `paxd`",
        "A receipt is final once its block is committed", "root path of these hosts",
        "no separate kernel daemon, sequencer service or checkpoint deployment",
        "`submitted`, `receipted` and `committed`", "make paxd-build",
        './build/paxd start --home "$PAXD_HOME"',
        "tools/release/build-paxd.sh", "[RELEASE.md](RELEASE.md)",
        "programs/target/release/liblayerx_programs_sandbox.a",
        "retired refusal surfaces retained for compatibility",
        "Historical anchor reads before store deletion are archive-only",
    ):
        assert fact in readme
    assert not re.search(r"\blayerxd\b|make\s+layerxd|merge remains a draft", readme, re.I)
    producer = read("precompiles/setup.go").split("var MergeAddressTable = []MergeAddress{", 1)[1].split("\n}", 1)[0]
    addresses = re.findall(r'HexToAddress\("0x0*([0-9A-Fa-f]+)"\), "([^"]+)"', producer)
    rows = re.findall(r"^\| `(0x[0-9A-Fa-f]+)` \| `([^`]+)` \|$", readme, re.M)
    assert len(addresses) == len(rows) == 13
    assert {(int(address, 16), name) for address, name in addresses} == {
        (int(address, 16), name) for address, name in rows
    }


def test_finding_22_release_script_syntax_guards_and_merged_artifacts():
    parsed = subprocess.run(
        ["bash", "-n", str(RELEASE_SCRIPT)], capture_output=True, text=True, check=False,
    )
    assert parsed.returncode == 0, parsed.stderr
    script = RELEASE_SCRIPT.read_text(encoding="utf-8")
    legacy_guard = re.search(
        r'^if \[ "\$\{PAXD_RELEASE_LAYERXD:-0\}" != 0 \]; then\n(.*?)\nfi',
        script, re.M | re.S,
    )
    assert legacy_guard and "exit 2" in legacy_guard[1]
    assert "is retired; release the merged paxd package" in legacy_guard[1]
    assert script.index(legacy_guard[0]) < script.index("have_go=$(go env GOVERSION)")
    assert script.index(legacy_guard[0]) < script.index('rm -rf -- "$out"')
    assert not re.search(r"\blayerxd\b", script)
    for fact in (
        "version=$(jq -er '.version' tools/release/version.json)",
        'tag="paxeer-network/$version"', 'out=${PAXD_DIST_DIR:-$root/dist/$tag}',
        'export GOTOOLCHAIN=local', 'if [ "$have_go" != "$want_go" ]; then',
        'rust_pin\ncc_pin', 'export CGO_ENABLED=1 CC=gcc CXX=g++',
        'export GOFLAGS="-mod=readonly"', 'export LC_ALL=C TZ=UTC',
        'native_libs=(build/liblayerx.a programs/target/release/liblayerx_programs_sandbox.a)',
        'paxd-native-libs', 'sha256sum "${native_libs[@]}" > "$out/native-libs.sha256"',
        'go build -trimpath -buildvcs=false -tags "netgo ledger"',
        '-o "$out/$paxd" ./daemon/paxd',
        'if [ "$goos/$goarch" = linux/amd64 ]; then',
        'if [ "${#expected}" -ne 64 ]; then',
        'sha256sum --check --strict --quiet',
        '-p layerx-oracle-feeder --bin price-sidecar',
        '"$out/price-sidecar-$version-$goos-$goarch"',
        'python3 tools/release/upgrade_info.py release --sums "$out/SHA256SUMS"',
    ):
        assert fact in script
    manifest = json.loads(read("tools/release/version.json"))
    assert set(manifest) == {"version", "upgrade"}
    for value in manifest.values():
        assert re.fullmatch(r"v\d+\.\d+\.\d+", value)
    guide = read("RELEASE.md")
    for key, value in manifest.items():
        assert f"`{key}`" in guide and f"`{value}`" in guide
    for name in ("libwasmvm.x86_64.so", "libwasmvm152.x86_64.so", "libwasmvm155.x86_64.so"):
        assert name in script and name in guide


def test_finding_22_guide_matches_builder_verification_and_recovery():
    guide = read("RELEASE.md")
    for fact in (
        "`PAXD_DIST_DIR`", "`PAXD_COMMIT`", "`SOURCE_DATE_EPOCH`",
        "no separate kernel-daemon artifact", "nonzero legacy `PAXD_RELEASE_LAYERXD` setting is refused",
        "paxd-vX.Y.Z-<os>-<arch>", "price-sidecar-vX.Y.Z-<os>-<arch>",
        "`native-libs.sha256`", "`upgrade-info.json`", "`SHA256SUMS`",
        "tools/release/verify-release.sh --tag", "sha256sum --check SHA256SUMS",
        "A receipt is final when its block is\ncommitted", "EVM chain ID is `125`",
        "refuses a same-chain snapshot restore", "`ROLLBACK_ISOLATED_FORK=true`",
        "rollback.sh --env-file <file> --forward --execute", "`RECOVERY_BIN`", "`RECOVERY_SHA256`",
        "journals the validator signing high-water mark", "`--forward --verify`",
        "still enable the retired daemon option", "refused by the builder",
    ):
        assert fact in guide
    assert "PAXD_RELEASE_LAYERXD=1" not in guide
    assert "opt-in branch" not in guide and "layerxd-v" not in guide
    commands = [
        shlex.split(line, comments=True)
        for block in re.findall(r"```sh\n(.*?)\n```", guide, re.S)
        for line in block.splitlines() if line.strip()
    ]
    assert ["tools/release/build-paxd.sh"] in commands
    assert ["tools/release/reproducible-check.sh"] in commands
    assert not any("PAXD_RELEASE_LAYERXD" in token for cmd in commands for token in cmd)


def test_record_maps_every_finding_to_declared_source_proofs_and_wiring():
    record = tomllib.loads(RECORD.read_text(encoding="utf-8"))
    assert set(record["finding"]) == {"7", "6", "5", "23", "22"}
    declared = {
        node.name for node in ast.parse(Path(__file__).read_text(encoding="utf-8")).body
        if isinstance(node, ast.FunctionDef) and node.name.startswith("test_")
    }
    for finding in record["finding"].values():
        assert finding["audit"] == "22.30.176"
        assert finding["tests"] and set(finding["tests"]) <= declared
        assert finding["file"] in {"Makefile", "README.md", "RELEASE.md"}
    task = record["task"]
    assert task["base_revision"] == "2a8418f3ce46275830d68ea57514c30466a3493f"
    assert "not run" in task["baseline"]
    assert task["evidence_ledger"] == "/out/run/gates.jsonl"
    assert task["build_cmd"] == "python3 --version"
    assert task["verify_cmd"] == "python3 -m pytest -q tools/qualification/audit_repairs/test_repair_22_33_788.py"
    assert "not executed" in task["review_only"]
    assert "106-byte" in task["review_only"]
    requests = "\n".join(task["wiring_requests"])
    for path in (
        "spec/layerx-merge/interfaces.kvx", ".github/workflows/paxeer-release-check.yml",
        ".github/workflows/paxeer-release-publish.yml", "tools/release/tests/build.test.sh",
        "tests/programs/test_ai_host_boundary.c",
    ):
        assert path in requests
    assert "domain 16" in requests and "domain 5" in requests
