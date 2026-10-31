import re
from pathlib import Path


ROOT = Path(__file__).resolve().parents[2]
PROGRAMS = ROOT / "tests" / "programs"


def test_blob_admission_has_no_capacity_ledger():
    source = (PROGRAMS / "test_blob_admission.c").read_text()
    assert "lxp_capacity_ledger" not in source, (
        "Blob admission still depends on the retired capacity ledger"
    )


def test_native_gaps_has_no_capacity_observe():
    source = (PROGRAMS / "test_paxai_native_gaps.c").read_text()
    assert "lxp_kernel_capacity_observe" not in source, (
        "Native gaps still depends on retired capacity observation"
    )


def test_programs_suites_have_no_feed_store():
    paths = sorted(
        path
        for path in PROGRAMS.rglob("*")
        if path.is_file() and path.suffix in {".c", ".h"}
    )
    assert paths, "No programs suite sources found"
    retired = re.compile(
        r"\b\w*feed_store\w*\b|\blxp_replay_\w+\b|"
        r"\blxp_programs_replay_(?:finalize|engine_bind)\b|"
        r'[#]\s*include\s*[<"]layerx/lxp_replay\.h[>"]'
    )
    references = [
        f"{path.relative_to(ROOT)}:{number}: {line.strip()}"
        for path in paths
        for number, line in enumerate(path.read_text().splitlines(), start=1)
        if retired.search(line)
    ]
    assert not references, (
        "Retired feed store or replay engine references remain:\n"
        + "\n".join(references)
    )
