defmodule Explorer.Market.Source.ChainflowFormatTest do
  use Explorer.DataCase

  alias Explorer.Chain
  alias Explorer.Chain.Token, as: ChainToken
  alias Explorer.Market.Source.Chainflow
  alias Plug.Conn

  setup do
    bypass = Bypass.open()
    old_source = Application.get_env(:explorer, Chainflow)
    old_supply = Application.get_env(:explorer, :supply)
    old_adapter = Application.get_env(:tesla, :adapter)
    key = Base.encode64(:crypto.strong_rand_bytes(24))
    base_url = "http://localhost:#{bypass.port}"

    wpax = insert(:token, symbol: "WPAX", decimals: 6, total_supply: 250_000_000)
    sid = insert(:token, symbol: "SID", decimals: 18, total_supply: Decimal.new("400000000000000000000"))

    config = [
      base_url: base_url,
      api_key: key,
      coin_id: "paxeer",
      secondary_coin_id: "sidiora",
      coin_symbol: "PAX",
      secondary_coin_symbol: "SID",
      tokens: [[to_string(wpax.contract_address_hash), "PAX"], [to_string(sid.contract_address_hash), "SID"]]
    ]

    Application.put_env(:explorer, Chainflow, config)
    Application.put_env(:explorer, :supply, Explorer.Chain.Supply.ProofOfAuthority)
    Application.put_env(:tesla, :adapter, Tesla.Adapter.Mint)

    on_exit(fn ->
      restore(:explorer, Chainflow, old_source)
      restore(:explorer, :supply, old_supply)
      restore(:tesla, :adapter, old_adapter)
    end)

    responses = %{
      "paxeer" => coin_response("Paxeer", "pax", base_url <> "/pax.png", "2.5", 123_456),
      "sidiora" => coin_response("Sidiora", "sid", base_url <> "/sid.png", "4", 654_321)
    }

    data = start_supervised!({Agent, fn -> %{coins: responses, status: 200} end})
    owner = self()

    Bypass.expect(bypass, fn conn ->
      conn = Conn.fetch_query_params(conn)
      send(owner, {:request, conn.request_path, Conn.get_req_header(conn, "x-api-key")})
      values = Agent.get(data, & &1)

      body =
        if values.status == 200 do
          case String.split(conn.request_path, "/", trim: true) do
            ["api", "v3", "coins", id] ->
              Map.fetch!(values.coins, id)

            ["api", market, "history"] when market in ["pax", "sid"] ->
              assert conn.query_params["symbol"] == String.upcase(market)
              assert conn.query_params["resolution"] == "1D"
              %{"s" => "ok", "t" => [1_767_312_000], "o" => ["2"], "c" => ["3"]}
          end
        else
          %{"error" => "Unauthorized"}
        end

      conn |> Conn.put_resp_content_type("application/json") |> Conn.resp(values.status, Jason.encode!(body))
    end)

    {:ok, config: config, key: key, data: data, wpax: wpax, sid: sid, base_url: base_url}
  end

  test "coin requests carry the key and map the feed figures", context do
    assert {:ok, pax} = Chainflow.fetch_native_coin()
    assert_received {:request, "/api/v3/coins/paxeer", [key]}
    assert key == context.key
    assert pax.name == "Paxeer"
    assert pax.symbol == "PAX"
    assert pax.image_url == context.base_url <> "/pax.png"
    assert Decimal.equal?(pax.fiat_value, "2.5")
    assert Decimal.equal?(pax.market_cap, 123_456)
    assert Decimal.equal?(pax.volume_24h, "987.65")
    assert Decimal.equal?(pax.available_supply, 777)
    assert Decimal.equal?(pax.total_supply, 999)
    assert pax.last_updated == ~U[2026-01-02 00:00:00Z]
    assert pax.btc_value == nil
    assert pax.tvl == nil

    assert {:ok, sid} = Chainflow.fetch_secondary_coin()
    assert_received {:request, "/api/v3/coins/sidiora", [^key]}
    assert sid.name == "Sidiora"
    assert sid.symbol == "SID"
    assert sid.image_url == context.base_url <> "/sid.png"
    assert Decimal.equal?(sid.fiat_value, 4)
    assert Decimal.equal?(sid.market_cap, 654_321)
    assert Decimal.equal?(sid.volume_24h, "987.65")
    assert Decimal.equal?(sid.available_supply, 777)
    assert Decimal.equal?(sid.total_supply, 999)
    assert sid.last_updated == pax.last_updated
  end

  test "token rows use the PAX and SID figures and key", %{key: key, wpax: wpax, sid: sid} do
    assert {:ok, nil, true, [wrapped, secondary]} = Chainflow.fetch_tokens(nil, 2)
    assert_received {:request, "/api/v3/coins/paxeer", [^key]}
    assert_received {:request, "/api/v3/coins/sidiora", [^key]}
    assert wrapped.contract_address_hash == wpax.contract_address_hash
    assert secondary.contract_address_hash == sid.contract_address_hash
    assert wrapped.type == wpax.type
    assert secondary.type == sid.type
    assert Decimal.equal?(wrapped.fiat_value, "2.5")
    assert Decimal.equal?(wrapped.circulating_market_cap, 123_456)
    assert Decimal.equal?(wrapped.volume_24h, "987.65")
    assert Decimal.equal?(secondary.fiat_value, 4)
    assert Decimal.equal?(secondary.circulating_market_cap, 654_321)
    assert Decimal.equal?(secondary.volume_24h, "987.65")
  end

  test "all history requests retain the history route and carry the key", %{key: key} do
    assert {:ok, [native]} = Chainflow.fetch_native_coin_price_history(2)
    assert_received {:request, "/api/pax/history", [^key]}
    assert_received {:request, "/api/v3/coins/paxeer", [^key]}
    assert {:ok, [secondary]} = Chainflow.fetch_secondary_coin_price_history(2)
    assert_received {:request, "/api/sid/history", [^key]}
    assert {:ok, [cap]} = Chainflow.fetch_market_cap_history(2)
    assert_received {:request, "/api/pax/history", [^key]}
    assert_received {:request, "/api/v3/coins/paxeer", [^key]}
    assert native.date == ~D[2026-01-02]
    assert Decimal.equal?(native.opening_price, 2)
    assert Decimal.equal?(native.closing_price, 3)
    assert Decimal.equal?(native.market_cap, 2331)
    assert secondary.secondary_coin
    assert Decimal.equal?(secondary.market_cap, 2331)
    assert cap == Map.take(native, [:date, :market_cap])
  end

  for key <- [nil, ""] do
    test "coin and history requests omit the key header when configured as #{inspect(key)}", %{config: config} do
      Application.put_env(:explorer, Chainflow, Keyword.put(config, :api_key, unquote(key)))
      assert {:ok, _} = Chainflow.fetch_native_coin()
      assert_received {:request, "/api/v3/coins/paxeer", []}
      assert {:ok, _} = Chainflow.fetch_secondary_coin()
      assert_received {:request, "/api/v3/coins/sidiora", []}
      assert {:ok, _} = Chainflow.fetch_native_coin_price_history(2)
      assert_received {:request, "/api/pax/history", []}
      assert {:ok, _} = Chainflow.fetch_secondary_coin_price_history(2)
      assert_received {:request, "/api/sid/history", []}
      assert {:ok, _} = Chainflow.fetch_market_cap_history(2)
      assert_received {:request, "/api/pax/history", []}
    end
  end

  test "null market caps use the native held supply and secondary feed circulating supply", %{data: data} do
    clear_market_caps(data)
    assert {:ok, native} = Chainflow.fetch_native_coin()
    assert Decimal.equal?(native.market_cap, Decimal.mult(Chain.circulating_supply(), "2.5"))
    assert Decimal.equal?(native.available_supply, 777)
    assert {:ok, secondary} = Chainflow.fetch_secondary_coin()
    assert Decimal.equal?(secondary.market_cap, 3108)
    assert {:ok, nil, true, [wpax, sid]} = Chainflow.fetch_tokens(nil, 2)
    assert Decimal.equal?(wpax.circulating_market_cap, 625)
    assert Decimal.equal?(sid.circulating_market_cap, 3108)
  end

  test "secondary feed circulating supply remains available without explorer held supply", %{data: data, sid: sid} do
    clear_market_caps(data)
    sid |> Ecto.Changeset.change(total_supply: nil) |> Repo.update!()
    assert {:ok, secondary} = Chainflow.fetch_secondary_coin()
    assert Decimal.equal?(secondary.fiat_value, 4)
    assert Decimal.equal?(secondary.available_supply, 777)
    assert Decimal.equal?(secondary.market_cap, 3108)
    assert {:ok, nil, true, [_wpax, token]} = Chainflow.fetch_tokens(nil, 2)
    assert Decimal.equal?(token.circulating_market_cap, 3108)
  end

  test "a zero feed market cap is retained", %{data: data} do
    Agent.update(data, &put_in(&1, [:coins, "paxeer", "market_data", "market_cap", "usd"], 0))
    assert {:ok, native} = Chainflow.fetch_native_coin()
    assert Decimal.equal?(native.market_cap, 0)
  end

  test "configured coin ids select both coin routes including token rows", %{config: config, data: data} do
    Agent.update(data, fn values ->
      %{values | coins: %{"native-coin" => values.coins["paxeer"], "secondary-coin" => values.coins["sidiora"]}}
    end)

    Application.put_env(
      :explorer,
      Chainflow,
      Keyword.merge(config, coin_id: "native-coin", secondary_coin_id: "secondary-coin")
    )

    assert {:ok, native} = Chainflow.fetch_native_coin()
    assert native.symbol == "PAX"
    assert_received {:request, "/api/v3/coins/native-coin", [_key]}
    assert {:ok, secondary} = Chainflow.fetch_secondary_coin()
    assert secondary.symbol == "SID"
    assert_received {:request, "/api/v3/coins/secondary-coin", [_key]}
    assert {:ok, nil, true, [wpax, sid]} = Chainflow.fetch_tokens(nil, 2)
    assert Decimal.equal?(wpax.fiat_value, native.fiat_value)
    assert Decimal.equal?(sid.fiat_value, secondary.fiat_value)
  end

  test "unauthorized replies return errors without market figures", %{data: data, wpax: wpax, sid: sid, key: key} do
    before = Enum.map([wpax, sid], &Repo.get!(ChainToken, &1.contract_address_hash))
    Agent.update(data, &%{&1 | status: 401})
    assert {:error, "401: " <> _} = Chainflow.fetch_native_coin()
    assert_received {:request, "/api/v3/coins/paxeer", [^key]}
    assert {:error, "401: " <> _} = Chainflow.fetch_secondary_coin()
    assert_received {:request, "/api/v3/coins/sidiora", [^key]}
    assert {:error, "401: " <> _} = Chainflow.fetch_tokens(nil, 2)
    assert {:error, "401: " <> _} = Chainflow.fetch_native_coin_price_history(2)
    assert {:error, "401: " <> _} = Chainflow.fetch_secondary_coin_price_history(2)
    assert {:error, "401: " <> _} = Chainflow.fetch_market_cap_history(2)
    assert Enum.map([wpax, sid], &Repo.get!(ChainToken, &1.contract_address_hash)) == before
  end

  defp coin_response(name, symbol, image, price, cap) do
    %{
      "name" => name,
      "symbol" => symbol,
      "image" => %{"small" => image},
      "market_data" => %{
        "current_price" => %{"usd" => price},
        "market_cap" => %{"usd" => cap},
        "total_volume" => %{"usd" => "987.65"},
        "circulating_supply" => 777,
        "total_supply" => 999,
        "last_updated" => "2026-01-02T00:00:00Z"
      }
    }
  end

  defp clear_market_caps(data) do
    Agent.update(data, fn values ->
      coins = Map.new(values.coins, fn {id, coin} -> {id, put_in(coin, ["market_data", "market_cap", "usd"], nil)} end)
      %{values | coins: coins}
    end)
  end

  defp restore(app, key, nil), do: Application.delete_env(app, key)
  defp restore(app, key, value), do: Application.put_env(app, key, value)
end
