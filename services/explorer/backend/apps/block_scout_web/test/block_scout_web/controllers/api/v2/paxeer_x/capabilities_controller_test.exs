Code.require_file("repair_evidence.exs", __DIR__)

defmodule BlockScoutWeb.API.V2.PaxeerX.CapabilitiesControllerTest do
  use BlockScoutWeb.ConnCase, async: false

  alias Explorer.Chain.PaxeerX.{Capabilities, PriceFeed}
  alias BlockScoutWeb.API.V2.PaxeerX.Repair2233416Evidence, as: Evidence

  @price_feed_fetcher Indexer.Fetcher.PaxeerX.PriceUpdates.Supervisor

  @surfaces ~w(addr exchange bridge launchpad)
  @valid_answers [:canonical, :nonempty_string, :word_aligned_string]

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
    prove_capability(conn, :canonical)
  end

  def prove_capability(conn, variant) do
    canonical = Evidence.unified_account_answer()
    bytes = Evidence.bytes(canonical)

    answer =
      case variant do
        :canonical ->
          %{result: canonical}

        :nonempty_string ->
          %{result: string_answer(String.duplicate("p", 33))}

        :word_aligned_string ->
          %{result: string_answer(String.duplicate("p", 32))}

        :refused ->
          %{error: %{code: -32_000, message: "execution reverted"}}

        :wrong_selector ->
          %{result: "0x357feed6"}

        :zero_offset ->
          %{result: Evidence.hex(replace_word(bytes, 1, 0))}

        :unaligned_offset ->
          %{result: Evidence.hex(replace_word(bytes, 1, 129))}

        :outside_offset ->
          %{result: Evidence.hex(replace_word(bytes, 1, 192))}

        :missing_tail ->
          %{result: Evidence.hex(binary_part(bytes, 0, 128))}

        :truncated_length ->
          %{result: Evidence.hex(binary_part(bytes, 0, 159))}

        :truncated_string ->
          %{result: Evidence.hex(replace_word(bytes, 4, 1))}

        :invalid_address_padding ->
          %{result: Evidence.hex(replace_word(bytes, 0, Integer.pow(2, 160)))}

        :invalid_string_padding ->
          %{result: Evidence.hex(replace_word(bytes, 4, 1) <> <<"p", 0::240, 1::8>>)}

        :extra_tail ->
          %{result: Evidence.hex(bytes <> <<0::256>>)}
      end

    node = Evidence.capability_node(answer)
    original = Application.fetch_env(:explorer, :json_rpc_named_arguments)
    on_exit(fn -> Evidence.restore_env(:explorer, :json_rpc_named_arguments, original) end)
    Application.put_env(:explorer, :json_rpc_named_arguments, Evidence.rpc_arguments(node))
    start_supervised!(Capabilities)
    assert {:ok, snapshot} = Capabilities.refresh()
    assert snapshot.unified_account == variant in @valid_answers
    assert_received :canonical_addr_call
    response = conn |> get("/api/v2/paxeer-x/capabilities") |> json_response(200)
    assert response["addr"] == variant in @valid_answers
    for surface <- ~w(exchange bridge launchpad), do: assert(response[surface] == true)
    assert response["checked_at"] != nil
  end

  defp string_answer(value) do
    [selector] = Evidence.abi() |> ABI.parse_specification() |> Enum.filter(&(&1.function == "getUnifiedAccount"))
    Evidence.hex(ABI.TypeEncoder.encode([<<0::160>>, value, <<0::256>>, <<0::256>>], selector.returns))
  end

  defp replace_word(bytes, index, value) do
    offset = index * 32
    <<prefix::binary-size(offset), _word::256, rest::binary>> = bytes
    prefix <> <<value::256>> <> rest
  end
end
