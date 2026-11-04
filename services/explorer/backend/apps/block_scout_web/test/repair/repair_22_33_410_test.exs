defmodule BlockScoutWeb.Repair.Repair2233410Test do
  @moduledoc """
  Repair record for audit 22.30.143, finding unified-account-description.
  The description test proves the current identities, totals and chain activity
  vocabulary; the recorded transaction test proves requirement 14265.1 through
  the repository, HTTP controller, JSON response and production OpenAPI schemas.
  The chain recording is the committed geth/trace/struct_logger.json fixture;
  the account balance and block timestamp are local database fixture values.
  No out-of-path wiring is required. Baseline verification was not run because
  the bounded gate order requires implementation before build and hf-done and
  forbids a separate verification run; no baseline failures are claimed.
  The runner's private evidence record names the verified candidate revision.
  """
  use BlockScoutWeb.ConnCase

  alias BlockScoutWeb.Schemas.API.V2.PaxeerX.UnifiedAccount
  alias BlockScoutWeb.Schemas.API.V2.PaxeerX.UnifiedAccount.{ActivityItem, Balance, Identities}
  alias EthereumJSONRPC
  alias Explorer.Chain.Address
  alias OpenApiSpex.{Components, Info, OpenApi}

  @recording Path.expand("../../../ethereum_jsonrpc/test/support/fixture/geth/trace/struct_logger.json", __DIR__)

  test "unified-account-description: describes three identities, total balances and chain activity" do
    schema = UnifiedAccount.schema()

    assert schema.description =~ "three identities (EVM, Paxeer and DID)"
    assert schema.description =~ "a single total balance per asset"
    assert schema.description =~ "chain's transactions and token transfers"
    refute schema.description =~ ~r/\b(four|kernel_account|parts|side|custody|kernel)\b/i

    assert Enum.sort(Map.keys(Identities.schema().properties)) == [:did, :evm, :pax]
    assert Enum.sort(Map.keys(Balance.schema().properties)) == [:asset, :total]
    assert ActivityItem.schema().properties.kind.enum == ["transaction", "token_transfer"]
    refute Map.has_key?(ActivityItem.schema().properties, :side)
  end

  test "unified-account-description: recorded transaction response casts without retired fields", %{conn: conn} do
    recording = @recording |> File.read!() |> Jason.decode!()
    tx = recording["tx"]
    receipt = recording["receipt"]
    address = insert(:address, hash: tx["from"], fetched_coin_balance: 1_000)
    counterparty = insert(:address, hash: tx["to"])
    block = insert(:block, hash: tx["blockHash"], number: EthereumJSONRPC.quantity_to_integer(tx["blockNumber"]))

    transaction =
      :transaction
      |> insert(
        hash: tx["hash"],
        from_address: address,
        to_address: counterparty,
        input: tx["input"],
        value: EthereumJSONRPC.quantity_to_integer(tx["value"])
      )
      |> with_block(block,
        index: EthereumJSONRPC.quantity_to_integer(tx["transactionIndex"]),
        gas_used: EthereumJSONRPC.quantity_to_integer(receipt["gasUsed"]),
        cumulative_gas_used: EthereumJSONRPC.quantity_to_integer(receipt["cumulativeGasUsed"]),
        status: :ok
      )

    assert receipt["status"] == "0x1"
    assert receipt["transactionHash"] == to_string(transaction.hash)
    path = "/api/v2/addresses/#{Address.checksum(address.hash)}/unified"
    body = conn |> get(path) |> json_response(200)

    assert Enum.sort(Map.keys(body)) == Enum.sort(Enum.map(UnifiedAccount.schema().required, &Atom.to_string/1))
    assert body["identities"] == %{"evm" => Address.checksum(address.hash), "pax" => nil, "did" => nil}

    assert [balance] = body["balances"]

    assert balance == %{
             "asset" => %{"id" => "native", "denom" => Explorer.coin(), "symbol" => Explorer.coin(), "decimals" => 18},
             "total" => "1000"
           }

    assert [activity] = body["activity"]

    assert activity == %{
             "kind" => "transaction",
             "hash" => tx["hash"],
             "block_number" => block.number,
             "ordinal" => EthereumJSONRPC.quantity_to_integer(tx["transactionIndex"]),
             "status" => "committed",
             "result" => "ok",
             "timestamp" => DateTime.to_iso8601(block.timestamp),
             "asset" => balance["asset"],
             "amount" => to_string(EthereumJSONRPC.quantity_to_integer(tx["value"])),
             "counterparty" => Address.checksum(counterparty.hash)
           }

    assert body["activity_total"] == 1
    assert body["page_number"] == 1
    assert body["next_page_params"] == nil
    assert is_binary(body["page_cursor"]) and byte_size(body["page_cursor"]) > 0
    assert is_binary(body["first_page_cursor"]) and byte_size(body["first_page_cursor"]) > 0

    spec =
      OpenApiSpex.resolve_schema_modules(%OpenApi{
        info: %Info{title: "Unified account repair 22.33.410", version: "1"},
        paths: %{},
        components: %Components{schemas: %{"PaxeerXUnifiedAccount" => UnifiedAccount.schema()}}
      })

    schema = spec.components.schemas["PaxeerXUnifiedAccount"]
    OpenApiSpex.TestAssertions.assert_raw_schema(body, schema, spec)
    assert {:ok, cast} = OpenApiSpex.cast_value(body, schema, spec)
    assert cast.identities.evm == Address.checksum(address.hash)
    assert [%{total: "1000"}] = cast.balances
    assert [%{hash: hash, kind: "transaction", result: "ok"}] = cast.activity
    assert hash == tx["hash"]

    for invalid <- [
          put_in(body, ["identities", "kernel_account"], "retired"),
          Map.put(body, "balances", [Map.put(balance, "parts", %{"chain" => "1000"})]),
          Map.put(body, "activity", [Map.put(activity, "side", "chain")]),
          Map.put(body, "activity", [Map.put(activity, "kind", "kernel_event")])
        ] do
      assert {:error, _} = OpenApiSpex.cast_value(invalid, schema, spec)
    end
  end
end
