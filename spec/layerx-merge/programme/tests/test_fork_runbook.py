import re
from pathlib import Path

from programme.kvx import load

ROOT = Path(__file__).resolve().parents[4]
PLAN = Path(__file__).resolve().parents[1] / "data/rehearsal.kvx"


def test_rehearsal_section_names_every_step() -> None:
    runbook = (ROOT / "docs/runbooks/paxeer-x-fork.md").read_text(encoding="utf-8")
    match = re.search(
        r"^## 4\. Rehearsal[^\n]*\n(.*?)(?=^## |\Z)",
        runbook,
        flags=re.MULTILINE | re.DOTALL,
    )
    assert match is not None, "runbook must contain section 4 Rehearsal"
    section = match.group(1)
    plan = load(PLAN)
    steps = [fields for name, fields in plan.items() if name.startswith("step.")]
    assert len(steps) == 4
    positions = []
    for step in steps:
        assert step["test"] in section, step["test"]
        positions.append(section.index(step["test"]))
    assert positions == sorted(positions), "runbook must preserve rehearsal step order"

    command = (
        "python3 -m pytest spec/layerx-merge/programme/tests/test_rehearsal_run.py::"
        "test_rehearsal_passes_on_merged_revision -q"
    )
    assert f"```sh\n{command}\n```" in section
    assert "programme.rehearsal.run_steps" in section
    assert "spec/layerx-merge/programme/data/rehearsal.kvx" in section
    assert "Any failed step is a no-go" in section
    assert "stop the go decision" in section
