defmodule BlockScoutWeb.API.V2.PaxeerX.Repair2233416Evidence do
  import ExUnit.Assertions
  import ExUnit.Callbacks

  alias EthereumJSONRPC.HTTP

  @price_fixtures Path.expand("../../../../../../../explorer/test/support/fixture/paxeer_x/price_feed", __DIR__)
  @capability_fixtures Path.expand(
                         "../../../../../../../explorer/test/explorer/chain/paxeer_x/fixtures/launchpad_rpc",
                         __DIR__
                       )

  def abi do
    directory =
      System.get_env("PAXEER_PRECOMPILES_DIR") || Path.expand("../../../../../../../../../../../precompiles", __DIR__)

    directory |> Path.join("addr/abi.json") |> File.read!() |> Jason.decode!()
  end

  def fixture(name), do: @price_fixtures |> Path.join(name) |> File.read!() |> Jason.decode!()

  def unified_account_answer do
    [selector] = abi() |> ABI.parse_specification() |> Enum.filter(&(&1.function == "getUnifiedAccount"))
    assert selector.types == [:address]
    assert selector.returns == [:address, :string, {:bytes, 32}, {:bytes, 32}]
    assert selector.method_id == <<0x35, 0x7F, 0xEE, 0xD6>>
    encoded = ABI.TypeEncoder.encode([<<0::160>>, "", <<0::256>>, <<0::256>>], selector.returns)
    assert <<0::256, 128::256, 0::256, 0::256, 0::256>> == encoded

    transcript = @capability_fixtures |> Path.join("live.json") |> File.read!() |> Jason.decode!()

    exchange =
      Enum.find(transcript["exchanges"], fn
        %{"request" => %{"method" => "eth_call", "params" => [%{"to" => to}, _height]}} -> to == addr()
        _exchange -> false
      end)

    assert exchange["request"]["params"] == [%{"to" => addr(), "data" => calldata()}, "0x16C1320"]
    assert exchange["response"]["result"] == hex(encoded)
    exchange["response"]["result"]
  end

  def addr, do: "0x0000000000000000000000000000000000001004"
  def calldata, do: "0x357feed6" <> String.duplicate("0", 64)
  def hex(bytes), do: "0x" <> Base.encode16(bytes, case: :lower)
  def bytes("0x" <> digits), do: Base.decode16!(digits, case: :mixed)

  def rpc_arguments(bypass) do
    url = "http://127.0.0.1:#{bypass.port}"

    [
      transport: HTTP,
      transport_options: [
        url: url,
        urls: [url],
        fallback_urls: [],
        http: HTTP.HTTPoison,
        http_options: [timeout: 5_000, recv_timeout: 5_000]
      ]
    ]
  end

  def capability_node(answer) do
    bypass = Bypass.open()
    owner = self()

    calls = %{
      "0x0000000000000000000000000000000000001015" => "0xb71d031c" <> String.duplicate("0", 64),
      "0x0000000000000000000000000000000000001016" => "0xb187bd26",
      "0x0000000000000000000000000000000000001017" => "0x8eec5d70"
    }

    Bypass.expect(bypass, "POST", "/", fn conn ->
      {:ok, body, conn} = Plug.Conn.read_body(conn)

      response =
        case Jason.decode!(body) do
          %{"method" => "eth_blockNumber", "params" => [], "id" => id} ->
            %{jsonrpc: "2.0", id: id, result: "0x16C1320"}

          requests when is_list(requests) ->
            assert length(requests) == 4
            assert length(Enum.uniq_by(requests, & &1["id"])) == 4

            Enum.map(requests, fn %{"method" => "eth_call", "params" => [call, "0x16C1320"], "id" => id} ->
              result =
                if call["to"] == addr() do
                  assert call["data"] == calldata()
                  send(owner, :canonical_addr_call)
                  answer
                else
                  assert call["data"] == Map.fetch!(calls, call["to"])
                  %{result: "0x" <> String.duplicate("0", 64)}
                end

              Map.merge(%{jsonrpc: "2.0", id: id}, result)
            end)
        end

      conn |> Plug.Conn.put_resp_content_type("application/json") |> Plug.Conn.resp(200, Jason.encode!(response))
    end)

    bypass
  end

  def feed_node(results \\ fixture("block_results.json")) do
    bypass = Bypass.open()
    txs = fixture("block_txs.json")
    markets = fixture("markets.json")
    owner = self()

    Bypass.expect(bypass, fn conn ->
      response =
        case {conn.method, conn.request_path} do
          {"GET", "/cosmos/tx/v1beta1/txs/block/" <> height} ->
            assert conn.query_string == "pagination.offset=0&pagination.limit=100"
            send(owner, {:price_block_read, String.to_integer(height)})
            txs

          {"GET", "/block_results"} ->
            assert conn.query_string =~ "height="
            results

          {"POST", "/"} ->
            {:ok, body, _conn} = Plug.Conn.read_body(conn)

            Enum.map(Jason.decode!(body), fn
              %{
                "id" => id,
                "method" => "eth_call",
                "params" => [%{"to" => "0x0000000000000000000000000000000000001020", "data" => "0xfd69f3c2"}, "latest"]
              } ->
                %{jsonrpc: "2.0", id: id, result: markets["getMarketCount"]}

              %{
                "id" => id,
                "method" => "eth_call",
                "params" => [
                  %{"to" => "0x0000000000000000000000000000000000001020", "data" => "0xeb44fdd3" <> word},
                  "latest"
                ]
              } ->
                {market, ""} = Integer.parse(word, 16)
                %{jsonrpc: "2.0", id: id, result: Map.fetch!(markets["getMarket"], to_string(market))}
            end)
        end

      conn |> Plug.Conn.put_resp_content_type("application/json") |> Plug.Conn.resp(200, Jason.encode!(response))
    end)

    state = %{
      json_rpc_named_arguments: rpc_arguments(bypass),
      cosmos_rest_urls: ["http://127.0.0.1:#{bypass.port}"],
      cometbft_rpc_urls: ["http://127.0.0.1:#{bypass.port}"],
      start_block: 10,
      precompile: "0x0000000000000000000000000000000000001020",
      markets_refresh_seconds: 300,
      markets_attempted_at: Indexer.Fetcher.PaxeerX.PriceUpdates.refresh_clock()
    }

    state
  end

  def restart_endpoint do
    original = Application.fetch_env(:block_scout_web, BlockScoutWeb.Endpoint)
    on_exit(fn -> restore_env(:block_scout_web, BlockScoutWeb.Endpoint, original) end)
    {:ok, config} = original
    http = config |> Keyword.fetch!(:http) |> Keyword.put(:port, 0)
    Application.put_env(:block_scout_web, BlockScoutWeb.Endpoint, Keyword.put(config, :http, http))
    previous = Process.whereis(BlockScoutWeb.Endpoint)
    assert is_pid(previous)
    assert :ok = Supervisor.terminate_child(BlockScoutWeb.Supervisor, BlockScoutWeb.Endpoint)
    refute Process.alive?(previous)
    assert {:ok, reopened} = Supervisor.restart_child(BlockScoutWeb.Supervisor, BlockScoutWeb.Endpoint)
    assert reopened != previous
    assert :ranch.get_port(BlockScoutWeb.Endpoint.HTTP) > 0
  end

  def restore_env(app, key, :error), do: Application.delete_env(app, key)
  def restore_env(app, key, {:ok, value}), do: Application.put_env(app, key, value)

  def wait_until(done?, milliseconds \\ 5_000)

  def wait_until(done?, remaining) when remaining > 0 do
    if done?.() do
      :ok
    else
      Process.sleep(10)
      wait_until(done?, remaining - 10)
    end
  end

  def wait_until(_done?, _remaining), do: flunk("producer did not reach its durable checkpoint in time")
end
