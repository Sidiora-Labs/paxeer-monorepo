defmodule Indexer.Repair2233546Test do
  use Explorer.DataCase, async: false

  alias EthereumJSONRPC.Arbitrum.Constants.Events
  alias Explorer.Chain.Arbitrum.{L1Execution, LifecycleTransaction, Message}
  alias Indexer.Fetcher.Arbitrum.Utils.Db.Messages, as: DbMessages
  alias Indexer.Fetcher.Arbitrum.Workers.{Backfill, BackfillRecovery, NewL1Executions}

  # Retained JSON-RPC recordings supply the block/receipt/log envelopes. The
  # transcript mutations below represent missing responses and alternate forks.
  setup do
    Repo.query!("CREATE TEMP TABLE blocks (LIKE public.blocks INCLUDING ALL)")
    Repo.query!("ALTER TABLE blocks ADD COLUMN IF NOT EXISTS send_count integer")
    Repo.query!("ALTER TABLE blocks ADD COLUMN IF NOT EXISTS send_root bytea")
    Repo.query!("ALTER TABLE blocks ADD COLUMN IF NOT EXISTS l1_block_number integer")
    Repo.query!("CREATE TEMP TABLE transactions (LIKE public.transactions INCLUDING ALL)")
    Repo.query!("ALTER TABLE transactions ADD COLUMN IF NOT EXISTS gas_used_for_l1 numeric(100)")

    Repo.query!("""
    CREATE TEMP TABLE arbitrum_lifecycle_l1_transactions (
      id integer PRIMARY KEY, hash bytea NOT NULL UNIQUE, block_number integer NOT NULL,
      timestamp timestamp NOT NULL, status varchar NOT NULL,
      inserted_at timestamp NOT NULL, updated_at timestamp NOT NULL
    )
    """)

    Repo.query!("""
    CREATE TEMP TABLE arbitrum_l1_executions (
      message_id integer PRIMARY KEY,
      execution_id integer NOT NULL REFERENCES arbitrum_lifecycle_l1_transactions(id),
      inserted_at timestamp NOT NULL, updated_at timestamp NOT NULL
    )
    """)

    Repo.query!("""
    CREATE TEMP TABLE arbitrum_crosslevel_messages (
      direction varchar NOT NULL, message_id integer NOT NULL,
      originator_address bytea, originating_transaction_hash bytea,
      origination_timestamp timestamp, originating_transaction_block_number bigint,
      completion_transaction_hash bytea, status varchar NOT NULL,
      inserted_at timestamp NOT NULL, updated_at timestamp NOT NULL,
      PRIMARY KEY (direction, message_id)
    )
    """)

    bypass = Bypass.open()

    {:ok, transcript} =
      Agent.start_link(fn -> %{blocks: %{}, receipts: %{}, logs: [], mode: :complete, requests: []} end)

    Bypass.expect(bypass, "POST", "/", fn conn ->
      {:ok, body, conn} = Plug.Conn.read_body(conn)
      request = Jason.decode!(body)

      response =
        Agent.get_and_update(transcript, fn state ->
          requests = if is_list(request), do: request, else: [request]
          responses = Enum.map(requests, &rpc_response(&1, state))
          response = if is_list(request), do: responses, else: hd(responses)
          {response, %{state | requests: state.requests ++ requests}}
        end)

      conn
      |> Plug.Conn.put_resp_content_type("application/json")
      |> Plug.Conn.resp(200, Jason.encode!(response))
    end)

    url = "http://localhost:#{bypass.port}/"

    arguments = [
      transport: EthereumJSONRPC.HTTP,
      transport_options: [http: EthereumJSONRPC.HTTP.HTTPoison, http_options: [], urls: [url], fallback_urls: []],
      variant: EthereumJSONRPC.Geth
    ]

    on_exit(fn -> Bypass.down(bypass) end)
    %{transcript: transcript, arguments: arguments}
  end

  test "backfill-transaction-only-holes: restart repairs a transaction with complete block metadata", ctx do
    {block, transaction, raw, receipt} = indexed_block(50, true)
    legacy = insert_collated_transaction(block, 1, 120)

    legacy_receipt =
      receipt
      |> Map.put("transactionHash", to_string(legacy.hash))
      |> Map.put("transactionIndex", "0x1")
      |> Map.delete("gasUsedForL1")

    put_transcript(ctx, [raw], %{50 => [receipt, legacy_receipt]})
    state = backfill_state(ctx, 50, 1)

    assert BackfillRecovery.blocks_with_missing_fields(50, 50) == [50]
    assert {:ok, 50} = reopen(fn -> Backfill.discover_blocks(50, state) end)
    assert gas(transaction) == Decimal.new(4132)
    assert gas(legacy) == nil
    assert block_fields(block) == [2, BackfillRecovery.bytes(raw["sendRoot"]), 42]
    assert BackfillRecovery.blocks_with_missing_fields(50, 50) == []

    # Simulate the durable transaction-only hole that a new worker must rediscover.
    Repo.query!("UPDATE transactions SET gas_used_for_l1 = NULL WHERE hash = $1", [transaction.hash.bytes])
    assert {:ok, 50} = reopen(fn -> Backfill.discover_blocks(50, backfill_state(ctx, 50, 1)) end)
    assert gas(transaction) == Decimal.new(4132)
    assert gas(legacy) == nil
    assert BackfillRecovery.blocks_with_missing_fields(50, 50) == []
    assert request_count(ctx, "eth_getBlockReceipts") == 2
  end

  test "backfill-partial-block-results: mixed success and error leaves the range unacknowledged across restart", ctx do
    {first, tx1, raw1, receipt1} = indexed_block(50, false)
    {second, tx2, raw2, receipt2} = indexed_block(51, false)
    put_transcript(ctx, [raw1, raw2], %{50 => [receipt1], 51 => [receipt2]}, {:block_error, 51})

    assert {:error, :discover_blocks_error} = reopen(fn -> Backfill.discover_blocks(51, backfill_state(ctx, 50, 2)) end)
    assert block_fields(first) == [nil, nil, nil]
    assert block_fields(second) == [nil, nil, nil]
    assert gas(tx1) == nil
    assert gas(tx2) == nil
    assert request_count(ctx, "eth_getBlockReceipts") == 0

    Agent.update(ctx.transcript, &%{&1 | mode: :complete})
    assert {:ok, 50} = reopen(fn -> Backfill.discover_blocks(51, backfill_state(ctx, 50, 2)) end)
    assert hd(block_fields(first)) == 2
    assert hd(block_fields(second)) == 2
    assert gas(tx1) == Decimal.new(4132)
    assert gas(tx2) == Decimal.new(4132)
    assert BackfillRecovery.blocks_with_missing_fields(50, 51) == []
  end

  test "backfill-partial-block-results: a successful envelope with duplicate block coverage cannot commit", ctx do
    {first, _, raw1, receipt1} = indexed_block(50, false)
    {second, _, raw2, receipt2} = indexed_block(51, false)
    put_transcript(ctx, [raw1, raw2], %{50 => [receipt1], 51 => [receipt2]}, {:duplicate_block, 51, raw1})

    assert {:error, :discover_blocks_error} = reopen(fn -> Backfill.discover_blocks(51, backfill_state(ctx, 50, 2)) end)
    assert block_fields(first) == [nil, nil, nil]
    assert block_fields(second) == [nil, nil, nil]
    assert request_count(ctx, "eth_getBlockReceipts") == 0

    Agent.update(ctx.transcript, &%{&1 | mode: :complete})
    assert {:ok, 50} = reopen(fn -> Backfill.discover_blocks(51, backfill_state(ctx, 50, 2)) end)
    assert BackfillRecovery.blocks_with_missing_fields(50, 51) == []
  end

  test "backfill-partial-block-results: missing receipts and wrong canonical identities retain recoverable work", ctx do
    {block, transaction, raw, receipt} = indexed_block(50, false)
    put_transcript(ctx, [raw], %{50 => []})
    state = backfill_state(ctx, 50, 1)

    assert {:error, :discover_blocks_error} = reopen(fn -> Backfill.discover_blocks(50, state) end)
    assert block_fields(block) == [nil, nil, nil]
    assert gas(transaction) == nil

    put_transcript(ctx, [raw], %{50 => [Map.put(receipt, "blockHash", raw["parentHash"])]})
    assert {:error, :discover_blocks_error} = reopen(fn -> Backfill.discover_blocks(50, state) end)
    assert block_fields(block) == [nil, nil, nil]
    assert gas(transaction) == nil

    put_transcript(ctx, [raw], %{50 => [receipt]})
    assert {:ok, 50} = reopen(fn -> Backfill.discover_blocks(50, state) end)
    assert gas(transaction) == Decimal.new(4132)
  end

  test "execution-reorg-recovery: an orphan execution is revoked and later replaced after worker restarts", ctx do
    raw = recorded_block(50)
    original = execution_log(raw, 7, recorded_receipt()["transactionHash"])
    put_transcript(ctx, [raw], %{}, :complete, [original])
    insert_message(7)

    assert {:ok, observed} =
             reopen(fn -> NewL1Executions.discover_new_l1_messages_executions(execution_state(ctx, 50)) end)

    assert observed.task_data.new_executions.start_block == 51
    assert observed.task_data.new_executions.observed_blocks == %{50 => raw["hash"]}
    assert_relayed(7, original["transactionHash"])

    fork = Map.put(raw, "hash", raw["parentHash"])
    put_transcript(ctx, [fork], %{}, :complete, [])
    restarted = execution_state(ctx, DbMessages.l1_block_to_discover_latest_execution(50))
    assert {:ok, recovered} = reopen(fn -> NewL1Executions.discover_new_l1_messages_executions(restarted) end)
    assert recovered.task_data.new_executions.observed_blocks == %{50 => fork["hash"]}
    refute Repo.get(L1Execution, 7)
    assert %{status: :confirmed, completion_transaction_hash: nil} = message(7)

    replacement = execution_log(fork, 7, recorded_block(50)["hash"])
    put_transcript(ctx, [fork], %{}, :complete, [replacement])
    restarted = execution_state(ctx, DbMessages.l1_block_to_discover_latest_execution(50))
    assert {:ok, _} = reopen(fn -> NewL1Executions.discover_new_l1_messages_executions(restarted) end)
    assert_relayed(7, replacement["transactionHash"])
    assert Repo.aggregate(LifecycleTransaction, :count) == 2

    assert {:ok, _} = reopen(fn -> NewL1Executions.discover_new_l1_messages_executions(execution_state(ctx, 51)) end)
    assert_relayed(7, replacement["transactionHash"])
    assert Repo.aggregate(L1Execution, :count) == 1
  end

  test "execution-reorg-recovery: replacement directly rebinds an already relayed message atomically", ctx do
    raw = recorded_block(50)
    original = execution_log(raw, 7, recorded_receipt()["transactionHash"])
    put_transcript(ctx, [raw], %{}, :complete, [original])
    insert_message(7)
    assert {:ok, _} = reopen(fn -> NewL1Executions.discover_new_l1_messages_executions(execution_state(ctx, 50)) end)
    assert_relayed(7, original["transactionHash"])

    fork = Map.put(raw, "hash", raw["parentHash"])
    replacement = execution_log(fork, 7, raw["hash"])
    put_transcript(ctx, [fork], %{}, :complete, [replacement])
    assert {:ok, _} = reopen(fn -> NewL1Executions.discover_new_l1_messages_executions(execution_state(ctx, 51)) end)
    assert_relayed(7, replacement["transactionHash"])
  end

  test "execution-reorg-recovery: inconsistent logs preserve the previous binding and cursor", ctx do
    raw = recorded_block(50)
    original = execution_log(raw, 7, recorded_receipt()["transactionHash"])
    put_transcript(ctx, [raw], %{}, :complete, [original])
    insert_message(7)
    assert {:ok, _} = reopen(fn -> NewL1Executions.discover_new_l1_messages_executions(execution_state(ctx, 50)) end)

    fork = Map.put(raw, "hash", raw["parentHash"])
    put_transcript(ctx, [fork], %{}, :complete, [original])
    state = execution_state(ctx, 51)

    assert {:error, :inconsistent_execution_blocks} =
             reopen(fn -> NewL1Executions.discover_new_l1_messages_executions(state) end)

    assert state.task_data.new_executions.start_block == 51
    assert_relayed(7, original["transactionHash"])
  end

  test "execution-reorg-recovery: import failure rolls back execution revocation and message status together", ctx do
    raw = recorded_block(50)
    original = execution_log(raw, 7, recorded_receipt()["transactionHash"])
    put_transcript(ctx, [raw], %{}, :complete, [original])
    insert_message(7)
    assert {:ok, _} = reopen(fn -> NewL1Executions.discover_new_l1_messages_executions(execution_state(ctx, 50)) end)
    assert_relayed(7, original["transactionHash"])
    old_execution = Repo.get!(L1Execution, 7)

    Repo.query!(
      "ALTER TABLE arbitrum_l1_executions ADD CONSTRAINT reject_replacement CHECK (execution_id = #{old_execution.execution_id})"
    )

    fork = Map.put(raw, "hash", raw["parentHash"])
    replacement = execution_log(fork, 7, raw["hash"])
    put_transcript(ctx, [fork], %{}, :complete, [replacement])

    assert_raise Postgrex.Error, fn ->
      NewL1Executions.discover_new_l1_messages_executions(execution_state(ctx, 51))
    end

    assert_relayed(7, original["transactionHash"])
    assert Repo.aggregate(LifecycleTransaction, :count) == 1
    Repo.query!("ALTER TABLE arbitrum_l1_executions DROP CONSTRAINT reject_replacement")
    assert {:ok, _} = reopen(fn -> NewL1Executions.discover_new_l1_messages_executions(execution_state(ctx, 51)) end)
    assert_relayed(7, replacement["transactionHash"])
  end

  test "execution-reorg-recovery: an idle head replays only the bounded observation window", ctx do
    blocks =
      Enum.map(50..55, fn number ->
        raw = recorded_block(50)
        hash = "0x" <> String.pad_leading(Integer.to_string(number, 16), 64, "0")
        parent = "0x" <> String.pad_leading(Integer.to_string(number - 1, 16), 64, "0")

        Map.merge(raw, %{
          "number" => EthereumJSONRPC.integer_to_quantity(number),
          "hash" => hash,
          "parentHash" => parent
        })
      end)

    put_transcript(ctx, blocks, %{})

    assert {:ok, state} =
             reopen(fn -> NewL1Executions.discover_new_l1_messages_executions(execution_state(ctx, 56)) end)

    assert state.task_data.new_executions.start_block == 56
    assert Map.keys(state.task_data.new_executions.observed_blocks) |> Enum.sort() == [54, 55]
    requests = Agent.get(ctx.transcript, & &1.requests)
    [%{"params" => [filter]}] = Enum.filter(requests, &(&1["method"] == "eth_getLogs"))
    assert filter["fromBlock"] == "0x36"
    assert filter["toBlock"] == "0x37"
    assert Repo.aggregate(L1Execution, :count) == 0
  end

  defp reopen(fun) do
    # Worker modules run in fetcher tasks. Reopen in a fresh OS-independent BEAM
    # process, with only the caller's reconstructed config/cursor and durable DB.
    task = Task.async(fun)
    result = Task.await(task)
    refute Process.alive?(task.pid)
    result
  end

  defp indexed_block(number, complete?) do
    raw = recorded_block(number)
    block = insert(:block, number: number, hash: raw["hash"], consensus: true)
    transaction = insert_collated_transaction(block, 0, 2)

    if complete? do
      Repo.query!("UPDATE blocks SET send_count = 2, send_root = $1, l1_block_number = 42 WHERE hash = $2", [
        BackfillRecovery.bytes(raw["sendRoot"]),
        block.hash.bytes
      ])
    end

    receipt =
      recorded_receipt()
      |> Map.merge(%{
        "transactionHash" => to_string(transaction.hash),
        "transactionIndex" => "0x0",
        "blockHash" => raw["hash"],
        "blockNumber" => raw["number"],
        "gasUsedForL1" => "0x1024",
        "logs" => []
      })

    {block, transaction, raw, receipt}
  end

  defp insert_collated_transaction(block, index, type) do
    used = EthereumJSONRPC.quantity_to_integer(recorded_receipt()["gasUsed"])

    insert(:transaction,
      block_hash: block.hash,
      block_number: block.number,
      index: index,
      type: type,
      gas: 200_000,
      gas_used: used,
      cumulative_gas_used: used * (index + 1),
      status: :ok
    )
  end

  defp backfill_state(ctx, first, depth) do
    %{
      config: %{
        rollup_rpc: %{first_block: first, chunk_size: depth, json_rpc_named_arguments: ctx.arguments},
        backfill_blocks_depth: depth
      }
    }
  end

  defp execution_state(ctx, first) do
    %{
      config: %{
        l1_rollup_init_block: 50,
        l1_outbox_address: hd(recorded_receipt()["logs"])["address"],
        l1_rpc: %{
          json_rpc_named_arguments: ctx.arguments,
          logs_block_range: 2,
          chunk_size: 2,
          execution_reorg_depth: 2,
          track_finalization: true
        }
      },
      task_data: %{new_executions: %{start_block: first}}
    }
  end

  defp put_transcript(ctx, blocks, receipts, mode \\ :complete, logs \\ []) do
    Agent.update(
      ctx.transcript,
      &%{
        &1
        | blocks: Map.new(blocks, fn b -> {EthereumJSONRPC.quantity_to_integer(b["number"]), b} end),
          receipts: receipts,
          logs: logs,
          mode: mode
      }
    )
  end

  defp rpc_response(%{"id" => id, "method" => method, "params" => params}, state) do
    result =
      case {method, params} do
        {"eth_getBlockByNumber", ["latest", _]} ->
          state.blocks |> Enum.max_by(&elem(&1, 0)) |> elem(1)

        {"eth_getBlockByNumber", [number, _]} ->
          number = EthereumJSONRPC.quantity_to_integer(number)

          case state.mode do
            {:block_error, ^number} -> :block_error
            {:duplicate_block, ^number, other} -> other
            _ -> state.blocks[number]
          end

        {"eth_getBlockReceipts", [number]} ->
          state.receipts[EthereumJSONRPC.quantity_to_integer(number)]

        {"eth_getTransactionReceipt", [hash]} ->
          state.receipts |> Map.values() |> List.flatten() |> Enum.find(&(&1["transactionHash"] == hash))

        {"eth_getLogs", [filter]} ->
          first = EthereumJSONRPC.quantity_to_integer(filter["fromBlock"])
          last = EthereumJSONRPC.quantity_to_integer(filter["toBlock"])
          assert filter["topics"] == [Events.outbox_transaction_executed()]
          Enum.filter(state.logs, fn log -> EthereumJSONRPC.quantity_to_integer(log["blockNumber"]) in first..last end)
      end

    case result do
      :block_error -> %{jsonrpc: "2.0", id: id, error: %{code: -32000, message: "header not found"}}
      _ -> %{jsonrpc: "2.0", id: id, result: result}
    end
  end

  defp recorded_block(number) do
    block = recorded_response("block_importer_import_1_pending.json")
    hash = if number == 50, do: block["hash"], else: recorded_receipt()["blockHash"]

    Map.merge(block, %{
      "parentHash" => if(number == 51, do: block["hash"], else: block["parentHash"]),
      "hash" => hash,
      "number" => EthereumJSONRPC.integer_to_quantity(number),
      "transactions" => [],
      "sendCount" => "0x2",
      "sendRoot" => block["stateRoot"],
      "l1BlockNumber" => "0x2a"
    })
  end

  defp recorded_receipt, do: recorded_response("import_receipt_perform_1.json")

  defp recorded_response(name) do
    Path.expand("../../../explorer/test/support/fixture/vcr_cassettes/#{name}", __DIR__)
    |> File.read!()
    |> Jason.decode!()
    |> hd()
    |> get_in(["response", "body"])
    |> Jason.decode!()
    |> Map.fetch!("result")
  end

  defp execution_log(block, id, transaction_hash) do
    log = hd(recorded_receipt()["logs"])
    encoded = ABI.TypeEncoder.encode([id], Events.outbox_transaction_executed_unindexed_params())

    Map.merge(log, %{
      "blockHash" => block["hash"],
      "blockNumber" => block["number"],
      "transactionHash" => transaction_hash,
      "topics" => [Events.outbox_transaction_executed()],
      "data" => "0x" <> Base.encode16(encoded, case: :lower),
      "removed" => false
    })
  end

  defp gas(tx) do
    Repo.one!(from(t in "transactions", where: t.hash == ^tx.hash.bytes, select: t.gas_used_for_l1))
  end

  defp block_fields(block) do
    Repo.one!(
      from(b in "blocks", where: b.hash == ^block.hash.bytes, select: [b.send_count, b.send_root, b.l1_block_number])
    )
  end

  defp request_count(ctx, method) do
    Agent.get(ctx.transcript, &Enum.count(&1.requests, fn request -> request["method"] == method end))
  end

  defp insert_message(id) do
    %Message{} |> Message.changeset(%{direction: :from_l2, message_id: id, status: :confirmed}) |> Repo.insert!()
  end

  defp message(id), do: Repo.get_by!(Message, direction: :from_l2, message_id: id)

  defp assert_relayed(id, hash) do
    execution = Repo.get!(L1Execution, id) |> Repo.preload(:execution_transaction)
    assert to_string(execution.execution_transaction.hash) == hash
    assert %{status: :relayed, completion_transaction_hash: completion} = message(id)
    assert to_string(completion) == hash
  end
end
