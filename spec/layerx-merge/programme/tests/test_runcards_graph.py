from pathlib import Path

import pytest

from programme.runcards import CardError, check_against_spec, load_card

CARDS = Path(__file__).resolve().parents[1] / "data/runcards"


@pytest.mark.parametrize(
    ("second_id", "second_wave", "second_profile", "max_parallel"),
    [
        ("3.0.2", 0, "go", 2),
        ("3.0.2", 1, "go", 1),
        ("13.0.1", 0, "rust", 1),
    ],
)
def test_missing_anchor_is_refused(
    tmp_path: Path,
    second_id: str,
    second_wave: int,
    second_profile: str,
    max_parallel: int,
) -> None:
    spec_path = tmp_path / "spec.kvx"
    spec_path.write_text(
        '[task.3.0.1]\nwave = 0\nsandbox_profile = "go"\n'
        f'[task.{second_id}]\nwave = {second_wave}\n'
        f'sandbox_profile = "{second_profile}"\n'
        '[acceptance.3.0.9]\ntitle = "An acceptance section is not a task"\n',
        encoding="utf-8",
    )
    card = load_card(CARDS / "fm-markets.kvx")
    missing_anchor = {**card, "anchors": ["3.0.1", "3.0.9"]}
    with pytest.raises(CardError, match=r"missing anchors: 3\.0\.9"):
        check_against_spec([missing_anchor], spec_path)

    valid = {**card, "anchors": ["3.0.1"], "max_parallel": 99}
    checked = check_against_spec([valid], spec_path)
    assert checked == [{**valid, "max_parallel": max_parallel}]
    assert valid["max_parallel"] == 99
    assert check_against_spec(iter([valid, valid]), spec_path) == checked * 2
    with pytest.raises(CardError, match="sandbox_profile.*domain 3.*go.*rust"):
        check_against_spec([{**valid, "sandbox_profile": "rust"}], spec_path)

    assert check_against_spec([{**valid, "domain": "3"}], spec_path) == [
        {**valid, "domain": "3", "max_parallel": max_parallel}
    ]
    with pytest.raises(CardError, match=r"missing anchors: 3\.0\.9"):
        check_against_spec([valid, missing_anchor], spec_path)

    if second_id == "13.0.1":
        other = {
            **valid,
            "domain": 13,
            "anchors": [second_id],
            "sandbox_profile": "rust",
        }
        assert check_against_spec([valid, other], spec_path) == [
            {**valid, "max_parallel": 1},
            {**other, "max_parallel": 1},
        ]
        with pytest.raises(CardError, match="anchors belong to another domain: 13.0.1"):
            check_against_spec([{**valid, "anchors": [second_id]}], spec_path)
    else:
        with spec_path.open("a", encoding="utf-8") as spec:
            spec.write('[task.3.1.1]\nwave = 0\nsandbox_profile = "rust"\n')
        assert check_against_spec([valid], spec_path) == [
            {**valid, "max_parallel": 3 if second_wave == 0 else 2}
        ]
        with pytest.raises(CardError, match="sandbox_profile.*domain 3.*go.*rust"):
            check_against_spec([{**valid, "sandbox_profile": "rust"}], spec_path)
