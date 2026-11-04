"""Exercise the signature regression with recorded compiler output after relocation."""

import copy
import importlib.util
import json
import pathlib
import shutil

import pytest


INPUTS = pathlib.Path(__file__).resolve().parent
PROTOCOL = INPUTS.parent
REPOSITORY = INPUTS.parents[3]
RECORDING = PROTOCOL / "catalog/testdata/repair-m924"


def load_export_tests(path):
    spec = importlib.util.spec_from_file_location("relocated_export_tests", path)
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


def recorded_checkout(checkout):
    """Materialize real compiler bytes in the exporter's Hardhat file layout."""
    inputs = checkout / "sdk/launchpad/protocol/generator-inputs"
    inputs.mkdir(parents=True)
    for name in ("export.py", "test_export.py"):
        shutil.copyfile(INPUTS / name, inputs / name)
    tests = load_export_tests(inputs / "test_export.py")
    exporter = tests.m
    root = tests.CONTRACTS_ROOT
    assert root == checkout / "contracts/launchpad"

    review = json.loads((RECORDING / "review.json").read_bytes())
    bundle = review["bundle"]
    artifact_raw = (RECORDING / "artifact.json").read_bytes()
    build_raw = (RECORDING / "build-input.json").read_bytes()
    artifact = json.loads(artifact_raw)
    build = json.loads(build_raw)
    assert exporter.sha(artifact_raw) == bundle["artifactSha256"]
    assert exporter.sha(build_raw) == bundle["buildInfoSha256"]
    assert artifact["abi"] == bundle["abi"]
    assert exporter.sha(exporter.canonical(artifact["abi"])) == bundle["abiSha256"]
    assert artifact["evm"]["bytecode"]["object"]
    assert artifact["evm"]["deployedBytecode"]["object"]

    # Compare every recorded compiler input with this checkout's current source.
    provenance = []
    for name, source in build["input"]["sources"].items():
        raw = (REPOSITORY / "contracts/launchpad" / name).read_bytes()
        assert raw == source["content"].encode(), name
        destination = root / name
        destination.parent.mkdir(parents=True, exist_ok=True)
        destination.write_bytes(raw)
        provenance.append({"path": name, "sha256": exporter.sha(raw)})

    artifact_path = root / bundle["artifactPath"]
    artifact_path.parent.mkdir(parents=True)
    artifact_path.write_bytes(artifact_raw)
    build_path = root / "artifacts/build-info/recorded.json"
    build_path.parent.mkdir(parents=True)
    build_path.write_bytes(build_raw)
    artifact_path.with_suffix(".dbg.json").write_bytes(
        exporter.canonical({"buildInfo": "../../../build-info/recorded.json"})
    )
    assert exporter.export_optical_registry(review, root) == review

    # Hardhat adds identity headers around the unmodified Solidity compiler output.
    artifact["contractName"] = bundle["name"]
    artifact["sourceName"] = bundle["sourceName"]
    artifact_raw = exporter.canonical(artifact)
    artifact_path.write_bytes(artifact_raw)
    contract = copy.deepcopy(bundle)
    contract["artifactSha256"] = exporter.sha(artifact_raw)
    contract["events"] = []
    contract["functions"] = []
    for entry in artifact["abi"]:
        if entry["type"] not in ("event", "function"):
            continue
        signature = (
            entry["name"]
            + "("
            + ",".join(exporter.typ(item) for item in entry["inputs"])
            + ")"
        )
        digest = exporter.keccak256(signature.encode())
        if entry["type"] == "event":
            contract["events"].append(
                {
                    "signature": signature,
                    "topic0": "0x" + digest.hex(),
                    "anonymous": entry["anonymous"],
                }
            )
        else:
            contract["functions"].append(
                {
                    "signature": signature,
                    "selector": "0x" + digest[:4].hex(),
                    "stateMutability": entry["stateMutability"],
                }
            )
    assert contract["events"]
    assert contract["functions"]
    reviewed = {
        "schemaVersion": 1,
        "chainId": 125,
        "sourceProvenance": provenance,
        "contracts": [contract],
    }
    catalog = inputs.parent / "catalog/reviewed.json"
    catalog.parent.mkdir()
    catalog.write_bytes(exporter.canonical(reviewed))
    return tests, reviewed


def test_generator_machine_path_uses_repository_contracts_root():
    tests = load_export_tests(INPUTS / "test_export.py")
    assert tests.CONTRACTS_ROOT == REPOSITORY / "contracts/launchpad"


@pytest.mark.parametrize("relative", ["checkout with spaces", "nested/checkout"])
def test_generator_machine_path_real_artifact_signature_assertions(tmp_path, relative):
    tests, reviewed = recorded_checkout(tmp_path / relative)
    exporter = tests.m
    exported = exporter.export(reviewed, tests.CONTRACTS_ROOT)
    transported = json.loads(exporter.canonical(exported))
    assert transported == exported
    assert transported["chainId"] == 125
    assert transported["reviewedHash"] == exporter.sha(exporter.canonical(reviewed))
    assert len(transported["bundles"]) == 1
    actual = transported["bundles"][0]
    expected = reviewed["contracts"][0]
    for field in (
        "name",
        "artifactSha256",
        "abiSha256",
        "sourceSha256",
        "buildInfoSha256",
        "compilerVersion",
    ):
        assert actual[field] == expected[field]
    assert {e["signature"]: e["topic0"] for e in actual["events"]} == {
        e["signature"]: e["topic0"] for e in expected["events"]
    }
    assert {f["signature"]: f["selector"] for f in actual["functions"]} == {
        f["signature"]: f["selector"] for f in expected["functions"]
    }

    # Run the original regression unchanged, including all three refusal assertions.
    name = "test_signature_hashes_are_verified_against_real_current_artifacts"
    case = tests.ExportTests(name)
    case.test_signature_hashes_are_verified_against_real_current_artifacts()

