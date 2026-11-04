defmodule Indexer.Repair2233473Test do
  use Explorer.DataCase, async: false

  alias Explorer.Chain.Cache.Counters.Shibarium.DepositsAndWithdrawalsCount, as: Counters
  alias Explorer.Chain.Shibarium.{Bridge, Reader}
  alias Indexer.Fetcher.RollupL1ReorgMonitor
  alias Indexer.Fetcher.Shibarium.{L1, L2}

  @moduletag :capture_log
  @empty_hash "0x0000000000000000000000000000000000000000000000000000000000000000"
  @user "0x9a4a90e2732f3fa4087b0bb4bf85c76d14833df1"
  @predicate "0x867305d19606aadba405ce534e303d0e225f9556"
  @weth "0x7301cfa0e1756b71869e93d4e4dca5c7d0eb0aa6"
  @locked_ether "0x3e799b2d61372379e767ef8f04d65089179b7a6f63f9be3065806456c7309f1b"
  @exited_ether "0x0fc0eed41f72d3da77d0f53b9594fc7073acd15ee9d7c536819a70a67c57ef3c"
  @transfer "0xddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef"

  setup_all do
    migration =
      Path.expand("../../../explorer/priv/shibarium/migrations/20231024091228_add_bridge_table.exs", __DIR__)

    Code.require_file(migration)

    Ecto.Adapters.SQL.Sandbox.unboxed_run(Repo, fn ->
      Ecto.Migrator.up(Repo, 20_231_024_091_228, Explorer.Repo.Shibarium.Migrations.AddBridgeTable, log: false)
    end)

    :ok
  end

  setup do
    configs = [
      {:explorer, :chain_type},
      {:indexer, L1},
      {:indexer, L2},
      {:indexer, Indexer.Block.Realtime.Fetcher},
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

    Application.put_env(:tesla, :adapter, Tesla.Adapter.Mint)
    Application.put_env(:explorer, :chain_type, :shibarium)
    start_supervised!(Explorer.Chain.Cache.LatestL1BlockNumber.child_spec())
    Application.put_env(:indexer, Indexer.Block.Realtime.Fetcher, polling_period: 3_750)

    bypass = Bypass.open()
    url = "http://localhost:#{bypass.port}"
    rpc = Indexer.Helper.json_rpc_named_arguments(url)
    templates = recorded_templates()

    store =
      start_supervised!({Agent, fn -> %{head: 10, l1: [], l2: [], replay: nil, requests: []} end})

    Bypass.expect(bypass, "POST", "/", fn conn ->
      {:ok, body, conn} = Plug.Conn.read_body(conn)
      request = Jason.decode!(body)

      response =
        Agent.get_and_update(store, fn state ->
          response =
            if is_list(request) do
              Enum.map(request, &rpc_response(&1, state, templates))
            else
              rpc_response(request, state, templates)
            end

          {response, %{state | requests: state.requests ++ List.wrap(request)}}
        end)

      conn
      |> Plug.Conn.put_resp_content_type("application/json")
      |> Plug.Conn.resp(200, Jason.encode!(response))
    end)

    Application.put_env(:indexer, L1,
      start_block: 1,
      rpc: url,
      deposit_manager_proxy: @predicate,
      ether_predicate_proxy: @predicate,
      erc20_predicate_proxy: @predicate,
      withdraw_manager_proxy: @predicate
    )

    Application.put_env(:indexer, L2,
      start_block: 1,
      child_chain: @predicate,
      weth: @weth,
      bone_withdraw: @predicate
    )

    %{store: store, rpc: rpc, templates: templates}
  end

  test "shibarium-genesis-division: heads 0 and 1 use configured polling before head 2 estimates timestamps", %{
    store: store,
    rpc: rpc
  } do
    for head <- [0, 1, 2] do
      Agent.update(store, &%{&1 | head: head, requests: []})
      expected = if head < 2, do: 3_750, else: 30_000
      assert L1.get_block_check_interval(rpc) == {:ok, expected, head}

      requests = Agent.get(store, & &1.requests)
      samples = for %{"params" => [tag, false]} <- requests, tag != "latest", do: tag
      assert samples == if(head < 2, do: [], else: ["0x1", "0x2"])
      assert Enum.all?(requests, &(&1["method"] == "eth_getBlockByNumber"))
    end

    Application.put_env(:indexer, Indexer.Block.Realtime.Fetcher, polling_period: nil)
    Agent.update(store, &%{&1 | head: 0, requests: []})
    assert L1.get_block_check_interval(rpc) == {:ok, 2_000, 0}
    assert length(Agent.get(store, & &1.requests)) == 1
  end

  test "shibarium-l1-reorg-sentinel: rollback survives L1 restart and binds replacement deposits and withdrawals",
       context do
    prove_restart_replay(L1, context)
  end

  test "shibarium-l2-reorg-sentinel: rollback survives L2 restart and binds replacement deposits and withdrawals",
       context do
    prove_restart_replay(L2, context)
  end

  test "both rollbacks restore deletions and timestamp changes if clearing a matched primary key fails", context do
    for module <- [L1, L2] do
      seed_operations(module, context)
      rows = bridge_rows()
      row = rows[101]
      side = if module == L1, do: :l1_transaction_hash, else: :l2_transaction_hash

      row
      |> Map.from_struct()
      |> Map.put(side, hash(999))
      |> then(&Bridge.changeset(%Bridge{}, &1))
      |> Repo.insert!()

      before = bridge_rows_by_key()
      counters = {Counters.deposits_count(), Counters.withdrawals_count()}

      assert_raise Postgrex.Error, ~r/unique/, fn -> module.reorg_handle(10) end

      assert bridge_rows_by_key() == before
      assert {Counters.deposits_count(), Counters.withdrawals_count()} == counters
      Repo.delete_all(Bridge)
      stop_supervised!(RollupL1ReorgMonitor)
    end
  end

  defp prove_restart_replay(module, context) do
    seed_operations(module, context)
    original = bridge_rows()
    assert map_size(original) == 5
    assert_complete_counts(2, 1)

    module.reorg_handle(10)
    rolled_back = bridge_rows()
    assert Map.keys(rolled_back) |> Enum.sort() == [101, 102, 105]
    assert rolled_back[105] == original[105]
    assert_complete_counts(1, 0)

    {side_hash, side_block, other_hash, timestamp_owner} =
      if module == L1 do
        {:l1_transaction_hash, :l1_block_number, :l2_transaction_hash, 101}
      else
        {:l2_transaction_hash, :l2_block_number, :l1_transaction_hash, 102}
      end

    for amount <- [101, 102] do
      assert to_string(Map.fetch!(rolled_back[amount], side_hash)) == @empty_hash
      assert is_nil(Map.fetch!(rolled_back[amount], side_block))
      assert Map.fetch!(rolled_back[amount], other_hash) == Map.fetch!(original[amount], other_hash)

      if amount == timestamp_owner do
        assert is_nil(rolled_back[amount].timestamp)
      else
        assert rolled_back[amount].timestamp == original[amount].timestamp
      end
    end

    # The component that imported the original branch has stopped. Its new process
    # must recover the surviving height and transaction from the production schema.
    Agent.update(context.store, &%{&1 | replay: module, requests: []})
    run_fetcher(module, context.rpc)
    recovered = bridge_rows()
    assert map_size(recovered) == 5
    assert recovered[105] == original[105]
    assert_complete_counts(2, 1)

    for amount <- [101, 102, 103, 104] do
      assert Map.fetch!(recovered[amount], side_block) == 10
      assert to_string(Map.fetch!(recovered[amount], side_hash)) == hash(tx_id(module, amount) + 1_000)

      if amount in [101, 102] do
        assert Map.fetch!(recovered[amount], other_hash) == Map.fetch!(original[amount], other_hash)
        refute is_nil(recovered[amount].timestamp)
      else
        assert to_string(Map.fetch!(recovered[amount], other_hash)) == @empty_hash
      end
    end

    requests = Agent.get(context.store, & &1.requests)
    stable_hash = to_string(Map.fetch!(original[105], side_hash))
    assert Enum.any?(requests, &(&1["method"] == "eth_getTransactionByHash" && &1["params"] == [stable_hash]))

    if module == L1 do
      assert Enum.any?(requests, fn request ->
               request["method"] == "eth_getLogs" && hd(request["params"])["fromBlock"] == "0x1"
             end)
    else
      assert Enum.any?(requests, &(&1["method"] == "eth_getBlockByNumber" && &1["params"] == ["0x1", true]))
    end

    # Reopening and replaying the same committed branch is idempotent.
    run_fetcher(module, context.rpc)
    assert bridge_rows() == recovered
    assert_complete_counts(2, 1)
  end

  defp seed_operations(module, context) do
    Agent.update(context.store, fn state ->
      if module == L1 do
        %{state | l1: [101, 102, 103, 104, 105], l2: [101, 102, 105], replay: nil}
      else
        %{state | l1: [101, 102, 105], l2: [101, 102, 103, 104, 105], replay: nil}
      end
    end)

    start_supervised!({RollupL1ReorgMonitor, [[], []]})
    run_fetcher(module, context.rpc)
    run_fetcher(if(module == L1, do: L2, else: L1), context.rpc)
  end

  defp run_fetcher(L1, _rpc) do
    pid = start_supervised!({L1, [[], []]})
    await_l1_scan(pid, System.monotonic_time(:millisecond) + 10_000)
    assert :ok = stop_supervised(L1)
    refute Process.alive?(pid)
  end

  defp run_fetcher(L2, rpc) do
    pid = start_supervised!({L2, [[json_rpc_named_arguments: rpc], []]})
    ref = Process.monitor(pid)
    assert_receive {:DOWN, ^ref, :process, ^pid, :normal}, 10_000
    refute Process.alive?(pid)
    {:ok, supervisor} = ExUnit.fetch_test_supervisor()
    assert :ok = Supervisor.delete_child(supervisor, L2)
  end

  defp await_l1_scan(pid, deadline) do
    state = :sys.get_state(pid)

    if state[:start_block] == 11 do
      :ok
    else
      assert System.monotonic_time(:millisecond) < deadline, inspect(state)
      Process.sleep(20)
      await_l1_scan(pid, deadline)
    end
  end

  defp bridge_rows do
    Map.new(Repo.all(Bridge), &{Decimal.to_integer(&1.amount_or_id), &1})
  end

  defp bridge_rows_by_key do
    Map.new(Repo.all(Bridge), &{{&1.operation_hash, &1.l1_transaction_hash, &1.l2_transaction_hash}, &1})
  end

  defp assert_complete_counts(deposits, withdrawals) do
    assert Reader.deposits_count() == deposits
    assert Reader.withdrawals_count() == withdrawals
    assert Decimal.equal?(Counters.deposits_count(), Decimal.new(deposits))
    assert Decimal.equal?(Counters.withdrawals_count(), Decimal.new(withdrawals))
  end

  defp rpc_response(request, state, templates) do
    %{"jsonrpc" => "2.0", "id" => request["id"], "result" => rpc_result(request, state, templates)}
  end

  defp rpc_result(%{"method" => "eth_getBlockByNumber", "params" => [tag, hydrated]}, state, templates) do
    number = if tag in ["latest", "safe"], do: state.head, else: EthereumJSONRPC.quantity_to_integer(tag)

    if number > state.head do
      nil
    else
      transactions =
        for amount <- state.l2, height(amount) == number do
          log = bridge_log(L2, amount, state, templates)

          Map.merge(templates.transaction, %{
            "hash" => log["transactionHash"],
            "from" => if(withdrawal?(amount), do: @user, else: Explorer.Chain.SmartContract.burn_address_hash_string()),
            "input" => if(withdrawal?(amount), do: "0x2e1a7d4d" <> word(amount), else: "0x")
          })
        end

      Map.merge(templates.block, %{
        "number" => EthereumJSONRPC.integer_to_quantity(number),
        "timestamp" => EthereumJSONRPC.integer_to_quantity(timestamp(number, state)),
        "transactions" => if(hydrated, do: transactions, else: Enum.map(transactions, & &1["hash"]))
      })
    end
  end

  defp rpc_result(%{"method" => "eth_getLogs", "params" => [filter]}, state, templates) do
    first = EthereumJSONRPC.quantity_to_integer(filter["fromBlock"])
    last = EthereumJSONRPC.quantity_to_integer(filter["toBlock"])

    # The second query selects unknown-token transfers. This transcript contains
    # the predicate's LockedEther/ExitedEther event shapes instead.
    if is_list(filter["address"]) do
      for amount <- state.l1, height(amount) >= first, height(amount) <= last do
        bridge_log(L1, amount, state, templates)
      end
    else
      []
    end
  end

  defp rpc_result(%{"method" => "eth_getTransactionReceipt", "params" => [transaction_hash]}, state, templates) do
    amount =
      Enum.find(state.l2, fn amount ->
        bridge_log(L2, amount, state, templates)["transactionHash"] == transaction_hash
      end)

    log = bridge_log(L2, amount, state, templates)
    Map.merge(templates.receipt, %{"transactionHash" => transaction_hash, "logs" => [log]})
  end

  defp rpc_result(%{"method" => "eth_getTransactionByHash", "params" => [transaction_hash]}, _state, templates) do
    Map.put(templates.transaction, "hash", transaction_hash)
  end

  defp bridge_log(module, amount, state, templates) do
    topics =
      case {module, withdrawal?(amount)} do
        {L1, false} -> [@locked_ether, address_topic(@user), address_topic(@user)]
        {L1, true} -> [@exited_ether, address_topic(@user)]
        {L2, false} -> [@transfer, @empty_hash, address_topic(@user)]
        {L2, true} -> [@transfer, address_topic(@user), @empty_hash]
      end

    replacement = if state.replay == module and height(amount) == 10, do: 1_000, else: 0

    Map.merge(templates.log, %{
      "address" => if(module == L1, do: @predicate, else: @weth),
      "topics" => topics,
      "data" => "0x" <> word(amount),
      "blockNumber" => EthereumJSONRPC.integer_to_quantity(height(amount)),
      "transactionHash" => hash(tx_id(module, amount) + replacement)
    })
  end

  defp recorded_templates do
    block = recorded_response("block_importer_import_1_pending.json")
    receipt = recorded_response("import_receipt_perform_1.json")
    %{block: block, transaction: hd(block["transactions"]), receipt: receipt, log: hd(receipt["logs"])}
  end

  defp recorded_response(name) do
    path = Path.expand("../../../explorer/test/support/fixture/vcr_cassettes/#{name}", __DIR__)
    [record] = path |> File.read!() |> Jason.decode!()
    record["response"]["body"] |> Jason.decode!() |> Map.fetch!("result")
  end

  defp height(105), do: 1
  defp height(_amount), do: 10
  defp withdrawal?(amount), do: amount in [102, 104]
  defp tx_id(L1, amount), do: amount
  defp tx_id(L2, amount), do: 200 + amount
  defp timestamp(number, state), do: 1_500_000_000 + number * 60 + if(state.replay && number == 10, do: 5, else: 0)
  defp address_topic("0x" <> address), do: "0x" <> String.pad_leading(address, 64, "0")
  defp hash(number), do: "0x" <> word(number)
  defp word(number), do: number |> Integer.to_string(16) |> String.downcase() |> String.pad_leading(64, "0")
end
