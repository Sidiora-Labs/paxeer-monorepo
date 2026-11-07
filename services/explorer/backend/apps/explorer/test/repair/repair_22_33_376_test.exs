defmodule Explorer.Repair.Repair2233376Test do
  use ExUnit.Case, async: false

  alias Explorer.History.Process, as: HistoryProcess
  alias Explorer.Market.{MarketHistory, Source}
  alias Explorer.Market.Fetcher.{Coin, History}
  alias Explorer.Market.Source.{CoinGecko, CryptoCompare}
  alias Explorer.Repo

  @moduletag :capture_log
  @date ~D[2018-04-01]
  @timestamp 1_522_540_800_000

  setup do
    :ok = Ecto.Adapters.SQL.Sandbox.checkout(Repo)
    Ecto.Adapters.SQL.Sandbox.mode(Repo, {:shared, self()})

    settings = [
      {:explorer, Source},
      {:explorer, Coin},
      {:explorer, History},
      {:explorer, CoinGecko},
      {:explorer, CryptoCompare},
      {:explorer, HistoryProcess},
      {:explorer, :http_client},
      {:tesla, :adapter}
    ]

    previous = Enum.map(settings, fn {app, key} -> {app, key, Application.fetch_env(app, key)} end)

    on_exit(fn ->
      Enum.each(previous, fn
        {app, key, {:ok, value}} -> Application.put_env(app, key, value)
        {app, key, :error} -> Application.delete_env(app, key)
      end)
    end)

    bypass = Bypass.open()
    fixture = start_supervised!({Agent, fn -> %{mode: :healthy, requests: %{}} end})

    Bypass.expect(bypass, fn conn ->
      conn = Plug.Conn.fetch_query_params(conn)
      key = request_key(conn)

      mode =
        Agent.get_and_update(fixture, fn state ->
          {state.mode, update_in(state, [:requests, key], &((&1 || 0) + 1))}
        end)

      response(conn, key, mode)
    end)

    url = "http://localhost:#{bypass.port}"
    Application.put_env(:explorer, :http_client, Explorer.HttpClient.Tesla)
    Application.put_env(:tesla, :adapter, Tesla.Adapter.Mint)
    Application.put_env(:explorer, HistoryProcess, base_backoff: 1)

    Application.put_env(:explorer, CoinGecko,
      base_url: url,
      api_key: nil,
      coin_id: "ethereum",
      secondary_coin_id: "ethereum-classic",
      currency: "usd"
    )

    Application.put_env(:explorer, CryptoCompare,
      base_url: url,
      api_key: nil,
      coin_symbol: "ETH",
      secondary_coin_symbol: "ETC",
      currency: "USD"
    )

    Application.put_env(:explorer, Source,
      native_coin_source: CoinGecko,
      secondary_coin_source: CoinGecko,
      native_coin_history_source: CoinGecko,
      secondary_coin_history_source: CryptoCompare,
      market_cap_history_source: CoinGecko,
      tvl_history_source: CryptoCompare
    )

    Application.put_env(:explorer, Coin,
      store: :ets,
      table_name: :repair_22_33_376_rates,
      enabled: true,
      enable_consolidation: true,
      cache_period: 500
    )

    Application.put_env(:explorer, History,
      first_fetch_day_count: 365,
      history_fetch_interval: 500
    )

    %{fixture: fixture}
  end

  test "coin-task-recovery bounds crashes for both coins, retains rates and recovers before and after reopening", %{
    fixture: fixture
  } do
    pid = start_supervised!(Coin)
    eventually(fn -> Coin.get_coin_exchange_rate() && Coin.get_secondary_coin_exchange_rate() end)
    native = Coin.get_coin_exchange_rate()
    secondary = Coin.get_secondary_coin_exchange_rate()
    assert Decimal.equal?(native.fiat_value, 10)
    assert Decimal.equal?(secondary.fiat_value, 20)
    assert native.symbol == "ETH"
    assert secondary.symbol == "ETC"

    set_mode(fixture, :coin_crash)

    eventually(fn ->
      state = :sys.get_state(pid)
      get_in(state.failures, [false, :attempts]) == 4 && get_in(state.failures, [true, :attempts]) == 4
    end)

    assert Coin.get_coin_exchange_rate() == native
    assert Coin.get_secondary_coin_exchange_rate() == secondary
    assert :sys.get_state(pid).tasks == %{}
    assert {:monitors, []} = Process.info(pid, :monitors)
    assert request_count(fixture, :native_coin) == 5
    assert request_count(fixture, :secondary_coin) == 5
    Process.sleep(100)
    assert request_count(fixture, :native_coin) == 5
    assert request_count(fixture, :secondary_coin) == 5

    set_mode(fixture, :recovered)

    eventually(fn ->
      Decimal.equal?(Coin.get_coin_exchange_rate().fiat_value, 30) &&
        Decimal.equal?(Coin.get_secondary_coin_exchange_rate().fiat_value, 40)
    end)

    assert Process.alive?(pid)
    assert :sys.get_state(pid).failures == %{}
    assert :sys.get_state(pid).tasks == %{}
    assert {:monitors, []} = Process.info(pid, :monitors)

    :ok = stop_supervised(Coin)
    refute Process.alive?(pid)
    assert Coin.get_coin_exchange_rate() == nil
    reopened = start_supervised!(Coin)
    eventually(fn -> Coin.get_coin_exchange_rate() && Coin.get_secondary_coin_exchange_rate() end)
    assert reopened != pid
    assert Decimal.equal?(Coin.get_coin_exchange_rate().fiat_value, 30)
    assert Decimal.equal?(Coin.get_secondary_coin_exchange_rate().fiat_value, 40)
  end

  test "coin-task-recovery also retries returned provider errors without losing cached data", %{fixture: fixture} do
    pid = start_supervised!(Coin)
    eventually(fn -> Coin.get_coin_exchange_rate() && Coin.get_secondary_coin_exchange_rate() end)
    native = Coin.get_coin_exchange_rate()
    set_mode(fixture, :http_error)
    eventually(fn -> get_in(:sys.get_state(pid).failures, [false, :attempts]) == 4 end)
    assert Coin.get_coin_exchange_rate() == native
    assert :sys.get_state(pid).failures[false].reason == "503: provider unavailable"
    set_mode(fixture, :recovered)
    eventually(fn -> Decimal.equal?(Coin.get_coin_exchange_rate().fiat_value, 30) end)
    assert Process.alive?(pid)
  end

  test "history-task-recovery persists other types after exhaustion and recovers on the next cycle and reopening", %{
    fixture: fixture
  } do
    set_mode(fixture, :history_crash)
    pid = start_supervised!(History)

    eventually(fn ->
      state = :sys.get_state(pid)
      state.cycle_finished? && state.types_states.native_coin_price_history.failed_attempts == 11
    end)

    state = :sys.get_state(pid)
    assert state.types_states.native_coin_price_history.records == []
    assert state.types_states.secondary_coin_price_history.finished?
    assert state.types_states.market_cap_history.finished?
    assert state.types_states.tvl_history.source == nil
    assert state.tasks == %{}
    assert {:monitors, []} = Process.info(pid, :monitors)
    assert request_count(fixture, :chart_365) == 12
    assert request_count(fixture, :secondary_history_365) == 1

    native = MarketHistory.price_at_date(@date)
    secondary = MarketHistory.price_at_date(@date, true)
    assert native.closing_price == nil
    assert Decimal.equal?(native.market_cap, 100_500)
    assert Decimal.equal?(secondary.opening_price, 5)
    assert Decimal.equal?(secondary.closing_price, 20)
    Process.sleep(100)
    assert request_count(fixture, :chart_365) == 12

    set_mode(fixture, :recovered)

    eventually(fn ->
      record = MarketHistory.price_at_date(@date)
      record && record.closing_price && Decimal.equal?(record.closing_price, 30)
    end)

    assert request_count(fixture, :chart_1) == 2
    assert request_count(fixture, :secondary_history_1) == 1
    state = :sys.get_state(pid)
    assert state.cycle_finished?
    assert state.types_states.native_coin_price_history.failed_attempts == 0
    assert state.tasks == %{}
    assert {:monitors, []} = Process.info(pid, :monitors)
    assert Decimal.equal?(MarketHistory.price_at_date(@date).market_cap, 100_500)
    assert Decimal.equal?(MarketHistory.price_at_date(@date, true).closing_price, 40)

    :ok = stop_supervised(History)
    refute Process.alive?(pid)
    assert Decimal.equal?(MarketHistory.price_at_date(@date).closing_price, 30)
    reopened = start_supervised!(History)
    eventually(fn -> :sys.get_state(reopened).cycle_finished? end)
    assert reopened != pid
    assert Decimal.equal?(MarketHistory.price_at_date(@date).closing_price, 30)
    assert Decimal.equal?(MarketHistory.price_at_date(@date, true).closing_price, 40)
  end

  test "history-task-recovery bounds returned errors and refreshes in the subsequent cycle", %{fixture: fixture} do
    set_mode(fixture, :http_error)
    pid = start_supervised!(History)
    eventually(fn -> :sys.get_state(pid).cycle_finished? end)
    state = :sys.get_state(pid)
    assert state.types_states.native_coin_price_history.failed_attempts == 11
    assert state.types_states.secondary_coin_price_history.failed_attempts == 11
    assert state.types_states.market_cap_history.failed_attempts == 4
    assert state.tasks == %{}
    assert MarketHistory.price_at_date(@date) == nil
    set_mode(fixture, :recovered)
    eventually(fn -> MarketHistory.price_at_date(@date) end)
    assert Decimal.equal?(MarketHistory.price_at_date(@date).closing_price, 30)
    assert Decimal.equal?(MarketHistory.price_at_date(@date, true).closing_price, 40)
    assert Process.alive?(pid)
  end

  defp request_key(%{request_path: "/coins/ethereum"}), do: :native_coin
  defp request_key(%{request_path: "/coins/ethereum-classic"}), do: :secondary_coin

  defp request_key(%{
         request_path: "/coins/ethereum/market_chart",
         query_params: %{"days" => days, "vs_currency" => "usd"}
       })
       when days in ["365", "1"],
       do: if(days == "365", do: :chart_365, else: :chart_1)

  defp request_key(%{
         request_path: "/data/v2/histoday",
         query_params: %{"limit" => days, "fsym" => "ETC", "tsym" => "USD"}
       })
       when days in ["365", "1"],
       do: if(days == "365", do: :secondary_history_365, else: :secondary_history_1)

  defp response(conn, _key, :http_error), do: Plug.Conn.resp(conn, 503, "provider unavailable")

  defp response(conn, key, mode) do
    data =
      case key do
        coin when coin in [:native_coin, :secondary_coin] ->
          secondary? = coin == :secondary_coin
          price = if mode == :recovered, do: if(secondary?, do: 40, else: 30), else: if(secondary?, do: 20, else: 10)

          %{
            "name" => if(secondary?, do: "Ethereum Classic", else: "Ethereum"),
            "symbol" => if(mode == :coin_crash, do: nil, else: if(secondary?, do: "etc", else: "eth")),
            "image" => %{"small" => nil, "thumb" => nil},
            "market_data" => %{
              "circulating_supply" => 100_000,
              "total_supply" => 100_000,
              "current_price" => %{"usd" => price, "btc" => 1},
              "market_cap" => %{"usd" => 100_500},
              "total_volume" => %{"usd" => 1000},
              "last_updated" => "2018-04-01T00:00:00Z"
            }
          }

        chart when chart in [:chart_365, :chart_1] ->
          %{
            "prices" => if(mode == :history_crash, do: 42, else: [[@timestamp, 5], [@timestamp + 86_400_000, 30]]),
            "market_caps" => [[@timestamp, 100_000], [@timestamp + 86_400_000, 100_500]]
          }

        history when history in [:secondary_history_365, :secondary_history_1] ->
          %{
            "Response" => "Success",
            "Data" => %{
              "Data" => [
                %{"time" => div(@timestamp, 1000), "open" => 5, "close" => if(mode == :recovered, do: 40, else: 20)}
              ]
            }
          }
      end

    conn |> Plug.Conn.put_resp_content_type("application/json") |> Plug.Conn.resp(200, Jason.encode!(data))
  end

  defp set_mode(fixture, mode), do: Agent.update(fixture, &%{&1 | mode: mode})
  defp request_count(fixture, key), do: Agent.get(fixture, &Map.get(&1.requests, key, 0))

  defp eventually(fun), do: eventually(fun, System.monotonic_time(:millisecond) + 5000)

  defp eventually(fun, deadline) do
    case fun.() do
      value when value not in [false, nil] ->
        value

      _ ->
        assert System.monotonic_time(:millisecond) < deadline, "fetcher did not recover within 5 seconds"
        Process.sleep(5)
        eventually(fun, deadline)
    end
  end
end
