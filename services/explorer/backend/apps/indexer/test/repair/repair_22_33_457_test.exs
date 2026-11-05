defmodule Indexer.Repair2233457Test do
  @moduledoc """
  Repair record 22.33.457:
  scroll-unbundled-reorg is proved by the unfinalized and bundled batch recovery cases.
  scroll-l1-relay-reorg is proved by the withdrawal relay recovery cases.
  scroll-l2-relay-reorg is proved by the deposit relay recovery cases.
  Provenance uses the existing constants storage, so no schema wiring is required.
  Baseline verification was not run: the ordered gate instructions require full
  implementation before the single hf-build/hf-done qualification pass.
  """

  use ExUnit.Case, async: false

  import Ecto.Query

  alias Ecto.Adapters.SQL.Sandbox
  alias Explorer.Application.Constants
  alias Explorer.Chain.Scroll.{Batch, BatchBundle, Bridge, Reader}
  alias Explorer.Chain.RollupReorgMonitorQueue
  alias Explorer.Repo
  alias Indexer.Fetcher.RollupL1ReorgMonitor
  alias Indexer.Fetcher.Scroll.{BridgeL1, BridgeL2}
  alias Indexer.Fetcher.Scroll.Batch, as: BatchFetcher
  alias Indexer.Helper

  @moduletag :capture_log
  @moduletag timeout: 60_000
  @sent "0x104371f3b442861a2a7b82a070afbbaab748bb13757bf47769e170e37809ec1e"
  @relayed "0x4641df4a962071e12719d8c8c8e5ac7fc4d97b927346a3d7a335b1f7517e133c"
  @commit "0x2c32d4ae151744d0bf0b9464a3e897a1d17ed2f1af71f7c9a75f12ce0d28238f"
  @finalize "0x26ba82f907317eedc97d0cbef23de76a43dd6edb563bdb6e9407645b950a7a2d"
  @messenger "0x5300000000000000000000000000000000000007"
  @rollup "0xa13baf47339d9bc8b8457f7a9d6a31c7df57e625"

  setup_all do
    # Apply the real optional-chain migrations to the disposable gate database.
    # An independent pool allows migration transactions outside the test sandbox.
    {:ok, migration_repo} = Repo.start_link(name: nil, pool: DBConnection.ConnectionPool, pool_size: 2)
    previous = Repo.put_dynamic_repo(migration_repo)

    try do
      for {version, file, module} <- [
            {20_240_815_102_318, "20240815102318_add_bridge_table.exs", Explorer.Repo.Scroll.Migrations.AddBridgeTable},
            {20_240_913_114_630, "20240913114630_add_batches_tables.exs",
             Explorer.Repo.Scroll.Migrations.AddBatchesTables}
          ] do
        Code.require_file(Path.expand("../../../explorer/priv/scroll/migrations/#{file}", __DIR__))
        Ecto.Migrator.up(Repo, version, module, log: false)
      end
    after
      Repo.put_dynamic_repo(previous)
      Supervisor.stop(migration_repo)
    end

    :ok
  end

  setup do
    :ok = Sandbox.checkout(Repo)
    Sandbox.mode(Repo, {:shared, self()})

    settings = [
      {:tesla, :adapter},
      {:explorer, :chain_type},
      {:indexer, Indexer.Fetcher.Scroll},
      {:indexer, BatchFetcher},
      {:indexer, BridgeL1},
      {:indexer, BridgeL2},
      {:indexer, BatchFetcher.Supervisor},
      {:indexer, BridgeL1.Supervisor},
      {:indexer, BridgeL2.Supervisor}
    ]

    previous = Enum.map(settings, fn {app, key} -> {app, key, Application.fetch_env(app, key)} end)

    on_exit(fn ->
      for {app, key, value} <- previous do
        case value do
          {:ok, config} -> Application.put_env(app, key, config)
          :error -> Application.delete_env(app, key)
        end
      end
    end)

    Application.put_env(:tesla, :adapter, Tesla.Adapter.Mint)
    Application.put_env(:explorer, :chain_type, :scroll)
    start_supervised!(Explorer.Chain.Cache.LatestL1BlockNumber.child_spec())

    block_template =
      Path.expand("../../../ethereum_jsonrpc/test/support/fixture/paxeer_x/consecutive_blocks.json", __DIR__)
      |> File.read!()
      |> Jason.decode!()
      |> Map.values()
      |> hd()

    [receipt_record] =
      Path.expand("../../../explorer/test/support/fixture/vcr_cassettes/import_receipt_perform_1.json", __DIR__)
      |> File.read!()
      |> Jason.decode!()

    log_template = receipt_record["response"]["body"] |> Jason.decode!() |> get_in(["result", "logs"]) |> hd()

    chain = %{head: 60, logs: [], transactions: %{}, requests: []}
    store = start_supervised!({Agent, fn -> %{l1: chain, l2: chain} end})
    bypass = Bypass.open()

    Bypass.expect(bypass, fn conn ->
      assert conn.method == "POST"

      layer =
        case conn.request_path do
          "/l1" -> :l1
          "/l2" -> :l2
        end

      {:ok, body, conn} = Plug.Conn.read_body(conn)
      requests = Jason.decode!(body)

      responses =
        Agent.get_and_update(store, fn state ->
          chain = state[layer]
          updated = %{chain | requests: chain.requests ++ List.wrap(requests)}

          response =
            if is_list(requests),
              do: Enum.map(requests, &rpc_response(&1, chain, block_template)),
              else: rpc_response(requests, chain, block_template)

          {response, Map.put(state, layer, updated)}
        end)

      conn
      |> Plug.Conn.put_resp_content_type("application/json")
      |> Plug.Conn.resp(200, Jason.encode!(responses))
    end)

    l1_url = "http://localhost:#{bypass.port}/l1"
    l2_rpc = Helper.json_rpc_named_arguments("http://localhost:#{bypass.port}/l2")

    Application.put_env(:indexer, Indexer.Fetcher.Scroll,
      rpc: l1_url,
      l1_eth_get_logs_range_size: 100,
      l2_eth_get_logs_range_size: 100
    )

    Application.put_env(:indexer, BatchFetcher,
      start_block: 1,
      scroll_chain_contract: @rollup,
      eip4844_blobs_api_url: "http://localhost:#{bypass.port}/unused"
    )

    Application.put_env(:indexer, BridgeL1, start_block: 1, messenger_contract: @messenger)
    Application.put_env(:indexer, BridgeL2, start_block: 1, messenger_contract: @messenger)

    # Keep ETS queues owned by the test, independent of the fetcher lifecycle.
    for module <- [BatchFetcher, BridgeL1, BridgeL2] do
      Application.put_env(:indexer, Module.concat(module, Supervisor), disabled?: false)
      while_pop(module)
    end

    start_supervised!({RollupL1ReorgMonitor, [[]]})
    %{store: store, template: log_template, l2_rpc: l2_rpc, bypass: bypass}
  end

  test "scroll-unbundled-reorg: an unfinalized commit is removed before restart and rebuilt with canonical bundle links",
       context do
    commit = batch_log(context, @commit, 0, 20, 201)
    put_chain(context, :l1, [commit])
    old = open_fetcher(BatchFetcher, context)
    await(fn -> Repo.get(Batch, 0) != nil end)
    assert Repo.get!(Batch, 0).bundle_id == nil
    :ok = :sys.suspend(old)

    put_chain(context, :l1, [])
    RollupReorgMonitorQueue.reorg_block_push(20, BatchFetcher)
    send(old, :continue)
    :ok = :sys.resume(old)
    await(fn -> Repo.get(Batch, 0) == nil end)
    close_fetcher(BatchFetcher)
    assert Reader.last_l1_batch_item() == {0, nil}
    assert Repo.all(BatchBundle) == []

    canonical = batch_log(context, @commit, 0, 22, 202)
    finalized = batch_log(context, @finalize, 0, 40, 203)
    put_chain(context, :l1, [canonical, finalized])
    reopened = open_fetcher(BatchFetcher, context)
    assert reopened != old
    await(fn -> match?(%Batch{bundle_id: id} when not is_nil(id), Repo.get(Batch, 0)) end)
    batch = Repo.get!(Batch, 0)
    bundle = Repo.get!(BatchBundle, batch.bundle_id)
    assert to_string(batch.commit_transaction_hash) == canonical["transactionHash"]
    assert batch.commit_block_number == 22
    assert bundle.final_batch_number == 0
    assert to_string(bundle.finalize_transaction_hash) == finalized["transactionHash"]
    assert Reader.batch_by_l2_block_number(0) == {0, bundle.id}
    assert Repo.aggregate(Batch, :count) == 1
    assert Repo.aggregate(BatchBundle, :count) == 1
    finish_fetchers(context, BatchFetcher)
  end

  test "scroll-unbundled-reorg: affected bundle members rewind to their first commit; finalization-only rewinds preserve commits",
       context do
    logs = [
      batch_log(context, @commit, 0, 10, 301),
      batch_log(context, @commit, 1, 20, 302),
      batch_log(context, @finalize, 1, 40, 303)
    ]

    put_chain(context, :l1, logs)
    worker = open_fetcher(BatchFetcher, context)
    await(fn -> Repo.aggregate(Batch, :count) == 2 and Repo.aggregate(BatchBundle, :count) == 1 end)
    await(fn -> Enum.all?(Repo.all(Batch), &(not is_nil(&1.bundle_id))) end)
    :ok = :sys.suspend(worker)
    RollupReorgMonitorQueue.reorg_block_push(40, BatchFetcher)
    put_chain(context, :l1, Enum.take(logs, 2))
    send(worker, :continue)
    :ok = :sys.resume(worker)
    await(fn -> Repo.aggregate(BatchBundle, :count) == 0 end)
    assert Enum.map(Repo.all(from(b in Batch, order_by: b.number)), &{&1.number, &1.bundle_id}) == [{0, nil}, {1, nil}]

    close_fetcher(BatchFetcher)
    put_chain(context, :l1, logs)
    worker = open_fetcher(BatchFetcher, context)
    await(fn -> Repo.aggregate(BatchBundle, :count) == 1 end)
    await(fn -> Enum.all?(Repo.all(Batch), &(not is_nil(&1.bundle_id))) end)
    :ok = :sys.suspend(worker)
    put_chain(context, :l1, [])
    RollupReorgMonitorQueue.reorg_block_push(20, BatchFetcher)
    send(worker, :continue)
    :ok = :sys.resume(worker)
    await(fn -> Repo.aggregate(Batch, :count) == 0 end)
    assert Repo.all(BatchBundle) == []
    close_fetcher(BatchFetcher)

    put_chain(context, :l1, [
      hd(logs),
      batch_log(context, @commit, 1, 25, 304),
      batch_log(context, @finalize, 1, 45, 305)
    ])

    open_fetcher(BatchFetcher, context)
    await(fn -> Repo.aggregate(Batch, :count) == 2 and Repo.aggregate(BatchBundle, :count) == 1 end)
    await(fn -> Enum.all?(Repo.all(Batch), &(not is_nil(&1.bundle_id))) end)
    [bundle] = Repo.all(BatchBundle)

    assert Enum.map(Repo.all(from(b in Batch, order_by: b.number)), &{&1.number, &1.bundle_id}) == [
             {0, bundle.id},
             {1, bundle.id}
           ]

    finish_fetchers(context, BatchFetcher)
  end

  for {finding, source, destination, source_module, destination_module, type, completion} <- [
        {"scroll-l1-relay-reorg", :l2, :l1, BridgeL2, BridgeL1, :withdrawal, :l1_transaction_hash},
        {"scroll-l2-relay-reorg", :l1, :l2, BridgeL1, BridgeL2, :deposit, :l2_transaction_hash}
      ],
      recovery <- [:online, :offline] do
    test "#{finding}: #{recovery} withdrawal/deposit relay recovery survives restart and canonical replay", context do
      source = unquote(source)
      destination = unquote(destination)
      source_module = unquote(source_module)
      destination_module = unquote(destination_module)
      type = unquote(type)
      completion = unquote(completion)
      recovery = unquote(recovery)

      sent = sent_log(context, 10, 401, 4)
      put_chain(context, source, [sent])
      open_fetcher(source_module, context)
      await(fn -> Repo.get_by(Bridge, type: type, index: 4) != nil end)
      initiated = Repo.get_by!(Bridge, type: type, index: 4)
      close_fetcher(source_module)
      message_hash = to_string(initiated.message_hash)

      relay = relay_log(context, message_hash, 30, 402)
      # A second canonical relay before the boundary must retain its completion.
      stable = relay_log(context, hash(777), 29, 403)
      put_chain(context, destination, [stable, relay])
      worker = open_fetcher(destination_module, context)
      await(fn -> Map.get(Repo.get_by!(Bridge, type: type, index: 4), completion) != nil end)
      assert to_string(Map.fetch!(Repo.get_by!(Bridge, type: type, index: 4), completion)) == relay["transactionHash"]
      assert relay_provenance(destination, message_hash)["block_number"] == 30
      assert relay_provenance(destination, message_hash)["block_hash"] == relay["blockHash"]
      close_fetcher(destination_module)

      # A later initiation makes recovery depend on the persisted rewind rather
      # than Reader's usual last-initiation starting point.
      later_sent = sent_log(context, 50, 404, 5)
      put_chain(context, destination, [stable, relay, later_sent])
      open_fetcher(destination_module, context)
      later_type = if type == :deposit, do: :withdrawal, else: :deposit
      await(fn -> Repo.get_by(Bridge, type: later_type, index: 5) != nil end)
      close_fetcher(destination_module)
      put_chain(context, destination, [stable, later_sent])

      Agent.update(context.store, fn state ->
        Map.update!(state, destination, &%{&1 | requests: []})
      end)

      if recovery == :online do
        destination_module.reorg_handle(30)
        assert Map.get(Repo.get_by!(Bridge, type: type, index: 4), completion) == nil
        assert relay_provenance(destination, message_hash) == nil
      end

      reopened = open_fetcher(destination_module, context)
      assert reopened != worker
      await(fn -> Map.get(Repo.get_by!(Bridge, type: type, index: 4), completion) == nil end)
      await(fn -> relay_provenance(destination, message_hash) == nil end)
      recovered = Repo.get_by!(Bridge, type: type, index: 4)
      assert recovered.block_number == initiated.block_number
      assert recovered.block_timestamp == initiated.block_timestamp
      assert recovered.amount == initiated.amount
      assert recovered.message_hash == initiated.message_hash
      assert recovered.index == 4

      assert to_string(Map.fetch!(Repo.get_by!(Bridge, type: type, message_hash: cast_hash(777)), completion)) ==
               stable["transactionHash"]

      await(fn -> Map.get(:sys.get_state(reopened), :start_block) == 61 end)

      canonical = relay_log(context, message_hash, 35, 405)
      put_chain(context, destination, [stable, canonical, later_sent])
      destination_module.reorg_handle(30)

      replay_worker =
        if recovery == :online do
          restart_fetcher(destination_module, reopened)
        else
          close_fetcher(destination_module)
          open_fetcher(destination_module, context)
        end

      assert replay_worker != reopened

      await(fn ->
        state = :sys.get_state(replay_worker)
        value = Map.get(Repo.get_by!(Bridge, type: type, index: 4), completion)

        Map.get(state, :start_block) == 61 and not is_nil(value) and
          to_string(value) == canonical["transactionHash"]
      end)

      assert relay_provenance(destination, message_hash)["block_number"] == 35
      assert relay_provenance(destination, message_hash)["transaction_hash"] == canonical["transactionHash"]
      assert Repo.aggregate(from(b in Bridge, where: b.type == ^type and b.index == 4), :count) == 1

      requests = Agent.get(context.store, & &1[destination].requests)

      assert Enum.any?(
               requests,
               &(&1["method"] == "eth_getLogs" and
                   EthereumJSONRPC.quantity_to_integer(hd(&1["params"])["fromBlock"]) <= 30)
             )

      assert Enum.any?(
               requests,
               &(&1["method"] == "eth_getTransactionByHash" and
                   &1["params"] == [relay["transactionHash"]])
             ) == (recovery == :offline)

      finish_fetchers(context, destination_module)
    end
  end

  defp open_fetcher(module, context) do
    args = if module == BridgeL2, do: [json_rpc_named_arguments: context.l2_rpc], else: []
    start_supervised!({Module.concat(module, Supervisor), [args]})
    Process.whereis(module)
  end

  defp close_fetcher(module) do
    :ok = :sys.suspend(module)
    :ok = stop_supervised(Module.concat(module, Supervisor))
  end

  defp finish_fetchers(context, module) do
    close_fetcher(module)
    :ok = :sys.suspend(RollupL1ReorgMonitor)
    :ok = stop_supervised(RollupL1ReorgMonitor)
    :ok = Bypass.down(context.bypass)
  end

  defp restart_fetcher(module, worker) do
    :ok = :sys.suspend(worker)
    monitor = Process.monitor(worker)
    Process.exit(worker, :kill)
    assert_receive {:DOWN, ^monitor, :process, ^worker, :killed}

    await(fn ->
      restarted = Process.whereis(module)
      is_pid(restarted) and restarted != worker
    end)

    Process.whereis(module)
  end

  defp await(predicate, attempts \\ 1_000) do
    if predicate.() do
      :ok
    else
      assert attempts > 0, "fetcher did not produce the expected durable state"
      Process.sleep(10)
      await(predicate, attempts - 1)
    end
  end

  defp while_pop(module) do
    if RollupReorgMonitorQueue.reorg_block_pop(module), do: while_pop(module), else: :ok
  end

  defp relay_provenance(layer, message_hash) do
    case Constants.get_constant_value("scroll_bridge:relay:#{layer}:#{message_hash}") do
      nil -> nil
      value -> Jason.decode!(value)
    end
  end

  defp put_chain(context, layer, logs) do
    transactions =
      Map.new(logs, fn log ->
        chunk = <<1, 100::unsigned-big-integer-size(64), 0::size(416)>>
        input = ABI.encode("commitBatch(uint8,bytes,bytes[],bytes)", [0, <<>>, [chunk], <<>>])

        transaction = %{
          "hash" => log["transactionHash"],
          "blockNumber" => log["blockNumber"],
          "blockHash" => log["blockHash"],
          "input" => "0x" <> Base.encode16(input, case: :lower)
        }

        {log["transactionHash"], transaction}
      end)

    Agent.update(context.store, fn state ->
      Map.update!(state, layer, &%{&1 | logs: logs, transactions: transactions})
    end)
  end

  defp batch_log(context, topic, number, block, tx) do
    rpc_log(context, @rollup, [topic, hash(number), hash(999)], "0x" <> String.duplicate("0", 128), block, tx)
  end

  defp sent_log(context, block, tx, index) do
    data = ABI.TypeEncoder.encode([7, index, 100_000, <<>>], [{:uint, 256}, {:uint, 256}, {:uint, 256}, :bytes])
    rpc_log(context, @messenger, [@sent, hash(1), hash(2)], "0x" <> Base.encode16(data, case: :lower), block, tx)
  end

  defp relay_log(context, message_hash, block, tx) do
    rpc_log(context, @messenger, [@relayed, message_hash], "0x", block, tx)
  end

  defp rpc_log(context, address, topics, data, block, tx) do
    Map.merge(context.template, %{
      "address" => address,
      "topics" => topics,
      "data" => data,
      "blockNumber" => quantity(block),
      "blockHash" => hash(block + 10_000),
      "transactionHash" => hash(tx),
      "transactionIndex" => "0x0",
      "logIndex" => "0x0",
      "removed" => false
    })
  end

  defp rpc_response(%{"id" => id} = request, chain, template) do
    %{"jsonrpc" => "2.0", "id" => id, "result" => rpc_result(request, chain, template)}
  end

  defp rpc_result(%{"method" => "eth_getLogs", "params" => [filter]}, chain, _template) do
    first = EthereumJSONRPC.quantity_to_integer(filter["fromBlock"])
    last = EthereumJSONRPC.quantity_to_integer(filter["toBlock"])
    assert first <= last
    [topics] = filter["topics"]

    Enum.filter(chain.logs, fn log ->
      number = EthereumJSONRPC.quantity_to_integer(log["blockNumber"])
      number >= first and number <= last and log["address"] == filter["address"] and hd(log["topics"]) in topics
    end)
  end

  defp rpc_result(%{"method" => "eth_getBlockByNumber", "params" => [tag, hydrated]}, chain, template) do
    number = if tag in ["latest", "safe"], do: chain.head, else: EthereumJSONRPC.quantity_to_integer(tag)
    transactions = chain.transactions |> Map.values() |> Enum.filter(&(&1["blockNumber"] == quantity(number)))
    transactions = if hydrated, do: transactions, else: Enum.map(transactions, & &1["hash"])

    Map.merge(template, %{
      "number" => quantity(number),
      "hash" => hash(number + 10_000),
      "timestamp" => quantity(1_500_000_000 + number * 120),
      "transactions" => transactions
    })
  end

  defp rpc_result(%{"method" => "eth_getTransactionByHash", "params" => [hash]}, chain, _template),
    do: chain.transactions[hash]

  defp rpc_result(%{"method" => "eth_chainId"}, _chain, _template), do: "0x1"

  defp quantity(number), do: EthereumJSONRPC.integer_to_quantity(number)
  defp hash(number), do: "0x" <> (number |> Integer.to_string(16) |> String.downcase() |> String.pad_leading(64, "0"))

  defp cast_hash(number) do
    {:ok, hash} = Explorer.Chain.Hash.Full.cast(hash(number))
    hash
  end
end
