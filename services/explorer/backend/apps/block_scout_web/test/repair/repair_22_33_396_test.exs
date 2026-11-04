defmodule BlockScoutWeb.Repair22_33_396Test do
  @moduledoc """
  Repair record 22.33.396 for audit 22.30.142:
  27: "finding 27: both write tabs honor interaction settings on both token routes".
  31: "finding 31: restricted counters deny access before populating either cache".
  10: "finding 10: invalid counters are rejected before the token query" and
  "finding 10: valid counters preserve the recorded token and rendered rank".
  Contract tabs use the committed SimpleStorage bytecode, source and ABI fixture;
  counter and listing requests use the CryptoKitties contract and holder recorded
  in Indexer.Fetcher.TokenInstance.HelperTest and repair_22_31_391_test.exs.
  Requests traverse the production WebRouter, controller, repository and views.
  Baseline verification was not run because the bounded gate order requires full
  implementation before hf-build and hf-done; no baseline failures are claimed.
  First qualification: the contract case lacked its persisted proxy snapshot;
  the listing case lacked query isolation from other persisted tokens. Both
  fixtures are completed without weakening assertions or adding RPC mocks.
  No out-of-path wiring changes are required. Gate evidence records the verified revision.
  """

  use BlockScoutWeb.ConnCase, async: false

  alias Explorer.Chain.Address
  alias Explorer.Repo

  @endpoint BlockScoutWeb.Routers.WebRouter
  @key "repair+key&22=33?396"
  @kitty_contract "0x06012c8cf97bead5deae237070f9587f8e7a266d"
  @kitty_owner "0x7b9ea9ac69b8fde875554321472c732eeff06ca0"

  setup %{conn: conn} do
    for {app, key} <- [
          {:block_scout_web, :contract},
          {:explorer, :addresses_blacklist},
          {:explorer, :addresses_blacklist_key}
        ] do
      previous = Application.fetch_env(app, key)

      on_exit(fn ->
        case previous do
          {:ok, value} -> Application.put_env(app, key, value)
          :error -> Application.delete_env(app, key)
        end
      end)
    end

    Application.put_env(:explorer, :addresses_blacklist, "")
    Application.put_env(:explorer, :addresses_blacklist_key, @key)

    conn =
      conn
      |> Plug.Test.init_test_session(%{})
      |> put_private(:phoenix_endpoint, BlockScoutWeb.Endpoint)
      |> Map.put(:secret_key_base, BlockScoutWeb.Endpoint.config(:secret_key_base))

    {:ok, conn: conn}
  end

  test "finding 27: both write tabs honor interaction settings on both token routes", %{conn: conn} do
    contract = insert(:smart_contract, language: :solidity, is_vyper_contract: false)
    insert(:proxy_implementation, proxy_address_hash: contract.address_hash, address_hashes: [], names: [])
    address = Repo.get!(Address, contract.address_hash)
    insert(:token, contract_address: address)
    hash = Address.checksum(contract.address_hash)
    config = Application.fetch_env!(:block_scout_web, :contract)

    for disabled <- [true, false] do
      Application.put_env(:block_scout_web, :contract, Keyword.put(config, :disable_interaction, disabled))

      for prefix <- ["/token", "/tokens"], tab <- ["write-contract", "write-proxy"] do
        response = get(conn, "#{prefix}/#{hash}/#{tab}")

        if disabled do
          assert html_response(response, 404)
          refute response.resp_body =~ "data-smart-contract-functions"
        else
          body = html_response(response, 200)
          assert body =~ ~s(data-action="write")
          assert body =~ ~s(data-type="#{if tab == "write-proxy", do: "proxy", else: "regular"}")
          assert body =~ ~s(data-hash="#{hash}")
        end
      end

      for prefix <- ["/token", "/tokens"], tab <- ["read-contract", "read-proxy"] do
        body = conn |> get("#{prefix}/#{hash}/#{tab}") |> html_response(200)
        assert body =~ ~s(data-action="read")
        assert body =~ ~s(data-type="#{if tab == "read-proxy", do: "proxy", else: "regular"}")
        assert body =~ ~s(data-hash="#{hash}")
      end
    end
  end

  test "finding 31: restricted counters deny access before populating either cache", %{conn: conn} do
    token = recorded_token()
    hash = Address.checksum(token.contract_address_hash)
    Application.put_env(:explorer, :addresses_blacklist, String.upcase(hash))
    cache_key = "hash_#{token.contract_address_hash}"
    tables = [:token_holders_count, :token_transfers_counter]

    for table <- tables, key <- [cache_key, cache_key <> "_last_update"] do
      :ets.delete(table, key)

      on_exit(fn -> :ets.delete(table, key) end)
    end

    for params <- [%{"id" => hash}, %{"id" => hash, "key" => "wrong"}, %{"id" => "invalid"}] do
      response = get(conn, "/token-counters", params)
      assert html_response(response, 404)
      refute response.resp_body =~ "token_holder_count"

      for table <- tables do
        assert :ets.lookup(table, cache_key) == []
        assert :ets.lookup(table, cache_key <> "_last_update") == []
      end
    end

    authorized = conn |> get("/token-counters", %{"id" => hash, "key" => @key}) |> json_response(200)
    assert authorized == %{"transfer_count" => 0, "token_holder_count" => 1}
    assert :ets.lookup(:token_holders_count, cache_key) == [{cache_key, 1}]
    assert :ets.lookup(:token_transfers_counter, cache_key) == [{cache_key, 0}]

    assert conn |> get("/token-counters", %{"id" => hash}) |> html_response(404)

    Application.put_env(:explorer, :addresses_blacklist, "")
    assert conn |> get("/token-counters", %{"id" => hash}) |> json_response(200) == authorized
  end

  test "finding 10: invalid counters are rejected before the token query", %{conn: conn} do
    recorded_token()
    observe_token_queries()

    for value <- ["abc", "1trailing", "-1", "", "9223372036854775808", ["1"], %{"value" => "1"}] do
      response = get(conn, "/tokens", %{"type" => "JSON", "items_count" => value})
      assert html_response(response, 422)
      refute_received :token_listing_query
    end
  end

  test "finding 10: valid counters preserve the recorded token and rendered rank", %{conn: conn} do
    token = recorded_token()
    insert(:token, name: "OtherToken", symbol: "OTHER")
    observe_token_queries()

    for {counter, expected_rank} <- [{nil, "1"}, {"0", "1"}, {"50", "51"}] do
      query = %{"type" => "JSON", "filter" => "CryptoKitties"}
      params = if is_nil(counter), do: query, else: Map.put(query, "items_count", counter)
      response = conn |> get("/tokens", params) |> json_response(200)
      assert_received :token_listing_query
      assert %{"items" => [item], "next_page_path" => nil} = response
      assert item =~ to_string(token.contract_address_hash)
      assert item =~ "CryptoKitties"

      assert item |> Floki.parse_fragment!() |> Floki.find(".color-lighten") |> Floki.text() |> String.trim() ==
               expected_rank
    end
  end

  def record_query(_event, _measurements, metadata, owner) do
    if self() == owner && metadata.repo == Repo && String.contains?(metadata.query, ~s(FROM "tokens")) do
      send(owner, :token_listing_query)
    end
  end

  defp observe_token_queries do
    id = {__MODULE__, self()}
    prefix = Repo.config()[:telemetry_prefix] || [:explorer, :repo]
    :ok = :telemetry.attach(id, prefix ++ [:query], &__MODULE__.record_query/4, self())
    on_exit(fn -> :telemetry.detach(id) end)
  end

  defp recorded_token do
    token =
      insert(:token,
        contract_address: insert(:address, hash: @kitty_contract),
        type: "ERC-721",
        name: "CryptoKitties",
        symbol: "CK",
        decimals: 0
      )

    insert(:address_current_token_balance,
      address: insert(:address, hash: @kitty_owner),
      token_contract_address_hash: token.contract_address_hash,
      token_type: "ERC-721",
      value: 1
    )

    token
  end
end
