Code.require_file("../explorer/market/fetcher/token_test.exs", __DIR__)

defmodule Explorer.Repair.Repair2233308Test do
  @moduledoc """
  Task 22.33.308, audit 22.30.135 repair record.

  token-fetcher-test-key is proved by TokenTest's "success fetch" and the
  "token-fetcher-test-key: recorded provider prices reach the matching tokens"
  case below. The gate includes the original fetcher suite, without removing
  or skipping any cases. Recorded Ethereum token addresses and CoinGecko
  response shapes traverse loopback HTTP, the production CoinGecko adapter,
  TokenFetcher and Chain.import into the real Token schema and database.

  Continued from attempt 1 at 1ad700960226f5b4d67f75e1c39eda82bb418086.
  Its gate failed the original "error in /coins/list response" and "empty body
  in fetch response" cases: their 100ms sleeps ended before HTTP arrived.
  No new unchanged-tree gate ran because the ordered gates require completed
  implementation first. No untouched-mainline baseline result is claimed.
  Each original negative case now waits for its real provider request and
  synchronizes with the fetcher's completed handler before checking the
  database. All fetchers use the test supervisor to stop them before teardown.
  Both setups preserve, enable and restore the market startup flag required
  by FiatValue.load/1, and use HTTPoison against the loopback provider.
  The gate runner records the verified revision, command, exit and private log.
  No out-of-path wiring is required. Spec and gotcha ledger edits are outside
  the authorized scope, so this module is the task record.
  """

  use Explorer.DataCase, async: false

  alias Explorer.Chain.Hash.Address
  alias Explorer.Chain.Token
  alias Explorer.Market.Fetcher.Token, as: TokenFetcher
  alias Explorer.Market.Source
  alias Explorer.Market.Source.CoinGecko
  alias Plug.Conn

  @moduletag :capture_log

  @coins_list """
  [
    {"id":"usd-coin","symbol":"usdc","name":"USDC","platforms":{"ethereum":"0xa0b86991c6218b36c1d19d4a2e9eb0ce3606eb48","polygon-pos":"0x2791bca1f2de4661ed88a30c99a7a9449aa84174"}},
    {"id":"weth","symbol":"weth","name":"WETH","platforms":{"ethereum":"0xc02aaa39b223fe8d0a0e5c4f27ead9083c756cc2"}},
    {"id":"matic-network","symbol":"matic","name":"Polygon","platforms":{"polygon-pos":"0x0000000000000000000000000000000000001010"}}
  ]
  """

  @prices """
  {
    "usd-coin":{"usd":1.001,"usd_market_cap":26000000000,"usd_24h_vol":2400000000},
    "weth":{"usd":1641.72,"usd_market_cap":2800000000,"usd_24h_vol":180000000}
  }
  """

  setup do
    bypass = Bypass.open()
    token_fetcher_enabled = :persistent_term.get(:market_token_fetcher_enabled, :absent)
    :persistent_term.put(:market_token_fetcher_enabled, true)

    on_exit(fn ->
      if token_fetcher_enabled == :absent do
        :persistent_term.erase(:market_token_fetcher_enabled)
      else
        :persistent_term.put(:market_token_fetcher_enabled, token_fetcher_enabled)
      end
    end)

    for {app, key, value} <- [
          {:explorer, Source, [tokens_source: CoinGecko]},
          {:explorer, TokenFetcher, [enabled: true, interval: 0, refetch_interval: 10_000, max_batch_size: 10]},
          {:explorer, CoinGecko, [platform: "ethereum", currency: "usd", base_url: "http://127.0.0.1:#{bypass.port}"]},
          {:explorer, :http_client, HTTPoison},
          {:tesla, :adapter, Tesla.Adapter.Mint}
        ] do
      previous = Application.fetch_env(app, key)
      Application.put_env(app, key, value)

      on_exit(fn ->
        case previous do
          {:ok, original} -> Application.put_env(app, key, original)
          :error -> Application.delete_env(app, key)
        end
      end)
    end

    %{bypass: bypass}
  end

  test "token-fetcher-test-key: recorded provider prices reach the matching tokens", %{bypass: bypass} do
    provider_ids = %{
      "0xa0b86991c6218b36c1d19d4a2e9eb0ce3606eb48" => "usd-coin",
      "0xc02aaa39b223fe8d0a0e5c4f27ead9083c756cc2" => "weth"
    }

    tokens =
      Map.new(provider_ids, fn {address, provider_id} ->
        {:ok, hash} = Address.cast(address)
        token = insert(:token, contract_address: insert(:address, hash: hash), fiat_value: nil)
        {provider_id, token}
      end)

    {:ok, excluded_hash} = Address.cast("0x0000000000000000000000000000000000001010")
    excluded_token = insert(:token, contract_address: insert(:address, hash: excluded_hash), fiat_value: nil)
    observer = self()

    Bypass.expect_once(bypass, "GET", "/coins/list", fn conn ->
      assert conn.query_string == "include_platform=true"
      Conn.resp(conn, 200, @coins_list)
    end)

    Bypass.expect_once(bypass, "GET", "/simple/price", fn conn ->
      query = URI.decode_query(conn.query_string)

      assert query == %{
               "vs_currencies" => "usd",
               "include_market_cap" => "true",
               "include_24hr_vol" => "true",
               "ids" => "weth,usd-coin"
             }

      send(observer, :recorded_prices_requested)
      Conn.resp(conn, 200, @prices)
    end)

    start_supervised!(TokenFetcher)
    assert_receive :recorded_prices_requested, 5_000
    await_fetched_tokens(Map.values(tokens), System.monotonic_time(:millisecond) + 5_000)

    prices = Jason.decode!(@prices)
    assert Repo.aggregate(Token, :count) == 3

    for {address, provider_id} <- provider_ids do
      token = tokens |> Map.fetch!(provider_id) |> Repo.reload!()
      expected = Map.fetch!(prices, provider_id)

      assert String.downcase(to_string(token.contract_address_hash)) == address
      refute is_nil(token.fiat_value)
      assert Decimal.equal?(token.fiat_value, Decimal.from_float(Map.fetch!(expected, "usd")))
      assert Decimal.equal?(token.circulating_market_cap, Decimal.new(Map.fetch!(expected, "usd_market_cap")))
      assert Decimal.equal?(token.volume_24h, Decimal.new(Map.fetch!(expected, "usd_24h_vol")))
    end

    assert is_nil(Repo.reload!(excluded_token).fiat_value)
  end

  defp await_fetched_tokens(tokens, deadline) do
    unless Enum.all?(tokens, &(not is_nil(Repo.reload!(&1).fiat_value))) do
      assert System.monotonic_time(:millisecond) < deadline, "recorded prices were not imported before the deadline"
      Process.sleep(10)
      await_fetched_tokens(tokens, deadline)
    end
  end
end
