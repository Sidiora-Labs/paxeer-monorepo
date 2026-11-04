defmodule Explorer.Market.Fetcher.TokenTest do
  use Explorer.DataCase

  import Mox

  alias Plug.Conn
  alias Explorer.Chain.Token
  alias Explorer.Market.Fetcher.Token, as: TokenFetcher

  @moduletag :capture_log

  setup :verify_on_exit!

  describe "handle_info(:fetch, state)" do
    setup do
      bypass = Bypass.open()

      http_client_configuration = Application.fetch_env(:explorer, :http_client)
      token_fetcher_enabled = :persistent_term.get(:market_token_fetcher_enabled, :absent)
      :persistent_term.put(:market_token_fetcher_enabled, true)
      Application.put_env(:explorer, :http_client, HTTPoison)

      source_configuration = Application.get_env(:explorer, Explorer.Market.Source)
      fetcher_configuration = Application.get_env(:explorer, Explorer.Market.Fetcher.Token)
      coin_gecko_configuration = Application.get_env(:explorer, Explorer.Market.Source.CoinGecko)

      Application.put_env(:explorer, Explorer.Market.Source, tokens_source: Explorer.Market.Source.CoinGecko)

      Application.put_env(:explorer, Explorer.Market.Fetcher.Token,
        enabled: true,
        interval: 0,
        refetch_interval: 10000,
        max_batch_size: 10
      )

      Application.put_env(:explorer, Explorer.Market.Source.CoinGecko,
        platform: "ethereum",
        currency: "usd",
        base_url: "http://localhost:#{bypass.port}"
      )

      Application.put_env(:tesla, :adapter, Tesla.Adapter.Mint)

      on_exit(fn ->
        case http_client_configuration do
          {:ok, original} -> Application.put_env(:explorer, :http_client, original)
          :error -> Application.delete_env(:explorer, :http_client)
        end

        if token_fetcher_enabled == :absent do
          :persistent_term.erase(:market_token_fetcher_enabled)
        else
          :persistent_term.put(:market_token_fetcher_enabled, token_fetcher_enabled)
        end

        Application.put_env(:explorer, Explorer.Market.Source, source_configuration)
        Application.put_env(:explorer, Explorer.Market.Fetcher.Token, fetcher_configuration)
        Application.put_env(:explorer, Explorer.Market.Source.CoinGecko, coin_gecko_configuration)
        Application.put_env(:tesla, :adapter, Explorer.Mock.TeslaAdapter)
      end)

      [token_with_no_exchange_rate | tokens] =
        for _ <- 0..4 do
          insert(:token, fiat_value: nil)
        end

      {:ok, %{bypass: bypass, tokens: tokens, excluded_token: token_with_no_exchange_rate}}
    end

    test "success fetch", %{bypass: bypass, tokens: tokens, excluded_token: excluded_token} do
      provider_ids = Map.new(tokens, &{&1.contract_address_hash, "#{&1.contract_address_hash}_id"})

      coins_list =
        tokens
        |> Enum.map(fn %{contract_address_hash: contract_address_hash} ->
          %{
            "id" => Map.fetch!(provider_ids, contract_address_hash),
            "symbol" => "#{contract_address_hash}_symbol",
            "name" => "#{contract_address_hash}_name",
            "platforms" => %{
              "some_other_chain" => "we do not want this to appear in the /simple/token_price/ request",
              "ethereum" => "#{contract_address_hash}"
            }
          }
        end)

      Bypass.expect_once(bypass, "GET", "/coins/list", fn conn ->
        assert conn.query_string == "include_platform=true"
        Conn.resp(conn, 200, Jason.encode!(coins_list))
      end)

      token_exchange_rates =
        tokens
        |> Enum.with_index(1)
        |> Map.new(fn {%{contract_address_hash: contract_address_hash}, index} ->
          {Map.fetch!(provider_ids, contract_address_hash),
           %{"usd" => index * 7 / 10, "usd_market_cap" => index * 1000, "usd_24h_vol" => index * 100}}
        end)

      joined_ids =
        tokens
        |> Enum.reverse()
        |> Enum.map_join(",", &Map.fetch!(provider_ids, &1.contract_address_hash))

      Bypass.expect_once(
        bypass,
        "GET",
        "/simple/price",
        fn conn ->
          assert conn.query_string ==
                   "vs_currencies=usd&include_market_cap=true&include_24hr_vol=true&ids=#{joined_ids}"

          Conn.resp(conn, 200, Jason.encode!(token_exchange_rates))
        end
      )

      start_supervised!(TokenFetcher)

      await_fetched_tokens(tokens, System.monotonic_time(:millisecond) + 5_000)

      Enum.each(tokens, fn %{contract_address_hash: contract_address_hash} = token ->
        provider_id = Map.fetch!(provider_ids, contract_address_hash)
        expected_usd = token_exchange_rates |> Map.fetch!(provider_id) |> Map.fetch!("usd") |> Decimal.from_float()
        fiat_value = Repo.reload!(token).fiat_value

        refute is_nil(fiat_value)
        assert Decimal.equal?(fiat_value, expected_usd)
      end)

      assert is_nil(Repo.reload!(excluded_token).fiat_value)
    end

    test "empty body in /coins/list response", %{bypass: bypass} do
      observer = self()

      Bypass.expect(bypass, "GET", "/coins/list", fn conn ->
        assert conn.query_string == "include_platform=true"
        send(observer, :fetch_response_requested)
        Conn.resp(conn, 200, "[]")
      end)

      fetcher = start_supervised!(TokenFetcher)

      assert_receive :fetch_response_requested, 5_000
      assert %TokenFetcher{} = :sys.get_state(fetcher, 5_000)

      Repo.all(Token)
      |> Enum.each(fn %{fiat_value: fiat_value} -> assert is_nil(fiat_value) end)
    end

    test "empty body in fetch response", %{bypass: bypass, tokens: tokens} do
      observer = self()

      coins_list =
        tokens
        |> Enum.map(fn %{contract_address_hash: contract_address_hash} ->
          %{
            "id" => "#{contract_address_hash}_id",
            "symbol" => "#{contract_address_hash}_symbol",
            "name" => "#{contract_address_hash}_name",
            "platforms" => %{
              "some_other_chain" => "we do not want this to appear in the /simple/token_price/ request",
              "ethereum" => "#{contract_address_hash}"
            }
          }
        end)

      Bypass.expect_once(bypass, "GET", "/coins/list", fn conn ->
        assert conn.query_string == "include_platform=true"
        Conn.resp(conn, 200, Jason.encode!(coins_list))
      end)

      joined_ids =
        tokens
        |> Enum.reverse()
        |> Enum.map_join(",", fn %{contract_address_hash: contract_address_hash} -> "#{contract_address_hash}_id" end)

      Bypass.expect_once(
        bypass,
        "GET",
        "/simple/price",
        fn conn ->
          assert conn.query_string ==
                   "vs_currencies=usd&include_market_cap=true&include_24hr_vol=true&ids=#{joined_ids}"

          send(observer, :fetch_response_requested)
          Conn.resp(conn, 200, "{}")
        end
      )

      fetcher = start_supervised!(TokenFetcher)

      assert_receive :fetch_response_requested, 5_000
      assert %TokenFetcher{} = :sys.get_state(fetcher, 5_000)

      Repo.all(Token)
      |> Enum.each(fn %{fiat_value: fiat_value} -> assert is_nil(fiat_value) end)
    end

    test "error in /coins/list response", %{bypass: bypass} do
      observer = self()

      Bypass.expect(bypass, "GET", "/coins/list", fn conn ->
        assert conn.query_string == "include_platform=true"
        send(observer, :fetch_response_requested)
        Conn.resp(conn, 429, "Too many requests")
      end)

      fetcher = start_supervised!(TokenFetcher)

      assert_receive :fetch_response_requested, 5_000
      assert %TokenFetcher{} = :sys.get_state(fetcher, 5_000)

      Repo.all(Token)
      |> Enum.each(fn %{fiat_value: fiat_value} -> assert is_nil(fiat_value) end)
    end

    test "error in fetch response", %{bypass: bypass, tokens: tokens} do
      observer = self()

      coins_list =
        tokens
        |> Enum.map(fn %{contract_address_hash: contract_address_hash} ->
          %{
            "id" => "#{contract_address_hash}_id",
            "symbol" => "#{contract_address_hash}_symbol",
            "name" => "#{contract_address_hash}_name",
            "platforms" => %{
              "some_other_chain" => "we do not want this to appear in the /simple/token_price/ request",
              "ethereum" => "#{contract_address_hash}"
            }
          }
        end)

      Bypass.expect(bypass, "GET", "/coins/list", fn conn ->
        assert conn.query_string == "include_platform=true"
        Conn.resp(conn, 200, Jason.encode!(coins_list))
      end)

      joined_ids =
        tokens
        |> Enum.reverse()
        |> Enum.map_join(",", fn %{contract_address_hash: contract_address_hash} -> "#{contract_address_hash}_id" end)

      Bypass.expect(
        bypass,
        "GET",
        "/simple/price",
        fn conn ->
          assert conn.query_string ==
                   "vs_currencies=usd&include_market_cap=true&include_24hr_vol=true&ids=#{joined_ids}"

          send(observer, :fetch_response_requested)
          Conn.resp(conn, 429, "Too many requests")
        end
      )

      fetcher = start_supervised!(TokenFetcher)

      assert_receive :fetch_response_requested, 5_000
      assert %TokenFetcher{} = :sys.get_state(fetcher, 5_000)

      Repo.all(Token)
      |> Enum.each(fn %{fiat_value: fiat_value} -> assert is_nil(fiat_value) end)
    end
  end

  defp await_fetched_tokens(tokens, deadline) do
    unless Enum.all?(tokens, &(not is_nil(Repo.reload!(&1).fiat_value))) do
      assert System.monotonic_time(:millisecond) < deadline, "token prices were not imported before the deadline"
      Process.sleep(10)
      await_fetched_tokens(tokens, deadline)
    end
  end
end
