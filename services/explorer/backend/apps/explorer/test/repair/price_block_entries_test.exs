defmodule Explorer.Repair.PriceBlockEntriesTest do
  use ExUnit.Case, async: false

  import Bitwise

  alias Explorer.PaxeerX.CosmosRest

  @type_url "/paxprotocol.paxchain.pricefeed.MsgSubmitPrices"
  @feeder "pax1h9lpwllxuuqakt8ctsd6zvrk3qtxxxxvyfs9jc"
  @feeder_hash "0xb97e177fe6e701db2cf85c1ba1307688166318cc"
  @height 1_000
  @batch "PAXEER/v1/sequence-batch\0" <> <<1, 2, 3>>
  @backend Path.expand("../../../..", __DIR__)

  setup do
    node_context()
  end

  test "joins a submission after a sequence batch by its entry position", context do
    tx = submission()

    refusal = %{
      "type" => "pricefeed_observation_refused",
      "attributes" => [%{"key" => "market_id", "value" => "7"}]
    }

    serve(context, [@batch, tx], [%{"code" => 9}, %{"gas_used" => "31", "events" => [refusal]}])

    assert {:ok, [update]} = read(context)
    assert update.hash == hash(tx)
    assert update.index == 1
    assert update.feeder_bech32 == @feeder
    assert update.feeder_hash == @feeder_hash
    assert update.gas_wanted == 200_000
    assert update.gas_used == 31
    assert update.status == 1

    assert update.observations == [
             %{market_id: 0, sequence: 0, index_price: 101, mark_price: 102, source_timestamp_ms: 0, accepted: true},
             %{
               market_id: 7,
               sequence: 12,
               index_price: 201,
               mark_price: 202,
               source_timestamp_ms: 1_000,
               accepted: false
             }
           ]
  end

  test "a block containing only a sequence batch asks for no results", context do
    serve(context, [@batch])
    assert {:ok, []} = read(context)
  end

  test "empty and absent entry lists ask for no results" do
    for data <- [%{"txs" => []}, %{"txs" => nil}, %{}] do
      context = node_context()

      Bypass.expect_once(context.server, "GET", "/block", fn conn ->
        Plug.Conn.resp(conn, 200, Jason.encode!(%{"block" => %{"data" => data}}))
      end)

      assert {:ok, []} = read(context)
    end
  end

  test "another message between submissions preserves indexes zero and two", context do
    first = submission()
    second = submission(300_000)
    other = transaction(message("/cosmos.bank.v1beta1.MsgSend", <<>>), 1)
    serve(context, [first, other, second], [%{}, %{"code" => 8}, %{"code" => 5, "gas_used" => "41"}])

    assert {:ok, [one, two]} = read(context)
    assert {one.index, two.index} == {0, 2}
    assert {one.hash, two.hash} == {hash(first), hash(second)}
    assert {one.status, two.status} == {1, 0}
    assert two.gas_wanted == 300_000
    assert two.gas_used == 41
    assert Enum.all?(two.observations, &(&1.accepted == false))
  end

  test "a consensus response without a result wrapper reads the same entries", context do
    tx = submission()
    serve(context, [@batch, tx], [%{}, %{}], false)
    assert {:ok, [%{index: 1, hash: tx_hash, feeder_bech32: @feeder, gas_wanted: 200_000}]} = read(context)
    assert tx_hash == hash(tx)
  end

  test "a truncated submission is malformed", context do
    value = bytes(1, @feeder) <> <<18, 10, 8>>
    serve(context, [transaction(message(@type_url, value), 0)])
    assert {:error, {:malformed, _}} = read(context)
  end

  test "a truncated transaction carrying the submission type is malformed", context do
    tx = submission()
    serve(context, [binary_part(tx, 0, byte_size(tx) - 1)])
    assert {:error, {:malformed, _}} = read(context)
  end

  test "submissions require a feeder and observations" do
    for value <- [<<>>, bytes(1, @feeder), bytes(2, observation(7, 1, "1", "2", 3))] do
      context = node_context()
      serve(context, [transaction(message(@type_url, value), 0)])
      assert {:error, {:malformed, _}} = read(context)
    end
  end

  test "the last observation for each market is returned in market order", context do
    value =
      bytes(1, @feeder) <>
        bytes(2, observation(7, 1, "3", "4", 5)) <>
        bytes(2, observation(0, 2, "6", "7", 8)) <>
        bytes(2, observation(7, 9, "10", "11", 12))

    serve(context, [transaction(message(@type_url, value), 0)], [%{}])
    assert {:ok, [update]} = read(context)
    assert update.gas_wanted == 0
    assert Enum.map(update.observations, &{&1.market_id, &1.sequence, &1.index_price}) == [{0, 2, 6}, {7, 9, 10}]
  end

  test "block results must cover every entry including the sequence batch", context do
    serve(context, [@batch, submission()], [%{}])
    assert {:error, {:malformed, _}} = read(context)
  end

  test "REST failures do not affect consensus block reads", context do
    rest_calls = :counters.new(1, [])

    Bypass.stub(context.server, "GET", "/cosmos", fn conn ->
      assert String.starts_with?(conn.request_path, "/cosmos")
      :counters.add(rest_calls, 1, 1)
      Plug.Conn.resp(conn, 500, "{}")
    end)

    serve(context, [submission()], [%{}])
    assert {:ok, [%{index: 0, feeder_bech32: @feeder}]} = read(context, [context.url <> "/cosmos"])
    assert :counters.get(rest_calls, 1) == 0
  end

  test "block reads classify pruned and ahead heights and fail over from unreachable nodes", context do
    for {text, kind} <- [
          {"height 1000 is not available, lowest height is 1050", :height_unavailable},
          {"height is not available (requested height: 1000, base height: 1050)", :height_unavailable},
          {"height 1000 must be less than or equal to the current blockchain height 950", :height_ahead}
        ] do
      context = node_context()

      Bypass.expect(context.server, "GET", "/block", fn conn ->
        body = %{error: %{code: -32603, message: "Internal error", data: text}}
        Plug.Conn.resp(conn, 200, Jason.encode!(body))
      end)

      assert {:error, {^kind, _}} = read(context)
    end

    down = Bypass.open()
    Bypass.down(down)
    down_url = "http://localhost:#{down.port}"
    assert {:error, {:unreachable, ^down_url}} = CosmosRest.block_price_updates([], [down_url], @height)
    serve(context, [submission()], [%{}])
    assert {:ok, [_]} = CosmosRest.block_price_updates([], [down_url, context.url], @height)
  end

  test "price feed runtime configuration accepts empty REST URLs and validates supplied URLs" do
    names = [
      "PAXEER_X_PRICE_FEED_ENABLED",
      "PAXEER_X_PRICE_FEED_START_BLOCK",
      "PAXEER_X_COSMOS_REST_URLS",
      "PAXEER_X_COMETBFT_RPC_URLS"
    ]

    previous = Map.new(names, &{&1, System.get_env(&1)})

    on_exit(fn ->
      for {name, value} <- previous do
        if value, do: System.put_env(name, value), else: System.delete_env(name)
      end
    end)

    System.put_env("PAXEER_X_PRICE_FEED_ENABLED", "true")
    System.put_env("PAXEER_X_PRICE_FEED_START_BLOCK", "0")
    System.put_env("PAXEER_X_COMETBFT_RPC_URLS", "http://localhost:26657")
    System.put_env("PAXEER_X_COSMOS_REST_URLS", "")
    config = runtime_config()
    assert config[:indexer][Indexer.Fetcher.PaxeerX.PriceUpdates][:cosmos_rest_urls] == []
    assert config[:indexer][Indexer.Fetcher.PaxeerX.PriceUpdates.Supervisor][:disabled?] == false

    for invalid <- ["localhost:1317", "ftp://localhost:1317", "http://"] do
      System.put_env("PAXEER_X_COSMOS_REST_URLS", invalid)
      assert_raise ArgumentError, ~r/PAXEER_X_COSMOS_REST_URLS/, &runtime_config/0
    end

    System.put_env("PAXEER_X_COSMOS_REST_URLS", "https://localhost:1317,http://localhost:1318/")

    assert runtime_config()[:indexer][Indexer.Fetcher.PaxeerX.PriceUpdates][:cosmos_rest_urls] ==
             ["https://localhost:1317", "http://localhost:1318"]
  end

  defp runtime_config do
    File.cd!(@backend, fn -> Config.Reader.read!("config/runtime.exs", env: :test, target: :host) end)
  end

  defp node_context do
    server = Bypass.open()
    %{server: server, url: "http://localhost:#{server.port}"}
  end

  defp read(context, rest_urls \\ []), do: CosmosRest.block_price_updates(rest_urls, [context.url], @height)

  defp serve(context, entries, results \\ nil, wrapped \\ true) do
    block = %{"block" => %{"data" => %{"txs" => Enum.map(entries, &Base.encode64/1)}}}

    Bypass.expect_once(context.server, "GET", "/block", fn conn ->
      assert conn.query_string == "height=#{@height}"
      Plug.Conn.resp(conn, 200, Jason.encode!(if(wrapped, do: %{"result" => block}, else: block)))
    end)

    if results do
      Bypass.expect_once(context.server, "GET", "/block_results", fn conn ->
        assert conn.query_string == "height=#{@height}"
        response = %{"txs_results" => results}
        Plug.Conn.resp(conn, 200, Jason.encode!(if(wrapped, do: %{"result" => response}, else: response)))
      end)
    end
  end

  defp submission(gas \\ 200_000) do
    # The zero market, sequence and source time use the proto3 omitted-field form.
    value =
      bytes(1, @feeder) <>
        bytes(2, bytes(3, "101") <> bytes(4, "102")) <>
        bytes(2, observation(7, 12, "201", "202", 1_000))

    transaction(message(@type_url, value), gas)
  end

  defp transaction(message, gas) do
    bytes(1, bytes(1, message)) <> bytes(2, bytes(2, number(2, gas))) <> bytes(3, <<1, 2, 3>>)
  end

  defp message(type, value), do: bytes(1, type) <> bytes(2, value)

  defp observation(market, sequence, index, mark, time) do
    number(1, market) <> number(2, sequence) <> bytes(3, index) <> bytes(4, mark) <> number(5, time)
  end

  defp hash(bytes), do: "0x" <> Base.encode16(:crypto.hash(:sha256, bytes), case: :lower)
  defp bytes(field, value), do: varint(field <<< 3 ||| 2) <> varint(byte_size(value)) <> value
  defp number(_field, 0), do: <<>>
  defp number(field, value), do: varint(field <<< 3) <> varint(value)
  defp varint(value) when value < 128, do: <<value>>
  defp varint(value), do: <<(value &&& 127) ||| 128>> <> varint(value >>> 7)
end
