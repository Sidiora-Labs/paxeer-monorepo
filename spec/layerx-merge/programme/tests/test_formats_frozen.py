from pathlib import Path

import pytest

from programme.kvx import KvxError, check_format, load

DATA = Path(__file__).resolve().parents[1] / "data"


def test_every_data_file_declares_format_1(tmp_path: Path) -> None:
    paths = sorted(DATA.glob("*.kvx"))
    assert {path.name for path in paths} == {
        "activation-plan.kvx",
        "decisions.kvx",
        "external-gaps.kvx",
        "parity.kvx",
        "precompile-activation.kvx",
        "refusals.kvx",
        "rehearsal.kvx",
        "retired.kvx",
        "sandbox-profiles.kvx",
        "services.kvx",
        "wave-gates.kvx",
    }
    for path in paths:
        sections = check_format(path)
        assert sections == load(path)
        assert type(sections["meta"]["format_version"]) is int
        assert sections["meta"]["format_version"] == 1

        original = path.read_text(encoding="utf-8")
        declaration = "format_version = 1\n"
        assert original.count(declaration) == 1
        copy = tmp_path / path.name
        for version in (None, "0", "2", "-1", "true", "false", '"1"', '["1"]'):
            replacement = "" if version is None else f"format_version = {version}\n"
            copy.write_text(original.replace(declaration, replacement), encoding="utf-8")
            with pytest.raises(KvxError) as caught:
                check_format(copy)
            assert str(caught.value) == (
                f"{copy}: [meta] format_version must be integer 1"
            )

    original = (DATA / "decisions.kvx").read_text(encoding="utf-8")
    _, body = original.split("\n\n", 1)
    copy = tmp_path / "without-meta.kvx"
    copy.write_text(body, encoding="utf-8")
    assert "meta" not in load(copy)
    with pytest.raises(KvxError) as caught:
        check_format(copy)
    assert str(caught.value) == f"{copy}: [meta] format_version must be integer 1"

    copy.write_text(body + "\n[other]\nformat_version = 1\n", encoding="utf-8")
    with pytest.raises(KvxError) as caught:
        check_format(copy)
    assert str(caught.value) == f"{copy}: [meta] format_version must be integer 1"
