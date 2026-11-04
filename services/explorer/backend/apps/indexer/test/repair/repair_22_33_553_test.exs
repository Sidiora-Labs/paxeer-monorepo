Code.require_file("../indexer/block/realtime/paxeer_batch_case.exs", __DIR__)

defmodule Indexer.Repair2233553Test do
  use Explorer.DataCase, async: false
  use Indexer.Block.Realtime.PaxeerBatchCase

  alias Indexer.Block.Realtime.TestConfig

  @moduletag capture_log: true

  test "005: chain 125 batches survive a real fetcher restart without gaps or duplicate imports", context do
    assert_batch_recovery(context)
  end

  test "006: shared configured HTTP origin serves Paxeer blocks receipts balances and supported debug tracing",
       context do
    fetcher = TestConfig.block_fetcher(context.arguments)
    assert fetcher.json_rpc_named_arguments == context.arguments
    assert fetcher.json_rpc_named_arguments[:variant] == EthereumJSONRPC.PaxeerX
    assert {:ok, 125} = EthereumJSONRPC.fetch_chain_id(fetcher.json_rpc_named_arguments)

    assert {:ok, %{errors: [], inserted: %{blocks: blocks, transactions: [transaction]}}} =
             Indexer.Block.Fetcher.fetch_and_import_range(fetcher, context.first..(context.first + 4))

    assert length(blocks) == 5
    assert_imported(context.blocks)

    recorded = context.blocks |> Enum.flat_map(& &1["transactions"]) |> hd()

    params = [
      %{hash_data: recorded["hash"], block_number: transaction.block_number, transaction_index: transaction.index}
    ]

    assert {:ok, [trace]} = EthereumJSONRPC.fetch_internal_transactions(params, fetcher.json_rpc_named_arguments)
    assert trace.transaction_hash == recorded["hash"]
    assert trace.block_number == transaction.block_number
    assert trace.transaction_index == transaction.index
    assert trace.from_address_hash == recorded["from"]
    assert trace.to_address_hash == recorded["to"]
    assert trace.type == "call"
    assert trace.call_type == "call"
    assert trace.gas_used == Decimal.to_integer(transaction.gas_used)
    assert trace.input == recorded["input"]

    changeset = Explorer.Chain.InternalTransaction.blockless_changeset(%Explorer.Chain.InternalTransaction{}, trace)
    assert changeset.valid?, inspect(changeset.errors)
    decoded_trace = Ecto.Changeset.apply_changes(changeset)
    assert decoded_trace.transaction_hash == transaction.hash
    assert decoded_trace.from_address_hash == transaction.from_address_hash
    assert decoded_trace.to_address_hash == transaction.to_address_hash

    balance_exchange =
      Enum.find(context.fixture["exchanges"], fn exchange ->
        exchange["request"]["method"] == "eth_getBalance" and
          exchange["request"]["params"] == [recorded["from"], recorded["blockNumber"]]
      end)

    assert balance_exchange

    balance_request =
      EthereumJSONRPC.request(%{id: 91, method: "eth_getBalance", params: balance_exchange["request"]["params"]})

    assert {:ok, balance} = EthereumJSONRPC.json_rpc(balance_request, fetcher.json_rpc_named_arguments)
    assert balance == balance_exchange["response"]["result"]

    requests = context.node |> calls() |> Enum.flat_map(&List.wrap/1)
    assert Enum.any?(requests, &(&1["method"] == "eth_getTransactionReceipt"))

    assert Enum.any?(
             requests,
             &(&1["method"] == "debug_traceTransaction" and
                 &1["params"] == [recorded["hash"], %{"tracer" => "callTracer", "timeout" => "5s"}])
           )

    refute Enum.any?(requests, &String.starts_with?(&1["method"], "trace_"))
  end

  test "007: early registered cleanup restores absent nil and configured keys after an assertion failure during import",
       context do
    assert_configuration_restored(context, :absent)
  end

  test "007: cleanup restores explicitly nil keys after an assertion failure during import", context do
    assert_configuration_restored(context, nil)
  end

  test "007: cleanup restores nondefault settings after an assertion failure during import", context do
    assert_configuration_restored(context, :configured)
  end

  defp assert_configuration_restored(context, prior) do
    unrelated_key = :repair_22_33_553_unrelated
    TestConfig.preserve_env([{:indexer, unrelated_key}])

    for {app, key} <- TestConfig.keys() do
      case prior do
        :absent -> Application.delete_env(app, key)
        nil -> Application.put_env(app, key, nil)
        :configured -> Application.put_env(app, key, previous_value(key))
      end
    end

    expected = Map.new(TestConfig.keys(), fn {app, key} = pair -> {pair, Application.fetch_env(app, key)} end)
    owner = self()

    {pid, monitor} =
      spawn_monitor(fn ->
        :ok = ExUnit.OnExitHandler.register(self())
        :ok = TestConfig.preserve_env()
        Application.put_env(:indexer, :fetch_rewards_way, "manual")

        for {app, key} <- TestConfig.keys(), key != :fetch_rewards_way do
          value = if app == :explorer, do: [contracts: %{"addresses" => %{}}], else: [disabled?: true]
          Application.put_env(app, key, value)
        end

        Application.put_env(:indexer, unrelated_key, {:changed_during_test, prior})
        send(owner, {:overrides_ready, self()})

        receive do
          :fail -> flunk("deliberate assertion failure after the real import")
        end
      end)

    on_exit(fn ->
      if Process.alive?(pid), do: Process.exit(pid, :kill)
      if :ets.member(ExUnit.Server, pid), do: ExUnit.OnExitHandler.run(pid, 5_000)
    end)

    assert_receive {:overrides_ready, ^pid}, 5_000
    assert Application.get_env(:indexer, :fetch_rewards_way) == "manual"

    assert {:ok, %{errors: [], inserted: %{transactions: [_]}}} =
             Indexer.Block.Fetcher.fetch_and_import_range(context.block_fetcher, context.first..(context.first + 4))

    assert_imported(context.blocks)
    send(pid, :fail)
    assert_receive {:DOWN, ^monitor, :process, ^pid, {%ExUnit.AssertionError{}, _}}, 5_000
    assert :ok = ExUnit.OnExitHandler.run(pid, 5_000)

    for {{app, key}, value} <- expected do
      assert Application.fetch_env(app, key) == value
    end

    assert Application.fetch_env(:indexer, unrelated_key) == {:ok, {:changed_during_test, prior}}
  end

  defp previous_value(:fetch_rewards_way), do: "node"

  defp previous_value(Explorer.Chain.Cache.CeloCoreContracts),
    do: [
      contracts: %{
        "addresses" => %{
          "GoldToken" => [%{"address" => "0x0000000000000000000000000000000000001234", "updated_at_block_number" => 7}]
        }
      },
      cache_period: 123
    ]

  defp previous_value(_supervisor), do: [disabled?: false, retained_option: :preserve_me]
end
