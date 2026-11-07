Code.require_file("realtime/paxeer_batch_case.exs", __DIR__)

defmodule Indexer.Block.PaxeerImportCase do
  @moduledoc false
  use ExUnit.CaseTemplate

  import ExUnit.Assertions

  alias Explorer.Chain
  alias Explorer.Chain.{Address, Block, Log, Transaction, Wei}
  alias Explorer.Repo
  alias Indexer.Block.Fetcher
  alias Indexer.Block.Realtime.TestConfig
  alias Indexer.Fetcher.CoinBalance.Catchup, as: CoinBalanceCatchup
  alias Indexer.Fetcher.InternalTransaction

  using do
    quote do
      use Indexer.Block.Realtime.PaxeerBatchCase
      import Indexer.Block.PaxeerImportCase

      @moduletag json_rpc_variant: EthereumJSONRPC.Geth

      setup context do
        Indexer.Block.PaxeerImportCase.setup_import(context)
      end
    end
  end

  def setup_import(context) do
    TestConfig.preserve_env([
      {:indexer, :json_rpc_named_arguments},
      {:explorer, :json_rpc_named_arguments}
    ])

    arguments = context.arguments
    Application.put_env(:indexer, :json_rpc_named_arguments, arguments)
    Application.put_env(:explorer, :json_rpc_named_arguments, arguments)
    Application.put_env(:indexer, InternalTransaction.Supervisor, disabled?: false)

    Application.put_env(:ethereum_jsonrpc, EthereumJSONRPC.Geth,
      tracer: "call_tracer",
      debug_trace_timeout: "5s",
      block_traceable?: false
    )

    CoinBalanceCatchup.Supervisor.Case.start_supervised!(json_rpc_named_arguments: arguments, poll: false)
    InternalTransaction.Supervisor.Case.start_supervised!(json_rpc_named_arguments: arguments)

    %{
      arguments: arguments,
      block_fetcher: %Fetcher{
        broadcast: false,
        callback_module: Indexer.Block.Catchup.Fetcher,
        json_rpc_named_arguments: arguments,
        task_supervisor: Indexer.TaskSupervisor
      }
    }
  end

  def assert_single_block_import(context) do
    [recorded | _] = context.blocks
    assert recorded["transactions"] == []
    number = EthereumJSONRPC.quantity_to_integer(recorded["number"])

    assert {:ok, %{errors: [], inserted: %{blocks: [block], addresses: [address]} = inserted}} =
             fetch_and_await_import(context, number)

    assert Map.fetch(inserted, :transactions) == :error
    assert block.number == number
    assert to_string(block.hash) == recorded["hash"]
    assert Repo.aggregate(Block, :count) == 1
    assert Repo.aggregate(Transaction, :count) == 0
    assert Repo.aggregate(Address, :count) == 1
    assert to_string(address.hash) == recorded["miner"]
    assert_recorded_balances(context, recorded)
    assert_requested(context, "eth_getBlockByNumber", [recorded["number"], true])
  end

  def assert_full_schema_import(context) do
    assert context.fixture["capture"]["kind"] == "recorded"
    assert context.fixture["capture"]["chain_id"] == 125
    assert context.arguments[:transport] == EthereumJSONRPC.HTTP
    assert context.arguments[:variant] == EthereumJSONRPC.Geth
    assert context.block_fetcher.json_rpc_named_arguments == Application.fetch_env!(:indexer, :json_rpc_named_arguments)
    assert {:ok, 125} = EthereumJSONRPC.fetch_chain_id(context.arguments)

    recorded = Enum.find(context.blocks, &(&1["transactions"] != []))
    assert recorded
    [recorded_transaction] = recorded["transactions"]
    number = EthereumJSONRPC.quantity_to_integer(recorded["number"])
    receipt = recorded_result(context, "eth_getTransactionReceipt", [recorded_transaction["hash"]])

    assert {:ok, %{errors: [], inserted: %{blocks: [block], transactions: [transaction]} = inserted}} =
             fetch_and_await_import(context, number)

    assert receipt["logs"] == []
    assert Map.fetch(inserted, :logs) == :error
    logs = Repo.all(Log)
    persisted_block = Repo.get!(Block, block.hash)
    assert persisted_block.consensus
    assert persisted_block.number == number
    assert to_string(persisted_block.hash) == recorded["hash"]
    assert to_string(persisted_block.parent_hash) == recorded["parentHash"]
    assert persisted_block.gas_used == decimal_quantity(recorded["gasUsed"])
    assert persisted_block.base_fee_per_gas == %Wei{value: decimal_quantity(recorded["baseFeePerGas"])}
    assert Repo.aggregate(Block, :count) == 1
    assert Repo.aggregate(Transaction, :count) == 1

    persisted = Repo.get!(Transaction, transaction.hash)
    assert to_string(persisted.hash) == recorded_transaction["hash"]
    assert persisted.block_hash == block.hash
    assert persisted.block_number == number
    assert persisted.index == EthereumJSONRPC.quantity_to_integer(receipt["transactionIndex"])
    assert persisted.status == :ok
    assert persisted.gas_used == decimal_quantity(receipt["gasUsed"])
    assert persisted.cumulative_gas_used == decimal_quantity(receipt["cumulativeGasUsed"])
    assert to_string(persisted.from_address_hash) == recorded_transaction["from"]
    assert to_string(persisted.to_address_hash) == recorded_transaction["to"]
    assert to_string(persisted.input) == recorded_transaction["input"]
    assert length(logs) == length(receipt["logs"])
    assert Repo.aggregate(Log, :count) == length(receipt["logs"])

    assert_recorded_balances(context, recorded)
    assert Repo.aggregate(Chain.InternalTransaction, :count) == 1
    [trace] = Repo.all(Chain.InternalTransaction)
    assert trace.transaction_hash == transaction.hash
    assert trace.block_number == number
    assert trace.from_address_hash == transaction.from_address_hash
    assert trace.to_address_hash == transaction.to_address_hash
    assert trace.type == :call
    assert trace.call_type_enum == :call
    assert Chain.InternalTransaction.call_type(trace) == :call
    assert trace.input == transaction.input
    assert trace.gas_used == persisted.gas_used

    assert_requested(context, "eth_getBlockByNumber", [recorded["number"], true])
    assert_requested(context, "eth_getTransactionReceipt", [recorded_transaction["hash"]])

    assert_requested(context, "debug_traceTransaction", [
      recorded_transaction["hash"],
      %{"tracer" => "callTracer", "timeout" => "5s"}
    ])

    :ok
  end

  defp assert_recorded_balances(context, block) do
    balances =
      for %{
            "request" => %{"method" => "eth_getBalance", "params" => [address, number]},
            "response" => %{"result" => value}
          } <- context.fixture["exchanges"],
          number == block["number"],
          do: {address, value}

    assert balances != []
    assert Repo.aggregate(Address, :count) == length(balances)
    number = EthereumJSONRPC.quantity_to_integer(block["number"])

    for {address, value} <- balances do
      {:ok, hash} = Chain.string_to_address_hash(address)

      address_record = Repo.get!(Address, hash)
      assert address_record.fetched_coin_balance == %Wei{value: decimal_quantity(value)}
      assert address_record.fetched_coin_balance_block_number == number

      balance = Repo.get_by!(Chain.Address.CoinBalance, address_hash: hash, block_number: number)
      assert balance.value == %Wei{value: decimal_quantity(value)}
      assert balance.value_fetched_at

      day = block["timestamp"] |> EthereumJSONRPC.quantity_to_integer() |> DateTime.from_unix!() |> DateTime.to_date()
      daily = Repo.get_by!(Chain.Address.CoinBalanceDaily, address_hash: hash, day: day)
      assert daily.value == %Wei{value: decimal_quantity(value)}
      assert_requested(context, "eth_getBalance", [address, block["number"]])
    end

    requests = context.node |> Indexer.Block.Realtime.PaxeerBatchCase.calls() |> Enum.flat_map(&List.wrap/1)

    assert Enum.count(requests, &(&1["method"] == "eth_getBlockByNumber" and &1["params"] == [block["number"], true])) >
             1
  end

  defp recorded_result(context, method, params) do
    exchange =
      Enum.find(
        context.fixture["exchanges"],
        &(&1["request"]["method"] == method and &1["request"]["params"] == params)
      )

    assert exchange
    exchange["response"]["result"]
  end

  defp assert_requested(context, method, params) do
    requests = context.node |> Indexer.Block.Realtime.PaxeerBatchCase.calls() |> Enum.flat_map(&List.wrap/1)
    assert Enum.any?(requests, &(&1["method"] == method and &1["params"] == params))
  end

  defp decimal_quantity(value), do: value |> EthereumJSONRPC.quantity_to_integer() |> Decimal.new()

  defp fetch_and_await_import(context, number) do
    assert Application.fetch_env!(:explorer, :json_rpc_named_arguments) == context.arguments
    assert Application.fetch_env!(:indexer, :json_rpc_named_arguments) == context.arguments

    workers =
      Enum.map(
        [InternalTransaction, CoinBalanceCatchup, Indexer.Fetcher.ContractCode, Indexer.Fetcher.ReplacedTransaction],
        &Process.whereis/1
      )

    for worker <- workers do
      assert is_pid(worker)
      assert :erlang.trace(worker, true, [:receive, {:tracer, self()}]) == 1
    end

    for worker <- [InternalTransaction, CoinBalanceCatchup, Indexer.Fetcher.ContractCode] do
      assert :sys.get_state(worker).callback_module_state == context.arguments
    end

    try do
      result = Fetcher.fetch_and_import_range(context.block_fetcher, number..number)
      await_import_workers(workers, System.monotonic_time(:millisecond) + 5_000)
      result
    after
      for worker <- workers, Process.alive?(worker), do: :erlang.trace(worker, false, [:receive])
    end
  end

  defp await_import_workers(workers, deadline) do
    idle? =
      Enum.all?(workers, fn worker ->
        counts = Indexer.BufferedTask.debug_count(worker)
        state = :sys.get_state(worker)
        counts.buffer == 0 and counts.tasks == 0 and state.init_task in [nil, :complete, :delay]
      end)

    if idle? do
      assert Task.Supervisor.children(Indexer.TaskSupervisor) == []
    else
      remaining = max(deadline - System.monotonic_time(:millisecond), 0)

      receive do
        {:trace, worker, :receive, {_ref, {:ok, task}}} when is_pid(task) ->
          assert worker in workers
          await_import_workers(workers, deadline)

        {:trace, worker, :receive, {:DOWN, ref, :process, task, reason}}
        when is_reference(ref) and is_pid(task) ->
          assert worker in workers
          assert reason == :normal
          await_import_workers(workers, deadline)

        {:trace, worker, :receive, {ref, result}} when is_reference(ref) ->
          assert worker in workers
          assert result == :ok
          await_import_workers(workers, deadline)
      after
        remaining -> flunk("recorded import did not complete within 5000ms")
      end
    end
  end
end
