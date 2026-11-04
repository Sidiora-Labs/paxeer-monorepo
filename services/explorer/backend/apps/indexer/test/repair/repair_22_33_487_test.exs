defmodule Indexer.Repair2233487Test do
  use Explorer.DataCase, async: false

  alias Explorer.Chain.{InternalTransaction, PendingBlockOperation, PendingTransactionOperation}
  alias Explorer.Chain.Cache.BlockNumber
  alias Indexer.BufferedTask
  alias Indexer.Fetcher.InternalTransaction, as: Fetcher
  alias Indexer.Fetcher.CoinBalance.Catchup, as: CoinBalance

  @moduletag :capture_log
  @fixtures Path.expand("../../../ethereum_jsonrpc/test/support/fixture/paxeer_x", __DIR__)

  defmodule RecordedRPC do
    @behaviour Plug
    def init(options), do: options

    def call(conn, {source, blocks, trace, observer}) do
      {:ok, body, conn} = Plug.Conn.read_body(conn)
      requests = Jason.decode!(body)

      responses =
        Enum.map(requests, fn request ->
          %{"jsonrpc" => "2.0", "id" => id, "method" => method, "params" => params} = request
          send(observer, {:rpc, source, method, params})

          result =
            case {method, params} do
              {"eth_getBlockByNumber", [quantity, true]} ->
                Map.fetch!(blocks, quantity)

              {"trace_replayBlockTransactions", [quantity, ["trace"]]} ->
                %{"transactions" => []} = Map.fetch!(blocks, quantity)
                []

              {"debug_traceTransaction", [hash, %{"tracer" => "callTracer"}]} ->
                true = hash == trace["txHash"]
                trace["result"]
            end

          if source == :unavailable_archive do
            %{jsonrpc: "2.0", id: id, error: %{code: -32000, message: "historical state unavailable"}}
          else
            %{jsonrpc: "2.0", id: id, result: result}
          end
        end)

      conn
      |> Plug.Conn.put_resp_content_type("application/json")
      |> Plug.Conn.send_resp(200, Jason.encode!(responses))
    end
  end

  setup do
    configs = [
      {:indexer, Fetcher},
      {:indexer, :trace_block_ranges},
      {:indexer, :trace_first_block},
      {:explorer, :json_rpc_named_arguments},
      {:ethereum_jsonrpc, EthereumJSONRPC.Geth},
      {:tesla, :adapter}
    ]

    previous = Enum.map(configs, fn {app, key} -> {app, key, Application.fetch_env(app, key)} end)

    on_exit(fn ->
      for {app, key, value} <- previous do
        case value do
          {:ok, config} -> Application.put_env(app, key, config)
          :error -> Application.delete_env(app, key)
        end
      end
    end)

    Application.put_env(:indexer, Fetcher, lookback_blocks: 100)
    Application.put_env(:indexer, :trace_block_ranges, "0..latest")
    Application.put_env(:indexer, :trace_first_block, 0)
    Application.put_env(:tesla, :adapter, Tesla.Adapter.Mint)

    geth = Application.get_env(:ethereum_jsonrpc, EthereumJSONRPC.Geth, [])

    Application.put_env(
      :ethereum_jsonrpc,
      EthereumJSONRPC.Geth,
      Keyword.merge(geth, block_traceable?: false, tracer: "call_tracer")
    )

    blocks =
      @fixtures
      |> Path.join("consecutive_blocks.json")
      |> File.read!()
      |> Jason.decode!()
      |> Map.new(fn {quantity, block} ->
        {quantity |> EthereumJSONRPC.quantity_to_integer() |> EthereumJSONRPC.integer_to_quantity(), block}
      end)

    trace =
      @fixtures
      |> Path.join("call_tracer_block.json")
      |> File.read!()
      |> Jason.decode!()
      |> Map.fetch!("result")
      |> hd()

    {:ok, _} = Application.ensure_all_started(:plug_cowboy)
    pruned = rpc_server(:pruned, blocks, trace)
    archive = rpc_server(:archive, blocks, trace)

    start_supervised!({Task.Supervisor, name: Fetcher.TaskSupervisor})
    start_supervised!({Task.Supervisor, name: CoinBalance.TaskSupervisor})

    balance_fetcher =
      start_supervised!(
        {CoinBalance, [[json_rpc_named_arguments: pruned, flush_interval: 60_000, poll: false], [name: CoinBalance]]}
      )

    await_idle(balance_fetcher)

    %{pruned: pruned, archive: archive, blocks: blocks, trace: trace}
  end

  test "008 block backlog survives pruned restarts and archive import recovers coverage", context do
    rpc = Keyword.put(context.pruned, :variant, EthereumJSONRPC.Nethermind)
    Application.put_env(:explorer, :json_rpc_named_arguments, rpc)
    numbers = context.blocks |> Map.keys() |> Enum.map(&EthereumJSONRPC.quantity_to_integer/1) |> Enum.sort()
    [old, boundary] = numbers
    BlockNumber.set_max(boundary + 100)

    assert {:ok, %{blocks_params: blocks}} = EthereumJSONRPC.fetch_blocks_by_numbers(numbers, rpc)
    assert {:ok, _} = Explorer.Chain.import(%{blocks: %{params: Enum.map(blocks, &Map.put(&1, :consensus, true))}})
    assert Repo.aggregate(PendingBlockOperation, :count) == 2
    assert %{pending_blocks: 2, deferred_blocks: 1, oldest_traceable_block_number: ^boundary} = Fetcher.trace_coverage()

    first = start_fetcher(rpc)
    await_idle(first)
    assert_receive {:rpc, :pruned, "trace_replayBlockTransactions", [quantity, ["trace"]]}
    assert quantity == EthereumJSONRPC.integer_to_quantity(boundary)
    refute_received {:rpc, :pruned, "trace_replayBlockTransactions", ["0x1714EFD", _]}
    assert Repo.one!(PendingBlockOperation).block_number == old
    assert :ok = Fetcher.run([old], rpc)
    assert Repo.one!(PendingBlockOperation).block_number == old

    assert :ok = stop_supervised(Fetcher)
    refute Process.alive?(first)
    second = start_fetcher(rpc)
    await_idle(second)
    assert Fetcher.init([], fn number, acc -> [number | acc] end, rpc) == []
    assert %{pending_blocks: 1, deferred_blocks: 1} = Fetcher.trace_coverage()
    refute_received {:rpc, :pruned, "trace_replayBlockTransactions", _}
    assert :ok = stop_supervised(Fetcher)

    Application.put_env(:indexer, Fetcher, lookback_blocks: nil)
    archive = Keyword.put(context.archive, :variant, EthereumJSONRPC.Nethermind)
    Application.put_env(:explorer, :json_rpc_named_arguments, archive)
    recovered = start_fetcher(archive)
    refute recovered in [first, second]
    await_idle(recovered)
    assert_receive {:rpc, :archive, "trace_replayBlockTransactions", ["0x1714EFD", ["trace"]]}
    assert Repo.aggregate(PendingBlockOperation, :count) == 0
    assert %{pending_blocks: 0, deferred_blocks: 0} = Fetcher.trace_coverage()
    assert :ok = stop_supervised(Fetcher)
    await_idle(start_fetcher(archive))
    refute_received {:rpc, :archive, "trace_replayBlockTransactions", _}
  end

  test "008 transaction identity survives restart and recorded archive traces reach the importer", context do
    rpc = Keyword.put(context.pruned, :variant, EthereumJSONRPC.Geth)
    Application.put_env(:explorer, :json_rpc_named_arguments, rpc)
    block = insert(:block, number: 24_203_005)
    trace = context.trace
    root = trace["result"]
    from = insert(:address, hash: root["from"])

    transaction =
      insert(:transaction, hash: trace["txHash"], from_address: from, to_address: nil, input: root["input"], value: 42)
      |> with_block(block, status: :ok, index: 0)

    insert(:pending_transaction_operation, transaction_hash: transaction.hash)
    BlockNumber.set_max(block.number + 101)
    params = Map.take(transaction, [:block_number, :hash, :index, :type])

    assert :ok = Fetcher.run([params], rpc)
    assert Repo.get!(PendingTransactionOperation, transaction.hash)
    assert %{pending_transactions: 1, deferred_transactions: 1} = Fetcher.trace_coverage()
    first = start_fetcher(rpc)
    await_idle(first)
    assert :ok = stop_supervised(Fetcher)
    refute Process.alive?(first)
    await_idle(start_fetcher(rpc))
    assert Repo.get!(PendingTransactionOperation, transaction.hash)
    assert Fetcher.init([], fn entry, acc -> [entry | acc] end, rpc) == []
    refute_received {:rpc, :pruned, "debug_traceTransaction", _}
    assert :ok = stop_supervised(Fetcher)

    Application.put_env(:indexer, Fetcher, lookback_blocks: 0)

    unavailable =
      rpc_server(:unavailable_archive, context.blocks, context.trace)
      |> Keyword.put(:variant, EthereumJSONRPC.Geth)

    assert {:retry, [^params]} = Fetcher.run([params], unavailable)
    assert_receive {:rpc, :unavailable_archive, "debug_traceTransaction", _}
    assert Repo.get!(PendingTransactionOperation, transaction.hash)
    assert Repo.aggregate(InternalTransaction, :count) == 0
    assert %{pending_transactions: 1, deferred_transactions: 0} = Fetcher.trace_coverage()

    archive = Keyword.put(context.archive, :variant, EthereumJSONRPC.Geth)
    Application.put_env(:explorer, :json_rpc_named_arguments, archive)

    decoder_rpc =
      rpc_server(:decoder, context.blocks, context.trace)
      |> Keyword.put(:variant, EthereumJSONRPC.Geth)

    assert {:ok, decoded} =
             EthereumJSONRPC.fetch_internal_transactions(
               [%{block_number: block.number, hash_data: trace["txHash"], transaction_index: transaction.index}],
               decoder_rpc
             )

    call = Enum.find(decoded, &(&1.index == 1))
    assert call.trace_address == [0]
    assert_receive {:rpc, :decoder, "debug_traceTransaction", _}

    recovered = start_fetcher(archive)
    await_idle(recovered)
    assert_receive {:rpc, :archive, "debug_traceTransaction", [hash, %{"tracer" => "callTracer"}]}
    assert hash == trace["txHash"]
    refute Repo.get(PendingTransactionOperation, transaction.hash)

    imported =
      Repo.all(from(it in InternalTransaction, where: it.transaction_hash == ^transaction.hash, order_by: it.index))

    assert length(imported) == 2
    [creation, call] = imported
    assert creation.type == :create
    assert to_string(creation.created_contract_address_hash) == root["to"]
    assert creation.value.value == Decimal.new(42)
    assert call.type == :call
    assert is_nil(call.trace_address)
    assert call.index == 1
    assert call.transaction_hash == transaction.hash
    assert to_string(call.to_address_hash) == hd(root["calls"])["to"]
    assert call.value.value == Decimal.new(7)
    assert %{pending_transactions: 0, deferred_transactions: 0} = Fetcher.trace_coverage()
    assert :ok = stop_supervised(Fetcher)
    await_idle(start_fetcher(archive))
    refute_received {:rpc, :archive, "debug_traceTransaction", _}
    assert Repo.aggregate(InternalTransaction, :count) == 2
  end

  defp rpc_server(source, blocks, trace) do
    ref = {__MODULE__, source, make_ref()}

    start_supervised!(
      {Plug.Cowboy, scheme: :http, plug: {RecordedRPC, {source, blocks, trace, self()}}, options: [port: 0, ref: ref]},
      id: ref
    )

    Indexer.Helper.json_rpc_named_arguments("http://127.0.0.1:#{:ranch.get_port(ref)}/")
  end

  defp start_fetcher(rpc) do
    start_supervised!(
      {Fetcher,
       [[json_rpc_named_arguments: rpc, flush_interval: 60_000, max_concurrency: 1, max_batch_size: 1], [name: Fetcher]]}
    )
  end

  defp await_idle(fetcher, attempts \\ 500) do
    state = GenServer.call(fetcher, :state)

    if state.init_task == :complete and BufferedTask.debug_count(fetcher) == %{buffer: 0, tasks: 0} do
      :ok
    else
      assert attempts > 0, "fetcher did not complete startup: #{inspect(state)}"
      Process.sleep(10)
      await_idle(fetcher, attempts - 1)
    end
  end
end
