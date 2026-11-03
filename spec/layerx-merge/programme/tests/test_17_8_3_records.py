import subprocess
from pathlib import Path

from programme.decisions import load_index, render_record
from programme.kvx import load

ROOT = Path(__file__).resolve().parents[4]
SPEC = load(ROOT / "spec/layerx-merge/spec.kvx")


def test_d12_basis_cites_the_enforcing_code() -> None:
    basis = load_index()[12]["basis"]
    assert "spec/layerx-merge/spec.kvx" not in basis
    declarations = (
        ("modules/xweb/types/observation.go", 11, "ObservationMaxSignatures = 16"),
        ("modules/xweb/types/state.go", 213, "const MaxAttestorSetSize = 16"),
        ("modules/xweb/types/keys.go", 21, "MaxAttestors = 64"),
    )
    for path, line_number, declaration in declarations:
        assert f"{path}:{line_number}" in basis
        assert declaration.removeprefix("const ") in basis
        source = subprocess.check_output(
            ["git", "show", f"HEAD:{path}"], cwd=ROOT, text=True
        )
        assert " ".join(source.splitlines()[line_number - 1].split()) == declaration

    assert "modules/xweb/keeper/program_fulfil.go:41, :47" in basis
    assert (
        "bounds only origin-1 fulfil signatures (modules/xweb/keeper/fulfil.go:21)"
        in basis
    )
    assert "NetworkID must equal the KernelNetworkId xweb param" in basis
    assert "modules/xweb/keeper/program_fulfil.go:124" in basis
    assert "modules/xweb/types/state.go:106" in basis


def test_records_12_to_22_match_index() -> None:
    index = load_index()
    identifiers = set(range(12, 23))
    assert all(index[i]["basis"] for i in identifiers)
    assert identifiers <= index.keys()
    assert {
        identifier
        for identifier in identifiers
        if index[identifier]["carry"] == "rewritten"
    } == {20, 22}
    assert {
        identifier for identifier in identifiers if "value" in index[identifier]
    } == {20, 22}

    for identifier in sorted(identifiers):
        entry = index[identifier]
        decision = SPEC[f"decision.{identifier}"]
        assert entry["record"] == f"spec/layerx-merge/docs/decisions/D{identifier}.md"
        assert entry["carry"] == decision["carry"]
        assert entry["binds"]
        assert all(f"task.{task}" in SPEC for task in entry["binds"])
        if entry["carry"] == "rewritten":
            assert entry["value"] != decision["value"]
        expected = render_record(entry, SPEC)
        assert (ROOT / entry["record"]).read_bytes() == expected.encode("utf-8"), (
            f"D{identifier} differs from its carry-forward index rendering"
        )
