from collections import Counter
from copy import deepcopy
from pathlib import Path

from programme.kvx import load
from programme.runcards import (
    check_against_spec,
    check_card,
    load_card,
    render_counts,
)

ROOT = Path(__file__).resolve().parents[4]
SPEC = ROOT / "spec/layerx-merge/spec.kvx"
CARDS = ROOT / "spec/layerx-merge/programme/data/runcards"


def test_cards_agree_with_installed_spec() -> None:
    spec = load(SPEC)
    schema = load_card(CARDS / "schema.kvx")
    cards = [load_card(path) for path in sorted(CARDS.glob("fm-*.kvx"))]
    original = deepcopy(cards)
    assert len(cards) == 17
    assert {card["domain"] for card in cards} == set(range(1, 18))

    checked = check_against_spec(iter(cards), SPEC)
    rendered = render_counts(iter(cards), SPEC)
    assert len(checked) == len(rendered) == 17

    for card, validated, counted in zip(cards, checked, rendered, strict=True):
        domain = card["domain"]
        assert check_card(counted, schema) is None
        assert card["role"] == f"fm-{spec[f'feature.{domain}']['name']}"
        tasks = [
            task for name, task in spec.items() if name.startswith(f"task.{domain}.")
        ]
        waves = Counter(task["wave"] for task in tasks)
        assert waves
        assert validated == {**card, "max_parallel": max(waves.values())}
        assert counted == {
            **validated,
            "waves": [f"wave {wave}: {waves[wave]}" for wave in sorted(waves)],
        }
        assert sum(waves.values()) == len(tasks)

    stale = [{**card, "max_parallel": 0, "waves": ["stale"]} for card in cards]
    stale_original = deepcopy(stale)
    assert render_counts(iter(stale), SPEC) == rendered
    assert stale == stale_original
    assert cards == original
