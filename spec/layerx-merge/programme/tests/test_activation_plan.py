from copy import deepcopy
from pathlib import Path

import pytest

from programme.activation import load_plans, validate
from programme.kvx import load

PLAN_PATH = Path(__file__).resolve().parents[1] / "data/activation-plan.kvx"


def test_plan_order_matches_known_plans() -> None:
    plans = load_plans()
    names = ["v6.9", "v6.10", "v6.12.0", "v7.0", "v7.0-anchor-store-delete"]
    assert [plan["name"] for plan in plans] == names
    assert list(load(PLAN_PATH)) == [f"plan.{name}" for name in names]
    assert load_plans(PLAN_PATH) == plans
    assert [plan["handler"] for plan in plans] == [
        "runActivationUpgrade",
        "runV610Upgrade",
        "runV6120Upgrade",
        "mergeUpgradePlan",
        "runAnchorStoreDeleteUpgrade",
    ]
    validate(plans)

    with pytest.raises(ValueError, match="repeated plan name: v6.9"):
        validate([plans[0], *plans])
    missing_handler = deepcopy(plans)
    del missing_handler[1]["handler"]
    with pytest.raises(ValueError, match="v6.10 is missing a handler"):
        validate(missing_handler)
    for handler in ("", "   "):
        blank_handler = deepcopy(plans)
        blank_handler[2]["handler"] = handler
        with pytest.raises(ValueError, match="v6.12.0 is missing a handler"):
            validate(blank_handler)
    with pytest.raises(ValueError, match="follows v7.0-anchor-store-delete"):
        validate([*plans, plans[0]])


def test_merge_plans_match_store_changes() -> None:
    plans = load_plans(PLAN_PATH)
    by_name = {plan["name"]: plan for plan in plans}
    assert by_name["v6.9"]["store_added"] == [
        "layerxcustody",
        "layerxanchor",
        "layerxexchange",
        "layerxbridge",
        "launchpad",
        "xweb",
    ]
    assert by_name["v6.10"]["store_added"] == []
    assert by_name["v6.12.0"]["store_added"] == ["perps", "spot", "pricefeed"]
    assert by_name["v7.0"]["store_added"] == [
        "sequencer",
        "layerxledger",
        "layerxauth",
        "layerxpay",
        "layerxprograms",
        "layerxagent",
        "layerxhost",
    ]
    assert by_name["v7.0-anchor-store-delete"]["store_added"] == []
    assert [plan["store_deleted"] for plan in plans] == [
        [],
        [],
        [],
        [],
        ["layerxanchor"],
    ]
