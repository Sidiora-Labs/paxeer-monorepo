defmodule Explorer.Migrator.PendingOperationsRestart do
  import Ecto.Query
  import ExUnit.Assertions
  import ExUnit.Callbacks

  alias EthereumJSONRPC.{Block, Transaction}
  alias Explorer.Chain
  alias Explorer.Chain.{PendingBlockOperation, PendingOperationsHelper, PendingTransactionOperation}
  alias Explorer.Migrator.SwitchPendingOperations
  alias Explorer.Repo

  @recording Path.join(__DIR__, "selfdestruct_chain_data/geth_history.json")

  def setup do
    original_repo = Repo.get_dynamic_repo()
    schema = "repair_2233537_#{System.unique_integer([:positive])}"

    Ecto.Adapters.SQL.Sandbox.unboxed_run(Repo, fn ->
      Repo.query!("CREATE SCHEMA #{schema}")

      for table <- ~w(blocks transactions pending_block_operations pending_transaction_operations) do
        Repo.query!("CREATE TABLE #{schema}.#{table} (LIKE public.#{table} INCLUDING ALL)")
      end
    end)

    on_exit(fn ->
      Repo.put_dynamic_repo(original_repo)

      Ecto.Adapters.SQL.Sandbox.unboxed_run(Repo, fn ->
        Repo.query!("DROP SCHEMA #{schema} CASCADE")
      end)
    end)

    json_rpc = Application.fetch_env!(:explorer, :json_rpc_named_arguments)
    geth = Application.fetch_env!(:ethereum_jsonrpc, EthereumJSONRPC.Geth)
    Application.put_env(:explorer, :json_rpc_named_arguments, Keyword.put(json_rpc, :variant, EthereumJSONRPC.Geth))
    Application.put_env(:ethereum_jsonrpc, EthereumJSONRPC.Geth, Keyword.put(geth, :block_traceable?, false))

    on_exit(fn ->
      Application.put_env(:explorer, :json_rpc_named_arguments, json_rpc)
      Application.put_env(:ethereum_jsonrpc, EthereumJSONRPC.Geth, geth)
    end)

    repo = open_repo(schema)
    records = @recording |> File.read!() |> Jason.decode!() |> Map.fetch!("records")

    blocks =
      records
      |> Enum.map(& &1["block"]["response"]["result"])
      |> Enum.uniq_by(& &1["hash"])
      |> Enum.map(fn block ->
        params = block |> Block.to_elixir() |> Block.elixir_to_params() |> Map.put(:consensus, true)
        changeset = Chain.Block.changeset(%Chain.Block{}, params)
        assert changeset.valid?, inspect(changeset.errors)
        Repo.insert!(changeset)
      end)

    transactions =
      Enum.map(records, fn record ->
        transaction = record["transaction"]["response"]["result"]
        receipt = record["receipt"]["response"]["result"]
        assert record["block"]["request"]["method"] == "eth_getBlockByNumber"
        assert record["transaction"]["request"]["method"] == "eth_getTransactionByHash"
        assert record["transaction"]["request"]["params"] == [transaction["hash"]]
        assert record["receipt"]["request"]["method"] == "eth_getTransactionReceipt"
        assert transaction["blockHash"] == record["block"]["response"]["result"]["hash"]
        assert receipt["transactionHash"] == transaction["hash"]
        assert receipt["status"] == "0x1"

        params =
          transaction
          |> Transaction.to_elixir()
          |> Transaction.elixir_to_params()
          |> Map.merge(%{
            status: :ok,
            gas_used: EthereumJSONRPC.quantity_to_integer(receipt["gasUsed"]),
            cumulative_gas_used: EthereumJSONRPC.quantity_to_integer(receipt["cumulativeGasUsed"])
          })

        changeset = Chain.Transaction.changeset(%Chain.Transaction{}, params)
        assert changeset.valid?, inspect(changeset.errors)
        Repo.insert!(changeset)
      end)

    assert length(blocks) == 12
    assert length(transactions) == 13

    Enum.each(blocks, &enqueue_block/1)

    # Repeat delivery can leave both representations present before a restart.
    for transaction <- [hd(transactions)] do
      %PendingTransactionOperation{}
      |> PendingTransactionOperation.changeset(%{transaction_hash: transaction.hash})
      |> Repo.insert!()
    end

    %{repo: repo, schema: schema, blocks: blocks, transactions: transactions}
  end

  def assert_partial_batch_recovers(context) do
    reference = make_ref()
    event = (Repo.config()[:telemetry_prefix] || [:explorer, :repo]) ++ [:query]

    :ok = :telemetry.attach(reference, event, &__MODULE__.observe_commit/4, {self(), reference, context.repo})
    on_exit(fn -> :telemetry.detach(reference) end)
    {pid, monitor} = start_migrator(context.repo)

    assert_receive {:batch_committed, ^reference, ^pid}, 10_000
    remaining = Repo.all(PendingBlockOperation)
    assert length(remaining) == 2
    remaining_numbers = Enum.map(remaining, & &1.block_number)
    converted = Enum.reject(context.transactions, &(&1.block_number in remaining_numbers))
    assert length(converted) >= 10
    pending = Repo.all(PendingTransactionOperation)

    expected_partial =
      [hd(context.transactions).hash | Enum.map(converted, & &1.hash)] |> Enum.uniq() |> Enum.sort()

    assert pending_hashes() == expected_partial
    assert pending_hashes() != expected_hashes(context)
    assert_represented_once(context)
    assert Process.alive?(pid)

    Process.exit(pid, :kill)
    assert_receive {:DOWN, ^monitor, :process, ^pid, :killed}, 10_000
    :ok = :telemetry.detach(reference)

    # Closing every connection proves these rows survived a real COMMIT rather
    # than a savepoint in the ordinary test sandbox transaction.
    stop_supervised!(Repo)
    repo = open_repo(context.schema)
    assert Repo.all(PendingBlockOperation) |> Enum.sort() == Enum.sort(remaining)
    assert Repo.all(PendingTransactionOperation) |> Enum.sort() == Enum.sort(pending)
    assert_represented_once(context)

    assert_complete(context, repo)
    durable = Repo.all(PendingTransactionOperation) |> Enum.sort()
    assert_complete(context, repo)
    assert Repo.all(PendingTransactionOperation) |> Enum.sort() == durable

    # Replay the original source after completion as well as restarting an
    # empty source: on_conflict must preserve the existing durable work.
    Enum.each(context.blocks, &enqueue_block/1)
    assert_complete(context, repo)
    assert Repo.all(PendingTransactionOperation) |> Enum.sort() == durable
  end

  def assert_uninterrupted_conversion(context) do
    assert_complete(context, context.repo)
    durable = Repo.all(PendingTransactionOperation) |> Enum.sort()
    assert_complete(context, context.repo)
    assert Repo.all(PendingTransactionOperation) |> Enum.sort() == durable
  end

  def observe_commit(_event, _measurements, metadata, {owner, reference, repo}) do
    if Repo.get_dynamic_repo() == repo and metadata.query == "commit" and
         match?({:ok, _}, metadata.result) and not Process.get(reference, false) do
      Process.put(reference, true)
      send(owner, {:batch_committed, reference, self()})

      receive do
        {:resume, ^reference} -> :ok
      end
    end
  end

  defp open_repo(schema) do
    repo = start_supervised!({Repo, name: nil, parameters: [search_path: schema]})
    Ecto.Adapters.SQL.Sandbox.mode(repo, :auto)
    Repo.put_dynamic_repo(repo)
    repo
  end

  defp enqueue_block(block) do
    %PendingBlockOperation{}
    |> PendingBlockOperation.changeset(%{block_hash: block.hash, block_number: block.number})
    |> Repo.insert!()
  end

  defp start_migrator(repo) do
    owner = self()

    pid =
      :proc_lib.spawn(fn ->
        Repo.put_dynamic_repo(repo)
        send(owner, {:migrator_ready, self()})

        receive do
          :start ->
            {:ok, state, continuation} = SwitchPendingOperations.init(:ok)
            :gen_server.enter_loop(SwitchPendingOperations, [], state, continuation)
        end
      end)

    on_exit(fn -> if Process.alive?(pid), do: Process.exit(pid, :kill) end)
    monitor = Process.monitor(pid)
    assert_receive {:migrator_ready, ^pid}, 10_000
    send(pid, :start)
    {pid, monitor}
  end

  defp assert_complete(context, repo) do
    {pid, monitor} = start_migrator(repo)
    assert_receive {:DOWN, ^monitor, :process, ^pid, :normal}, 10_000
    assert PendingOperationsHelper.pending_operations_type() == "transactions"
    assert PendingOperationsHelper.actual_entity() == PendingTransactionOperation
    assert Repo.all(PendingBlockOperation) == []
    assert pending_hashes() == expected_hashes(context)
    assert Repo.aggregate(PendingTransactionOperation, :count) == length(context.transactions)

    assert {:ok, work} =
             PendingTransactionOperation.stream_transactions_with_unfetched_internal_transactions(
               [],
               fn transaction, acc -> [transaction | acc] end
             )

    assert Enum.sort_by(work, & &1.hash) ==
             context.transactions
             |> Enum.map(&Map.take(&1, [:block_number, :hash, :index, :type]))
             |> Enum.sort_by(& &1.hash)
  end

  defp assert_represented_once(context) do
    remaining = Repo.all(PendingBlockOperation) |> MapSet.new(& &1.block_number)

    represented =
      context.transactions
      |> Enum.filter(&MapSet.member?(remaining, &1.block_number))
      |> Enum.map(& &1.hash)
      |> Kernel.++(pending_hashes())
      |> MapSet.new()

    assert represented == MapSet.new(expected_hashes(context))
    hashes = pending_hashes()
    assert length(hashes) == MapSet.size(MapSet.new(hashes))
  end

  defp pending_hashes do
    PendingTransactionOperation |> select([operation], operation.transaction_hash) |> Repo.all() |> Enum.sort()
  end

  defp expected_hashes(context), do: context.transactions |> Enum.map(& &1.hash) |> Enum.sort()
end
