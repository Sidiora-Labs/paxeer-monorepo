"""Offline documentation proof for audit 22.30.128/programs-readme-retired.

The corrected gate checks the real documentation against production sources.
It neither starts a node nor invokes the companion Rust test or a toolchain.
"""

import ast
from pathlib import Path
import re
import tomllib


ROOT = Path(__file__).resolve().parents[3]
PROGRAMS = ROOT / "programs"
REGISTRY = PROGRAMS / "crates/layerx-programs-registry/src"
README = (PROGRAMS / "README.md").read_text(encoding="utf-8")


def source(path):
    return path.read_text(encoding="utf-8")


def prose(text):
    return " ".join(text.split())


def read_bridge():
    marker = "### Registry reads from mounted chain state\n"
    assert README.count(marker) == 1
    return README.split(marker, 1)[1].split("\n---\n", 1)[0]


def test_programs_readme_merged_node():
    overview = prose(README.split("## Workspace layout", 1)[0])
    for fact in (
        "one BFT network: `paxd`, EVM chain ID `125` (`0x7d`)",
        "kernel runs inside the node",
        "participates in block proposal and execution",
        "A receipt is final when its block is committed",
        "programs do not wait for settlement on another chain",
    ):
        assert fact in overview, fact
    assert "app.LayerXHost, err = NewHost(" in source(ROOT / "node/app.go")
    assert 'import "C"' in source(ROOT / "modules/lxphost/host.go")
    assert "same chain-125 network" in prose(read_bridge())
    for stale in (
        r"periodic checkpoints settle",
        r"settles? execution on a separate chain",
        r"two execution domains",
        r"\blayerxd\b",
    ):
        assert not re.search(stale, prose(README), re.IGNORECASE), stale


def test_programs_readme_mounted_registry_proof_path():
    bridge = prose(read_bridge())
    for fact in (
        "The read bridge is **`layerx-programs-registry`**",
        "node mounts the kernel state in the `lxphost` store",
        "`decode_commitment_proof`",
        "`ChainProofVerifier`",
        "trusted chain identity and validator information",
        "untrusted bundle alone does not establish trust",
        "`ics23:iavl` key/value proof",
        "`ics23:simple` multistore proof",
        "authenticated header's `app_hash`",
        "header at `H + 1`; `verify_state` returns `H`",
        "`verify_absent` authenticates non-membership",
        "`protocol_evidence` bind the verified state",
        "`VerifiedProgramCatalog` admits executable programs",
        "execution receipt alone is insufficient",
    ):
        assert fact in bridge, fact

    mount = source(ROOT / "storage/common/keys/store_keys.go")
    assert re.search(r'LayerXHostStoreKey\s*=\s*"lxphost"', mount)
    node = source(ROOT / "node/app.go")
    assert "lxphost.NewStoreAppModule(keys[storekeys.LayerXHostStoreKey])" in node
    assert 'pub const HOST_STORE_NAME: &str = "lxphost";' in source(
        REGISTRY / "chain_evidence.rs"
    )
    codec = source(REGISTRY / "chain_proof/decode.rs")
    assert "pub fn decode_commitment_proof(" in codec
    assert 'reader.take(4)? != b"LXCP"' in codec
    verifier = source(REGISTRY / "chain_proof/head_verify.rs")
    for fact in (
        "self.verify_header_commit(header, commit)?",
        "header.chain_id != self.chain_id",
        "header.validators_hash != self.validators.hash()",
        "verify_zip215(&validator.public_key",
        "has_two_thirds(signed_power, self.validators.total_power())",
    ):
        assert fact in verifier, fact
    state = source(REGISTRY / "chain_proof/state.rs")
    for fact in (
        'ops[0].op_type != "ics23:iavl"',
        'ops[1].op_type != "ics23:simple"',
        "ops[1].key != store_name.as_bytes()",
        "verify_existence(&proof, &ProofSpec::iavl(), &root, key, value)?",
        "verify_store_root(store_name, root, &ops[1].data, next.app_hash())?",
        "verify_non_existence(&proof, &ProofSpec::iavl(), &root, key)?",
        "Ok((next.height() - 1, root))",
    ):
        assert fact in state, fact
    consumer = source(REGISTRY / "protocol_evidence.rs")
    for fact in (
        "validators != verifier.validators",
        "proof.store_name != HOST_STORE_NAME",
        ".verify_head(&header, &commit)",
        "Some(value) => verify_state(",
        "None => verify_absent(",
    ):
        assert fact in consumer, fact
    sandbox = source(PROGRAMS / "crates/layerx-programs-sandbox/src/activity.rs")
    assert 'const SANDBOX_STATE_STORE: &str = "lxphost";' in sandbox
    assert "evidence.commitment.store_name != SANDBOX_STATE_STORE" in sandbox
    assert "pub struct VerifiedProgramCatalog" in source(REGISTRY / "resolver.rs")


def test_programs_readme_inventory_matches_crates():
    manifest = tomllib.loads(source(PROGRAMS / "Cargo.toml"))
    members = {
        member for member in manifest["workspace"]["members"]
        if member.startswith("crates/")
    }
    rows = dict(re.findall(r"^\| `(crates/[^`]+)` \| ([^\n]+) \|$", README, re.MULTILINE))
    assert set(rows) == members
    for member in members:
        package = tomllib.loads(source(PROGRAMS / member / "Cargo.toml"))["package"]
        assert package["name"] == Path(member).name
    assert "Chain-proof read bridge for the mounted `lxphost` store" in rows[
        "crates/layerx-programs-registry"
    ]
    assert "layerx-programs-protocol-adapter" not in " ".join(rows)


def test_programs_readme_adapter_is_retired():
    retirement = [
        paragraph for paragraph in README.split("\n\n")
        if "layerx-programs-protocol-adapter" in paragraph
    ]
    assert len(retirement) == 1
    assert "crate is retired" in retirement[0]
    assert "do not add the retired adapter as a dependency" in retirement[0]
    current = README.replace(retirement[0], "")
    assert "protocol-adapter" not in current
    manifest = tomllib.loads(source(PROGRAMS / "Cargo.toml"))
    assert all("protocol-adapter" not in path for path in manifest["workspace"]["members"])
    for member in [".", *manifest["workspace"]["members"]]:
        cargo = tomllib.loads(source(PROGRAMS / member / "Cargo.toml"))
        for kind in ("dependencies", "dev-dependencies", "build-dependencies"):
            assert "layerx-programs-protocol-adapter" not in cargo.get(kind, {}), member
    lock = tomllib.loads(source(PROGRAMS / "Cargo.lock"))
    assert "layerx-programs-protocol-adapter" not in {
        package["name"] for package in lock["package"]
    }


def test_programs_readme_proof_links_resolve():
    links = re.findall(r"\]\(([^)]+)\)", read_bridge())
    assert set(links) == {
        "../storage/common/keys/store_keys.go",
        "../node/app.go",
        "crates/layerx-programs-registry/src/chain_evidence.rs",
        "crates/layerx-programs-registry/src/chain_proof.rs",
    }
    for link in links:
        assert (PROGRAMS / link).is_file(), link


def test_programs_readme_record_and_rust_companion():
    record = tomllib.loads(source(Path(__file__).with_name("repair_22_33_238.kvx")))
    assert set(record["finding"]) == {"programs-readme-retired"}
    finding = record["finding"]["programs-readme-retired"]
    assert finding["audit"] == "22.30.128"
    assert finding["file"] == "programs/README.md"
    assert finding["class"] == "doc-drift" and finding["severity"] == "medium"
    declared = {
        node.name for node in ast.parse(source(Path(__file__))).body
        if isinstance(node, ast.FunctionDef) and node.name.startswith("test_")
    }
    assert set(finding["tests"]) == declared - {
        "test_programs_readme_record_and_rust_companion"
    }
    task = record["task"]["22"]["33"]["238"]
    assert task["base_revision"] == "e17e9d0ab04b95ee1c18eb85444220f992e70ff1"
    assert task["verify_cmd"] == (
        "python3 -m pytest -q tools/qualification/audit_repairs/test_repair_22_33_238.py"
    )
    assert task["evidence_ledger"] == "/out/run/gates.jsonl"
    assert task["baseline"] and task["proof_scope"] and task["rust_companion"]
    assert task["wiring_requests"] == []

    manifest = tomllib.loads(source(PROGRAMS / "Cargo.toml"))
    assert manifest["package"]["name"] == "layerx-programs-integration-tests"
    assert manifest["package"]["publish"] is False
    assert manifest["test"] == [{
        "name": "repair_22_33_238", "path": "tests/repair_22_33_238.rs"
    }]
    assert manifest["dev-dependencies"]["layerx-programs-registry"]["path"] == (
        "crates/layerx-programs-registry"
    )
    lock = tomllib.loads(source(PROGRAMS / "Cargo.lock"))
    packages = [p for p in lock["package"] if p["name"] == manifest["package"]["name"]]
    assert len(packages) == 1
    assert packages[0]["version"] == manifest["workspace"]["package"]["version"]
    assert set(packages[0]["dependencies"]) == set(manifest["dev-dependencies"])
    rust = source(PROGRAMS / "tests/repair_22_33_238.rs")
    assert set(re.findall(r"#\[test\]\s+fn (\w+)\(", rust)) == set(finding["rust_tests"])
    assert '#[path = "../crates/layerx-programs-registry/tests/support/chain.rs"]' in rust
    for call in (
        "ChainFixture::new(", "fixture.encode_key(", "decode_commitment_proof(",
        "commitment_trust_step(", ".verify_head(", "verify_state(", "verify_absent(",
    ):
        assert call in rust, call
    assert "forged_header.app_hash[0] ^= 1" in rust
    assert 'b"forged"' in rust and '"bank"' in rust
