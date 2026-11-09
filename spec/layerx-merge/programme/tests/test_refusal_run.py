import json
import subprocess
from pathlib import Path
from tempfile import mkdtemp

from programme.evidence import evidence_line
from programme.kvx import load
from programme.parity import command_for
from programme.retired import run_refusals

ROOT = Path(__file__).resolve().parents[4]
TABLE = ROOT / "spec/layerx-merge/programme/data/refusals.kvx"


def test_every_refusal_runs_and_passes() -> None:
    table = load(TABLE)
    names = [name for name in table if name.startswith("refusal.")]
    assert set(names) == {
        "refusal.0x1012",
        "refusal.0x1013",
        "refusal.0x1014",
        "refusal.merge-plan",
        "refusal.custody-release",
        "refusal.exchange-intents",
    }
    for name in names:
        row = table[name]
        assert row["command"] == command_for({"selectors": [row["test"]]}), name

    revision = subprocess.run(
        ["git", "rev-parse", "HEAD"],
        cwd=ROOT,
        capture_output=True,
        text=True,
        check=True,
    ).stdout.strip()
    log_dir = Path(mkdtemp(prefix="layerx-merge-refusals-"))
    results = run_refusals(table, ROOT, log_dir)

    assert list(results) == names
    failures = [result.evidence for result in results.values() if not result.passed]
    assert not failures, "\n".join(failures)
    for name, result in results.items():
        assert result.passed is True, result.evidence
        assert result.count >= 1, result.evidence
        assert result.exit_code == 0, result.evidence
        log_path = log_dir / f"{name}.log"
        assert json.loads(log_path.read_text(encoding="utf-8")) == {"count": result.count}
        assert result.evidence == evidence_line(
            revision, table[name]["command"], 0, log_path
        )
    assert (log_dir / "evidence.log").read_text(encoding="utf-8").splitlines() == [
        result.evidence for result in results.values()
    ]
