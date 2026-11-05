from collections import Counter
from pathlib import Path

import pytest

from programme.kvx import KvxError, check_spec, load

SPEC_PATH = Path(__file__).resolve().parents[2] / "spec.kvx"


def test_spec_is_version_2() -> None:
    meta = load(SPEC_PATH)["meta"]
    assert meta["version"] == "2.0.0"
    assert meta["revision"] == "2a1033aa2"


def test_spec_has_tasks_in_every_wave(tmp_path: Path) -> None:
    counts = check_spec(SPEC_PATH)
    for wave in range(8):
        assert counts[wave] > 0, f"wave {wave} has no tasks"
    sections = load(SPEC_PATH)
    expected = Counter(
        section["wave"]
        for name, section in sections.items()
        if name.startswith("task.")
    )
    assert counts == expected

    source = SPEC_PATH.read_text(encoding="utf-8")
    task_name = next(name for name in sections if name.startswith("task."))
    header = f"[{task_name}]"
    task_body = source.split(header, 1)[1].split("\n[", 1)[0]
    duplicate = tmp_path / "spec.kvx"
    duplicate.write_text(source + "\n" + header + task_body, encoding="utf-8")
    with pytest.raises(KvxError) as caught:
        check_spec(duplicate)
    assert f"duplicate section [{task_name}]" in str(caught.value)
