defmodule BlockScoutWeb.Chain.MarketHistoryChartControllerTest do
  use BlockScoutWeb.ConnCase

  alias Explorer.Market.{MarketHistoryCache, Token}
  alias Explorer.Market.Fetcher.Coin

  alias BlockScoutWeb.Chain.MarketHistoryChartController

  describe "encode_market_history_data/2" do
    test "leaves market cap unknown when supply or closing price is unknown" do
      for {supply, closing_price} <- [{nil, Decimal.new(2)}, {Decimal.new(100), nil}, {nil, nil}] do
        day = build(:market_history, closing_price: closing_price, market_cap: nil)

        assert [%{"market_cap" => nil}] = MarketHistoryChartController.encode_market_history_data([day], supply)
      end
    end

    test "keeps stored market cap even when supply and closing price are unknown" do
      for market_cap <- [Decimal.new(0), Decimal.new(42)] do
        day = build(:market_history, closing_price: nil, market_cap: market_cap)

        assert [%{"market_cap" => encoded_cap}] = MarketHistoryChartController.encode_market_history_data([day], nil)
        assert Decimal.equal?(Decimal.new(encoded_cap), market_cap)
      end
    end

    test "calculates market cap from known supply and closing price including zero" do
      for {supply, closing_price, expected} <- [{"100", 2, 200}, {0, 2, 0}, {100, 0, 0}] do
        day = build(:market_history, closing_price: Decimal.new(closing_price), market_cap: nil)

        assert [%{"market_cap" => encoded_cap}] = MarketHistoryChartController.encode_market_history_data([day], supply)
        assert Decimal.equal?(Decimal.new(encoded_cap), expected)
      end
    end
  end

  describe "GET show/2" do
    setup do
      configuration = [
        {Coin, [enabled: true, store: :ets, table_name: :market_history_chart_rates]},
        {MarketHistoryCache, [cache_period: 60_000]},
        {:supply, Explorer.Chain.Supply.ExchangeRate}
      ]

      previous_configuration = Enum.map(configuration, fn {key, _} -> {key, Application.fetch_env(:explorer, key)} end)
      history_enabled = :persistent_term.get(:market_history_fetcher_enabled, nil)
      cache_name = MarketHistoryCache.cache_name()
      cache_keys = [MarketHistoryCache.data_key(), MarketHistoryCache.updated_at_key()]
      cached_values = Enum.map(cache_keys, fn key -> {key, ConCache.get(cache_name, key)} end)

      on_exit(fn ->
        Enum.each(previous_configuration, fn
          {key, {:ok, value}} -> Application.put_env(:explorer, key, value)
          {key, :error} -> Application.delete_env(:explorer, key)
        end)

        if is_nil(history_enabled) do
          :persistent_term.erase(:market_history_fetcher_enabled)
        else
          :persistent_term.put(:market_history_fetcher_enabled, history_enabled)
        end

        Enum.each(cached_values, fn
          {key, nil} -> ConCache.delete(cache_name, key)
          {key, value} -> ConCache.put(cache_name, key, value)
        end)
      end)

      Enum.each(configuration, fn {key, value} -> Application.put_env(:explorer, key, value) end)
      :ets.new(Coin.table_name(), [:set, :named_table, :public])
      :persistent_term.put(:market_history_fetcher_enabled, true)
      Enum.each(cache_keys, &ConCache.delete(cache_name, &1))
      :ok
    end

    test "keeps stored closing prices when the current rate is unknown", %{conn: conn} do
      today = insert(:market_history, closing_price: Decimal.new("5.25"), market_cap: nil, tvl: nil)

      yesterday =
        insert(:market_history,
          date: Date.add(today.date, -1),
          closing_price: Decimal.new("4.25"),
          market_cap: nil,
          tvl: nil
        )

      assert Explorer.Market.get_coin_exchange_rate() == Token.null()

      response =
        conn
        |> put_req_header("x-requested-with", "xmlhttprequest")
        |> get(market_history_chart_path(@endpoint, :show))
        |> json_response(200)

      assert response == %{
               "history_data" => [
                 %{
                   "date" => Date.to_iso8601(today.date),
                   "closing_price" => "5.25",
                   "market_cap" => nil,
                   "tvl" => nil
                 },
                 %{
                   "date" => Date.to_iso8601(yesterday.date),
                   "closing_price" => "4.25",
                   "market_cap" => nil,
                   "tvl" => nil
                 }
               ]
             }
    end

    test "uses the current fiat value only for the newest closing price", %{conn: conn} do
      today = insert(:market_history, closing_price: Decimal.new("5.25"), market_cap: nil, tvl: nil)

      yesterday =
        insert(:market_history,
          date: Date.add(today.date, -1),
          closing_price: Decimal.new("4.25"),
          market_cap: nil,
          tvl: nil
        )

      token = %{Token.null() | fiat_value: Decimal.new("7.5"), available_supply: Decimal.new(100)}
      ref = make_ref()
      assert {:noreply, _} = Coin.handle_info({ref, {{:ok, token}, false}}, %Coin{tasks: %{ref => false}})
      assert Explorer.Market.get_coin_exchange_rate().fiat_value == token.fiat_value

      response =
        conn
        |> put_req_header("x-requested-with", "xmlhttprequest")
        |> get(market_history_chart_path(@endpoint, :show))
        |> json_response(200)

      assert response == %{
               "history_data" => [
                 %{
                   "date" => Date.to_iso8601(today.date),
                   "closing_price" => "7.5",
                   "market_cap" => "750.0",
                   "tvl" => nil
                 },
                 %{
                   "date" => Date.to_iso8601(yesterday.date),
                   "closing_price" => "4.25",
                   "market_cap" => "425.00",
                   "tvl" => nil
                 }
               ]
             }
    end

    test "returns error when not an ajax request" do
      path = market_history_chart_path(BlockScoutWeb.Endpoint, :show)

      conn = get(build_conn(), path)

      assert conn.status == 422
    end

    test "returns ok when request is ajax" do
      path = market_history_chart_path(BlockScoutWeb.Endpoint, :show)

      conn =
        build_conn()
        |> put_req_header("x-requested-with", "xmlhttprequest")
        |> get(path)

      assert json_response(conn, 200)
    end
  end
end
