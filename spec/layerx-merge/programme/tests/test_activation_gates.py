from copy import deepcopy

import pytest

from programme.activation import PlanError, check_gates, load_plans, pending_rule


def test_gated_plans_name_their_gate() -> None:
    plans = load_plans()
    for name, gate in (
        ("v6.9", "activationPrecompileGate"),
        ("v7.0", "mergePrecompileGate"),
    ):
        plan = deepcopy(next(plan for plan in plans if plan["name"] == name))
        assert plan["precompile_gate"] == gate
        plan.pop("precompile_gate", None)
        with pytest.raises(
            PlanError, match=f"plan {name}: precompile_gate must name {gate}"
        ):
            check_gates([plan])
        other_gate = (
            "activationPrecompileGate" if name == "v7.0" else "mergePrecompileGate"
        )
        for invalid in ("", "   ", "unknownGate", other_gate):
            plan["precompile_gate"] = invalid
            with pytest.raises(
                PlanError, match=f"plan {name}: precompile_gate must name {gate}"
            ):
                check_gates([plan])
        plan["precompile_gate"] = gate
        check_gates([plan])

    ungated = deepcopy(next(plan for plan in plans if plan["name"] == "v6.10"))
    ungated["precompile_gate"] = "activationPrecompileGate"
    with pytest.raises(PlanError, match="no precompile_gate is declared in node"):
        check_gates([ungated])

    check_gates(plans)


def test_unknown_plan_is_not_pending() -> None:
    plans = load_plans()
    unknown = deepcopy(plans[0])
    unknown["name"] = "v99.0"
    rules = pending_rule([*plans, unknown])
    assert rules == {
        "v6.9": "done_height == 0 and not activation_modules_present",
        "v6.10": "done_height == 0",
        "v6.12.0": "done_height == 0",
        "v7.0": "done_height == 0",
        "v7.0-anchor-store-delete": "never applied",
        "v99.0": "never applied",
    }
    assert pending_rule([]) == {}
