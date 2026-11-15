defmodule BlockScoutWeb.UnifiedAssetSchemaTest do
  @moduledoc "Unified asset schema."
  use BlockScoutWeb.ConnCase

  alias BlockScoutWeb.Schemas.API.V2.PaxeerX.UnifiedAccount.{Asset, AssetNullable}
  alias Explorer.Chain.Address

  @recording Path.expand(
               "../../../ethereum_jsonrpc/test/support/fixture/geth/trace/struct_logger.json",
               __DIR__
             )

  test "unified-asset-description: published schema describes native and ERC-20 identifiers" do
    schema = Asset.schema()
    published = schema |> OpenApiSpex.OpenApi.to_map() |> Jason.encode!() |> Jason.decode!()

    assert published["title"] == "PaxeerXAsset"
    assert published["description"] =~ "native coin or an ERC-20 token on the chain"
    assert published["description"] =~ "18 decimals"

    assert published["properties"]["id"]["description"] ==
             "`native` for the coin, or the checksummed ERC-20 token contract address"

    assert published["properties"]["denom"]["description"] =~
             "token symbol when known, otherwise the checksummed token contract address"

    assert published["properties"]["symbol"]["description"] =~ "Configured coin symbol"
    assert published["properties"]["symbol"]["description"] =~ "null when unknown"
    assert published["properties"]["decimals"]["description"] =~ "18 for the native coin"
    assert published["properties"]["decimals"]["description"] =~ "null when unknown"
    refute Jason.encode!(published) =~ ~r/\b(custody|kernel)\b/i

    assert Enum.sort(schema.required) == [:decimals, :denom, :id, :symbol]
    assert Enum.sort(Map.keys(schema.properties)) == Enum.sort(schema.required)
    assert schema.additionalProperties == false
    assert schema.nullable == false
    assert schema.properties.id.nullable == false
    assert schema.properties.denom.nullable == false
    assert schema.properties.symbol.nullable == true
    assert schema.properties.decimals.nullable == true
    assert AssetNullable.schema().properties == schema.properties
    assert AssetNullable.schema().example == schema.example
    assert AssetNullable.schema().nullable == true
  end

  test "unified-asset-description: recorded chain assets match the routed response and published example",
       %{conn: conn} do
    previous_coin = Application.fetch_env!(:explorer, :coin)
    Application.put_env(:explorer, :coin, "PAX")
    on_exit(fn -> Application.put_env(:explorer, :coin, previous_coin) end)

    recording = @recording |> File.read!() |> Jason.decode!()
    tx = recording["tx"]
    receipt = recording["receipt"]
    [known_log, unknown_log | _] = receipt["logs"]
    address = insert(:address, hash: tx["from"], fetched_coin_balance: 1_000)
    counterparty = insert(:address, hash: tx["to"])

    block =
      insert(:block,
        hash: tx["blockHash"],
        number: EthereumJSONRPC.quantity_to_integer(tx["blockNumber"])
      )

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
        status: :ok
      )

    assert receipt["transactionHash"] == to_string(transaction.hash)
    assert receipt["status"] == "0x1"

    known =
      insert(:token,
        contract_address: insert(:address, hash: known_log["address"]),
        type: "ERC-20",
        symbol: "TOKEN",
        decimals: 6
      )

    unknown =
      insert(:token,
        contract_address: insert(:address, hash: unknown_log["address"]),
        type: "ERC-20",
        symbol: nil,
        decimals: nil
      )

    for {token, value} <- [{known, 250}, {unknown, 500}] do
      insert(:address_current_token_balance,
        address: address,
        token_contract_address_hash: token.contract_address_hash,
        token_type: token.type,
        value: value
      )
    end

    response =
      conn
      |> get("/api/v2/addresses/#{Address.checksum(address.hash)}/unified")
      |> json_response(200)

    balances = Map.new(response["balances"], &{&1["asset"]["id"], &1})
    known_id = Address.checksum(known.contract_address_hash)
    unknown_id = Address.checksum(unknown.contract_address_hash)

    assert Enum.sort(Map.keys(balances)) == Enum.sort(["native", known_id, unknown_id])

    assert balances["native"] == %{
             "asset" => %{
               "id" => "native",
               "denom" => Explorer.coin(),
               "symbol" => Explorer.coin(),
               "decimals" => 18
             },
             "total" => "1000"
           }

    assert balances[known_id] == %{
             "asset" => %{
               "id" => known_id,
               "denom" => "TOKEN",
               "symbol" => "TOKEN",
               "decimals" => 6
             },
             "total" => "250"
           }

    assert balances[unknown_id] == %{
             "asset" => %{
               "id" => unknown_id,
               "denom" => unknown_id,
               "symbol" => nil,
               "decimals" => nil
             },
             "total" => "500"
           }

    assert [activity] = response["activity"]
    assert activity["hash"] == tx["hash"]
    assert activity["asset"] == balances["native"]["asset"]
    assert activity["amount"] == to_string(EthereumJSONRPC.quantity_to_integer(tx["value"]))

    published =
      Asset.schema() |> OpenApiSpex.OpenApi.to_map() |> Jason.encode!() |> Jason.decode!()

    assert published["example"] == balances["native"]["asset"]

    for asset <-
          Enum.map(response["balances"], & &1["asset"]) ++
            [activity["asset"], published["example"]] do
      OpenApiSpex.TestAssertions.assert_raw_schema(asset, Asset.schema(), %{})
      assert {:ok, cast} = OpenApiSpex.cast_value(asset, Asset.schema())
      assert cast.id == asset["id"]
      assert cast.denom == asset["denom"]
      assert cast.symbol == asset["symbol"]
      assert cast.decimals == asset["decimals"]
    end
  end
end
