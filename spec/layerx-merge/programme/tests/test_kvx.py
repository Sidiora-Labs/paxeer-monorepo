from pathlib import Path

import pytest

from programme.kvx import KvxError, load

ROOT = Path(__file__).resolve().parents[4]


def test_loads_workflow_sections(tmp_path: Path) -> None:
    path = ROOT / "spec/workflow.kvx"
    sections = load(path)
    assert sections["limits"]["max_task_minutes"] == 30
    assert sections["limits"]["max_gate_runs_per_task"] == 1
    assert sections["meta"]["active_feature"] == "layerx-merge"
    assert sections["status_values"]["task"] == ["pending", "in_progress", "done"]
    assert sections["render"]["markdown_mirror"] is False
    assert sections["hierarchy"]["enabled"] is True
    assert 'cg commit -m "Specgen re-render"' in sections["meta"]["source_of_truth"]

    copy = tmp_path / "workflow.kvx"
    copy.write_text(
        path.read_text(encoding="utf-8")
        + "\n[parser_cases]\n"
        + 'quoted = "a=b # \\"quoted\\"" # outside comment\n'
        + 'strings = ["a,b", "c=d", "#", "\\\\"] # raw backslashes\n'
        + r'raw = "\0\n\t\u0041\\path"'
        + "\ntrue_value = true # boolean comment\nfalse_value = false\n"
        + 'raw_list = ["\\0", "\\n", "\\\\path", "\\\"quoted\\\""] # list comment\n'
        + "negative = -7\nempty = []\n",
        encoding="utf-8",
    )
    assert load(copy)["parser_cases"] == {
        "quoted": 'a=b # "quoted"',
        "strings": ["a,b", "c=d", "#", r"\\"],
        "raw": r"\0\n\t\u0041\\path",
        "true_value": True,
        "false_value": False,
        "raw_list": [r"\0", r"\n", r"\\path", '"quoted"'],
        "negative": -7,
        "empty": [],
    }


def test_refuses_duplicate_section(tmp_path: Path) -> None:
    source = ROOT / "spec/layerx-merge/qualification.kvx"
    original = source.read_text(encoding="utf-8")
    assert "finding.1.21.1" in load(source)
    assert "finding.0.0.0" not in load(source)
    finding = (
        "[finding.0.0.0]\n"
        'task = "0.0.0"\n'
        'file = "spec/workflow.kvx:32"\n'
        'symbol = "max_task_minutes"\n'
        'observed = "Duplicate finding is rejected."\n'
        'assumption = "Ledger section names are unique."\n'
        'severity = "note"\n'
    )
    prefix = original + "\n" + finding + "\n"
    duplicate_line = prefix.count("\n") + 1
    copy = tmp_path / "qualification.kvx"
    copy.write_text(prefix + finding, encoding="utf-8")
    with pytest.raises(KvxError) as caught:
        load(copy)
    assert str(caught.value) == (
        f"{copy}:{duplicate_line}: duplicate section [finding.0.0.0]"
    )

    prefix = original + "\n[parser_errors]\n"
    first_line = prefix.count("\n") + 1
    cases = [
        ('key = "first"\nkey = "second"\n', first_line + 1, "duplicate key key"),
        ('key\t= "value"\n', first_line, "tab is not allowed"),
        ('# comment with\ttab\n', first_line, "tab is not allowed"),
    ]
    for invalid in (
        "bare",
        "True",
        "False",
        "1.5",
        "[1]",
        '["ok", false]',
        '"unclosed',
        '["ok",]',
        '["ok" "missing comma"]',
        '"ok" trailing',
    ):
        cases.append((f"key = {invalid}\n", first_line, "value"))
    for suffix, line, message in cases:
        copy.write_text(prefix + suffix, encoding="utf-8")
        with pytest.raises(KvxError) as caught:
            load(copy)
        assert f"{copy}:{line}:" in str(caught.value)
        assert message in str(caught.value)
