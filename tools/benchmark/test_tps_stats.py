import subprocess
import sys
from pathlib import Path

import pytest

from tps_stats import summarise


SCRIPT = Path(__file__).with_name("tps_stats.py")


def run_script(*args: object) -> subprocess.CompletedProcess[str]:
    return subprocess.run(
        [sys.executable, str(SCRIPT), *(str(arg) for arg in args)],
        capture_output=True,
        text=True,
        check=False,
    )


def write_log(path: Path, readings: list[float]) -> None:
    path.write_text(
        "starting benchmark\n"
        + "".join(f"\x1b[32mINFO\x1b[0m tps=\x1b[1;36m{n}\x1b[0m\n" for n in readings),
        encoding="utf-8",
    )


def test_summarise_drops_warmup_and_low_readings(tmp_path):
    log = tmp_path / "benchmark.log"
    write_log(log, [50, 60, 70, 1200, 1400, 1600, 900, 1000])
    assert summarise(log) == {
        "count": 3,
        "median": 1400,
        "average": 1400,
        "min": 1200,
        "max": 1600,
    }
    result = run_script("summarise", log)
    assert result.returncode == 0, result.stderr
    assert result.stdout == (
        f"{log}: Readings: 3 | Median: 1400 | Avg: 1400 | Min: 1200 | Max: 1600\n"
    )

    write_log(log, [1800, 1200, 1600, 1400, 2000])
    assert summarise(log) == {
        "count": 2,
        "median": 1900,
        "average": 1900,
        "min": 1800,
        "max": 2000,
    }

    write_log(log, [0, 1, 2, 1200.25, 1400.5, 2000.75, 2100.5])
    with log.open("a", encoding="utf-8") as stream:
        stream.write("tps=9999 tps=1000\n")
    summary = summarise(log)
    assert summary == {
        "count": 4,
        "median": 1700.625,
        "average": 1675.5,
        "min": 1200.25,
        "max": 2100.5,
    }


def test_compare_exits_1_on_regression(tmp_path):
    baseline = tmp_path / "baseline.log"
    candidate = tmp_path / "candidate.log"
    write_log(baseline, [50, 60, 70, 1800, 2000, 2200])
    write_log(candidate, [50, 60, 70, 1600, 1800, 2000])
    args = ("compare", "--baseline", baseline, "--candidate", candidate)
    result = run_script(*args, "--max-regression-pct", 5)
    assert result.returncode == 1, result.stderr
    assert result.stdout.splitlines() == [
        f"{baseline}: Readings: 3 | Median: 2000 | Avg: 2000 | Min: 1800 | Max: 2200",
        f"{candidate}: Readings: 3 | Median: 1800 | Avg: 1800 | Min: 1600 | Max: 2000",
        "Median regression: 10.0 percent",
    ]

    for threshold in (10, 100):
        result = run_script(*args, "--max-regression-pct", threshold)
        assert result.returncode == 0, result.stderr
    write_log(candidate, [50, 60, 70, 1900, 2000, 2100])
    result = run_script(*args, "--max-regression-pct", 0)
    assert result.returncode == 0, result.stderr
    write_log(candidate, [50, 60, 70, 2100, 2200, 2300])
    result = run_script(*args, "--max-regression-pct", 0)
    assert result.returncode == 0, result.stderr


@pytest.mark.parametrize("empty_side", ["baseline", "candidate", "both"])
@pytest.mark.parametrize(
    "contents",
    ["no TPS readings\n", "", "tps=2000\ntps=2100\ntps=2200\n", "tps=900\n" * 5],
)
def test_compare_exits_2_on_no_data(tmp_path, empty_side, contents):
    baseline = tmp_path / "baseline.log"
    candidate = tmp_path / "candidate.log"
    for side, path in (("baseline", baseline), ("candidate", candidate)):
        if empty_side in (side, "both"):
            path.write_text(contents, encoding="utf-8")
            assert summarise(path) is None
        else:
            write_log(path, [50, 60, 70, 1200, 1400, 1600])
    result = run_script(
        "compare",
        "--baseline",
        baseline,
        "--candidate",
        candidate,
        "--max-regression-pct",
        5,
    )
    assert result.returncode == 2, result.stderr
    assert len(result.stdout.splitlines()) == 2
    for side, path in (("baseline", baseline), ("candidate", candidate)):
        if empty_side in (side, "both"):
            assert f"{path}: NO DATA\n" in result.stdout
