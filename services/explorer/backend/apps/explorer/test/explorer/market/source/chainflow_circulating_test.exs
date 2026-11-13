defmodule Explorer.Market.Source.ChainflowCirculatingTest do
  use Explorer.DataCase

  alias Explorer.Chain.Token, as: ChainToken
  alias Explorer.Market
  alias Explorer.Market.Fetcher.Coin, as: CoinFetcher
  alias Explorer.Market.Fetcher.Token, as: TokenFetcher
  alias Explorer.Market.{MarketHistory, Source}
  alias Explorer.Market.Source.Chainflow
  alias Plug.Conn

  setup do
    bypass = Bypass.open()
    sid = insert(:token, symbol: "SID", decimals: 18, total_supply: Decimal.new("400000000000000000000"))
    wpax = insert(:token, symbol: "WPAX", decimals: 6, total_supply: 250_000_000)

    config = [
      base_url: "http://localhost:#{bypass.port}",
      coin_id: "paxeer",
      secondary_coin_id: "sidiora",
      coin_symbol: "PAX",
      secondary_coin_symbol: "SID",
      secondary_coin_non_circulating_addresses: [],
      tokens: [[to_string(wpax.contract_address_hash), "PAX"], [to_string(sid.contract_address_hash), "SID"]]
    ]

    settings = [
      {:explorer, Chainflow, config},
      {:explorer, Source, [native_coin_source: nil, secondary_coin_source: Chainflow, tokens_source: Chainflow]},
      {:explorer, CoinFetcher,
       [store: :ets, enabled: true, enable_consolidation: false, table_name: :chainflow_circulating_rates]},
      {:explorer, :supply, Explorer.Chain.Supply.ProofOfAuthority},
      {:tesla, :adapter, Tesla.Adapter.Mint}
    ]

    previous = Enum.map(settings, fn {app, key, _value} -> {app, key, Application.get_env(app, key)} end)
    Enum.each(settings, fn {app, key, value} -> Application.put_env(app, key, value) end)
    tokens_enabled = :persistent_term.get(:market_token_fetcher_enabled, :absent)
    :persistent_term.put(:market_token_fetcher_enabled, true)

    on_exit(fn ->
      Enum.each(previous, fn {app, key, value} -> restore(app, key, value) end)

      if tokens_enabled == :absent do
        :persistent_term.erase(:market_token_fetcher_enabled)
      else
        :persistent_term.put(:market_token_fetcher_enabled, tokens_enabled)
      end
    end)

    data = start_supervised!({Agent, fn -> %{market_cap: nil, circulating_supply: nil, total_supply: nil} end})

    Bypass.expect(bypass, fn conn ->
      conn = Conn.fetch_query_params(conn)
      values = Agent.get(data, & &1)

      body =
        case conn.request_path do
          "/api/v3/coins/sidiora" ->
            %{
              "name" => "Sidiora",
              "symbol" => "sid",
              "market_data" => %{
                "current_price" => %{"usd" => "4"},
                "market_cap" => %{"usd" => values.market_cap},
                "circulating_supply" => values.circulating_supply,
                "total_supply" => values.total_supply,
                "total_volume" => %{"usd" => "120"}
              }
            }

          "/api/v3/coins/paxeer" ->
            %{
              "name" => "Paxeer",
              "symbol" => "pax",
              "market_data" => %{
                "current_price" => %{"usd" => "2.5"},
                "market_cap" => %{"usd" => nil},
                "circulating_supply" => 777
              }
            }

          "/api/sid/history" ->
            assert conn.query_params["symbol"] == "SID"
            assert conn.query_params["resolution"] == "1D"
            %{"s" => "ok", "t" => [1_767_312_000], "o" => ["2"], "c" => ["3"]}
        end

      conn |> Conn.put_resp_content_type("application/json") |> Conn.resp(200, Jason.encode!(body))
    end)

    {:ok, data: data, sid: sid, wpax: wpax, config: config}
  end

  test "a feed market cap is retained across the quote, statistics cache and SID row", context do
    Agent.update(context.data, &%{&1 | market_cap: "9876", circulating_supply: "125.5"})
    assert_market_figures(context, "125.5", "9876", "376.5")
  end

  test "a null market cap uses the price times feed circulating supply", context do
    Agent.update(context.data, &%{&1 | circulating_supply: "125.5"})
    exclude_balances(context, ["70", "30"])
    assert_market_figures(context, "125.5", "502", "376.5")
  end

  test "null feed supply and an empty address list use the indexed total supply", context do
    Agent.update(context.data, &%{&1 | total_supply: "999999"})
    insert_balance(context.sid, insert(:address), "50")
    assert_market_figures(context, "400", "1600", "1200")
  end

  test "two listed balances are subtracted from the indexed SID supply", context do
    [first, _second] = exclude_balances(context, ["70.25", "29.75"])
    insert_balance(context.wpax, first, "200")
    insert_balance(context.sid, insert(:address), "50")
    assert_market_figures(context, "300", "1200", "900")
  end

  test "null feed supply and absent indexed total supply leave market cap empty", context do
    context.sid |> Ecto.Changeset.change(total_supply: nil) |> Repo.update!()
    Agent.update(context.data, &%{&1 | total_supply: "999999"})
    exclude_balances(context, ["70", "30"])
    assert_market_figures(context, nil, nil, nil)
  end

  test "excluded balances greater than total supply produce zero circulating supply", context do
    exclude_balances(context, ["350", "60"])
    assert_market_figures(context, "0", "0", "0")
  end

  test "a zero feed market cap is retained", context do
    Agent.update(context.data, &%{&1 | market_cap: 0})
    assert_market_figures(context, "400", "0", "1200")
  end

  test "a zero feed circulating supply is retained", context do
    Agent.update(context.data, &%{&1 | circulating_supply: 0})
    assert_market_figures(context, "0", "0", "0")
  end

  test "listed addresses without indexed balances do not subtract supply", context do
    addresses = for _ <- 1..2, do: to_string(insert(:address).hash)

    Application.put_env(
      :explorer,
      Chainflow,
      Keyword.put(context.config, :secondary_coin_non_circulating_addresses, addresses)
    )

    assert_market_figures(context, "400", "1600", "1200")
  end

  test "duplicate listed addresses subtract each current balance once", context do
    [address] = exclude_balances(context, ["100"])
    addresses = [to_string(address.hash), String.upcase(to_string(address.hash)) |> String.replace_prefix("0X", "0x")]

    Application.put_env(
      :explorer,
      Chainflow,
      Keyword.put(context.config, :secondary_coin_non_circulating_addresses, addresses)
    )

    assert_market_figures(context, "300", "1200", "900")
  end

  test "the circulating supply reflects updated indexed balances", context do
    [address] = exclude_balances(context, ["100"])
    assert {:ok, quote} = Chainflow.fetch_secondary_coin()
    assert Decimal.equal?(quote.available_supply, 300)
    assert Decimal.equal?(quote.market_cap, 1200)

    balance =
      Repo.get_by!(Explorer.Chain.Address.CurrentTokenBalance,
        address_hash: address.hash,
        token_contract_address_hash: context.sid.contract_address_hash
      )

    balance |> Ecto.Changeset.change(value: Decimal.new("150000000000000000000")) |> Repo.update!()
    assert_market_figures(context, "250", "1000", "750")
  end

  test "feed circulating supply does not require indexed token supply", context do
    context.sid |> Ecto.Changeset.change(total_supply: nil) |> Repo.update!()
    Agent.update(context.data, &%{&1 | circulating_supply: "125.5"})
    assert_market_figures(context, "125.5", "502", "376.5")
  end

  test "a feed market cap does not require a circulating supply", context do
    context.sid |> Ecto.Changeset.change(total_supply: nil) |> Repo.update!()
    Agent.update(context.data, &%{&1 | market_cap: "9876"})
    assert_market_figures(context, nil, "9876", nil)
  end

  test "secondary exclusions preserve PAX and WPAX market values", context do
    exclude_balances(context, ["70", "30"])
    assert {:ok, native} = Chainflow.fetch_native_coin()
    assert Decimal.equal?(native.market_cap, Decimal.mult(Explorer.Chain.circulating_supply(), "2.5"))
    assert Decimal.equal?(native.available_supply, 777)
    assert {:ok, nil, true, [wpax, sid]} = Chainflow.fetch_tokens(nil, 2)
    assert Decimal.equal?(wpax.circulating_market_cap, 625)
    assert Decimal.equal?(sid.circulating_market_cap, 1200)
  end

  defp assert_market_figures(context, supply, cap, history_cap) do
    assert {:ok, quote} = Chainflow.fetch_secondary_coin()
    assert Decimal.equal?(quote.fiat_value, 4)
    assert_decimal(quote.available_supply, supply)
    assert_decimal(quote.market_cap, cap)

    assert {:ok, state} = CoinFetcher.init([])
    assert_received {:update, true}
    assert {:noreply, state} = CoinFetcher.handle_info({:update, true}, state)
    assert_receive message = {_ref, {{:ok, _quote}, true}}, 5_000
    assert {:noreply, _state} = CoinFetcher.handle_info(message, state)
    cached = Market.get_secondary_coin_exchange_rate()
    assert_decimal(cached.market_cap, cap)
    assert_decimal(cached.available_supply, supply)

    token_state = %TokenFetcher{source: Chainflow, max_batch_size: 2, interval: 60_000, refetch_interval: 60_000}
    assert {:noreply, _state} = TokenFetcher.handle_info({:fetch, 0}, token_state)
    row = Repo.get!(ChainToken, context.sid.contract_address_hash)
    assert Decimal.equal?(row.fiat_value, 4)
    assert_decimal(row.circulating_market_cap, cap)
    assert Decimal.equal?(row.volume_24h, 120)

    assert {:ok, [point]} = Chainflow.fetch_secondary_coin_price_history(2)
    assert point.secondary_coin
    assert Decimal.equal?(point.opening_price, 2)
    assert Decimal.equal?(point.closing_price, 3)
    assert_decimal(point.market_cap, history_cap)
    assert {:ok, {1, _}} = MarketHistory.bulk_insert([point])
    assert_decimal(MarketHistory.price_at_date(point.date, true).market_cap, history_cap)
  end

  defp exclude_balances(context, amounts) do
    addresses =
      Enum.map(amounts, fn amount ->
        address = insert(:address)
        insert_balance(context.sid, address, amount)
        address
      end)

    Application.put_env(
      :explorer,
      Chainflow,
      Keyword.put(context.config, :secondary_coin_non_circulating_addresses, Enum.map(addresses, &to_string(&1.hash)))
    )

    addresses
  end

  defp insert_balance(token, address, amount) do
    insert(:address_current_token_balance,
      address: address,
      token_contract_address_hash: token.contract_address_hash,
      token_type: token.type,
      value: Decimal.mult(Decimal.new(amount), Decimal.new(Integer.pow(10, Decimal.to_integer(token.decimals))))
    )
  end

  defp assert_decimal(actual, nil), do: assert(is_nil(actual))
  defp assert_decimal(actual, expected), do: assert(Decimal.equal?(actual, expected))

  defp restore(app, key, nil), do: Application.delete_env(app, key)
  defp restore(app, key, value), do: Application.put_env(app, key, value)
end
