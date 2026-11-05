defmodule Explorer.Repair.Repair2231389Test do
  use Explorer.DataCase

  alias Explorer.Chain.PaxeerX.{PriceFeed, PriceObservation}
  alias Explorer.PaxeerX.CosmosRest

  @fixtures Path.expand("../support/fixture/paxeer_x/price_feed", __DIR__)
  @uint64_max 18_446_744_073_709_551_615
  @int64_boundary 9_223_372_036_854_775_808
  @fields [:market_id, :sequence, :source_timestamp_ms]

  test "pricefeed_unsigned_sequence replays recorded node responses into the real persistence path" do
    body = fixture("block_txs.json")
    results = fixture("block_results.json")
    height = String.to_integer(results["height"])
    block = insert(:block, number: height)

    assert {:ok, [update]} = read_block(body, results, height)
    assert {:ok, 1} = PriceFeed.replace_block_updates(block, [update])
    assert [stored] = PriceFeed.block_updates(height)
    assert to_string(stored.hash) == update.hash
    assert length(stored.observations) == 18

    Enum.zip(update.observations, stored.observations)
    |> Enum.each(fn {decoded, %PriceObservation{} = persisted} ->
      for field <- @fields ++ [:accepted] do
        assert Map.fetch!(persisted, field) == Map.fetch!(decoded, field)
      end

      assert Decimal.equal?(persisted.index_price, decoded.index_price)
      assert Decimal.equal?(persisted.mark_price, decoded.mark_price)
    end)
  end

  test "pricefeed_unsigned_sequence retains uint64 boundaries through HTTP and refusal result decoding" do
    body =
      fixture("block_txs.json")
      |> put_observation_field(0, :sequence, to_string(@int64_boundary))
      |> put_observation_field(1, :sequence, to_string(@uint64_max))
      |> put_observation_field(1, :source_timestamp_ms, to_string(@uint64_max))
      |> put_observation_field(1, :market_id, to_string(@uint64_max))

    for encode <- [&Function.identity/1, &Base.encode64/1] do
      results =
        update_in(fixture("block_results.json"), ["txs_results", Access.at(0), "events"], fn events ->
          events ++
            [
              %{
                "type" => "pricefeed_observation_refused",
                "attributes" => [
                  %{"key" => encode.("market_id"), "value" => encode.(to_string(@uint64_max))},
                  %{"key" => encode.("sequence"), "value" => encode.(to_string(@uint64_max))},
                  %{"key" => encode.("reason"), "value" => encode.("unknown_market")}
                ]
              }
            ]
        end)

      assert {:ok, [%{status: 1, observations: observations}]} =
               read_block(body, results, String.to_integer(results["height"]))

      assert length(observations) == 18
      assert hd(observations).sequence == @int64_boundary
      assert hd(observations).accepted

      assert List.last(observations) == %{
               market_id: @uint64_max,
               sequence: @uint64_max,
               source_timestamp_ms: @uint64_max,
               index_price: 2_438_450_000_000_000_000_000,
               mark_price: 2_438_450_000_000_000_000_000,
               accepted: false
             }

      assert Enum.count(observations, & &1.accepted) == 17
    end
  end

  test "pricefeed_unsigned_sequence matches the uint64 domain for every observation field" do
    for field <- @fields,
        value <- [0, 2_147_483_648, @int64_boundary - 1, @int64_boundary, @uint64_max],
        representation <- [value, Integer.to_string(value)] do
      body = put_observation_field(fixture("block_txs.json"), 0, field, representation)
      assert {:ok, [submission]} = decode(body)
      observation = Enum.find(submission.observations, &(Map.fetch!(&1, field) == value))
      assert observation != nil
      assert Map.fetch!(observation, field) === value
      assert is_integer(Map.fetch!(observation, field))
    end
  end

  test "pricefeed_unsigned_sequence rejects overflow and noncanonical observation integers" do
    for field <- @fields,
        value <- [
          @uint64_max + 1,
          Integer.to_string(@uint64_max + 1),
          -1,
          "-1",
          "+1",
          "01",
          "1.0",
          "1e3",
          " 1",
          "1 ",
          "",
          nil,
          1.5
        ] do
      body = put_observation_field(fixture("block_txs.json"), 0, field, value)
      assert {:error, {:malformed, reason}} = decode(body)
      assert reason =~ "not an unsigned integer in range:"
    end
  end

  test "pricefeed_unsigned_sequence retains refused boundary observations in failed committed transactions" do
    body =
      fixture("block_txs.json")
      |> put_observation_field(0, :sequence, to_string(@int64_boundary))
      |> put_observation_field(0, :source_timestamp_ms, to_string(@int64_boundary))

    results =
      update_in(fixture("block_results.json"), ["txs_results", Access.at(0)], fn result ->
        Map.merge(result, %{"code" => 5, "gas_used" => "41"})
      end)

    assert {:ok, [%{status: 0, gas_used: 41, observations: observations}]} =
             read_block(body, results, String.to_integer(results["height"]))

    assert length(observations) == 18
    assert Enum.all?(observations, &(&1.accepted == false))
    assert hd(observations).sequence == @int64_boundary
    assert hd(observations).source_timestamp_ms == @int64_boundary
  end

  test "pricefeed_unsigned_sequence leaves the independent gas limit bound intact" do
    body =
      put_in(
        fixture("block_txs.json"),
        ["txs", Access.at(0), "auth_info", "fee", "gas_limit"],
        to_string(@int64_boundary)
      )

    assert {:error, {:malformed, _}} = decode(body)
  end

  defp fixture(name), do: @fixtures |> Path.join(name) |> File.read!() |> Jason.decode!()

  defp put_observation_field(body, index, field, value) do
    put_in(
      body,
      ["txs", Access.at(0), "body", "messages", Access.at(0), "observations", Access.at(index), Atom.to_string(field)],
      value
    )
  end

  defp decode(body),
    do: CosmosRest.decode_block_txs(%{"txs" => body["txs"], "raw" => body["block"]["data"]["txs"]})

  defp read_block(body, results, height) do
    server = Bypass.open()

    Bypass.expect_once(server, "GET", "/cosmos/tx/v1beta1/txs/block/#{height}", fn conn ->
      conn = Plug.Conn.fetch_query_params(conn)
      assert conn.query_params == %{"pagination.offset" => "0", "pagination.limit" => "100"}
      Plug.Conn.resp(conn, 200, Jason.encode!(body))
    end)

    Bypass.expect_once(server, "GET", "/block_results", fn conn ->
      conn = Plug.Conn.fetch_query_params(conn)
      assert conn.query_params == %{"height" => to_string(height)}
      Plug.Conn.resp(conn, 200, Jason.encode!(results))
    end)

    url = "http://localhost:#{server.port}"
    CosmosRest.block_price_updates([url], [url], height)
  end
end
