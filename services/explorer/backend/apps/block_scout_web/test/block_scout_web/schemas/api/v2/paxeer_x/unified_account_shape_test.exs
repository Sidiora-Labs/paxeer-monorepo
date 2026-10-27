defmodule BlockScoutWeb.Schemas.API.V2.PaxeerX.UnifiedAccountShapeTest do
  use BlockScoutWeb.ConnCase

  alias BlockScoutWeb.Schemas.API.V2.PaxeerX.UnifiedAccount
  alias BlockScoutWeb.Schemas.API.V2.PaxeerX.UnifiedAccount.{ActivityItem, Balance, Identities}
  alias Explorer.Chain.Address
  alias OpenApiSpex.{Components, Info, OpenApi}

  test "unified_answer_casts_against_schema", %{conn: conn} do
    unified_answer_casts_against_schema(conn)
  end

  defp unified_answer_casts_against_schema(conn) do
    address = insert(:address, fetched_coin_balance: 1_000)
    counterparty = insert(:address)
    block = insert(:block, number: 100)

    transaction =
      :transaction
      |> insert(from_address: address, to_address: counterparty, value: 42)
      |> with_block(block, status: :ok)

    body = json_response(get(conn, "/api/v2/addresses/#{Address.checksum(address.hash)}/unified"), 200)

    assert body["identities"] == %{"evm" => Address.checksum(address.hash), "pax" => nil, "did" => nil}
    assert [%{"asset" => %{"id" => "native"}, "total" => "1000"} = balance] = body["balances"]
    assert Enum.sort(Map.keys(balance)) == ["asset", "total"]

    assert [activity] = body["activity"]
    assert activity["hash"] == to_string(transaction.hash)
    assert activity["kind"] == "transaction"
    assert activity["amount"] == "42"
    assert activity["status"] == "committed"
    assert activity["result"] == "ok"
    assert activity["counterparty"] == Address.checksum(counterparty.hash)
    refute Map.has_key?(activity, "side")

    assert Enum.sort(Map.keys(Identities.schema().properties)) == [:did, :evm, :pax]
    assert Enum.sort(Map.keys(Balance.schema().properties)) == [:asset, :total]
    assert ActivityItem.schema().properties.kind.enum == ["transaction", "token_transfer"]
    refute Map.has_key?(ActivityItem.schema().properties, :side)

    spec =
      OpenApiSpex.resolve_schema_modules(%OpenApi{
        info: %Info{title: "Unified account schema test", version: "1"},
        paths: %{},
        components: %Components{schemas: %{"PaxeerXUnifiedAccount" => UnifiedAccount.schema()}}
      })

    assert {:ok, cast} =
             OpenApiSpex.cast_value(body, spec.components.schemas["PaxeerXUnifiedAccount"], spec)

    assert cast.identities.evm == Address.checksum(address.hash)
    assert [%{total: "1000"}] = cast.balances
    assert [%{hash: hash, amount: "42", result: "ok"}] = cast.activity
    assert hash == to_string(transaction.hash)
  end
end
