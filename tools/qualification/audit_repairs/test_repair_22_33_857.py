"""Offline production bootstrap proof; native commit verification is source-only."""

from concurrent.futures import ThreadPoolExecutor
import hashlib
import importlib.util
import os
from pathlib import Path
import shlex
import stat
import tomllib

import pytest


ROOT = Path(__file__).resolve().parents[3]
DIRECTORY = ROOT / "docker/platform-registry"
SCRIPT = (DIRECTORY / "init.sh").read_text()
SPEC = importlib.util.spec_from_file_location("registry_chain_trust", DIRECTORY / "chain-trust.py")
TRUST = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(TRUST)
CHAIN = "paxeer-trust-root-fuzz"


def recorded_root():
    # Committed canonical LXTR corpus also consumed by the native trust-root fuzzer.
    return bytes.fromhex((ROOT / "programs/fuzz/corpus/trust-root/one-validator.hex").read_text())


def material(tmp_path):
    data = recorded_root()
    source = tmp_path / "trust-root"
    source.write_bytes(data)
    source.chmod(0o600)
    destination = tmp_path / "registry-generation"
    destination.mkdir(mode=0o700)
    return source, destination, data, hashlib.sha256(data).hexdigest()


def pin(source, destination, digest, chain=CHAIN):
    return TRUST.pin_trust_root(source, destination, chain, digest)


def test_registry_trust_bootstrap_pins_real_root_and_reopens(tmp_path):
    source, destination, data, digest = material(tmp_path)
    result = pin(source, destination, digest)
    selected = Path(result["directory"])
    assert result == {"directory": str(destination / digest), "generation": digest}
    assert (selected / "trust-root").read_bytes() == data
    assert stat.S_IMODE(selected.stat().st_mode) == 0o700
    assert stat.S_IMODE((selected / "trust-root").stat().st_mode) == 0o400
    assert (selected / "trust-root").stat().st_uid == os.geteuid()
    identity = (selected / "trust-root").stat().st_ino
    # Reopen the module as a fresh bootstrap invocation against the retained store.
    fresh = importlib.util.module_from_spec(SPEC)
    SPEC.loader.exec_module(fresh)
    assert fresh.pin_trust_root(source, destination, CHAIN, digest) == result
    assert (selected / "trust-root").stat().st_ino == identity
    source.write_bytes(b"transport replaced after selection")
    assert (selected / "trust-root").read_bytes() == data
    with pytest.raises(ValueError, match="checkpoint pin"):
        fresh.pin_trust_root(source, destination, CHAIN, digest)
    assert (selected / "trust-root").read_bytes() == data


def test_registry_trust_bootstrap_concurrent_atomic_publication(tmp_path):
    source, destination, data, digest = material(tmp_path)
    with ThreadPoolExecutor(max_workers=4) as pool:
        results = list(pool.map(lambda _: pin(source, destination, digest), range(8)))
    assert all(result == results[0] for result in results)
    assert {p.name for p in destination.iterdir()} == {digest, ".lock"}
    assert (destination / digest / "trust-root").read_bytes() == data


@pytest.mark.parametrize("defect", ["wrong-chain", "wrong-pin", "missing-pin", "uppercase-pin", "retired-history", "truncated", "trailing", "validator-address", "zero-power", "oversize"])
def test_registry_trust_bootstrap_refuses_unbound_or_malformed_material(tmp_path, defect):
    source, destination, data, digest = material(tmp_path)
    chain = CHAIN
    if defect == "wrong-chain":
        chain = "foreign-chain"
    elif defect == "wrong-pin":
        digest = "0" * 64
    elif defect == "missing-pin":
        digest = ""
    elif defect == "uppercase-pin":
        digest = digest.upper()
    else:
        if defect == "retired-history":
            data = b"LayerX/sequencer-trust-history/v1\0"
        elif defect == "truncated":
            data = data[:-1]
        elif defect == "trailing":
            data += b"unexpected"
        elif defect == "validator-address":
            data = data[:-68] + bytes([data[-68] ^ 1]) + data[-67:]
        elif defect == "zero-power":
            data = data[:-8] + bytes(8)
        elif defect == "oversize":
            data = b"x" * (TRUST.MAX_ROOT_BYTES + 1)
        source.write_bytes(data)
        digest = hashlib.sha256(data).hexdigest()
    with pytest.raises(ValueError):
        pin(source, destination, digest, chain)
    assert list(destination.iterdir()) == []


@pytest.mark.parametrize("defect", ["source-link", "source-directory", "public-source", "missing-source", "store-link", "public-store", "lock-link", "lock-directory", "public-lock", "generation-link", "public-generation", "root-link", "corrupt-retained-root", "extra-retained-file"])
def test_registry_trust_bootstrap_refuses_unsafe_or_conflicting_store(tmp_path, defect):
    source, destination, data, digest = material(tmp_path)
    if defect.startswith("generation-") or defect in {"public-generation", "root-link", "corrupt-retained-root", "extra-retained-file"}:
        selected = Path(pin(source, destination, digest)["directory"])
        if defect == "generation-link":
            selected.rename(destination / "original")
            selected.symlink_to(destination / "original", target_is_directory=True)
        elif defect == "public-generation":
            selected.chmod(0o755)
        elif defect == "root-link":
            (selected / "trust-root").unlink()
            (selected / "trust-root").symlink_to(source)
        elif defect == "corrupt-retained-root":
            (selected / "trust-root").chmod(0o600)
            (selected / "trust-root").write_bytes(b"corrupt")
        else:
            (selected / "unexpected").write_bytes(data)
    elif defect == "source-link":
        original = tmp_path / "original"
        source.rename(original)
        source.symlink_to(original)
    elif defect == "source-directory":
        source.unlink()
        source.mkdir()
    elif defect == "public-source":
        source.chmod(0o644)
    elif defect == "missing-source":
        source.unlink()
    elif defect == "store-link":
        destination.rename(tmp_path / "store")
        destination.symlink_to(tmp_path / "store", target_is_directory=True)
    elif defect == "public-store":
        destination.chmod(0o755)
    elif defect == "lock-link":
        (destination / ".lock").symlink_to(source)
    elif defect == "lock-directory":
        (destination / ".lock").mkdir()
    elif defect == "public-lock":
        (destination / ".lock").write_text("")
        (destination / ".lock").chmod(0o644)
    with pytest.raises((ValueError, OSError)):
        pin(source, destination, digest)


def test_registry_trust_bootstrap_guards_producer_transport_consumer():
    assert "verify_kernel_material" not in SCRIPT
    assert "--verify-registry-material" not in SCRIPT
    assert "--validate-kernel-material" not in SCRIPT
    assert "/data/kernel" not in SCRIPT
    assert "LAYERX_REGISTRY_KERNEL_GENERATION" not in SCRIPT
    assert "kernel.paxeer.network" not in SCRIPT
    assert 'wait_for "committed paxd checkpoint provisioned by the chain deployment owner" "$LAYERX_REGISTRY_DEPLOYMENT_TRUST_ROOT"' in SCRIPT
    command = SCRIPT[SCRIPT.index("generation=$(python3"):SCRIPT.index("selected=$(printf")]
    assert shlex.split(command.split(") || {", 1)[0].removeprefix("generation=$(").replace("\\\n", "")) == [
        "python3", "/usr/local/lib/layerx-registry/chain-trust.py",
        "$LAYERX_REGISTRY_DEPLOYMENT_TRUST_ROOT", "$run/registry-generation",
        "$LAYERX_REGISTRY_CHAIN_ID", "$LAYERX_REGISTRY_DEPLOYMENT_TRUST_SHA256",
        "--uid", "4030", "--gid", "4030",
    ]
    assert 'missing "chain-bound-committed-deployment-trust" "committed-paxd-deployment-material"; exit 1;' in command
    assert 'install -d -o 4030 -g 4030 -m 0700 "$run/registry-generation"' in SCRIPT
    assert 'LAYERX_REGISTRY_SEQUENCER_TRUST_HISTORY="$selected/trust-root"\nexport LAYERX_REGISTRY_SEQUENCER_TRUST_HISTORY' in SCRIPT
    assert SCRIPT.count("LAYERX_REGISTRY_SEQUENCER_TRUST_HISTORY=") == 1
    assert 'export LAYERX_REGISTRY_DEPLOYMENT_GENERATION' in SCRIPT
    assert "nonzero-32-byte-receipt-authority-id" in SCRIPT
    assert SCRIPT.index("generation=$(python3") < SCRIPT.index("exec unshare")
    docker = [shlex.split(line) for line in (DIRECTORY / "Dockerfile").read_text().splitlines() if line.startswith("COPY ")]
    assert ["COPY", "--chmod=0444", "docker/platform-registry/chain-trust.py", "/usr/local/lib/layerx-registry/chain-trust.py"] in docker
    assert not any("/usr/local/lib/layerx-human/" in line for line in docker)
    journal = (ROOT / "services/platform/hosted/registry/src/journal.rs").read_text()
    main = (ROOT / "services/platform/hosted/registry/src/main.rs").read_text()
    routes = (ROOT / "services/platform/hosted/registry/src/routes.rs").read_text()
    assert 'env::var("LAYERX_REGISTRY_SEQUENCER_TRUST_HISTORY")' in main
    assert "crate::journal::persisted_chain_trust(\n        &config.sequencer_trust_history," in routes
    assert "TrustRoot::decode(&bytes)" in journal
    assert "ProtocolDeploymentVerifier::from_chain_trust(" in journal
    assert ".verify_head(&header, &commit)" in journal
    assert "commitment.signed_header" in journal and "commitment.commit" in journal
    assert "before.permissions().mode() & 0o077 != 0" in journal
    assert "LXTR" in (ROOT / "programs/crates/layerx-programs-registry/src/chain_proof/trust_store.rs").read_text()


def test_registry_trust_bootstrap_deployment_contract_and_record():
    env = dict(line.split("=", 1) for line in (DIRECTORY / "registry.env.example").read_text().splitlines() if line and not line.startswith("#"))
    assert env["LAYERX_REGISTRY_DEPLOYMENT_TRUST_ROOT"] == "/data/chain/trust-root"
    assert env["LAYERX_REGISTRY_DEPLOYMENT_TRUST_SHA256"] == "<authenticated-trust-root-sha256>"
    assert env["LAYERX_REGISTRY_CHAIN_ID"] == "<committed-paxd-chain-id>"
    assert env["LAYERX_REGISTRY_CHAIN_RPC_URL"] == "https://<paxd-rpc>"
    assert env["LAYERX_REGISTRY_NODE_ENDPOINT"] == "https://<chain-state-service>"
    assert env["LAYERX_REGISTRY_RECEIPT_AUTHORITY_ENDPOINT"] == "https://<chain-receipt-service>"
    assert "LAYERX_REGISTRY_RECEIPT_AUTHORITY_REPLICA_ID" in env
    assert "LAYERX_REGISTRY_SEQUENCER_TRUST_HISTORY" not in env
    service = (DIRECTORY / "layerx-registry.service").read_text()
    assert "--env-file /etc/layerx/registry.env" in service
    assert "-v /var/lib/layerx/registry:/data" in service
    record = tomllib.loads(Path(__file__).with_name("repair_22_33_857.kvx").read_text())
    finding = record["finding"]["registry-trust-bootstrap"]
    assert finding["audit"] == "22.30.182"
    assert finding["tests"] == [name for name in globals() if name.startswith("test_registry_trust_bootstrap_")]
    assert record["task"]["22"]["33"]["857"]["baseline"].startswith("Not run")
    assert record["wiring"]["chain-material"]["path"] == "services/platform/hosted/human/material.py"
    assert record["wiring"]["deployment"]["path"] == "tools/bringup/check-live.sh"
