defmodule BlockScoutWeb.API.V2.PaxeerX.TransactionStatusSchemaTest do
  use ExUnit.Case, async: true

  alias BlockScoutWeb.Schemas.API.V2.PaxeerX.{SettlementRung, TransactionStatus}

  test "status_schema_enum_is_three_rungs" do
    status_schema_enum_is_three_rungs()
  end

  defp status_schema_enum_is_three_rungs do
    assert SettlementRung.schema().enum == ["submitted", "receipted", "committed"]

    schema = TransactionStatus.schema()

    assert schema.required == [:rung, :block_number, :committed_height]
    assert schema.properties.rung == SettlementRung
  end
end
