defmodule Explorer.Market.Source.ChainflowHistoryTest do
  use Explorer.DataCase

  alias Explorer.Chain
  alias Explorer.Market
  alias Explorer.Market.Fetcher.{Coin, History}
  alias Explorer.Market.{MarketHistory, Source}
  alias Explorer.Market.Source.Chainflow
  alias Plug.Conn

  setup do
    bypass = Bypass.open()
    key = Base.encode64(:crypto.strong_rand_bytes(24))
    dates = Enum.map(2..0//-1, &Date.add(Date.utc_today(), -&1))
    times = Enum.map(dates, &DateTime.to_unix(DateTime.new!(&1, ~T[00:00:00])))

    settings = [
      {:explorer, Chainflow,
       [
         base_url: "http://localhost:#{bypass.port}",
         api_key: key,
         coin_id: "paxeer",
         secondary_coin_id: "sidiora",
         coin_symbol: "PAX",
         secondary_coin_symbol: "SID"
       ]},
      {:explorer, Source,
       [
         native_coin_history_source: Chainflow,
         secondary_coin_history_source: Chainflow,
         market_cap_history_source: Chainflow,
         tvl_history_source: Chainflow
       ]},
      {:explorer, Coin, [store: :ets, enabled: true, table_name: :chainflow_history_rates]},
      {:explorer, History, [first_fetch_day_count: 3, history_fetch_interval: 60_000]},
      {:explorer, :supply, Explorer.Chain.Supply.ExchangeRate},
      {:tesla, :adapter, Tesla.Adapter.Mint}
    ]

    previous = Enum.map(settings, fn {app, name, _value} -> {app, name, Application.get_env(app, name)} end)
    Enum.each(settings, fn {app, name, value} -> Application.put_env(app, name, value) end)
    :ets.new(:chainflow_history_rates, [:set, :named_table, :public])
    on_exit(fn -> Enum.each(previous, fn {app, name, value} -> restore(app, name, value) end) end)

    data =
      start_supervised!(
        {Agent,
         fn ->
           %{
             supply: "125.5",
             coin_status: 200,
             candles: %{"s" => "ok", "t" => times, "o" => ["1", "2", "3"], "c" => ["2.5", "4", "0.125"]}
           }
         end}
      )

    owner = self()

    Bypass.expect(bypass, fn conn ->
      conn = Conn.fetch_query_params(conn)
      assert Conn.get_req_header(conn, "x-api-key") == [key]
      send(owner, {:request, conn.request_path, conn.query_params})
      values = Agent.get(data, & &1)

      {status, body} =
        case conn.request_path do
          "/api/v3/coins/paxeer" ->
            {values.coin_status,
             %{
               "market_data" => %{
                 "current_price" => %{"usd" => "99"},
                 "market_cap" => %{"usd" => "999999"},
                 "circulating_supply" => values.supply,
                 "total_supply" => "9999999"
               }
             }}

          "/api/v3/coins/sidiora" ->
            {200, %{"market_data" => %{"circulating_supply" => "400"}}}

          "/api/" <> market ->
            assert market in ["pax/history", "sid/history"]
            assert conn.query_params["symbol"] == if(market == "pax/history", do: "PAX", else: "SID")
            assert conn.query_params["resolution"] == "1D"

            assert String.to_integer(conn.query_params["to"]) - String.to_integer(conn.query_params["from"]) ==
                     3 * 86_400

            {200, values.candles}
        end

      conn |> Conn.put_resp_content_type("application/json") |> Conn.resp(status, Jason.encode!(body))
    end)

    {:ok, data: data, dates: dates}
  end

  test "native price and market value histories use feed supply without a cached coin", %{dates: dates} do
    assert Coin.get_coin_exchange_rate() == nil
    assert Market.get_coin_exchange_rate().available_supply == nil
    assert Chain.circulating_supply() == nil

    assert {:ok, prices} = Chainflow.fetch_native_coin_price_history(3)
    assert Enum.map(prices, & &1.date) == dates
    assert Enum.map(prices, &Decimal.to_string(&1.closing_price, :normal)) == ["2.5", "4", "0.125"]
    assert Enum.map(prices, &Decimal.to_string(&1.opening_price, :normal)) == ["1", "2", "3"]
    assert Enum.all?(prices, &(&1.secondary_coin == false))
    assert_values(prices, ["313.75", "502", "15.6875"])
    assert_received {:request, "/api/v3/coins/paxeer", %{}}
    assert_received {:request, "/api/pax/history", _}

    assert {:ok, caps} = Chainflow.fetch_market_cap_history(3)
    assert caps == Enum.map(prices, &Map.take(&1, [:date, :market_cap]))
    assert_values(caps, ["313.75", "502", "15.6875"])
    assert_received {:request, "/api/v3/coins/paxeer", %{}}
    assert_received {:request, "/api/pax/history", _}
    assert Coin.get_coin_exchange_rate() == nil
  end

  test "feed supply takes precedence over explorer held supply" do
    Application.put_env(:explorer, :supply, Explorer.Chain.Supply.ProofOfAuthority)
    refute Decimal.equal?(Chain.circulating_supply(), "125.5")
    assert {:ok, prices} = Chainflow.fetch_native_coin_price_history(3)
    assert_values(prices, ["313.75", "502", "15.6875"])
    assert {:ok, caps} = Chainflow.fetch_market_cap_history(3)
    assert_values(caps, ["313.75", "502", "15.6875"])
  end

  test "null feed supply uses explorer held supply for every history point", %{data: data, dates: dates} do
    Agent.update(data, &%{&1 | supply: nil})
    Application.put_env(:explorer, :supply, Explorer.Chain.Supply.ProofOfAuthority)
    supply = Chain.circulating_supply()
    assert supply > 0

    assert {:ok, prices} = Chainflow.fetch_native_coin_price_history(3)
    assert {:ok, caps} = Chainflow.fetch_market_cap_history(3)
    assert Enum.map(prices, & &1.date) == dates
    assert Enum.map(caps, & &1.date) == dates
    expected = Enum.map(["2.5", "4", "0.125"], &Decimal.mult(&1, supply))
    assert_values(prices, expected)
    assert_values(caps, expected)
  end

  test "market values stay empty when feed and held supply are absent", %{data: data, dates: dates} do
    Agent.update(data, &%{&1 | supply: nil})
    assert Chain.circulating_supply() == nil
    assert {:ok, prices} = Chainflow.fetch_native_coin_price_history(3)
    assert {:ok, caps} = Chainflow.fetch_market_cap_history(3)
    assert Enum.map(prices, & &1.date) == dates
    assert Enum.map(caps, & &1.date) == dates
    assert Enum.map(prices, & &1.market_cap) == [nil, nil, nil]
    assert Enum.map(caps, & &1.market_cap) == [nil, nil, nil]
  end

  test "zero feed supply produces zero market values", %{data: data} do
    Agent.update(data, &%{&1 | supply: 0})
    Application.put_env(:explorer, :supply, Explorer.Chain.Supply.ProofOfAuthority)
    assert {:ok, prices} = Chainflow.fetch_native_coin_price_history(3)
    assert {:ok, caps} = Chainflow.fetch_market_cap_history(3)
    assert_values(prices, [0, 0, 0])
    assert_values(caps, [0, 0, 0])
  end

  test "coin request errors propagate instead of using held supply", %{data: data} do
    Agent.update(data, &%{&1 | coin_status: 401})
    Application.put_env(:explorer, :supply, Explorer.Chain.Supply.ProofOfAuthority)
    assert {:error, "401: " <> _} = Chainflow.fetch_native_coin_price_history(3)
    assert {:error, "401: " <> _} = Chainflow.fetch_market_cap_history(3)
  end

  test "each startup stores market values for the full range including existing empty rows", context do
    [oldest, middle, _today] = context.dates

    Repo.insert!(%MarketHistory{
      date: oldest,
      opening_price: Decimal.new("1"),
      closing_price: Decimal.new("2.5"),
      market_cap: nil
    })

    Repo.insert!(%MarketHistory{
      date: middle,
      opening_price: Decimal.new("2"),
      closing_price: Decimal.new("4"),
      market_cap: Decimal.new("1")
    })

    assert Coin.get_coin_exchange_rate() == nil
    pid = start_supervised!({History, []})
    await_cycle(pid)
    assert_stored_values(context.dates, ["313.75", "502", "15.6875"])
    secondary = stored_history(true)
    assert Enum.map(secondary, & &1.date) == context.dates
    assert_values(secondary, ["1000", "1600", "50"])
    stop_supervised!(History)

    Repo.update_all(from(h in MarketHistory, where: not h.secondary_coin and h.date == ^oldest),
      set: [market_cap: nil]
    )

    Agent.update(context.data, &%{&1 | supply: "200"})
    assert MarketHistory.price_at_date(oldest).market_cap == nil
    pid = start_supervised!({History, []})
    await_cycle(pid)
    assert_stored_values(context.dates, ["500", "800", "25"])
    assert stored_history(true) == secondary
    assert Coin.get_coin_exchange_rate() == nil
  end

  defp assert_values(records, expected) do
    assert length(records) == length(expected)

    Enum.zip(records, expected)
    |> Enum.each(fn {record, value} -> assert Decimal.equal?(record.market_cap, value) end)
  end

  defp assert_stored_values(dates, expected) do
    records = stored_history(false)
    assert Enum.map(records, & &1.date) == dates
    assert_values(records, expected)
    assert Enum.map(records, &Decimal.to_string(&1.closing_price, :normal)) == ["2.5", "4", "0.125"]
  end

  defp stored_history(secondary_coin?) do
    Repo.all(from(h in MarketHistory, where: h.secondary_coin == ^secondary_coin?, order_by: h.date))
  end

  defp await_cycle(pid, remaining \\ 500)
  defp await_cycle(_pid, 0), do: flunk("Market history fetch did not finish")

  defp await_cycle(pid, remaining) do
    state = :sys.get_state(pid)

    if state.cycle_finished? and Map.get(state, :day_count) == 3 do
      assert state.types_states.native_coin_price_history.failed_attempts == 0
      assert state.types_states.market_cap_history.failed_attempts == 0
      assert state.types_states.secondary_coin_price_history.failed_attempts == 0
      state
    else
      Process.sleep(10)
      await_cycle(pid, remaining - 1)
    end
  end

  defp restore(app, name, nil), do: Application.delete_env(app, name)
  defp restore(app, name, value), do: Application.put_env(app, name, value)
end
