defmodule BlockScoutWeb.Repair.Issue1674Test do
  use BlockScoutWeb.ConnCase, async: false

  alias BlockScoutWeb.Routers.ApiRouter
  alias Explorer.Chain.Cache.ChainId
  alias Explorer.Chain.Import.Runner.Optimism.InteropMessages
  alias Explorer.Chain.Optimism.InteropMessage
  alias Explorer.Repo
  alias Indexer.Fetcher.Optimism.Interop.MessageQueue

  @recording ~S({
    "nonce": 1,
    "init_chain_id": 23,
    "relay_chain_id": 45,
    "sender_address_hash": "0x06012c8cf97bead5deae237070f9587f8e7a266d",
    "target_address_hash": "0x7b9ea9ac69b8fde875554321472c732eeff06ca0",
    "init_transaction_hash": "0xf463ce43ac52251f60f437a0346b7168874e6eb13a9689d7efde4ce907826897",
    "timestamp": 1700000000,
    "payload": "0x30787849217424792273712777707576696a6c5d"
  })
  @path "/api/v2/optimism/interop/messages"
  @cursor_fields ~w(timestamp init_transaction_hash init_chain_id nonce)

  setup do
    original_modules =
      Enum.map([RPCTranslatorForwarder, ApiRouter], fn module ->
        {^module, binary, filename} = :code.get_object_code(module)
        {module, binary, filename}
      end)

    previous_chain_type = Application.fetch_env(:explorer, :chain_type)
    previous_queue = Application.fetch_env(:indexer, MessageQueue)
    previous_chain_id = ConCache.get(ChainId.cache_name(), :id)

    on_exit(fn ->
      restore_env(:explorer, :chain_type, previous_chain_type)
      restore_env(:indexer, MessageQueue, previous_queue)
      ChainId.set_id(previous_chain_id)

      for {module, binary, filename} <- original_modules do
        :code.purge(module)
        {:module, ^module} = :code.load_binary(module, filename, binary)
      end
    end)

    Application.put_env(:explorer, :chain_type, :optimism)
    Application.put_env(:indexer, MessageQueue, chainscout_api_url: nil, chainscout_fallback_map: %{})
    ChainId.set_id(45)

    previous_conflict_option = Code.get_compiler_option(:ignore_module_conflict)
    Code.compiler_options(ignore_module_conflict: true)

    try do
      Code.compile_file(Path.expand("../../lib/block_scout_web/routers/api_router.ex", __DIR__))
    after
      Code.compiler_options(ignore_module_conflict: previous_conflict_option)
    end

    cache = InteropMessage.interop_chain_id_to_instance_info_cache()

    if Process.whereis(cache) == nil do
      start_supervised!(Supervisor.child_spec({ConCache, name: cache, ttl_check_interval: false}, id: cache))
    end

    Repo.query!("""
    CREATE TEMP TABLE op_interop_messages (
      sender_address_hash bytea, target_address_hash bytea,
      nonce bigint NOT NULL, init_chain_id integer NOT NULL,
      init_transaction_hash bytea, block_number bigint, timestamp timestamp(6),
      relay_chain_id integer NOT NULL, relay_transaction_hash bytea,
      payload bytea, failed boolean, transfer_token_address_hash bytea,
      transfer_from_address_hash bytea, transfer_to_address_hash bytea,
      transfer_amount numeric, sent_to_multichain boolean,
      inserted_at timestamp(6) NOT NULL, updated_at timestamp(6) NOT NULL,
      PRIMARY KEY (nonce, init_chain_id)
    )
    """)

    :ok
  end

  test "interop_http_cursor_round_trip" do
    recorded = Jason.decode!(@recording)
    variants = for chain <- [3, 23], nonce <- 1..60, do: %{"init_chain_id" => chain, "nonce" => nonce}

    excluded = [
      %{"nonce" => 61, "sender_address_hash" => recorded["target_address_hash"]},
      %{"nonce" => 62, "target_address_hash" => recorded["sender_address_hash"]},
      %{"nonce" => 63, "relay_chain_id" => 46},
      %{"nonce" => 64, "relay_transaction_hash" => recorded["init_transaction_hash"], "failed" => true},
      %{"nonce" => 65, "init_transaction_hash" => "0x" <> String.duplicate("a", 64)},
      %{"nonce" => 66, "timestamp" => 1_700_000_100}
    ]

    changes =
      Enum.map(variants ++ excluded, fn variant ->
        attrs = recorded |> Map.merge(variant) |> Map.update!("timestamp", &DateTime.from_unix!/1)
        changeset = InteropMessage.changeset(%InteropMessage{}, attrs)
        assert changeset.valid?, inspect(changeset.errors)
        changeset.changes
      end)

    now = DateTime.utc_now()
    multi = InteropMessages.run(Ecto.Multi.new(), changes, %{timestamps: %{inserted_at: now, updated_at: now}})
    assert {:ok, %{insert_interop_messages: inserted}} = Repo.transaction(multi)
    assert length(inserted) == 126
    assert Repo.aggregate(InteropMessage, :count) == 126

    filters = %{
      "interop_message_init_transaction_hash" => recorded["init_transaction_hash"],
      "interop_message_sender_address_hashes_to_include" => recorded["sender_address_hash"],
      "interop_message_target_address_hashes_to_include" => recorded["target_address_hash"],
      "interop_message_statuses" => "Sent",
      "interop_message_direction" => "in",
      "interop_message_age_from" => "2023-11-14T22:13:20Z",
      "interop_message_age_to" => "2023-11-14T22:13:20Z"
    }

    assert %{"items" => unfiltered, "next_page_params" => unfiltered_cursor} = request(%{}, 200)
    assert length(unfiltered) == 50
    assert is_map(unfiltered_cursor)

    {items, terminal} =
      Enum.reduce([50, 50, 20], {[], filters}, fn page_size, {visited, params} ->
        assert is_map(params)
        assert Map.take(params, Map.keys(filters)) == filters
        assert %{"items" => page, "next_page_params" => cursor} = request(params, 200)
        assert length(page) == page_size

        for item <- page do
          assert item["timestamp"] == "2023-11-14T22:13:20.000000Z"
          assert item["init_transaction_hash"] == recorded["init_transaction_hash"]
          assert String.downcase(item["sender_address_hash"]) == recorded["sender_address_hash"]
          assert String.downcase(item["target_address_hash"]) == recorded["target_address_hash"]
          assert item["status"] == "Sent"
          assert item["payload"] == recorded["payload"]
        end

        if page_size == 20 do
          assert cursor == nil
        else
          last = List.last(page)
          {chain, nonce} = identity(last)

          assert Map.take(cursor, @cursor_fields) == %{
                   "timestamp" => 1_700_000_000,
                   "init_transaction_hash" => recorded["init_transaction_hash"],
                   "init_chain_id" => chain,
                   "nonce" => nonce
                 }

          assert Map.take(cursor, Map.keys(filters)) == filters
          assert cursor["items_count"] == 50
        end

        transported = if cursor, do: cursor |> URI.encode_query() |> URI.decode_query()
        {visited ++ page, transported}
      end)

    expected = for chain <- [23, 3], nonce <- 60..1, do: {chain, nonce}
    assert Enum.map(items, &identity/1) == expected
    assert length(Enum.uniq_by(items, & &1["unique_id"])) == 120
    assert terminal == nil

    valid_cursor = unfiltered_cursor |> URI.encode_query() |> URI.decode_query()

    incomplete = Enum.map(@cursor_fields, &Map.delete(valid_cursor, &1))
    partial = Enum.map(@cursor_fields, &Map.take(valid_cursor, [&1]))

    invalid =
      for {field, values} <- [
            {"init_transaction_hash", ["0x1234", "0x" <> String.duplicate("g", 64)]},
            {"timestamp", ["invalid", "1700000000.5", "999999999999999999999"]},
            {"init_chain_id", ["abc", "23.5", "0", "-1"]},
            {"nonce", ["abc", "1.5", "-1"]}
          ],
          value <- values,
          do: Map.put(valid_cursor, field, value)

    for cursor <- incomplete ++ partial ++ invalid do
      assert request(Map.merge(filters, cursor), 400) == %{"message" => "Invalid interop message cursor"}
    end
  end

  defp request(params, status) do
    query = params |> URI.encode_query() |> URI.decode_query() |> URI.encode_query()
    build_conn() |> get(@path <> "?" <> query) |> Phoenix.ConnTest.json_response(status)
  end

  defp identity(item) do
    {chain, ""} = item["unique_id"] |> String.slice(0, 8) |> Integer.parse(16)
    {nonce, ""} = item["unique_id"] |> String.slice(8, 8) |> Integer.parse(16)
    assert item["nonce"] == nonce
    {chain, nonce}
  end

  defp restore_env(app, key, {:ok, value}), do: Application.put_env(app, key, value)
  defp restore_env(app, key, :error), do: Application.delete_env(app, key)
end
