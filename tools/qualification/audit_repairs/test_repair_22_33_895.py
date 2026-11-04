"""Offline contributor-document source proof for audit 22.30.185.

Read the shipped contributor guide and its real build and qualification peers.
These assertions prove documentation facts, not native execution or consensus.
"""

from pathlib import Path
import re
import tomllib


ROOT = Path(__file__).resolve().parents[3]
GUIDE = (ROOT / "CONTRIBUTING.md").read_text(encoding="utf-8")
RECORD = Path(__file__).with_name("repair_22_33_895.kvx")


def section(heading):
    marker = "## " + heading + "\n"
    assert GUIDE.count(marker) == 1, heading
    return GUIDE.split(marker, 1)[1].split("\n## ", 1)[0]


def test_contributor_daemon_embedded_kernel_and_retirement():
    layout = section("Repository layout")
    for fact in (
        "one Paxeer network",
        "the C17 kernel executes inside `paxd`",
        "the node's execution domain and block proposal path",
        "The former `layerxd` daemon is retired.",
        "A transaction receipt is final once its block is committed.",
        "`daemon/paxd/`",
    ):
        assert fact in layout, fact
    # Every mention must explicitly describe retirement, never a runnable peer.
    mentions = [line for line in GUIDE.splitlines() if "layerxd" in line.lower()]
    assert len(mentions) == 1
    assert "The former `layerxd` daemon is retired." in mentions[0]
    commands = "\n".join(re.findall(r"```sh\n(.*?)\n```", GUIDE, re.S))
    assert "layerxd" not in commands.lower()
    for retired_guidance in ("separate settlement", "make layerxd", "cmd/layerxd"):
        assert retired_guidance not in GUIDE.lower()


def test_contributor_daemon_preserves_native_library_build_boundaries():
    layout = section("Repository layout")
    for fact in (
        "`src/` and `include/`",
        "[`modules/lxphost/`](modules/lxphost/)",
        "`build/liblayerx.a`",
        "`programs/target/release/liblayerx_programs_sandbox.a`",
        "native library build boundaries within the same network",
        "root Go module (`go.mod`, built through `chain.mk`)",
    ):
        assert fact in layout, fact
    host = (ROOT / "modules/lxphost/host.go").read_text(encoding="utf-8")
    flags = re.search(r"^#cgo LDFLAGS: (.+)$", host, re.M)
    assert flags is not None
    for flag in ("-llayerx", "-llayerx_programs_sandbox"):
        assert flag in flags[1].split()
    chain = (ROOT / "chain.mk").read_text(encoding="utf-8")
    archives = re.search(r"^NATIVE_ARCHIVES := (.+)$", chain, re.M)
    assert archives is not None
    assert set(archives[1].split()) == {
        "$(PAXEER_PROJECT_HOME)/build/liblayerx.a",
        "$(PAXEER_PROJECT_HOME)/programs/target/release/liblayerx_programs_sandbox.a",
    }
    gates = section("Gates")
    for command in ("make build", "make test", "make paxeer-build", "make paxeer-test"):
        assert re.search(r"^" + re.escape(command) + r"\s+#", gates, re.M)
    tools = section("Toolchains")
    for fact in ("CGO_ENABLED=1", "OpenSSL", "`make paxeer-build`"):
        assert fact in tools


def test_contributor_daemon_links_merged_network_qualification():
    tools = section("Toolchains")
    gates = section("Gates")
    target = "spec/layerx-merge/docs/plans/test-plan.md"
    assert "merged-network [test plan]" in tools
    assert f"({target}#command-shapes-and-manifest-constraints)" in tools
    assert f"[test plan]({target})" in gates
    assert "Merged-network acceptance coverage, domain ownership and release qualification" in gates
    assert "the active task declares the commands to run" in tools
    assert "Run only the task's declared build and verification commands." in gates
    assert "aggregate gates" in gates
    # Follow the actual contributor links into their committed qualification plan.
    plan = (ROOT / target).read_text(encoding="utf-8")
    for heading in (
        "## Command shapes and manifest constraints",
        "## Aggregate gates and completion",
    ):
        assert heading in plan
    assert "modules/lxphost/host.go" in plan
    assert "kernel static library" in plan
    assert "programs sandbox static library" in plan


def test_contributor_daemon_record_names_proofs_and_evidence_limits():
    record = tomllib.loads(RECORD.read_text(encoding="utf-8"))
    task = record["task"]["22"]["33"]["895"]
    finding = record["finding"]["contributor-daemon"]
    assert finding["audit"] == "22.30.185"
    assert finding["file"] == "CONTRIBUTING.md"
    assert finding["class"] == "retired-reference"
    assert finding["tests"] == [
        "test_contributor_daemon_embedded_kernel_and_retirement",
        "test_contributor_daemon_preserves_native_library_build_boundaries",
        "test_contributor_daemon_links_merged_network_qualification",
        "test_contributor_daemon_record_names_proofs_and_evidence_limits",
    ]
    assert task["wiring_requests"] == []
    assert task["baseline_failures_claimed"] == []
    assert task["base_revision"] == "04c1e9407e75f7ac92c260452f2232ab07bf3ef1"
    assert task["verify_cmd"] == (
        "python3 -m pytest -q tools/qualification/audit_repairs/test_repair_22_33_895.py"
    )
    assert task["evidence_ledger"] == "/out/run/gates.jsonl"
    assert "not executed" in task["review_only"]
