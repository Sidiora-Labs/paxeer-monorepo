defmodule BlockScoutWeb.API.V2.PaxeerX.CapabilitiesControllerTest do
  use BlockScoutWeb.ConnCase, async: false

  import Mox

  alias Explorer.Chain.PaxeerX.{Capabilities, PriceFeed}

  @price_feed_fetcher Indexer.Fetcher.PaxeerX.PriceUpdates.Supervisor

  @head "0x16c1320"

  # `EthereumJSONRPC.integer_to_quantity/1` spells the height back in uppercase
  # hexadecimal, which is what the probe requests carry.
  @head_quantity "0x16C1320"

  @code_addresses [
    "0x0000000000000000000000000000000000001015",
    "0x0000000000000000000000000000000000001016",
    "0x0000000000000000000000000000000000001017"
  ]

  @addr_precompile "0x0000000000000000000000000000000000001004"
  @view_calls %{
    "0x0000000000000000000000000000000000001015" => "0xb71d031c" <> String.duplicate("0", 64),
    "0x0000000000000000000000000000000000001016" => "0xb187bd26"
  }
  @view_word "0x" <> String.duplicate("0", 64)

  @unified_account_answer "0x" <> String.duplicate("0", 320)

  @bytecode "0x60806040523480156100105760006000fd"

  @surfaces ~w(addr exchange bridge launchpad)

  describe "/api/v2/paxeer-x/capabilities" do
    setup do
      for key <- [Indexer.Supervisor, @price_feed_fetcher] do
        previous = Application.fetch_env(:indexer, key)

        on_exit(fn ->
          case previous do
            {:ok, value} -> Application.put_env(:indexer, key, value)
            :error -> Application.delete_env(:indexer, key)
          end
        end)
      end

      :ok
    end

    test "omits price_feed when only failed, unavailable or stale scan rows exist", %{conn: conn} do
      Application.put_env(:indexer, Indexer.Supervisor, enabled: true)
      Application.put_env(:indexer, @price_feed_fetcher, disabled?: false)
      assert %{"price_feed" => true} = conn |> get("/api/v2/paxeer-x/capabilities") |> json_response(200)

      insert(:block, number: 2000)
      insert(:block, number: 1999)
      stale_block = insert(:block, number: 999)
      assert {:ok, 0} = PriceFeed.replace_block_updates(stale_block, [])
      assert :ok = PriceFeed.mark_unscannable(2000, "failed", "scan failed")
      assert :ok = PriceFeed.mark_unscannable(1999, "unavailable", "block pruned")

      Application.put_env(:indexer, @price_feed_fetcher, disabled?: true)
      response = conn |> get("/api/v2/paxeer-x/capabilities") |> json_response(200)
      refute Map.has_key?(response, "price_feed")

      Application.put_env(:indexer, Indexer.Supervisor, enabled: false)

      for disabled? <- [true, false] do
        Application.put_env(:indexer, @price_feed_fetcher, disabled?: disabled?)
        response = conn |> get("/api/v2/paxeer-x/capabilities") |> json_response(200)
        refute Map.has_key?(response, "price_feed")
      end
    end

    test "reports price_feed from a recent scanned row", %{conn: conn} do
      Application.put_env(:indexer, Indexer.Supervisor, enabled: false)
      Application.put_env(:indexer, @price_feed_fetcher, disabled?: true)

      insert(:block, number: 2000)
      recent_block = insert(:block, number: 1000)
      assert {:ok, 0} = PriceFeed.replace_block_updates(recent_block, [])

      assert %{"price_feed" => true} = conn |> get("/api/v2/paxeer-x/capabilities") |> json_response(200)

      Application.put_env(:indexer, Indexer.Supervisor, enabled: true)
      response = conn |> get("/api/v2/paxeer-x/capabilities") |> json_response(200)
      refute Map.has_key?(response, "price_feed")

      Application.put_env(:indexer, @price_feed_fetcher, disabled?: false)
      assert %{"price_feed" => true} = conn |> get("/api/v2/paxeer-x/capabilities") |> json_response(200)
    end
  end

  test "reports every surface absent while nothing has probed the node", %{conn: conn} do
    response =
      conn
      |> get("/api/v2/paxeer-x/capabilities")
      |> json_response(200)

    for surface <- @surfaces do
      assert response[surface] == false
    end
  end

  test "answers with the four surfaces and checked_at", %{conn: conn} do
    response =
      conn
      |> get("/api/v2/paxeer-x/capabilities")
      |> json_response(200)

    assert Enum.sort(Map.keys(response) -- ["price_feed"]) == Enum.sort(["checked_at" | @surfaces])
    refute Map.has_key?(response, "custody")
    refute Map.has_key?(response, "anchor")
  end

  test "reports the surfaces of the last probe", %{conn: conn} do
    set_mox_global()

    stub(EthereumJSONRPC.Mox, :json_rpc, fn
      %{method: "eth_blockNumber"}, _options ->
        {:ok, @head}

      requests, _options when is_list(requests) ->
        {:ok, Enum.map(requests, &answer/1)}
    end)

    start_supervised!(Capabilities)

    assert {:ok, _snapshot} = Capabilities.refresh()

    response =
      conn
      |> get("/api/v2/paxeer-x/capabilities")
      |> json_response(200)

    for surface <- @surfaces do
      assert response[surface] == true
    end
  end

  defp answer(%{id: id, method: "eth_getCode", params: [address, block]}) do
    assert address in @code_addresses
    assert address == "0x0000000000000000000000000000000000001017"
    assert block == @head_quantity

    %{id: id, jsonrpc: "2.0", result: @bytecode}
  end

  defp answer(%{
         id: id,
         method: "eth_call",
         params: [%{to: to, data: data}, block]
       })
       when is_map_key(@view_calls, to) do
    assert data == Map.fetch!(@view_calls, to)
    assert block == @head_quantity

    %{id: id, jsonrpc: "2.0", result: @view_word}
  end

  defp answer(%{
         id: id,
         method: "eth_call",
         params: [%{to: @addr_precompile, data: _data}, block]
       }) do
    assert block == @head_quantity

    %{id: id, jsonrpc: "2.0", result: @unified_account_answer}
  end
end
