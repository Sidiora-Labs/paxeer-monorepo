from pathlib import Path
import runpy
import subprocess


ROOT = Path(__file__).resolve().parents[4]
GENERATOR = ROOT / "tools/docs/gen/gen_system_docs.py"


def test_system_docs_are_current() -> None:
    result = subprocess.run(
        ["bash", "tools/docs/gen-system-docs.sh", "--check"],
        cwd=ROOT,
        capture_output=True,
        text=True,
    )
    assert result.returncode == 0, result.stdout + result.stderr
    assert "docs/system is current" in result.stdout


def test_precompile_summary_lists_venues() -> None:
    generator = runpy.run_path(str(GENERATOR))
    filename, document = generator["gen_precompiles"]()
    assert filename == "precompiles.md"
    summary = document.split("\n## Summary\n\n", 1)[1].split("\n## ", 1)[0]
    rows = [
        [cell.strip().strip("`") for cell in line.strip("|").split("|")]
        for line in summary.splitlines()
        if line.startswith("| ")
    ][1:]

    for name, address in (("perps", 0x1015), ("pricefeed", 0x1020)):
        venue = [row for row in rows if row[0] == name]
        assert len(venue) == 1, f"missing or duplicate {name} summary row"
        assert venue[0][1] == f"precompiles/{name}/abi/abi.json"
        assert venue[0][2] == f"0x{address:040x}"

    spot = [row for row in rows if row[0] == "spot"]
    assert {row[1] for row in spot} == {
        f"precompiles/spot/abi/{name}.json"
        for name in ("pecor", "orders", "stoporders", "quoter", "vault", "share")
    }
    for row in spot:
        addresses = {address.strip("`") for address in row[2].split(", ")}
        assert addresses == {f"0x{address:040x}" for address in range(0x1021, 0x1026)}

    agents = [row for row in rows if row[0].startswith("layerxagent/")]
    assert {row[0] for row in agents} == {
        f"layerxagent/{name}"
        for name in (
            "clearing",
            "eip712",
            "oracle",
            "orob",
            "pofq",
            "scheduler",
            "streams",
            "teeattestor",
        )
    }
    for row in agents:
        package = row[0].split("/")[1]
        assert row[1] == f"precompiles/{row[0]}/{package}.go:abiJSON"

    legacy = [row for row in rows if row[0] == "layerxexchange"]
    assert len(legacy) == 1
    assert legacy[0][1] == "precompiles/layerxexchange/abi.json"
