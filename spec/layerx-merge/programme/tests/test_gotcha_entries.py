"""Preserve the historical ledger prefix while validating appended lessons."""

from hashlib import sha256
from pathlib import Path

from programme.kvx import load

ROOT = Path(__file__).resolve().parents[4]
LEDGER_PREFIX_SHA256 = "62c2e85d9216a27a365cf2ec96e10ccb3ccb0d931b56d9af6c64f5d80463e5f5"


def test_new_entries_follow_ledger_format(tmp_path: Path) -> None:
    path = ROOT / "GOTCHA.kvx"
    raw = path.read_bytes()
    for name in (b"[gotcha.1844]", b"[gotcha.1845]"):
        assert raw.count(name) == 1

    prefix, separator, _ = raw.partition(b"[gotcha.1844]")
    assert separator == b"[gotcha.1844]"
    assert sha256(prefix).hexdigest() == LEDGER_PREFIX_SHA256

    task_entries = tmp_path / "GOTCHA.kvx"
    task_entries.write_bytes(
        raw[raw.index(b"[gotcha.1843]") : raw.index(b"[gotcha.1846]")]
    )
    sections = load(task_entries)
    names = list(sections)
    previous = names.index("gotcha.1843")
    assert names[previous + 1 : previous + 3] == ["gotcha.1844", "gotcha.1845"]

    for name, area in (("gotcha.1844", "test"), ("gotcha.1845", "build")):
        entry = sections[name]
        assert set(entry) == {"area", "symptom", "cause", "do"}
        assert entry["area"] == area
        for value in entry.values():
            assert isinstance(value, str) and value.strip()

    zero_test = sections["gotcha.1844"]
    assert "ok" in zero_test["symptom"]
    assert "no test output" in zero_test["symptom"]
    assert "-run" in zero_test["cause"]
    assert "matches no test" in zero_test["cause"]
    assert "go test -json" in zero_test["do"]
    assert "count" in zero_test["do"] and "run actions" in zero_test["do"]
    assert "17.4.3" in zero_test["do"]

    kernel_link = sections["gotcha.1845"]
    assert "go test" in kernel_link["symptom"]
    assert "precompiles/setup.go" in kernel_link["symptom"]
    assert "link time" in kernel_link["symptom"]
    assert "clean checkout" in kernel_link["cause"]
    assert "build/liblayerx.a" in kernel_link["cause"]
    assert "go-cgo" in kernel_link["do"]
    assert "sandbox prebuild" in kernel_link["do"]
    assert "17.6.1" in kernel_link["do"]
