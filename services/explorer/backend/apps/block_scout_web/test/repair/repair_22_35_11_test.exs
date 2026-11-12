defmodule BlockScoutWeb.API.V2.Repair223511Test do
  use BlockScoutWeb.ConnCase

  alias Explorer.Chain.Cache.Counters.{AddressesCount, AverageBlockTime}
  alias Explorer.Market.{MarketHistory, MarketHistoryCache}
  alias Explorer.Market.Fetcher.Coin
  alias Explorer.Market.Source.Chainflow

  @sid "0x86949e4CdB89496490890B67C9cfF63eD8efB4b1"

  setup do
    start_supervised!(AddressesCount)
    start_supervised!(AverageBlockTime)

    Application.put_env(:explorer, AverageBlockTime, enabled: true, cache_period: 1_800_000)

    on_exit(fn ->
      Application.put_env(:explorer, AverageBlockTime, enabled: false, cache_period: 1_800_000)
    end)

    bypass = Bypass.open()
    old_adapter = Application.get_env(:tesla, :adapter)
    old_chainflow = Application.get_env(:explorer, Chainflow)
    old_coin = Application.get_env(:explorer, Coin)
    old_cache = Application.get_env(:explorer, MarketHistoryCache)
    old_history_enabled = :persistent_term.get(:market_history_fetcher_enabled, false)
    Application.put_env(:tesla, :adapter, Tesla.Adapter.Mint)

    Application.put_env(:explorer, Chainflow,
      base_url: "http://localhost:#{bypass.port}",
      coin_symbol: "PAX",
      secondary_coin_symbol: "SID",
      tokens: [[@sid, "SID"]]
    )

    Application.put_env(:explorer, Coin,
      store: :ets,
      enabled: true,
      enable_consolidation: false,
      table_name: :chainflow_stats_exchange_rates
    )

    Application.put_env(:explorer, MarketHistoryCache, cache_period: 0)
    :ets.new(:chainflow_stats_exchange_rates, [:set, :named_table, :public])
    :persistent_term.put(:market_history_fetcher_enabled, true)
    clear_history_cache()
    insert(:token, contract_address: build(:address, hash: @sid), symbol: "SID", decimals: 6, total_supply: 400_000_000)

    on_exit(fn ->
      restore(:tesla, :adapter, old_adapter)
      restore(:explorer, Chainflow, old_chainflow)
      restore(:explorer, Coin, old_coin)
      restore(:explorer, MarketHistoryCache, old_cache)
      :persistent_term.put(:market_history_fetcher_enabled, old_history_enabled)
      clear_history_cache()
    end)

    {:ok, bypass: bypass}
  end

  test "stats exposes secondary_coin_market_cap and secondary_coin_price_change_percentage from the feed", context do
    serve_feed(context.bypass, 3)
    import_secondary_market()
    response = context.conn |> get("/api/v2/stats") |> json_response(200)
    assert response["secondary_coin_price"] == "4"
    assert Map.has_key?(response, "secondary_coin_market_cap")
    assert Decimal.equal?(Decimal.new(response["secondary_coin_market_cap"]), 1600)
    assert Map.has_key?(response, "secondary_coin_price_change_percentage")
    assert response["secondary_coin_price_change_percentage"] == 33.34
    properties = BlockScoutWeb.Schemas.API.V2.Stats.Response.schema().properties
    assert Map.has_key?(properties, :secondary_coin_market_cap)
    assert Map.has_key?(properties, :secondary_coin_price_change_percentage)
  end

  test "secondary chart items expose market_cap from the feed and scaled SID supply", context do
    serve_feed(context.bypass, 3)
    import_secondary_market()
    response = context.conn |> get("/api/v2/stats/charts/secondary-coin-market") |> json_response(200)
    assert [today, yesterday] = response["chart_data"]
    assert today["date"] == Date.to_iso8601(Date.utc_today())
    assert yesterday["date"] == Date.to_iso8601(Date.add(Date.utc_today(), -1))
    assert Map.has_key?(today, "market_cap")
    assert Decimal.equal?(Decimal.new(today["market_cap"]), 1600)
    assert Decimal.equal?(Decimal.new(yesterday["market_cap"]), 1200)
    assert Decimal.equal?(Decimal.new(today["closing_price"]), 4)
  end

  test "secondary statistics keep unavailable values nil", context do
    response = context.conn |> get("/api/v2/stats") |> json_response(200)
    assert Map.has_key?(response, "secondary_coin_market_cap")
    assert response["secondary_coin_market_cap"] == nil
    assert Map.has_key?(response, "secondary_coin_price_change_percentage")
    assert response["secondary_coin_price_change_percentage"] == nil
  end

  test "secondary price change is nil when yesterday closes at zero", context do
    serve_feed(context.bypass, 0)
    import_secondary_market()
    response = context.conn |> get("/api/v2/stats") |> json_response(200)
    assert response["secondary_coin_price_change_percentage"] == nil
    assert Decimal.equal?(Decimal.new(response["secondary_coin_market_cap"]), 1600)
  end

  defp serve_feed(bypass, yesterday_price) do
    Bypass.expect(bypass, fn conn ->
      response =
        case conn.request_path do
          "/api/sid/price" ->
            %{"symbol" => "SID", "price" => "4", "timestamp" => "2026-01-02T00:00:00Z"}

          "/api/sid/stats" ->
            %{"symbol" => "SID", "volume_24h" => "120"}

          "/api/sid/history" ->
            today = Date.utc_today() |> DateTime.new!(~T[00:00:00]) |> DateTime.to_unix()

            %{
              "s" => "ok",
              "t" => [today - 86_400, today],
              "o" => [1, 3],
              "c" => [yesterday_price, 4],
              "h" => [4, 5],
              "l" => [0, 2],
              "v" => [10, 20]
            }
        end

      Plug.Conn.resp(conn, 200, Jason.encode!(response))
    end)
  end

  defp import_secondary_market do
    assert {:ok, quote} = Chainflow.fetch_secondary_coin()
    ref = make_ref()
    assert {:noreply, _} = Coin.handle_info({ref, {{:ok, quote}, true}}, %Coin{tasks: %{ref => true}})
    assert {:ok, history} = Chainflow.fetch_secondary_coin_price_history(2)
    assert {:ok, {2, _}} = MarketHistory.bulk_insert(history)
    clear_history_cache()
  end

  defp clear_history_cache do
    ConCache.delete(MarketHistoryCache.cache_name(), MarketHistoryCache.data_key())
    ConCache.delete(MarketHistoryCache.cache_name(), MarketHistoryCache.updated_at_key())
  end

  defp restore(app, key, nil), do: Application.delete_env(app, key)
  defp restore(app, key, value), do: Application.put_env(app, key, value)
end
