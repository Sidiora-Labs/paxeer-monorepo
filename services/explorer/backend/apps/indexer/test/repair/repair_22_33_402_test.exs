defmodule Indexer.Repair2233402Test do
  use Explorer.DataCase, async: false

  alias Explorer.Chain.Cache.Counters.LastFetchedCounter
  alias Explorer.Chain.Optimism.{DisputeGame, FrameSequence, FrameSequenceBlob}
  alias Explorer.Chain.Optimism.TransactionBatch, as: Batch
  alias Explorer.Chain.Optimism.WithdrawalEvent, as: Event
  alias Indexer.Fetcher.Optimism
  alias Indexer.Fetcher.Optimism.DisputeGame, as: GameFetcher
  alias Indexer.Fetcher.Optimism.EIP1559ConfigUpdate, as: FeeFetcher
  alias Indexer.Fetcher.Optimism.TransactionBatch, as: BatchFetcher
  alias Indexer.Fetcher.Optimism.WithdrawalEvent, as: EventFetcher

  @batch_cursor "optimism_batches_fetcher_last_l1_block_hash"
  @fee_cursor "optimism_eip1559_config_updates_fetcher_last_l2_block_hash"
  @jovian_marker "optimism_eip1559_config_updates_fetcher_jovian_defined"
  @inbox "0xff00000000000000000000000000000000000010"
  @submitter "0x6887246668a3b87f54deb3b94ba47a6f63f32985"
  @portal "0xbeb5fc579115071764c7423a4f12edde41f106ed"
  @factory "0xe5965ab5962edc7477c8520243a95517cd252fa9"
  @proven "0x67a6208cfcc0801d50f6cbe764733f4fddf66ac0b04442061a8a8c0cb6b63f62"
  @moduletag :capture_log

  setup tags do
    start_supervised!(Explorer.Chain.Cache.LatestL1BlockNumber)

    # This gate also runs under the default Paxeer profile. Temporary tables use
    # the final Optimism schema and real import runners, on the sandbox connection.
    for sql <- tables() do
      Ecto.Adapters.SQL.query!(Repo, sql, [])
    end

    insert(:block, number: 7)

    old_chain = Application.get_env(:explorer, :chain_type)
    Application.put_env(:explorer, :chain_type, :optimism)
    on_exit(fn -> Application.put_env(:explorer, :chain_type, old_chain) end)

    if tags[:db_only] do
      %{args: []}
    else
      rpc = Bypass.open()

      {:ok, transcript} =
        Agent.start_link(fn -> %{blocks: %{}, latest: 100, games_unavailable: false, game_reads: []} end)

      Bypass.expect(rpc, "POST", "/", fn conn ->
        {:ok, body, conn} = Plug.Conn.read_body(conn)
        request = Jason.decode!(body)

        response =
          if is_list(request),
            do: Enum.map(request, &rpc_response(&1, transcript)),
            else: rpc_response(request, transcript)

        if is_list(response) and Enum.any?(response, &Map.has_key?(&1, "error")) do
          Plug.Conn.resp(conn, 502, "upstream unavailable")
        else
          Plug.Conn.resp(conn, 200, Jason.encode!(response))
        end
      end)

      args = [
        transport: EthereumJSONRPC.HTTP,
        transport_options: [http: EthereumJSONRPC.HTTP.HTTPoison, http_options: [], urls: [url(rpc)]],
        variant: EthereumJSONRPC.Geth
      ]

      %{rpc: rpc, args: args, transcript: transcript}
    end
  end

  @tag timeout: 30_000
  test "35: exhausted game RPC retries retain the failed chunk across process restart", context do
    Agent.update(context.transcript, &%{&1 | games_unavailable: true})
    initial = game_state(context.args, 0, 50)
    pid = open_fetcher(GameFetcher, initial)
    send(pid, :continue)
    state = :sys.get_state(pid, 20_000)
    assert state.start_index == 0
    assert DisputeGame.get_last_known_index() == -1
    assert length(Agent.get(context.transcript, & &1.game_reads)) == 3
    assert Enum.all?(Agent.get(context.transcript, & &1.game_reads), &(&1 == Enum.to_list(0..49)))
    GenServer.stop(pid)

    Agent.update(context.transcript, &%{&1 | games_unavailable: false, game_reads: []})
    reopened = open_fetcher(GameFetcher, game_state(context.args, DisputeGame.get_last_known_index() + 1, 50))
    send(reopened, :continue)
    assert :sys.get_state(reopened, 20_000).start_index == 51
    assert Repo.aggregate(DisputeGame, :count) == 51
    assert Repo.get!(DisputeGame, 0).extra_data.bytes == <<123::256>>
    assert Repo.get!(DisputeGame, 50).status == 0
    GenServer.stop(reopened)

    # Replay uses the production conflict target and cannot duplicate rows.
    replay = open_fetcher(GameFetcher, initial)
    send(replay, :continue)
    assert :sys.get_state(replay, 20_000).start_index == 51
    assert Repo.aggregate(DisputeGame, :count) == 51
    GenServer.stop(replay)
  end

  @tag db_only: true
  test "36: interrupted Jovian initialization rolls back marker and rewind together", context do
    activation = insert(:block, number: 100, timestamp: DateTime.from_unix!(1_700_000_000))
    old_hash = hash(999)
    {:ok, _} = Optimism.set_last_block_hash(old_hash, @fee_cursor)
    old_cursor = LastFetchedCounter.get(@fee_cursor)

    # Simulate interruption at the last persistence operation, before commit.
    # The worker exits; a new worker has only the durable database state.
    worker =
      Task.async(fn ->
        Repo.transaction(fn ->
          assert :ok = FeeFetcher.initialize_jovian_backfill(1_700_000_000, 1_700_000_010, 2, context.args)
          assert Decimal.equal?(LastFetchedCounter.get(@jovian_marker), 1)

          assert Decimal.to_integer(LastFetchedCounter.get(@fee_cursor)) ==
                   :binary.decode_unsigned(activation.hash.bytes)

          Repo.rollback(:interrupted_before_commit)
        end)
      end)

    assert Task.await(worker) == {:error, :interrupted_before_commit}
    refute Process.alive?(worker.pid)
    assert LastFetchedCounter.get(@jovian_marker, nullable: true) == nil
    assert LastFetchedCounter.get(@fee_cursor) == old_cursor

    reopened =
      Task.async(fn -> FeeFetcher.initialize_jovian_backfill(1_700_000_000, 1_700_000_010, 2, context.args) end)

    assert Task.await(reopened) == :ok
    assert Decimal.to_integer(LastFetchedCounter.get(@fee_cursor)) == :binary.decode_unsigned(activation.hash.bytes)
    assert Decimal.equal?(LastFetchedCounter.get(@jovian_marker), 1)

    {:ok, _} = Optimism.set_last_block_hash(old_hash, @fee_cursor)
    assert :ok = FeeFetcher.initialize_jovian_backfill(1_700_000_000, 1_700_000_010, 2, context.args)
    assert LastFetchedCounter.get(@fee_cursor) == old_cursor
  end

  test "40: a channel split across checkpoints completes identically after reopening", context do
    {first, final} = split_channel()
    put_blocks(context.transcript, [block(100, first), block(101, final)])
    Agent.update(context.transcript, &%{&1 | latest: 100})
    pid = open_fetcher(BatchFetcher, batch_state(context.args, 100, 100))
    send(pid, :continue)
    state = :sys.get_state(pid, 10_000)
    assert state.start_block == 101
    assert map_size(state.incomplete_channels) == 1
    assert BatchFetcher.load_incomplete_channels() == state.incomplete_channels
    assert Decimal.to_integer(LastFetchedCounter.get(@batch_cursor)) == 100
    assert Repo.aggregate(Batch, :count) == 0
    GenServer.stop(pid)

    Agent.update(context.transcript, &%{&1 | latest: 101})
    reopened_state = batch_state(context.args, recovered_block(context.args), 101)
    reopened = open_fetcher(BatchFetcher, reopened_state)
    send(reopened, :continue)
    assert :sys.get_state(reopened, 10_000).incomplete_channels == %{}
    restarted = imported_batch()
    assert restarted.blocks == [8]
    assert restarted.hashes == [hash(1100), hash(1101)]
    assert restarted.ready
    assert BatchFetcher.load_incomplete_channels() == %{}
    GenServer.stop(reopened)

    clear_batches()
    {:ok, _} = Optimism.set_last_block_hash(hash(99), @batch_cursor)
    uninterrupted = open_fetcher(BatchFetcher, batch_state(context.args, 100, 101))
    send(uninterrupted, :continue)
    assert :sys.get_state(uninterrupted, 10_000).start_block == 102
    assert imported_batch() == restarted
    GenServer.stop(uninterrupted)
  end

  test "40: interrupted checkpoint cannot persist frames without its matching cursor", context do
    {first, _} = split_channel()
    put_blocks(context.transcript, [block(100, first)])
    {:ok, channels, [], [], [], last_hash} = fetch_chunk(context.args, 100, %{}, {"", "", "", "", 1})
    {:ok, _} = Optimism.set_last_block_hash(hash(99), @batch_cursor)

    worker =
      Task.async(fn ->
        Repo.transaction(fn ->
          BatchFetcher.checkpoint_incomplete_channels(channels, last_hash)
          assert BatchFetcher.load_incomplete_channels() == channels
          Repo.rollback(:interrupted_checkpoint)
        end)
      end)

    assert Task.await(worker) == {:error, :interrupted_checkpoint}
    assert BatchFetcher.load_incomplete_channels() == %{}
    assert Decimal.to_integer(LastFetchedCounter.get(@batch_cursor)) == 99
  end

  test "42: beacon-only configuration imports EIP-4844 batch input", context do
    frame = complete_channel()
    blob = encode_blob(frame)
    commitment = :binary.copy(<<0x12>>, 48)
    blob_hash = commitment |> Explorer.Chain.Beacon.Blob.hash() |> to_string()
    beacon = Bypass.open()

    Bypass.expect(beacon, "GET", "/eth/v1/beacon/blob_sidecars/0", fn conn ->
      Plug.Conn.resp(
        conn,
        200,
        Jason.encode!(%{"data" => [%{"kzg_commitment" => hex(commitment), "blob" => hex(blob)}]})
      )
    end)

    configure_beacon(beacon)
    tx_block = block(100, <<>>, 3, [blob_hash])
    put_blocks(context.transcript, [tx_block])
    pid = open_fetcher(BatchFetcher, batch_state(context.args, 100, 100))
    send(pid, :continue)
    assert :sys.get_state(pid, 10_000).start_block == 101
    assert imported_batch().blocks == [8]
    assert Repo.one!(FrameSequenceBlob).type == :eip4844
    GenServer.stop(pid)
  end

  test "42: an unavailable primary blob API uses the same beacon fallback", context do
    primary = Bypass.open()
    beacon = Bypass.open()
    blob = encode_blob(complete_channel())
    commitment = :binary.copy(<<0x56>>, 48)
    blob_hash = commitment |> Explorer.Chain.Beacon.Blob.hash() |> to_string()
    Bypass.expect(primary, "GET", "/" <> blob_hash, fn conn -> Plug.Conn.resp(conn, 404, "missing blob") end)

    Bypass.expect(beacon, "GET", "/eth/v1/beacon/blob_sidecars/0", fn conn ->
      Plug.Conn.resp(
        conn,
        200,
        Jason.encode!(%{"data" => [%{"kzg_commitment" => hex(commitment), "blob" => hex(blob)}]})
      )
    end)

    configure_beacon(beacon)
    put_blocks(context.transcript, [block(100, <<>>, 3, [blob_hash])])
    state = %{batch_state(context.args, 100, 100) | eip4844_blobs_api_url: url(primary)}
    pid = open_fetcher(BatchFetcher, state)
    send(pid, :continue)
    assert :sys.get_state(pid, 10_000).start_block == 101
    assert imported_batch().blocks == [8]
    assert Repo.one!(FrameSequenceBlob).type == :eip4844
    GenServer.stop(pid)
  end

  test "41 and 42: missing primary and beacon data retain the cursor until restart recovery", context do
    beacon = Bypass.open()
    configure_beacon(beacon)
    commitment = :binary.copy(<<0x34>>, 48)
    blob_hash = commitment |> Explorer.Chain.Beacon.Blob.hash() |> to_string()
    {:ok, availability} = Agent.start_link(fn -> false end)
    blob = encode_blob(complete_channel())

    Bypass.expect(beacon, "GET", "/eth/v1/beacon/blob_sidecars/0", fn conn ->
      if Agent.get(availability, & &1) do
        Plug.Conn.resp(
          conn,
          200,
          Jason.encode!(%{"data" => [%{"kzg_commitment" => hex(commitment), "blob" => hex(blob)}]})
        )
      else
        Plug.Conn.resp(conn, 404, "missing sidecar")
      end
    end)

    put_blocks(context.transcript, [block(100, <<>>, 3, [blob_hash])])
    assert_da_restart(context, availability, %{})
    assert Repo.one!(FrameSequenceBlob).type == :eip4844
  end

  test "41: transient Celestia, EigenDA and Alt-DA failures recover through real imports", context do
    for kind <- [:celestia, :eigenda, :alt_da] do
      clear_batches()
      da = Bypass.open()
      {:ok, available} = Agent.start_link(fn -> false end)
      data = complete_channel()

      Bypass.expect(da, fn conn ->
        if Agent.get(available, & &1) do
          response =
            case kind do
              :celestia -> Jason.encode!(%{"namespace" => String.duplicate("00", 29), "data" => Base.encode64(data)})
              :eigenda -> Jason.encode!(%{"data" => hex(data)})
              :alt_da -> data
            end

          Plug.Conn.resp(conn, 200, response)
        else
          Plug.Conn.resp(conn, 404, "temporarily unavailable")
        end
      end)

      {input, setting} =
        case kind do
          :celestia -> {<<1, 1, 12, 123::little-64, 42::256>>, :celestia_blobs_api_url}
          :eigenda -> {<<1, 1, 0, 1, 2, 3>>, :eigenda_blobs_api_url}
          :alt_da -> {<<1, 0, 42::256>>, :alt_da_server_url}
        end

      put_blocks(context.transcript, [block(100, input)])
      assert_da_restart(context, available, %{setting => url(da)})
      assert Repo.one!(FrameSequenceBlob).type == kind
    end
  end

  test "56: real Super Root prove calldata imports ordinary and leading-zero game addresses", context do
    addresses = [@factory, "0x0000000000000000000000000000000000001234", "0x0000000000000000000000000000000000000000"]

    transactions =
      Enum.with_index(addresses, 1)
      |> Enum.map(fn {address, index} ->
        tx = hd(recorded_block()["transactions"])
        %{tx | "hash" => hash(2000 + index), "input" => super_root_calldata(address), "blockNumber" => "0x64"}
      end)

    raw = block(100, <<>>) |> Map.put("transactions", transactions)

    events =
      Enum.map(transactions, fn tx ->
        %{
          "blockNumber" => "0x64",
          "transactionHash" => tx["hash"],
          "topics" => [@proven, hash(3000), hash(1), hash(2)],
          "data" => "0x"
        }
      end)

    Agent.update(context.transcript, &Map.merge(&1, %{blocks: %{100 => raw}, events: events}))

    pid =
      open_fetcher(EventFetcher, %{
        contract_address: @portal,
        block_check_interval: 60_000,
        start_block: 100,
        end_block: 100,
        json_rpc_named_arguments: context.args,
        eth_get_logs_range_size: 1
      })

    send(pid, :continue)
    assert :sys.get_state(pid, 10_000).start_block == 101
    imported = Repo.all(from(e in Event, order_by: e.l1_transaction_hash))
    assert Enum.map(imported, &to_string(&1.game_address_hash)) == addresses
    assert Enum.all?(imported, &is_nil(&1.game_index))
    assert Enum.all?(imported, &(&1.l1_event_type == :WithdrawalProven))
    GenServer.stop(pid)
  end

  defp assert_da_restart(context, availability, settings) do
    {:ok, _} = Optimism.set_last_block_hash(hash(99), @batch_cursor)
    pid = open_fetcher(BatchFetcher, Map.merge(batch_state(context.args, 100, 100), settings))
    send(pid, :continue)
    failed = :sys.get_state(pid, 10_000)
    assert failed.start_block == 100
    assert Decimal.to_integer(LastFetchedCounter.get(@batch_cursor)) == 99
    assert Repo.aggregate(Batch, :count) == 0
    GenServer.stop(pid)
    Agent.update(availability, fn _ -> true end)

    reopened =
      open_fetcher(BatchFetcher, Map.merge(batch_state(context.args, recovered_block(context.args), 100), settings))

    send(reopened, :continue)
    assert :sys.get_state(reopened, 10_000).start_block == 101
    assert Decimal.to_integer(LastFetchedCounter.get(@batch_cursor)) == 100
    assert imported_batch().blocks == [8]
    GenServer.stop(reopened)
  end

  defp open_fetcher(module, state) do
    # Enter the production GenServer loop with chain/config fixtures. No callback
    # or transport is replaced. Reopening loads the same persisted state as init.
    pid = :proc_lib.spawn(fn -> :gen_server.enter_loop(module, [], state) end)
    on_exit(fn -> if Process.alive?(pid), do: GenServer.stop(pid) end)
    pid
  end

  defp batch_state(args, first, last) do
    %{
      batch_inbox: @inbox,
      batch_submitter: @submitter,
      eip4844_blobs_api_url: "",
      celestia_blobs_api_url: "",
      eigenda_blobs_api_url: "",
      alt_da_server_url: "",
      block_check_interval: 60_000,
      start_block: first,
      end_block: last,
      chunk_size: 1,
      incomplete_channels: BatchFetcher.load_incomplete_channels(),
      genesis_block_l2: 0,
      block_duration: 2,
      json_rpc_named_arguments: args,
      json_rpc_named_arguments_l2: args,
      chain_id_l1: nil
    }
  end

  defp game_state(args, first, last),
    do: %{
      dispute_game_factory: @factory,
      optimism_portal: @portal,
      start_index: first,
      end_index: last,
      json_rpc_named_arguments: args
    }

  defp recovered_block(args), do: Optimism.get_last_block_number_from_last_fetched_counter(args, @batch_cursor)

  defp imported_batch do
    sequence = Repo.one!(FrameSequence)

    %{
      blocks: Repo.all(from(b in Batch, order_by: b.l2_block_number, select: b.l2_block_number)),
      hashes: Enum.map(sequence.l1_transaction_hashes, &to_string/1),
      ready: sequence.view_ready,
      timestamp: sequence.l1_timestamp
    }
  end

  defp clear_batches do
    Repo.delete_all(FrameSequenceBlob)
    Repo.delete_all(Batch)
    Repo.delete_all(FrameSequence)
  end

  defp fetch_chunk(args, number, channels, sources),
    do:
      BatchFetcher.get_transaction_batches(
        number..number,
        @inbox,
        @submitter,
        {0, 2},
        channels,
        {args, args},
        sources,
        1
      )

  defp put_blocks(transcript, blocks) do
    Agent.update(transcript, fn state ->
      by_number = Map.new(blocks, &{EthereumJSONRPC.quantity_to_integer(&1["number"]), &1})
      %{state | blocks: by_number, latest: Enum.max(Map.keys(by_number))}
    end)
  end

  defp rpc_response(%{"id" => id, "method" => method, "params" => params}, transcript) do
    result =
      case {method, params} do
        {"eth_getBlockByNumber", [tag, _]} when tag in ["latest", "safe"] ->
          block(Agent.get(transcript, & &1.latest), <<>>) |> Map.put("transactions", [])

        {"eth_getBlockByNumber", [number, _]} ->
          n = EthereumJSONRPC.quantity_to_integer(number)
          Agent.get(transcript, &Map.get(&1.blocks, n, block(n, <<>>) |> Map.put("transactions", [])))

        {"eth_getBlockByHash", [block_hash, _]} ->
          n = EthereumJSONRPC.quantity_to_integer(block_hash)
          Agent.get(transcript, &Map.get(&1.blocks, n, block(n, <<>>) |> Map.put("transactions", [])))

        {"eth_getLogs", _} ->
          Agent.get(transcript, & &1.events)

        {"eth_call", [%{"data" => "0x3c9f397c"}, _]} ->
          word(0)

        {"eth_call", [%{"data" => "0x4d1975b4"}, _]} ->
          word(51)

        {"eth_call", [%{"data" => "0x609d3334"}, _]} ->
          hex(ABI.TypeEncoder.encode([<<123::256>>], [:bytes]))

        {"eth_call", [%{"data" => signature}, _]} when signature in ["0x19effeb4", "0x200d2ed2"] ->
          word(0)

        {"eth_call", [%{"data" => data}, _]} ->
          index = data |> String.slice(-64, 64) |> String.to_integer(16)

          Agent.update(transcript, fn state ->
            Map.update!(state, :game_reads, fn reads ->
              case reads do
                [current | rest] when index == hd(current) + length(current) -> [current ++ [index] | rest]
                _ -> [[index] | reads]
              end
            end)
          end)

          if Agent.get(transcript, & &1.games_unavailable) do
            :unavailable
          else
            hex(ABI.TypeEncoder.encode([0, 1_700_000_000, <<index + 1::160>>], [{:uint, 32}, {:uint, 64}, :address]))
          end
      end

    if result == :unavailable do
      %{"jsonrpc" => "2.0", "id" => id, "error" => %{"code" => -32000, "message" => "recorded RPC outage"}}
    else
      %{"jsonrpc" => "2.0", "id" => id, "result" => result}
    end
  end

  defp block(number, input, type \\ 2, blob_hashes \\ []) do
    recorded = recorded_block()
    tx = hd(recorded["transactions"])

    tx =
      Map.merge(tx, %{
        "from" => @submitter,
        "to" => @inbox,
        "hash" => hash(number + 1000),
        "input" => hex(input),
        "blockNumber" => EthereumJSONRPC.integer_to_quantity(number),
        "blockHash" => hash(number),
        "transactionIndex" => "0x0",
        "type" => EthereumJSONRPC.integer_to_quantity(type),
        "blobVersionedHashes" => blob_hashes
      })

    Map.merge(recorded, %{
      "number" => EthereumJSONRPC.integer_to_quantity(number),
      "hash" => hash(number),
      "timestamp" => "0x6553f100",
      "transactions" => [tx]
    })
  end

  defp recorded_block do
    path =
      Path.expand("../../../explorer/test/support/fixture/vcr_cassettes/block_importer_import_1_pending.json", __DIR__)

    [record] = path |> File.read!() |> Jason.decode!()
    record["response"]["body"] |> Jason.decode!() |> Map.fetch!("result")
  end

  defp channel_bytes do
    # Singular batch wire format; its parent is a real indexed block, consumed
    # through the production hash-to-number lookup and the real import runner.
    parent = Repo.get_by!(Explorer.Chain.Block, number: 7)
    payload = ExRLP.encode([parent.hash.bytes, <<1>>, <<0::256>>, <<1_700_000_000::32>>, []])
    :zlib.compress(ExRLP.encode(<<0>> <> payload))
  end

  defp split_channel do
    data = channel_bytes()
    size = div(byte_size(data), 2)
    <<first::binary-size(size), final::binary>> = data
    {frame(first, 0, false), frame(final, 1, true)}
  end

  defp complete_channel, do: frame(channel_bytes(), 0, true)

  defp frame(data, index, last),
    do: <<0, 22::128, index::16, byte_size(data)::32, data::binary, if(last, do: 1, else: 0)>>

  defp encode_blob(input) do
    # Encode the protocol's field-element packing (zero high bits for the first
    # 27 bytes, then three groups of 31 bytes separated by a reconstructed byte).
    padded = input <> :binary.copy(<<0>>, 123 - byte_size(input))
    <<a::binary-size(27), x, b::binary-size(31), y, c::binary-size(31), z, d::binary-size(31)>> = padded
    h0 = Bitwise.band(x, 63)
    h1 = Bitwise.bor(Bitwise.bsl(Bitwise.bsr(x, 6), 4), Bitwise.band(y, 15))
    h2 = Bitwise.band(z, 63)
    h3 = Bitwise.bor(Bitwise.bsr(y, 4), Bitwise.bsl(Bitwise.bsr(z, 6), 4))
    <<h0, 0, byte_size(input)::24, a::binary, h1, b::binary, h2, c::binary, h3, d::binary, 0::size(130_944 * 8)>>
  end

  defp super_root_calldata(address) do
    "0x" <> raw = address

    types = [
      {:tuple, [{:uint, 256}, :address, :address, {:uint, 256}, {:uint, 256}, :bytes]},
      :address,
      {:uint, 256},
      {:tuple, [{:uint, 8}, {:uint, 64}, {:array, {:tuple, [{:uint, 256}, {:bytes, 32}]}}]},
      {:tuple, [{:bytes, 32}, {:bytes, 32}, {:bytes, 32}, {:bytes, 32}]},
      {:array, :bytes}
    ]

    values = [
      {1, <<1::160>>, <<2::160>>, 0, 100_000, <<>>},
      Base.decode16!(raw, case: :mixed),
      0,
      {1, 1_700_000_000, [{10, <<0::256>>}]},
      {<<0::256>>, <<0::256>>, <<0::256>>, <<0::256>>},
      []
    ]

    "0x8c90dd65" <> Base.encode16(ABI.TypeEncoder.encode(values, types), case: :lower)
  end

  defp configure_beacon(beacon) do
    old = Application.get_env(:indexer, Indexer.Fetcher.Beacon)
    old_blob = Application.get_env(:indexer, Indexer.Fetcher.Beacon.Blob)
    Application.put_env(:indexer, Indexer.Fetcher.Beacon, beacon_rpc: url(beacon))

    Application.put_env(:indexer, Indexer.Fetcher.Beacon.Blob,
      reference_slot: 0,
      reference_timestamp: 1_700_000_000,
      slot_duration: 12
    )

    on_exit(fn ->
      Application.put_env(:indexer, Indexer.Fetcher.Beacon, old)
      Application.put_env(:indexer, Indexer.Fetcher.Beacon.Blob, old_blob)
    end)
  end

  defp url(server), do: "http://127.0.0.1:#{server.port}"
  defp hex(bytes), do: "0x" <> Base.encode16(bytes, case: :lower)
  defp hash(number), do: "0x" <> word_hex(number)
  defp word(number), do: hash(number)
  defp word_hex(number), do: Base.encode16(<<number::256>>, case: :lower)

  defp tables do
    [
      "CREATE TEMP TABLE op_dispute_games (index bigint PRIMARY KEY, game_type integer NOT NULL, address_hash bytea NOT NULL, extra_data bytea, created_at timestamp NOT NULL, resolved_at timestamp, status integer, inserted_at timestamp NOT NULL, updated_at timestamp NOT NULL)",
      "CREATE TEMP TABLE op_frame_sequences (id bigint PRIMARY KEY, l1_transaction_hashes bytea[] NOT NULL, l1_timestamp timestamp NOT NULL, view_ready boolean NOT NULL DEFAULT false, inserted_at timestamp NOT NULL, updated_at timestamp NOT NULL)",
      "CREATE TEMP TABLE op_transaction_batches (l2_block_number bigint PRIMARY KEY, frame_sequence_id bigint NOT NULL REFERENCES op_frame_sequences(id), frame_sequence_id_prev bigint NOT NULL DEFAULT 0, inserted_at timestamp NOT NULL, updated_at timestamp NOT NULL)",
      "CREATE TEMP TABLE op_frame_sequence_blobs (id bigint UNIQUE NOT NULL, key bytea NOT NULL, type text NOT NULL CHECK (type IN ('celestia', 'eip4844', 'eigenda', 'alt_da')), metadata jsonb NOT NULL DEFAULT '{}', l1_transaction_hash bytea NOT NULL, l1_timestamp timestamp NOT NULL, frame_sequence_id bigint NOT NULL REFERENCES op_frame_sequences(id) ON DELETE CASCADE, inserted_at timestamp NOT NULL, updated_at timestamp NOT NULL, PRIMARY KEY (key, type))",
      "CREATE TEMP TABLE op_withdrawal_events (withdrawal_hash bytea NOT NULL, l1_event_type text NOT NULL CHECK (l1_event_type IN ('WithdrawalProven', 'WithdrawalFinalized')), l1_timestamp timestamp NOT NULL, l1_transaction_hash bytea NOT NULL, l1_block_number bigint NOT NULL, game_index bigint, game_address_hash bytea, inserted_at timestamp NOT NULL, updated_at timestamp NOT NULL, PRIMARY KEY (withdrawal_hash, l1_event_type, l1_transaction_hash))"
    ]
  end
end
