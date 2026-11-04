defmodule Indexer.Repair2233547Test do
  use Explorer.DataCase, async: false

  alias EthereumJSONRPC.Arbitrum.Constants.{Contracts, Events}
  alias Explorer.Chain.Arbitrum.{BatchBlock, BatchToDaBlob, BatchTransaction, L1Batch, LifecycleTransaction, Message}
  alias Indexer.Fetcher.Arbitrum.Workers.Batches.{Discovery, Recovery, Tasks}

  setup do
    # The deployed chain's test database does not install the optional Arbitrum
    # tables. These transaction-local tables retain the production constraints.
    Repo.query!("""
    CREATE TEMP TABLE arbitrum_lifecycle_l1_transactions (
      id integer PRIMARY KEY, hash bytea NOT NULL UNIQUE, block_number integer NOT NULL,
      timestamp timestamp NOT NULL, status varchar NOT NULL,
      inserted_at timestamp NOT NULL, updated_at timestamp NOT NULL
    )
    """)

    Repo.query!("""
    CREATE TEMP TABLE arbitrum_l1_batches (
      number integer PRIMARY KEY, transactions_count integer NOT NULL,
      start_block integer NOT NULL, end_block integer NOT NULL,
      before_acc bytea NOT NULL, after_acc bytea NOT NULL,
      commitment_id integer NOT NULL REFERENCES arbitrum_lifecycle_l1_transactions(id),
      batch_container varchar, inserted_at timestamp NOT NULL, updated_at timestamp NOT NULL
    )
    """)

    Repo.query!("""
    CREATE TEMP TABLE arbitrum_batch_l2_blocks (
      block_number integer PRIMARY KEY,
      batch_number integer NOT NULL REFERENCES arbitrum_l1_batches(number),
      confirmation_id integer REFERENCES arbitrum_lifecycle_l1_transactions(id),
      inserted_at timestamp NOT NULL, updated_at timestamp NOT NULL
    )
    """)

    Repo.query!("""
    CREATE TEMP TABLE arbitrum_batch_l2_transactions (
      transaction_hash bytea PRIMARY KEY,
      batch_number integer NOT NULL REFERENCES arbitrum_l1_batches(number),
      inserted_at timestamp NOT NULL, updated_at timestamp NOT NULL
    )
    """)

    Repo.query!("""
    CREATE TEMP TABLE arbitrum_da_multi_purpose (
      data_key bytea PRIMARY KEY, data_type integer NOT NULL, data jsonb NOT NULL,
      batch_number integer REFERENCES arbitrum_l1_batches(number),
      inserted_at timestamp NOT NULL, updated_at timestamp NOT NULL
    )
    """)

    Repo.query!("""
    CREATE TEMP TABLE arbitrum_batches_to_da_blobs (
      batch_number integer PRIMARY KEY REFERENCES arbitrum_l1_batches(number),
      data_blob_id bytea NOT NULL REFERENCES arbitrum_da_multi_purpose(data_key),
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
    {:ok, transcript} = Agent.start_link(fn -> %{logs: [], transactions: %{}, requests: [], latest: 80} end)

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

  test "batch-reorg-replacement: same-hash relocation survives a worker restart", ctx do
    indexed_rollup_block(10)
    publish(ctx, 1, old_hash(), 50, 10, 11, celestia_data(42))
    assert :ok = reopen(fn -> discover(ctx, 50, 50) end)
    original = batch(1)
    old_blob = blob(1)
    assert original.commitment_transaction.block_number == 50
    assert original.transactions_count == 1

    Repo.update_all(from(b in BatchBlock, where: b.batch_number == 1), set: [confirmation_id: original.commitment_id])
    publish(ctx, 1, old_hash(), 51, 10, 11, celestia_data(42))
    assert :ok = reopen(fn -> discover(ctx, 50, 51) end)

    relocated = batch(1)
    assert relocated.commitment_id == original.commitment_id
    assert to_string(relocated.commitment_transaction.hash) == old_hash()
    assert relocated.commitment_transaction.block_number == 51
    assert DateTime.compare(relocated.commitment_transaction.timestamp, timestamp(51)) == :eq
    assert blob(1) == old_blob
    assert Repo.get!(BatchBlock, 10).confirmation_id == original.commitment_id
    assert Repo.aggregate(LifecycleTransaction, :count) == 1
  end

  test "batch-reorg-replacement: replacement hash atomically rebinds blocks transactions and DA after restart", ctx do
    old_tx = indexed_rollup_block(10)
    new_tx = indexed_rollup_block(11)
    insert_message(1, 10)
    insert_message(2, 11)
    publish(ctx, 1, old_hash(), 50, 10, 11, celestia_data(42))
    assert :ok = reopen(fn -> discover(ctx, 50, 50) end)
    original = batch(1)
    old_blob = blob(1)
    assert message(1).status == :sent
    assert message(2).status == :initiated

    publish(ctx, 1, replacement_hash(), 51, 11, 12, celestia_data(43))
    assert :ok = reopen(fn -> discover(ctx, 50, 51) end)

    replaced = batch(1)
    assert replaced.commitment_id != original.commitment_id
    assert to_string(replaced.commitment_transaction.hash) == replacement_hash()
    assert replaced.commitment_transaction.block_number == 51
    assert {replaced.start_block, replaced.end_block, replaced.transactions_count} == {11, 11, 1}
    assert Repo.get(BatchBlock, 10) == nil
    assert Repo.get!(BatchBlock, 11).batch_number == 1
    assert Repo.get_by(BatchTransaction, transaction_hash: old_tx.hash) == nil
    assert Repo.get_by!(BatchTransaction, transaction_hash: new_tx.hash).batch_number == 1
    refute blob(1) == old_blob
    assert blob(1).data["height"] == 43
    assert message(1).status == :initiated
    assert message(2).status == :sent
    assert Recovery.da_outcome(1)["transaction_hash"] == replacement_hash()
    assert Recovery.da_outcome(1)["status"] == "parsed"
  end

  test "batch-reorg-replacement: absent canonical event removes the orphan and its associations after restart", ctx do
    indexed_rollup_block(10)
    insert_message(1, 10)
    publish(ctx, 1, old_hash(), 50, 10, 11, celestia_data(42))
    assert :ok = reopen(fn -> discover(ctx, 50, 50) end)
    assert Repo.get!(L1Batch, 1)

    Agent.update(ctx.transcript, &%{&1 | logs: []})
    assert :ok = reopen(fn -> discover(ctx, 50, 50) end)

    assert Repo.get(L1Batch, 1) == nil
    assert Repo.aggregate(BatchBlock, :count) == 0
    assert Repo.aggregate(BatchTransaction, :count) == 0
    assert Repo.aggregate(BatchToDaBlob, :count) == 0
    assert Recovery.da_outcome(1) == nil
    assert message(1).status == :initiated
  end

  test "da-parse-failure-lost: restart schedules failed DA outside the live scan and imports recovered metadata", ctx do
    indexed_rollup_block(10)
    publish(ctx, 1, old_hash(), 50, 10, 11, <<99, 1>>)
    assert :ok = reopen(fn -> discover(ctx, 50, 50) end)
    assert batch(1).batch_container == nil
    assert blob(1) == nil
    assert %{"status" => "failed", "retry" => true, "error" => "unavailable_or_unsupported_da"} = Recovery.da_outcome(1)
    assert Recovery.unresolved_l1_blocks(10) == [50]

    # A recovered provider/decoder supplies the complete DA payload. The new
    # fetcher has only its head cursor; no in-memory retry state is carried over.
    publish(ctx, 1, old_hash(), 50, 10, 11, celestia_data(42))
    assert {:ok, state} = reopen(fn -> Tasks.check_new(worker_state(ctx)) end)
    assert state.task_data.new_batches.start_block == 81
    assert batch(1).batch_container == :in_celestia
    assert blob(1).data["height"] == 42
    assert %{"status" => "parsed", "retry" => false} = Recovery.da_outcome(1)
    assert Recovery.unresolved_l1_blocks(10) == []

    requests = Agent.get(ctx.transcript, & &1.requests)
    assert Enum.count(requests, &(&1["method"] == "eth_getTransactionByHash")) == 2
    assert Enum.count(requests, &match?(%{"method" => "eth_getLogs", "params" => [%{"fromBlock" => "0x32"}]}, &1)) == 2
  end

  test "da-parse-failure-lost: calldata and blobs persist known no-DA outcomes and are not queued", ctx do
    indexed_rollup_block(10)
    indexed_rollup_block(11)
    publish(ctx, 1, old_hash(), 50, 10, 11, <<0>>)
    assert :ok = reopen(fn -> discover(ctx, 50, 50) end)
    publish(ctx, 2, replacement_hash(), 51, 11, 12, nil)
    assert :ok = reopen(fn -> discover(ctx, 51, 51) end)

    assert batch(1).batch_container == :in_calldata
    assert batch(2).batch_container == :in_blob4844

    for number <- [1, 2] do
      assert %{"status" => "not_required", "retry" => false} = Recovery.da_outcome(number)
      assert blob(number) == nil
    end

    assert Recovery.unresolved_l1_blocks(10) == []
    count = request_count(ctx, "eth_getTransactionByHash")
    assert {:ok, _} = reopen(fn -> Tasks.check_new(worker_state(ctx)) end)
    assert request_count(ctx, "eth_getTransactionByHash") == count
  end

  test "da-parse-failure-lost: legacy null DA is reparsed and failed outcomes remain retryable across restarts", ctx do
    indexed_rollup_block(10)
    publish(ctx, 1, old_hash(), 50, 10, 11, <<32>>)
    assert :ok = reopen(fn -> discover(ctx, 50, 50) end)
    assert :ok = reopen(fn -> Tasks.check_new(worker_state(ctx)) |> elem(0) end)
    assert %{"status" => "failed", "retry" => true} = Recovery.da_outcome(1)
    assert Recovery.unresolved_l1_blocks(10) == [50]

    Repo.query!("DELETE FROM arbitrum_da_multi_purpose WHERE data_type = 2")
    assert Recovery.da_outcome(1) == nil
    assert Recovery.unresolved_l1_blocks(10) == [50]
    publish(ctx, 1, old_hash(), 50, 10, 11, <<0>>)
    assert {:ok, _} = reopen(fn -> Tasks.check_new(worker_state(ctx)) end)
    assert batch(1).batch_container == :in_calldata
    assert Recovery.unresolved_l1_blocks(10) == []
  end

  test "batch-reorg-replacement: a late import failure rolls back the whole replacement", ctx do
    indexed_rollup_block(10)
    indexed_rollup_block(11)
    publish(ctx, 1, old_hash(), 50, 10, 11, celestia_data(42))
    assert :ok = reopen(fn -> discover(ctx, 50, 50) end)
    original = batch(1)
    original_blob = blob(1)
    original_outcome = Recovery.da_outcome(1)

    # Reject the new DA descriptor at the real database boundary, after the
    # lifecycle, batch, block and transaction runners have performed their writes.
    Repo.query!(
      "ALTER TABLE arbitrum_da_multi_purpose ADD CONSTRAINT reject_replacement CHECK (data->>'height' IS DISTINCT FROM '43')"
    )

    publish(ctx, 1, replacement_hash(), 51, 11, 12, celestia_data(43))

    assert {:rejected, %Postgrex.Error{postgres: %{code: :check_violation}}} =
             reopen(fn ->
               try do
                 discover(ctx, 50, 51)
               rescue
                 error in Postgrex.Error -> {:rejected, error}
               end
             end)

    assert batch(1) == original
    assert blob(1) == original_blob
    assert Recovery.da_outcome(1) == original_outcome
    assert Repo.get!(BatchBlock, 10).batch_number == 1
    assert Repo.get(BatchBlock, 11) == nil
    assert Repo.get_by(LifecycleTransaction, hash: replacement_hash()) == nil
  end

  defp reopen(fun) do
    task = Task.async(fun)
    result = Task.await(task)
    refute Process.alive?(task.pid)
    result
  end

  defp discover(ctx, first, last) do
    Discovery.perform(
      inbox(),
      first,
      last,
      10,
      0,
      l1_rpc(ctx),
      Contracts.node_interface_contract_address(),
      rollup_rpc(ctx)
    )
  end

  defp worker_state(ctx) do
    %{
      config: %{
        l1_rpc: l1_rpc(ctx),
        rollup_rpc: rollup_rpc(ctx),
        l1_sequencer_inbox_address: inbox(),
        messages_to_blocks_shift: 0,
        new_batches_limit: 10,
        node_interface_address: Contracts.node_interface_contract_address()
      },
      task_data: %{new_batches: %{start_block: 81}, historical_batches: %{end_block: 79}}
    }
  end

  defp l1_rpc(ctx),
    do: %{json_rpc_named_arguments: ctx.arguments, chunk_size: 10, logs_block_range: 2, track_finalization: true}

  defp rollup_rpc(ctx), do: %{json_rpc_named_arguments: ctx.arguments, chunk_size: 10}

  defp indexed_rollup_block(number) do
    block = insert(:block, number: number, consensus: true)
    gas_used = EthereumJSONRPC.quantity_to_integer(recorded_receipt()["gasUsed"])

    insert(:transaction,
      block_hash: block.hash,
      block_number: number,
      index: 0,
      gas: 200_000,
      gas_used: gas_used,
      cumulative_gas_used: gas_used,
      status: :ok
    )
  end

  defp insert_message(id, block) do
    %Message{}
    |> Message.changeset(%{
      direction: :from_l2,
      message_id: id,
      status: :initiated,
      originating_transaction_block_number: block
    })
    |> Repo.insert!()
  end

  defp message(id), do: Repo.get_by!(Message, direction: :from_l2, message_id: id)
  defp batch(number), do: Repo.get!(L1Batch, number) |> Repo.preload(:commitment_transaction)

  defp blob(number) do
    link = Repo.get(BatchToDaBlob, number)
    if link, do: Repo.preload(link, :da_record).da_record, else: nil
  end

  # Replay the retained node envelopes through HTTP. Calldata uses the actual
  # SequencerInbox ABI; fork and truncated-provider variants change its payload.
  defp publish(ctx, number, hash, l1_block, first, next, data) do
    {selector, params, abi} =
      if is_nil(data) do
        {"3e5aa082", [number, 0, <<0::160>>, first, next],
         Contracts.add_sequencer_l2_batch_from_blobs_selector_with_abi()}
      else
        {"8f111f3c", [number, data, 0, <<0::160>>, first, next],
         Contracts.add_sequencer_l2_batch_from_origin_8f111f3c_selector_with_abi()}
      end

    calldata = "0x" <> selector <> Base.encode16(ABI.TypeEncoder.encode(params, abi.types), case: :lower)
    block = recorded_block(l1_block)

    transaction =
      recorded_response("transaction_importer_download_transaction.json")
      |> Map.merge(%{
        "hash" => hash,
        "blockNumber" => block["number"],
        "blockHash" => block["hash"],
        "input" => calldata
      })

    log =
      hd(recorded_receipt()["logs"])
      |> Map.merge(%{
        "address" => inbox(),
        "blockHash" => block["hash"],
        "blockNumber" => block["number"],
        "transactionHash" => hash,
        "removed" => false,
        "topics" => [
          Events.sequencer_batch_delivered(),
          "0x" <> String.pad_leading(Integer.to_string(number, 16), 64, "0"),
          block["stateRoot"],
          block["transactionsRoot"]
        ]
      })

    Agent.update(ctx.transcript, &%{&1 | logs: [log], transactions: %{hash => transaction}})
  end

  defp rpc_response(%{"id" => id, "method" => method, "params" => params}, state) do
    result =
      case {method, params} do
        {"eth_getLogs", [filter]} ->
          assert filter["address"] == inbox()
          assert filter["topics"] == [Events.sequencer_batch_delivered()]
          first = EthereumJSONRPC.quantity_to_integer(filter["fromBlock"])
          last = EthereumJSONRPC.quantity_to_integer(filter["toBlock"])
          Enum.filter(state.logs, &(EthereumJSONRPC.quantity_to_integer(&1["blockNumber"]) in first..last))

        {"eth_getBlockByNumber", [tag, _]} when tag in ["safe", "latest"] ->
          recorded_block(state.latest)

        {"eth_getBlockByNumber", [number, _]} ->
          recorded_block(EthereumJSONRPC.quantity_to_integer(number))

        {"eth_getTransactionByHash", [hash]} ->
          Map.fetch!(state.transactions, hash)
      end

    %{jsonrpc: "2.0", id: id, result: result}
  end

  defp recorded_block(number) do
    block = recorded_response("block_importer_import_1_pending.json")

    Map.merge(block, %{
      "number" => EthereumJSONRPC.integer_to_quantity(number),
      "timestamp" => EthereumJSONRPC.integer_to_quantity(1_700_000_000 + number),
      "hash" => if(number == 50, do: block["hash"], else: recorded_receipt()["blockHash"]),
      "transactions" => []
    })
  end

  defp timestamp(number), do: DateTime.from_unix!(1_700_000_000 + number)
  defp old_hash, do: recorded_receipt()["transactionHash"]
  defp replacement_hash, do: recorded_response("transaction_importer_download_transaction.json")["hash"]
  defp inbox, do: hd(recorded_receipt()["logs"])["address"]
  defp recorded_receipt, do: recorded_response("import_receipt_perform_1.json")
  defp celestia_data(height), do: <<99, height::unsigned-big-64, 0::64, 1::64, 7::256, 8::256>>

  defp recorded_response(name) do
    Path.expand("../../../explorer/test/support/fixture/vcr_cassettes/#{name}", __DIR__)
    |> File.read!()
    |> Jason.decode!()
    |> hd()
    |> get_in(["response", "body"])
    |> Jason.decode!()
    |> Map.fetch!("result")
  end

  defp request_count(ctx, method) do
    Agent.get(ctx.transcript, &Enum.count(&1.requests, fn request -> request["method"] == method end))
  end
end
