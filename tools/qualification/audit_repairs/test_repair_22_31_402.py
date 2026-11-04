"""Attestor-manifest source checks; these do not execute the Go loader.

Passing the data checks establishes agreement on explicit pending signer slots.
Governance approval and execution of LoadAttestorManifest/Check remain required
before deployment; the production loader refuses the pending manifest.
"""

import json
from pathlib import Path
import re

import pytest


ROOT = Path(__file__).resolve().parents[3]
DEPLOY = ROOT / "services/bridge/deploy"
MANIFEST = DEPLOY / "attestors.json"
CHAIN_CONFIGS = (
    "contracts/bridge/evm/chains/ethereum/config.json",
    "contracts/bridge/evm/chains/base/config.json",
    "contracts/bridge/evm/chains/arbitrum/config.json",
    "contracts/bridge/evm/chains/optimism/config.json",
    "contracts/bridge/evm/chains/bnb/config.json",
    "contracts/bridge/evm/chains/polygon/config.json",
    "contracts/bridge/evm/chains/avalanche/config.json",
    "contracts/bridge/evm/chains/hyperevm/config.json",
    "contracts/bridge/solana/chains/solana/config.json",
)


def read_json(path):
    def unique_fields(pairs):
        result = {}
        for key, value in pairs:
            assert key not in result, f"{path}: duplicate JSON field {key}"
            result[key] = value
        return result

    return json.loads(path.read_text(), object_pairs_hook=unique_fields)


def go_declaration(path, name):
    source = path.read_text()
    declaration = re.search(
        rf"^func (?:\([^\n]*\) )?{re.escape(name)}\(", source, re.MULTILINE
    )
    assert declaration is not None, f"{path}: missing function {name}"
    end = re.search(r"^\}", source[declaration.end() :], re.MULTILINE)
    assert end is not None, f"{path}: missing end of function {name}"
    return source[declaration.start() : declaration.end() + end.end()]


def assert_manifest_signers(manifest):
    pending = "pending_governance" in manifest
    expected_fields = {"attestors", "threshold"}
    if pending:
        expected_fields.add("pending_governance")
        assert manifest["pending_governance"] is True
    assert set(manifest) == expected_fields
    signers = manifest["attestors"]
    assert isinstance(signers, list)
    assert 1 <= len(signers) <= 64
    assert type(manifest["threshold"]) is int
    assert 1 <= manifest["threshold"] <= len(signers)
    decoded = []
    for index, signer in enumerate(signers):
        assert isinstance(signer, str)
        if pending and signer.startswith("PLACEHOLDER:"):
            assert signer == f"PLACEHOLDER:attestor-{index + 1}"
            continue
        assert re.fullmatch(r"0x[0-9a-fA-F]{40}", signer), (
            f"{MANIFEST}: attestors[{index}] is not a twenty-byte address"
        )
        address = bytes.fromhex(signer[2:])
        assert any(address), f"{MANIFEST}: attestors[{index}] is zero"
        if not pending:
            assert len(set(address)) > 1, (
                f"{MANIFEST}: attestors[{index}] is a repeated-byte placeholder; "
                "bridge governance must supply the approved public signer set"
            )
        decoded.append(address)
    assert decoded == sorted(set(decoded)), "attestors must be unique and ascending"
    assert not decoded or len(decoded) == len(signers), "mixed signer representations"


def test_attestor_manifest_contains_live_canonical_signers():
    assert_manifest_signers(read_json(MANIFEST))


def test_pending_governance_manifest_uses_named_slots():
    manifest = read_json(MANIFEST)
    assert manifest["pending_governance"] is True
    assert manifest["attestors"] == [
        f"PLACEHOLDER:attestor-{index + 1}"
        for index in range(len(manifest["attestors"]))
    ]


@pytest.mark.parametrize("pending", [False, None, 0, 1, "true", {}, []])
def test_pending_governance_requires_explicit_true(pending):
    manifest = read_json(MANIFEST)
    manifest["pending_governance"] = pending
    with pytest.raises(AssertionError):
        assert_manifest_signers(manifest)


def test_signer_checks_remain_strict_without_pending_governance():
    manifest = read_json(MANIFEST)
    del manifest["pending_governance"]
    with pytest.raises(AssertionError, match="not a twenty-byte address"):
        assert_manifest_signers(manifest)
    manifest["attestors"] = ["0x" + f"{index:02x}" * 20 for index in range(1, 6)]
    with pytest.raises(AssertionError, match="repeated-byte placeholder"):
        assert_manifest_signers(manifest)
    manifest["pending_governance"] = True
    assert_manifest_signers(manifest)
    assert_manifest_signers(read_json(DEPLOY / "proposals/testdata/attestors.json"))


def test_attestor_manifest_matches_every_committed_chain():
    manifest = read_json(MANIFEST)
    mismatches = []
    for relative in CHAIN_CONFIGS:
        config = read_json(ROOT / relative)
        for field in ("attestors", "threshold"):
            if config[field] != manifest[field]:
                mismatches.append(f"{relative}: {field} differs from {MANIFEST.name}")
    assert not mismatches, "\n".join(mismatches)


def test_manifest_loader_retains_placeholder_and_threshold_refusals():
    manifest_source = DEPLOY / "proposals/manifest.go"
    proposals_source = DEPLOY / "proposals/proposals.go"
    loader = go_declaration(manifest_source, "LoadAttestorManifest")
    assert "os.ReadFile(path)" in loader
    assert "decoder.DisallowUnknownFields()" in loader
    assert "decoder.Decode(&manifest)" in loader
    assert 'json:"pending_governance"' not in manifest_source.read_text()
    assert loader.index("decoder.Decode(&manifest)") < loader.index("parseSignerList(")
    assert "parseSignerList(path, manifest.Attestors)" in loader
    assert "checkThreshold(path, manifest.Threshold, len(signers))" in loader
    parser = go_declaration(manifest_source, "parseSignerList")
    assert "len(texts) == 0" in parser
    assert "len(texts) > types.MaxAttestors" in parser
    assert "liveAddress(file, field, text)" in parser
    assert "bytes.Compare(signer[:], signers[i-1][:]) <= 0" in parser
    live_address = go_declaration(proposals_source, "liveAddress")
    assert 'refuse(file, field, "zero address")' in live_address
    assert "if isPlaceholder(address[:])" in live_address
    assert 'refuse(file, field, "placeholder address %s", address.Hex())' in live_address
    placeholder = go_declaration(proposals_source, "isPlaceholder")
    assert "for _, b := range raw" in placeholder
    assert "if b != raw[0]" in placeholder
    assert "return true" in placeholder
    threshold = go_declaration(manifest_source, "checkThreshold")
    assert "threshold == 0" in threshold
    assert "int(threshold) > count" in threshold
    committed_refusal = go_declaration(
        DEPLOY / "proposals/proposals_test.go",
        "TestCommittedManifestCarriesPlaceholdersEveryToolRefuses",
    )
    assert "refusal.Field != fieldDocument" in committed_refusal
    assert 'unknown field "pending_governance"' in committed_refusal


def test_proposal_source_checks_manifest_before_emitting_attestors():
    proposals_source = DEPLOY / "proposals/proposals.go"
    manifest_source = DEPLOY / "proposals/manifest.go"
    run = go_declaration(proposals_source, "Run")
    assert "LoadAttestorManifest(*manifestPath)" in run
    assert "Generate(cfg, manifest)" in run
    assert run.index("LoadAttestorManifest(*manifestPath)") < run.index(
        "Generate(cfg, manifest)"
    ) < run.index("bundle.Write(flags.Arg(1))")
    conversion = go_declaration(proposals_source, "FromChainConfig")
    assert "cfg.RequireDeployable()" in conversion
    assert "append([]string(nil), cfg.Attestors...)" in conversion
    assert re.search(r"Threshold:\s+cfg.Threshold,", conversion)
    generate = go_declaration(proposals_source, "Generate")
    assert "SetAttestors(cfg, manifest)" in generate
    setter = go_declaration(proposals_source, "SetAttestors")
    assert "if err := manifest.Check(cfg); err != nil" in setter
    assert setter.index("manifest.Check(cfg)") < setter.index("types.AttestorSet{")
    assert "Threshold: cfg.Threshold" in setter
    assert "Signer: signer" in setter
    assert "msg.ValidateBasic()" in setter
    check = go_declaration(manifest_source, "Check")
    assert "parseSignerList(cfg.file(), cfg.Attestors)" in check
    assert "len(signers) != len(m.signers)" in check
    assert "signer != m.signers[i]" in check
    assert "cfg.Threshold != m.Threshold" in check
