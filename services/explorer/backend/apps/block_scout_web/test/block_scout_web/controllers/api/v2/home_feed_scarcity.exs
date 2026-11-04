defmodule BlockScoutWeb.API.V2.HomeFeedScarcity do
  import ExUnit.Assertions
  import Explorer.Factory
  import Phoenix.ConnTest, except: [json_response: 2]
  import BlockScoutWeb.TestApiSchemaAssertions, only: [json_response: 2]

  alias Explorer.Chain.PaxeerX.PriceFeed
  alias Explorer.PaxeerX.CosmosRest

  @endpoint BlockScoutWeb.Endpoint
  @fixtures Path.expand("../../../../../../explorer/test/support/fixture/paxeer_x/price_feed", __DIR__)

  def assert_page(conn, evm_count) do
    body = @fixtures |> Path.join("block_txs.json") |> File.read!() |> Jason.decode!()
    results = @fixtures |> Path.join("block_results.json") |> File.read!() |> Jason.decode!()
    raw = body["block"]["data"]["txs"]

    assert {:ok, submissions} = CosmosRest.decode_block_txs(%{"txs" => body["txs"], "raw" => raw})
    assert {:ok, [recorded]} = CosmosRest.decode_price_updates(submissions, results["txs_results"], length(raw))
    assert recorded.status == 1
    assert length(recorded.observations) == 18

    blocks =
      Map.new([101, 100, 103, 102], fn number ->
        block = insert(:block, number: number)

        updates =
          for index <- [0, 1] do
            %{recorded | hash: fixture_hash(number, index), index: index}
          end

        assert {:ok, 2} = PriceFeed.replace_block_updates(block, Enum.reverse(updates))
        {number, block}
      end)

    transactions =
      [{101, 1}, {103, 0}]
      |> Enum.take(evm_count)
      |> Enum.map(fn {number, index} ->
        insert(:transaction) |> with_block(blocks[number], index: index, status: :ok)
      end)

    evm_hashes = Enum.map(transactions, &to_string(&1.hash))

    expected =
      case evm_hashes do
        [] ->
          for {number, index} <- [{103, 1}, {103, 0}, {102, 1}, {102, 0}, {101, 1}, {101, 0}],
              do: fixture_hash(number, index)

        [older] ->
          [
            fixture_hash(103, 1),
            fixture_hash(103, 0),
            fixture_hash(102, 1),
            fixture_hash(102, 0),
            fixture_hash(101, 1),
            older
          ]

        [older, newer] ->
          [fixture_hash(103, 1), fixture_hash(103, 0), newer, fixture_hash(102, 1), fixture_hash(102, 0), older]
      end

    response = conn |> get("/api/v2/main-page/transactions") |> json_response(200)
    assert length(response) == 6
    assert Enum.map(response, & &1["hash"]) == expected
    assert length(Enum.uniq_by(response, & &1["hash"])) == 6

    {feeds, evm} = Enum.split_with(response, &(&1["transaction_types"] == ["price_update"]))
    assert length(feeds) == 6 - evm_count
    assert length(evm) == evm_count
    assert Enum.sort(Enum.map(evm, & &1["hash"])) == Enum.sort(evm_hashes)

    for transaction <- transactions do
      item = Enum.find(evm, &(&1["hash"] == to_string(transaction.hash)))
      assert item["block_number"] == transaction.block_number
      assert item["position"] == transaction.index
      assert item["status"] == "ok"
      assert item["value"] == to_string(transaction.value.value)
    end

    for feed <- feeds do
      assert feed["type"] == 127
      assert feed["hash"] == fixture_hash(feed["block_number"], feed["index"])
      assert feed["block_hash"] == to_string(blocks[feed["block_number"]].hash)
      assert feed["status"] == "ok"
      assert feed["method"] == "submitPrices"
      assert feed["price_update"]["feeder_bech32"] == recorded.feeder_bech32
      assert feed["price_update"]["observation_count"] == 18
      assert length(feed["price_update"]["observations"]) == 18

      assert hd(feed["price_update"]["observations"])["index_price"] ==
               to_string(hd(recorded.observations).index_price)
    end

    repeated = build_conn() |> get("/api/v2/main-page/transactions") |> json_response(200)
    assert Enum.map(repeated, & &1["hash"]) == expected
  end

  defp fixture_hash(number, index) do
    "0x" <> Base.encode16(:crypto.hash(:sha256, "home-feed-scarcity:#{number}:#{index}"), case: :lower)
  end
end
