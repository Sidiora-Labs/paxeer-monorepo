import os
import subprocess
from pathlib import Path

import pytest


SCRIPT = Path(__file__).with_name("benchmark-compare.sh").resolve()
LABELS = ("z-baseline", "candidate", "a-candidate")


def run_results(tmp_path, samples, threshold):
    base_dir = tmp_path / "benchmark results"
    logs = {}
    for label, sample in zip(LABELS, samples):
        if sample is None:
            continue
        log = base_dir / label / "output.log"
        log.parent.mkdir(parents=True)
        if isinstance(sample, int):
            sample = "starting benchmark\n" + "".join(
                f"\x1b[32mINFO\x1b[0m height={height} "
                f"tps=\x1b[1;36m{reading}\x1b[0m\n"
                for height, reading in enumerate(
                    (50, 60, 70, sample - 100, sample, sample + 100), start=1
                )
            )
        log.write_text(sample, encoding="utf-8")
        logs[log] = sample

    env = {**os.environ, "RESULTS_ONLY": "true", "BASE_DIR": str(base_dir)}
    env.pop("MAX_REGRESSION_PCT", None)
    if threshold is not None:
        env["MAX_REGRESSION_PCT"] = threshold
    result = subprocess.run(
        ["bash", str(SCRIPT), *(f"{label}=unused-{label}" for label in LABELS)],
        cwd=tmp_path,
        env=env,
        capture_output=True,
        text=True,
        timeout=10,
        check=False,
    )
    for log, contents in logs.items():
        assert log.read_text(encoding="utf-8") == contents
    for phase in (1, 2, 3):
        assert f"=== Phase {phase}:" not in result.stdout
    assert "Cleaning up..." not in result.stdout
    for label in LABELS:
        assert f"[{label}] (unused-{label})" in result.stdout
    for label in LABELS[1:]:
        assert f"Comparing [{label}] against [{LABELS[0]}]" in result.stdout
    assert "Compare CPU:" in result.stdout
    return result


@pytest.mark.parametrize(
    "samples,threshold",
    [
        ((2000, 1920, 2200), None),
        ((2000, 1900, 2000), None),
        ((2000, 1800, 2000), "10"),
        ((2000, 2000, 2200), "0"),
    ],
)
def test_results_only_passes_within_threshold(tmp_path, samples, threshold):
    result = run_results(tmp_path, samples, threshold)
    assert result.returncode == 0, result.stdout + result.stderr
    for sample in samples:
        assert f"Readings: 3 | Median: {sample}" in result.stdout
    assert "Height: 6" in result.stdout
    assert "Median regression:" in result.stdout


@pytest.mark.parametrize(
    "samples,threshold,expected",
    [
        ((2000, 1800, 2200), None, 1),
        ((2000, 2200, 1800), None, 1),
        ((2000, 1920, 2200), "3", 1),
        ((None, 1800, 2200), None, 2),
        ((2000, None, 1800), None, 2),
        ((2000, 1800, None), None, 2),
        (("", 1800, 2200), None, 2),
        ((2000, "", 1800), None, 2),
        ((2000, 1800, ""), None, 2),
        ((2000, "no TPS readings\n", 2200), None, 2),
        ((2000, "tps=2000\ntps=2100\ntps=2200\n", 2200), None, 2),
        ((2000, "tps=900\n" * 5, 2200), None, 2),
    ],
)
def test_results_only_fails_on_regression(tmp_path, samples, threshold, expected):
    result = run_results(tmp_path, samples, threshold)
    assert result.returncode == expected, result.stdout + result.stderr
    if expected == 1:
        assert "Median regression:" in result.stdout
    elif None in samples:
        assert "No such file or directory" in result.stderr
    else:
        assert "NO DATA" in result.stdout
