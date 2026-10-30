import importlib.util
import re
from pathlib import Path


def test_retired_human_operations_generate_no_route():
    repo = Path(__file__).resolve().parents[4]
    generator_path = repo / "sdk/platform/generators/generate_jvm.py"
    spec = importlib.util.spec_from_file_location("generate_jvm", generator_path)
    assert spec is not None and spec.loader is not None
    generator = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(generator)

    values = generator.sections(repo / "services/human/schema/human-api")
    human = generator.Plane("Human", values)
    retired_operations = {
        "binding.statement",
        "binding.submit",
        "binding.status",
        "binding.rebind.action",
        "binding.rebind",
        "deposit.start",
        "deposit.confirm",
        "withdraw.start",
        "withdraw.claim",
        "exit.start",
        "exit.eligibility",
    }
    for wire in retired_operations:
        assert values[f"operation.{wire}"]["retired"] == "true"

    wires = {operation["wire"] for operation in generator.operation_specs(human)}
    assert wires
    assert not any(
        wire.startswith(("binding.", "deposit.", "withdraw.", "exit."))
        for wire in wires
    )
    assert wires == {
        section.removeprefix("operation.")
        for section, entries in values.items()
        if section.startswith("operation.") and entries.get("retired") != "true"
    }

    retired_models = {
        "BindingStatementRequest",
        "BindingStatement",
        "BindingSubmission",
        "BindingState",
        "WalletBinding",
        "RebindingSubmission",
        "BindingRebindAction",
        "SettlementDomain",
        "DepositStartRequest",
        "DepositConfirmRequest",
        "WithdrawStartRequest",
        "WithdrawClaimRequest",
        "ExitStartRequest",
        "ExitEligibility",
    }
    for model in retired_models:
        assert values[f"type.{model}"]["retired"] == "true"
        assert model not in human.models
    assert human.models.keys() == {
        section.split(".", 1)[1]
        for section, entries in values.items()
        if section.startswith(("type.", "record."))
        and entries.get("retired") != "true"
    }

    contract = generator.generate_contract(repo)
    assert "/v1/wallet-binding" not in contract
    human_routes = contract.split("HUMAN_ROUTES = Map.ofEntries(", 1)[1]
    assert set(re.findall(r'Map.entry\("([^"]+)"', human_routes)) == wires

    java_root = repo / "sdk/platform/jvm/src/main/java/com/sidiora/layerx/sdk"
    assert (java_root / "GeneratedContract.java").read_text(encoding="utf-8") == contract
    schema = (java_root / "GeneratedSchema.java").read_text(encoding="utf-8")
    human_operations = schema.split(" HUMAN = Map.ofEntries(", 1)[1]
    assert set(re.findall(r'Map.entry\("([^"]+)"', human_operations)) == wires
