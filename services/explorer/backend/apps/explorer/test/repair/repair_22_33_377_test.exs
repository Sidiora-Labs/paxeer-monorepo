defmodule Explorer.Repair.Repair2233377Test do
  @moduledoc """
  Task 22.33.377 repair record (audit 22.30.140).

  btc-config-owner: the configured BTC conversion and disabled BTC conversion
  tests drive source selection, HTTP, the adapter and the coin fetcher's ETS consumer.
  dia-task-exits: the killed quote and timed out quote tests drive the recorded
  asset catalog, HTTP quotes, the token fetcher's retry callback and token schema.

  Baseline verification was not run: the ordered gate instructions require the
  entire implementation before the sole hf-build/hf-done qualification sequence.
  No pre-change test failures are claimed. The runner's gate ledger records the
  verified candidate revision, command, exit and private evidence location.

  Wiring request (domain 14): services/explorer/backend/config/runtime.exs must
  declare fetch_btc_value under Explorer.Market.Fetcher.Coin.
  Wiring request (domain 14): registry row 14.cfg-explorer-explorer-market-fetcher-coin
  must retain Coin ownership and document the runtime flag.
  Wiring request (domain 14): services/explorer/backend/apps/explorer/test/explorer/market/source/coin_market_cap_test.exs
  must configure Explorer.Market.Fetcher.Coin.fetch_btc_value explicitly for its fiat-only fixtures.

  Provider payloads preserve the CoinMarketCap latest-quotes and DIA quotedAssets /
  assetQuotation response shapes used by the committed market source fixtures.
  Slow responses and worker death are injected at the loopback server and process
  boundary; production adapters, transport, task supervision and schemas run intact.
  """

  use ExUnit.Case, async: false

  import ExUnit.CaptureLog

  alias Explorer.Chain.{Hash, Token}
  alias Explorer.Market.Fetcher.Coin
  alias Explorer.Market.Fetcher.Token, as: TokenFetcher
  alias Explorer.Market.Source
  alias Explorer.Market.Source.{CoinMarketCap, DIA}
  alias Plug.Conn

  @usdc "0xa0b86991c6218b36c1d19d4a2e9eb0ce3606eb48"
  @usdt "0xdac17f958d2ee523a2206206994597c13d831ec7"
  @table :repair_22_33_377_exchange_rates
  @catalog """
  [
    {"Asset":{"Symbol":"USDC","Name":"USD Coin","Address":"0xA0b86991c6218b36c1d19D4a2e9Eb0cE3606eB48","Decimals":6,"Blockchain":"Ethereum"},"Volume":2756384237.21397,"VolumeUSD":0,"Index":0},
    {"Asset":{"Symbol":"USDT","Name":"Tether USD","Address":"0xdAC17F958D2ee523a2206206994597C13D831ec7","Decimals":6,"Blockchain":"Ethereum"},"Volume":504424765.759824,"VolumeUSD":0,"Index":0}
  ]
  """
  @cmc """
  {
    "status":{"timestamp":"2025-02-14T14:36:19.381Z","error_code":0,"error_message":null,"elapsed":40,"credit_count":1,"notice":null},
    "data":{"1":{"id":1,"name":"Bitcoin","symbol":"BTC","slug":"bitcoin","circulating_supply":19824162,"total_supply":19824162,"last_updated":"2025-02-14T14:34:00.000Z",
      "quote":{
        "1":{"price":1,"last_updated":"2025-02-14T14:34:00.000Z"},
        "2781":{"price":96743.361278,"volume_24h":28724591782.645985,"market_cap":1917742660653.5574,"tvl":null,"last_updated":"2025-02-14T14:34:00.000Z"}
      }
    }}
  }
  """

  setup do
    previous =
      for {app, key} <- [
            {:explorer, :http_client},
            {:explorer, Explorer.Market},
            {:explorer, Source},
            {:explorer, Coin},
            {:explorer, CoinMarketCap},
            {:explorer, DIA},
            {:tesla, :adapter}
          ] do
        {app, key, Application.fetch_env(app, key)}
      end

    on_exit(fn ->
      for {app, key, value} <- previous do
        case value do
          {:ok, config} -> Application.put_env(app, key, config)
          :error -> Application.delete_env(app, key)
        end
      end
    end)

    Application.put_env(:explorer, :http_client, Explorer.HttpClient.Tesla)
    Application.put_env(:tesla, :adapter, {Tesla.Adapter.Mint, timeout: 70_000})
    server = Bypass.open()
    url = "http://127.0.0.1:#{server.port}"

    Application.put_env(:explorer, DIA,
      base_url: url,
      blockchain: "Ethereum",
      coin_address_hash: "0x0000000000000000000000000000000000000000"
    )

    Application.put_env(:explorer, CoinMarketCap,
      base_url: url,
      coin_id: "1",
      secondary_coin_id: "1",
      currency_id: "2781"
    )

    Application.put_env(:explorer, Source, native_coin_source: CoinMarketCap, secondary_coin_source: CoinMarketCap)
    Application.put_env(:explorer, Coin, enabled: true, store: :ets, table_name: @table, enable_consolidation: false)
    :ets.new(@table, [:named_table, :public, :set])
    assert is_pid(Process.whereis(Explorer.MarketTaskSupervisor))
    %{server: server}
  end

  test "configured BTC conversion follows Coin ownership through HTTP and the cache", %{server: server} do
    assert_coin_conversion(server, true)
  end

  test "disabled BTC conversion ignores the old Market owner through HTTP and the cache", %{server: server} do
    assert_coin_conversion(server, false)
  end

  test "killed quote preserves the token fetcher's retry state and recovers", %{server: server} do
    catalog(server)
    assert {:ok, [failed, pending] = tokens, false, []} = DIA.fetch_tokens(nil, 0)
    assert to_string(failed.contract_address_hash) == @usdt
    assert to_string(pending.contract_address_hash) == @usdc

    observer = self()

    Bypass.expect_once(server, "GET", quote_path(@usdt), fn conn ->
      send(observer, {:blocked_quote, self()})

      receive do
        :release -> Conn.resp(conn, 200, quote_response(@usdt))
      end
    end)

    before = Task.Supervisor.children(Explorer.MarketTaskSupervisor)
    state = %TokenFetcher{source: DIA, source_state: tokens, max_batch_size: 1, interval: 10, refetch_interval: 100}

    log =
      capture_log(fn ->
        caller =
          Task.async(fn ->
            reply = TokenFetcher.handle_info({:fetch, 0}, state)

            retry =
              receive do
                {:fetch, 1} = message -> message
              after
                1_000 -> flunk("the token fetcher did not schedule its retry")
              end

            {reply, retry}
          end)

        assert_receive {:blocked_quote, handler}, 2_000
        [worker] = Task.Supervisor.children(Explorer.MarketTaskSupervisor) -- before
        monitor = Process.monitor(worker)
        Process.exit(worker, :kill)
        assert_receive {:DOWN, ^monitor, :process, ^worker, :killed}, 2_000
        assert {{:noreply, ^state}, {:fetch, 1}} = Task.await(caller, 2_000)
        send(handler, :release)
      end)

    assert log =~ ":killed"

    expect_quote(server, @usdt)
    expect_quote(server, @usdc)
    assert {:ok, [], true, recovered} = DIA.fetch_tokens(state.source_state, 2)
    assert Enum.map(recovered, &to_string(&1.contract_address_hash)) == [@usdt, @usdc]
    Enum.each(recovered, &assert_token_schema/1)
  end

  @tag timeout: 75_000
  test "timed out quote is killed within the deadline while successful quotes and retries survive", %{server: server} do
    catalog(server)
    expect_quote(server, @usdt)
    observer = self()

    Bypass.expect_once(server, "GET", quote_path(@usdc), fn conn ->
      send(observer, {:blocked_quote, self()})

      receive do
        :release -> Conn.resp(conn, 200, quote_response(@usdc))
      end
    end)

    started = System.monotonic_time(:millisecond)

    log =
      capture_log(fn ->
        caller = Task.async(fn -> DIA.fetch_tokens(nil, 2) end)
        assert_receive {:blocked_quote, handler}, 2_000
        assert {:ok, [failed], false, [imported]} = Task.await(caller, 65_000)
        elapsed = System.monotonic_time(:millisecond) - started
        assert elapsed >= 60_000
        assert elapsed < 65_000
        assert to_string(failed.contract_address_hash) == @usdc
        assert to_string(imported.contract_address_hash) == @usdt
        assert_token_schema(imported)
        send(handler, :release)

        expect_quote(server, @usdc)
        assert {:ok, [], true, [recovered]} = DIA.fetch_tokens([failed], 1)
        assert to_string(recovered.contract_address_hash) == @usdc
        assert_token_schema(recovered)
      end)

    assert log =~ ":timeout"
  end

  defp assert_coin_conversion(server, enabled) do
    Application.put_env(
      :explorer,
      Coin,
      Keyword.put(Application.fetch_env!(:explorer, Coin), :fetch_btc_value, enabled)
    )

    Application.put_env(:explorer, Explorer.Market, fetch_btc_value: not enabled)
    response = Jason.decode!(@cmc)
    response = if enabled, do: response, else: update_in(response, ["data", "1", "quote"], &Map.delete(&1, "1"))
    observer = self()

    Bypass.expect(server, "GET", "/cryptocurrency/quotes/latest", fn conn ->
      request = Conn.fetch_query_params(conn)

      assert request.query_params == %{
               "id" => "1",
               "convert_id" => if(enabled, do: "1,2781", else: "2781"),
               "aux" => "circulating_supply,total_supply"
             }

      send(observer, :coin_request)
      Conn.resp(conn, 200, Jason.encode!(response))
    end)

    assert Source.native_coin_source() == CoinMarketCap
    assert {:ok, native} = Source.native_coin_source().fetch_native_coin()
    expected_btc = if enabled, do: Decimal.new(1), else: nil
    assert native.btc_value == expected_btc
    assert native.fiat_value == Decimal.new("96743.361278")
    assert native.last_updated == ~U[2025-02-14 14:34:00.000Z]
    assert_receive :coin_request

    state = %Coin{secondary_coin_source: Source.secondary_coin_source()}
    assert {:noreply, ^state} = Coin.handle_info({:update, true}, state)
    assert_receive {ref, {{:ok, secondary}, true}} = reply, 2_000
    Process.demonitor(ref, [:flush])
    assert {:noreply, ^state} = Coin.handle_info(reply, state)
    assert_receive :coin_request
    assert secondary == native
    assert Coin.get_secondary_coin_exchange_rate() == secondary
    assert Coin.get_secondary_coin_exchange_rate().btc_value == expected_btc
  end

  defp catalog(server) do
    Bypass.expect_once(server, "GET", "/quotedAssets", fn conn ->
      assert Conn.fetch_query_params(conn).query_params == %{"blockchain" => "Ethereum"}
      Conn.resp(conn, 200, @catalog)
    end)
  end

  defp expect_quote(server, address) do
    Bypass.expect_once(server, "GET", quote_path(address), fn conn ->
      Conn.resp(conn, 200, quote_response(address))
    end)
  end

  defp quote_path(address), do: "/assetQuotation/Ethereum/#{address}"

  defp quote_response(address) do
    {symbol, name, price} =
      case address do
        @usdc -> {"USDC", "USD Coin", "0.9997188333561603"}
        @usdt -> {"USDT", "Tether USD", "0.9999313493241913"}
      end

    """
    {"Symbol":"#{symbol}","Name":"#{name}","Address":"#{address}","Blockchain":"Ethereum","Price":#{price},"PriceYesterday":2935.2004802640054,"VolumeYesterdayUSD":6577658642.868258,"Time":"2025-11-27T08:09:59Z","Source":"diadata.org"}
    """
  end

  defp assert_token_schema(params) do
    changeset = Token.changeset(%Token{}, params)
    assert changeset.valid?, inspect(changeset.errors)
    token = Ecto.Changeset.apply_changes(changeset)
    assert token.contract_address_hash == params.contract_address_hash
    address_hash = token.contract_address_hash
    assert {:ok, ^address_hash} = Hash.Address.cast(to_string(params.contract_address_hash))
    assert token.decimals == Decimal.new("6")
    assert token.type == "ERC-20"

    {symbol, name, price} =
      case to_string(token.contract_address_hash) do
        @usdc -> {"USDC", "USD Coin", "0.9997188333561603"}
        @usdt -> {"USDT", "Tether USD", "0.9999313493241913"}
      end

    assert token.symbol == symbol
    assert token.name == name
    assert token.fiat_value == Decimal.new(price)
  end
end
