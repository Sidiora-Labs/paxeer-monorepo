import re
from collections import Counter
from pathlib import Path

import pytest

from programme.kvx import KvxError, load

ROOT = Path(__file__).resolve().parents[4]


def test_ledger_has_no_duplicate_section() -> None:
    path = ROOT / "spec/layerx-merge/qualification.kvx"
    try:
        sections = load(path)
    except KvxError as error:
        pytest.fail(f"Qualification ledger must load without KvxError: {error}")

    findings = {
        name: values for name, values in sections.items() if name.startswith("finding.")
    }
    finding_ids = re.findall(
        r"^[ \t]*\[(finding\.[A-Za-z0-9_.-]+)\][ \t]*(?:#.*)?$",
        path.read_text(encoding="utf-8"),
        flags=re.MULTILINE,
    )
    assert Counter(finding_ids) == Counter({name: 1 for name in findings})

    required_keys = {"task", "file", "symbol", "observed", "assumption", "severity"}
    for name, finding in findings.items():
        assert set(finding) == required_keys, (
            f"[{name}] must contain exactly {sorted(required_keys)}; "
            f"found {sorted(finding)}"
        )
