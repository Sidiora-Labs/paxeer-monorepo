defmodule Explorer.Market.Source.ChainflowTest do
  use Explorer.DataCase

  alias Explorer.Chain
  alias Explorer.Chain.Token, as: ChainToken
  alias Explorer.Market.Fetcher.Token, as: TokenFetcher
  alias Explorer.Market.{MarketHistory, Source}
  alias Explorer.Market.Source.Chainflow
  alias Plug.Conn

  @wpax "0x7D4B7B8CA7E1a24928Bb96D59249c7a5bd1DfBe6"
  @sid "0x86949e4CdB89496490890B67C9cfF63eD8efB4b1"

  setup do
    bypass = Bypass.open()
    old_source = Application.get_env(:explorer, Chainflow)
    old_adapter = Application.get_env(:tesla, :adapter)
    old_supply = Application.get_env(:explorer, :supply)
    old_tokens_enabled = :persistent_term.get(:market_token_fetcher_enabled, false)
    :persistent_term.put(:market_token_fetcher_enabled, true)

    config = [
      base_url: "http://localhost:#{bypass.port}",
      coin_symbol: "PAX",
      secondary_coin_symbol: "SID",
      tokens: [[@wpax, "PAX"], [@sid, "SID"]]
    ]

    Application.put_env(:explorer, Chainflow, config)
    Application.put_env(:explorer, :supply, Explorer.Chain.Supply.ProofOfAuthority)
    Application.put_env(:tesla, :adapter, Tesla.Adapter.Mint)

    on_exit(fn ->
      restore(:explorer, Chainflow, old_source)
      restore(:explorer, :supply, old_supply)
      :persistent_term.put(:market_token_fetcher_enabled, old_tokens_enabled)
      restore(:tesla, :adapter, old_adapter)
    end)

    wpax =
      insert(:token,
        contract_address: build(:address, hash: @wpax),
        symbol: "WPAX",
        decimals: 6,
        total_supply: 250_000_000,
        fiat_value: nil,
        circulating_market_cap: nil,
        volume_24h: nil
      )

    sid =
      insert(:token,
        contract_address: build(:address, hash: @sid),
        symbol: "SID",
        decimals: 18,
        total_supply: Decimal.new("400000000000000000000"),
        fiat_value: nil,
        circulating_market_cap: nil,
        volume_24h: nil
      )

    data = start_supervised!({Agent, fn -> %{pax: "2.5", sid: "4", volume: "120", history: :candles} end})

    Bypass.expect(bypass, fn conn ->
      conn = Conn.fetch_query_params(conn)
      values = Agent.get(data, & &1)
      ["api", market, route] = String.split(conn.request_path, "/", trim: true)
      assert market in ["pax", "sid", "native-pax"]
      price = if market == "sid", do: values.sid, else: values.pax

      body =
        case route do
          "price" ->
            %{
              "price" => price,
              "symbol" => if(market == "sid", do: "SID", else: "PAX"),
              "timestamp" => "2026-01-02T00:00:00Z"
            }

          "stats" ->
            if is_nil(values.volume), do: %{}, else: %{"volume_24h" => values.volume}

          "history" ->
            assert conn.query_params["symbol"] == if(market == "sid", do: "SID", else: "PAX")
            assert conn.query_params["resolution"] == "1D"

            assert String.to_integer(conn.query_params["to"]) - String.to_integer(conn.query_params["from"]) ==
                     2 * 86_400

            case values.history do
              :candles ->
                %{
                  "s" => "ok",
                  "t" => [1_767_225_600, 1_767_312_000],
                  "o" => ["1", "2"],
                  "c" => ["2", price],
                  "h" => ["3", "5"],
                  "l" => ["1", "1"],
                  "v" => [10, 20]
                }

              response ->
                response
            end
        end

      Conn.resp(conn, 200, Jason.encode!(body))
    end)

    {:ok, bypass: bypass, data: data, wpax: wpax, sid: sid, config: config}
  end

  test "native and secondary quotes use the feed and backend supplies", %{sid: sid} do
    assert Chainflow.native_coin_fetching_enabled?()
    assert Chainflow.secondary_coin_fetching_enabled?()
    assert Chainflow.tokens_fetching_enabled?()
    assert Chainflow.native_coin_price_history_fetching_enabled?()
    assert Chainflow.secondary_coin_price_history_fetching_enabled?()
    assert Chainflow.market_cap_history_fetching_enabled?()
    assert {:ok, native} = Chainflow.fetch_native_coin()
    assert native.symbol == "PAX"
    assert Decimal.equal?(native.fiat_value, "2.5")
    assert Decimal.equal?(native.market_cap, Decimal.mult(Chain.circulating_supply(), "2.5"))
    assert native.last_updated == ~U[2026-01-02 00:00:00Z]
    assert Decimal.equal?(native.volume_24h, 120)
    assert native.tvl == nil
    assert {:ok, secondary} = Chainflow.fetch_secondary_coin()
    assert secondary.symbol == "SID"
    assert Decimal.equal?(secondary.fiat_value, 4)
    assert Decimal.equal?(secondary.available_supply, Decimal.div(sid.total_supply, Decimal.new("1000000000000000000")))
    assert Decimal.equal?(secondary.market_cap, 1600)
  end

  test "token intervals import WPAX and SID prices, scaled market caps and volume", context do
    state = token_state()
    assert {:noreply, state} = TokenFetcher.handle_info({:fetch, 0}, state)
    assert_token(context.wpax, "2.5", 625, 120)
    assert_token(context.sid, 4, 1600, 120)
    Agent.update(context.data, &%{&1 | pax: "3", sid: "5", volume: "240"})
    assert {:noreply, _} = TokenFetcher.handle_info({:fetch, 0}, state)
    assert_token(context.wpax, 3, 750, 240)
    assert_token(context.sid, 5, 2000, 240)
    assert Repo.get!(ChainToken, context.wpax.contract_address_hash).symbol == "WPAX"
  end

  test "token batches resume and restart after the final batch" do
    assert {:ok, remaining, false, [wpax]} = Chainflow.fetch_tokens(nil, 1)
    assert to_string(wpax.contract_address_hash) == String.downcase(@wpax)
    assert {:ok, nil, true, [sid]} = Chainflow.fetch_tokens(remaining, 1)
    assert to_string(sid.contract_address_hash) == String.downcase(@sid)
    assert {:ok, _, false, [_]} = Chainflow.fetch_tokens(nil, 1)
  end

  test "native, secondary and market cap histories retain candle prices and caps" do
    assert {:ok, native} = Chainflow.fetch_native_coin_price_history(2)
    assert {:ok, secondary} = Chainflow.fetch_secondary_coin_price_history(2)
    assert {:ok, caps} = Chainflow.fetch_market_cap_history(2)
    assert length(native) == 2
    assert length(secondary) == 2
    assert Enum.map(native, & &1.date) == [~D[2026-01-01], ~D[2026-01-02]]
    assert Decimal.equal?(hd(native).opening_price, 1)
    assert Decimal.equal?(List.last(native).closing_price, "2.5")
    assert Decimal.equal?(List.last(secondary).closing_price, 4)
    assert Decimal.equal?(List.last(secondary).market_cap, 1600)
    assert Enum.all?(secondary, & &1.secondary_coin)
    assert caps == Enum.map(native, &Map.take(&1, [:date, :market_cap]))
    assert {:ok, {4, _}} = MarketHistory.bulk_insert(native ++ secondary)
    assert Decimal.equal?(MarketHistory.price_at_date(~D[2026-01-02], true).market_cap, 1600)
  end

  test "missing feed fields remain nil in quotes and token rows", context do
    Agent.update(context.data, &%{&1 | volume: nil})
    assert {:ok, quote} = Chainflow.fetch_native_coin()
    assert quote.volume_24h == nil
    assert quote.btc_value == nil
    assert quote.image_url == nil
    assert {:noreply, _} = TokenFetcher.handle_info({:fetch, 0}, token_state())
    assert Repo.get!(ChainToken, context.wpax.contract_address_hash).volume_24h == nil
    assert Repo.get!(ChainToken, context.sid.contract_address_hash).volume_24h == nil
    Agent.update(context.data, &%{&1 | pax: nil})
    assert {:ok, quote} = Chainflow.fetch_native_coin()
    assert quote.fiat_value == nil
    assert quote.market_cap == nil
  end

  test "unknown token supply leaves market cap nil", %{sid: sid} do
    sid |> Ecto.Changeset.change(total_supply: nil) |> Repo.update!()
    assert {:ok, quote} = Chainflow.fetch_secondary_coin()
    assert quote.available_supply == nil
    assert quote.market_cap == nil
    assert {:ok, nil, true, [_wpax, token]} = Chainflow.fetch_tokens(nil, 2)
    assert token.circulating_market_cap == nil
    assert Decimal.equal?(token.fiat_value, 4)
  end

  test "unreachable price and statistics feeds leave stored token rows unchanged", context do
    assert {:noreply, state} = TokenFetcher.handle_info({:fetch, 0}, token_state())
    before = Enum.map([context.wpax, context.sid], &Repo.get!(ChainToken, &1.contract_address_hash))
    Bypass.down(context.bypass)
    assert {:error, _} = Chainflow.fetch_native_coin()
    assert {:error, _} = Chainflow.fetch_secondary_coin()
    assert {:error, _} = Chainflow.fetch_native_coin_price_history(2)
    assert {:error, _} = Chainflow.fetch_secondary_coin_price_history(2)
    assert {:error, _} = Chainflow.fetch_market_cap_history(2)
    assert {:noreply, ^state} = TokenFetcher.handle_info({:fetch, 5}, state)
    assert Enum.map([context.wpax, context.sid], &Repo.get!(ChainToken, &1.contract_address_hash)) == before
  end

  test "a statistics error prevents importing a partially fetched batch", context do
    Bypass.expect(context.bypass, "GET", "/api/sid/stats", fn conn -> Conn.resp(conn, 503, "unavailable") end)
    before = Enum.map([context.wpax, context.sid], &Repo.get!(ChainToken, &1.contract_address_hash))
    assert {:error, _} = Chainflow.fetch_tokens(nil, 2)
    assert {:noreply, _} = TokenFetcher.handle_info({:fetch, 5}, token_state())
    assert Enum.map([context.wpax, context.sid], &Repo.get!(ChainToken, &1.contract_address_hash)) == before
  end

  test "history distinguishes no data from malformed candles", %{data: data} do
    Agent.update(data, &%{&1 | history: %{"s" => "no_data"}})
    assert {:ok, []} = Chainflow.fetch_native_coin_price_history(2)
    Agent.update(data, &%{&1 | history: %{"s" => "ok", "t" => [1], "o" => [], "c" => [2]}})
    assert {:error, _} = Chainflow.fetch_native_coin_price_history(2)
    Agent.update(data, &%{&1 | history: %{"s" => "error", "errmsg" => "unavailable"}})
    assert {:error, _} = Chainflow.fetch_market_cap_history(2)
  end

  test "native market override also serves the wrapped native token", %{config: config} do
    Application.put_env(:explorer, Chainflow, Keyword.put(config, :market, "native-pax"))
    assert {:ok, native} = Chainflow.fetch_native_coin()
    assert Decimal.equal?(native.fiat_value, "2.5")
    assert {:ok, nil, true, [wpax, _sid]} = Chainflow.fetch_tokens(nil, 2)
    assert Decimal.equal?(wpax.fiat_value, native.fiat_value)
    assert {:ok, secondary} = Chainflow.fetch_secondary_coin()
    assert secondary.symbol == "SID"
  end

  test "source selection recognizes Chainflow for each supported market stream" do
    old = Application.get_env(:explorer, Source)

    Application.put_env(:explorer, Source,
      native_coin_source: Chainflow,
      secondary_coin_source: Chainflow,
      tokens_source: Chainflow,
      native_coin_history_source: Chainflow,
      secondary_coin_history_source: Chainflow,
      market_cap_history_source: Chainflow
    )

    on_exit(fn -> restore(:explorer, Source, old) end)
    assert Source.native_coin_source() == Chainflow
    assert Source.secondary_coin_source() == Chainflow
    assert Source.tokens_source() == Chainflow
    assert Source.native_coin_price_history_source() == Chainflow
    assert Source.secondary_coin_price_history_source() == Chainflow
    assert Source.market_cap_history_source() == Chainflow
    assert {:ok, _} = Source.native_coin_source().fetch_native_coin()
  end

  defp token_state do
    %TokenFetcher{source: Chainflow, max_batch_size: 2, interval: 60_000, refetch_interval: 60_000}
  end

  defp assert_token(token, price, cap, volume) do
    stored = Repo.get!(ChainToken, token.contract_address_hash)
    assert Decimal.equal?(stored.fiat_value, price)
    assert Decimal.equal?(stored.circulating_market_cap, cap)
    assert Decimal.equal?(stored.volume_24h, volume)
  end

  defp restore(app, key, nil), do: Application.delete_env(app, key)
  defp restore(app, key, value), do: Application.put_env(app, key, value)
end
