defmodule Explorer.Repair.Repair2233494Test do
  @moduledoc """
  Task 22.33.494 / audit 22.30.151 repair record:
  unbounded-address-gas -> "unbounded-address-gas: recorded gas survives eviction and expiry"
  unbounded-address-transactions -> "unbounded-address-transactions: recorded transactions are bounded and refresh atomically"
  unbounded-address-token-transfers -> "unbounded-address-token-transfers: recorded transfers survive eviction"
  unbounded-address-usd -> "unbounded-address-usd: schema balances retain a bounded working set"
  unbounded-address-tab-state -> "unbounded-address-tab-state: real producers release progress and cached counters"
  unbounded-address-tab-state -> "unbounded-address-tab-state: terminal progress drains without losing in-flight coordination"
  average-block-period-drift -> "average-block-period-drift: two scheduled refreshes use cache_period"
  unbounded-block-burnt-fee -> "unbounded-block-burnt-fee: history browsing evicts and recomputes recorded gas"
  unbounded-block-priority-fee -> "unbounded-block-priority-fee: history browsing evicts and recomputes recorded fees"

  Wiring request: explorer configuration domain (14), services/explorer/backend/config/runtime.exs:
  remove the unused AverageBlockTime :period setting; :cache_period is authoritative at startup and on every refresh.

  Baseline: no scoped changes or gate ledger existed. The explicit implementation-first
  gate order prohibits the older step's unchanged-tree verify, so no baseline test result
  is claimed. hf-build and hf-done retain the verified revision and private evidence.
  """

  use Explorer.DataCase, async: false

  alias Explorer.Chain.{Address, Block, Token, TokenTransfer, Transaction}
  alias Explorer.Chain.Address.{Counters, CurrentTokenBalance}

  alias Explorer.Chain.Cache.Counters.{
    AddressTabsElementsCount,
    AddressTokenTransfersCount,
    AddressTokensUsdSum,
    AddressTransactionsCount,
    AddressTransactionsGasUsageSum,
    AverageBlockTime,
    BlockBurntFeeCount,
    BlockPriorityFeeCount
  }

  @recording Path.expand("../../../ethereum_jsonrpc/test/support/fixture/geth/trace/struct_logger.json", __DIR__)
  @limit 4
  @caches [
    AddressTransactionsGasUsageSum,
    AddressTransactionsCount,
    AddressTokenTransfersCount,
    AddressTokensUsdSum,
    BlockBurntFeeCount,
    BlockPriorityFeeCount
  ]

  setup do
    for module <- @caches ++ [AddressTabsElementsCount, AverageBlockTime] do
      config = Application.get_env(:explorer, module)

      Application.put_env(
        :explorer,
        module,
        Keyword.merge(config || [], max_entries: @limit, cache_period: 60_000, ttl: 60_000)
      )

      on_exit(fn ->
        if config, do: Application.put_env(:explorer, module, config), else: Application.delete_env(:explorer, module)
      end)
    end

    for module <- @caches, do: start_counter(module)
    :ets.delete_all_objects(:addresses_tabs_counters)
    :ets.delete_all_objects(:addresses_tabs_counter_tasks)
    :sys.replace_state(AddressTabsElementsCount, fn _ -> %{} end)

    recording = @recording |> File.read!() |> Jason.decode!()
    tx = recording["tx"] |> EthereumJSONRPC.Transaction.to_elixir() |> EthereumJSONRPC.Transaction.elixir_to_params()
    receipt = recording["receipt"] |> EthereumJSONRPC.Receipt.to_elixir() |> EthereumJSONRPC.Receipt.elixir_to_params()
    params = Map.merge(tx, Map.take(receipt, [:gas_used, :cumulative_gas_used, :status]))

    block =
      insert(:block,
        hash: params.block_hash,
        number: params.block_number,
        base_fee_per_gas: 13,
        timestamp: ~U[2024-01-01 00:00:00Z]
      )

    for hash <- [params.from_address_hash, params.to_address_hash], do: store_address(hash)
    transaction = store_transaction(params, block)
    address = Repo.get!(Address, transaction.from_address_hash)
    log = hd(recording["receipt"]["logs"]) |> EthereumJSONRPC.Log.to_elixir() |> EthereumJSONRPC.Log.elixir_to_params()
    from = topic_address(log.second_topic)
    to = topic_address(log.third_topic)
    for hash <- [from, to, log.address_hash], do: store_address(hash)

    transfer_params = %{
      block_hash: block.hash,
      block_number: block.number,
      transaction_hash: transaction.hash,
      from_address_hash: from,
      to_address_hash: to,
      token_contract_address_hash: log.address_hash,
      token_type: "ERC-20",
      log_index: log.index,
      amount: EthereumJSONRPC.quantity_to_integer(log.data),
      block_consensus: true
    }

    transfer = %TokenTransfer{} |> TokenTransfer.changeset(transfer_params) |> Repo.insert!()

    token =
      %Token{}
      |> Token.changeset(%{contract_address_hash: log.address_hash, type: "ERC-20", decimals: 18, fiat_value: 2})
      |> Repo.insert!()

    {:ok,
     address: address,
     transaction: transaction,
     params: params,
     block: block,
     transfer: transfer,
     transfer_params: transfer_params,
     token: token}
  end

  test "unbounded-address-gas: recorded gas survives eviction and expiry", ctx do
    assert Decimal.equal?(Counters.gas_usage_count(ctx.address), ctx.transaction.gas_used)
    assert Repo.get!(Address, ctx.address.hash).gas_used == Decimal.to_integer(ctx.transaction.gas_used)
    churn_addresses(&Counters.gas_usage_count/1, AddressTransactionsGasUsageSum)
    assert :ets.lookup(AddressTransactionsGasUsageSum.cache_name(), to_string(ctx.address.hash)) == []
    gas = Decimal.mult(ctx.transaction.gas_used, 2)
    ctx.transaction |> Ecto.Changeset.change(gas_used: gas) |> Repo.update!()
    assert Decimal.equal?(Counters.gas_usage_count(ctx.address), gas)
    expire_and_wait(AddressTransactionsGasUsageSum)
    assert Decimal.equal?(Counters.gas_usage_count(ctx.address), gas)
  end

  test "unbounded-address-transactions: recorded transactions are bounded and refresh atomically", ctx do
    assert Counters.transactions_count(ctx.address) == 1
    assert Repo.get!(Address, ctx.address.hash).transactions_count == 1
    churn_addresses(&Counters.transactions_count/1, AddressTransactionsCount)
    refute :ets.member(AddressTransactionsCount.cache_name(), to_string(ctx.address.hash))
    store_transaction(Map.merge(ctx.params, %{hash: build(:transaction).hash, index: ctx.params.index + 1}), ctx.block)
    results = 1..12 |> Task.async_stream(fn _ -> Counters.transactions_count(ctx.address) end, max_concurrency: 12)
    assert Enum.to_list(results) == List.duplicate({:ok, 2}, 12)
    assert Repo.get!(Address, ctx.address.hash).transactions_count == 2
    assert :ets.info(AddressTransactionsCount.cache_name(), :size) <= @limit
    expire_and_wait(AddressTransactionsCount)
  end

  test "unbounded-address-token-transfers: recorded transfers survive eviction", ctx do
    assert Counters.token_transfers_count(ctx.address) == 1
    assert Repo.get!(Address, ctx.address.hash).token_transfers_count == 1
    churn_addresses(&Counters.token_transfers_count/1, AddressTokenTransfersCount)
    refute :ets.member(AddressTokenTransfersCount.cache_name(), to_string(ctx.address.hash))

    %TokenTransfer{}
    |> TokenTransfer.changeset(%{ctx.transfer_params | log_index: ctx.transfer.log_index + 100})
    |> Repo.insert!()

    assert Counters.token_transfers_count(ctx.address) == 2
    expire_and_wait(AddressTokenTransfersCount)
  end

  test "unbounded-address-usd: schema balances retain a bounded working set", ctx do
    market_status = :persistent_term.get(:market_token_fetcher_enabled, :absent)
    :persistent_term.put(:market_token_fetcher_enabled, true)

    on_exit(fn ->
      if market_status == :absent do
        :persistent_term.erase(:market_token_fetcher_enabled)
      else
        :persistent_term.put(:market_token_fetcher_enabled, market_status)
      end
    end)

    addresses = [ctx.address | Enum.map(1..24, fn _ -> insert(:address) end)]
    expected = ctx.transfer.amount |> Decimal.div(Decimal.new("1000000000000000000")) |> Decimal.mult(2)

    for address <- addresses do
      balance =
        %CurrentTokenBalance{}
        |> CurrentTokenBalance.changeset(%{
          address_hash: address.hash,
          block_number: ctx.block.number,
          token_contract_address_hash: ctx.token.contract_address_hash,
          token_type: "ERC-20",
          value: ctx.transfer.amount
        })
        |> Repo.insert!()
        |> Repo.preload(:token)

      assert Decimal.equal?(balance.token.fiat_value, 2)
      AddressTokensUsdSum.fetch(address.hash, [balance])

      eventually(fn ->
        case AddressTokensUsdSum.fetch(address.hash, [balance]) do
          %Decimal{} = value -> Decimal.equal?(value, expected)
          _ -> false
        end
      end)

      assert :ets.info(AddressTokensUsdSum.cache_name(), :size) <= @limit
      assert map_size(:sys.get_state(AddressTokensUsdSum).tasks) <= @limit
    end

    refute :ets.member(AddressTokensUsdSum.cache_name(), to_string(ctx.address.hash))
    eventually(fn -> map_size(:sys.get_state(AddressTokensUsdSum).tasks) == 0 end)
    expire_and_wait(AddressTokensUsdSum)
  end

  test "unbounded-address-tab-state: real producers release progress and cached counters", ctx do
    result = Counters.address_limited_counters(ctx.address.hash, [])
    assert result.transactions == 1
    assert result.token_transfers == 1
    eventually(fn -> :sys.get_state(AddressTabsElementsCount) == %{} end)

    for address <- Enum.map(1..16, fn _ -> insert(:address) end) do
      assert Counters.address_limited_counters(address.hash, []).transactions == 0
      assert :ets.info(:addresses_tabs_counters, :size) <= @limit
    end

    eventually(fn -> :sys.get_state(AddressTabsElementsCount) == %{} end)
    assert :ets.info(:addresses_tabs_counter_tasks, :size) == 0
    config = Application.fetch_env!(:explorer, AddressTabsElementsCount)
    Application.put_env(:explorer, AddressTabsElementsCount, Keyword.merge(config, cache_period: 20, ttl: 20))
    send(AddressTabsElementsCount, :prune_cache)
    eventually(fn -> :ets.info(:addresses_tabs_counters, :size) == 0 end)
  end

  test "unbounded-address-tab-state: terminal progress drains without losing in-flight coordination", ctx do
    for index <- 1..50 do
      store_transaction(
        Map.merge(ctx.params, %{hash: build(:transaction).hash, index: ctx.params.index + index}),
        ctx.block
      )
    end

    observer = self()
    latch = :atomics.new(1, [])
    handler = {__MODULE__, make_ref()}

    :ok =
      :telemetry.attach(
        handler,
        [:explorer, :repo, :query],
        fn _, _, metadata, {observer, latch} ->
          if Regex.match?(~r/WHERE.*created_contract_address_hash.*=/s, metadata.query) and
               :atomics.compare_exchange(latch, 1, 0, 1) == :ok do
            send(observer, {:paused_contract, self()})

            receive do
              :release_contract -> :ok
            after
              15_000 -> raise "recorded contract query was not released"
            end
          end
        end,
        {observer, latch}
      )

    on_exit(fn -> :telemetry.detach(handler) end)

    request = Task.async(fn -> Counters.address_limited_counters(ctx.address.hash, []) end)
    assert_receive {:paused_contract, producer}, 5000
    on_exit(fn -> send(producer, :release_contract) end)
    assert Task.await(request, 5000).transactions == 51
    key = to_string(ctx.address.hash)
    assert %{^key => %{limit_reached: true}} = :sys.get_state(AddressTabsElementsCount)
    assert AddressTabsElementsCount.get_task(:transactions_contract, key)

    for _ <- 1..16 do
      address = insert(:address)
      assert Counters.address_limited_counters(address.hash, []).transactions == 0
    end

    send(AddressTabsElementsCount, :prune_cache)
    assert %{^key => %{limit_reached: true}} = :sys.get_state(AddressTabsElementsCount)
    assert AddressTabsElementsCount.get_task(:transactions_contract, key)
    assert :ets.info(:addresses_tabs_counters, :size) <= @limit

    send(producer, :release_contract)
    eventually(fn -> :sys.get_state(AddressTabsElementsCount) == %{} end)
    eventually(fn -> AddressTabsElementsCount.get_task(:transactions_contract, key) == nil end)
    assert {_, 51, :limit_value} = AddressTabsElementsCount.get_counter(:transactions, key)
    assert :ets.info(:addresses_tabs_counters, :size) <= @limit
  end

  test "average-block-period-drift: two scheduled refreshes use cache_period", ctx do
    config = Application.fetch_env!(:explorer, AverageBlockTime)

    Application.put_env(
      :explorer,
      AverageBlockTime,
      Keyword.merge(config, enabled: true, cache_period: 40, period: 60_000, num_of_blocks: 3)
    )

    blocks =
      for n <- 1..103 do
        insert(:block, number: ctx.block.number + n, timestamp: DateTime.add(ctx.block.timestamp, n * 2, :second))
      end

    pid = start_counter(AverageBlockTime)
    assert Timex.Duration.to_milliseconds(AverageBlockTime.average_block_time()) == 2000
    :erlang.trace(pid, true, [:receive])
    on_exit(fn -> if Process.alive?(pid), do: :erlang.trace(pid, false, [:receive]) end)

    for {seconds, milliseconds} <- [{8, 3000}, {10, 4000}] do
      Enum.at(blocks, 2)
      |> Ecto.Changeset.change(timestamp: DateTime.add(ctx.block.timestamp, seconds, :second))
      |> Repo.update!()

      assert_receive {:trace, ^pid, :receive, :refresh_timestamps}, 1000
      eventually(fn -> Timex.Duration.to_milliseconds(AverageBlockTime.average_block_time()) == milliseconds end)
    end

    stop_supervised!(AverageBlockTime)
  end

  test "unbounded-block-burnt-fee: history browsing evicts and recomputes recorded gas", ctx do
    assert Decimal.equal?(BlockBurntFeeCount.fetch(ctx.block.hash), ctx.transaction.gas_used)
    churn_blocks(ctx, BlockBurntFeeCount)
    refute :ets.member(BlockBurntFeeCount.cache_name(), Base.encode16(ctx.block.hash.bytes, case: :lower))
    gas = Decimal.mult(ctx.transaction.gas_used, 2)
    ctx.transaction |> Ecto.Changeset.change(gas_used: gas) |> Repo.update!()
    assert Decimal.equal?(BlockBurntFeeCount.fetch(ctx.block.hash), gas)
  end

  test "unbounded-block-priority-fee: history browsing evicts and recomputes recorded fees", ctx do
    price = ctx.transaction.max_priority_fee_per_gas.value
    expected = Decimal.mult(price, ctx.transaction.gas_used)
    assert Decimal.equal?(BlockPriorityFeeCount.fetch(ctx.block.hash), expected)
    churn_blocks(ctx, BlockPriorityFeeCount)
    refute :ets.member(BlockPriorityFeeCount.cache_name(), Base.encode16(ctx.block.hash.bytes, case: :lower))
    ctx.transaction |> Ecto.Changeset.change(gas_used: Decimal.mult(ctx.transaction.gas_used, 2)) |> Repo.update!()
    assert Decimal.equal?(BlockPriorityFeeCount.fetch(ctx.block.hash), Decimal.mult(expected, 2))
  end

  defp start_counter(module) do
    if pid = Process.whereis(module) do
      :ok = Supervisor.terminate_child(Explorer.Supervisor, module)
      on_exit(fn -> Supervisor.restart_child(Explorer.Supervisor, module) end)
      refute Process.alive?(pid)
    end

    start_supervised!(module)
  end

  defp churn_addresses(fetch, module) do
    for _ <- 1..24 do
      fetch.(insert(:address))
      assert :ets.info(module.cache_name(), :size) <= @limit
    end
  end

  defp churn_blocks(ctx, module) do
    for n <- 1..24 do
      block = insert(:block, number: ctx.block.number + n, base_fee_per_gas: 13)
      transaction = store_transaction(Map.put(ctx.params, :hash, build(:transaction).hash), block)
      assert %Decimal{} = module.fetch(block.hash)
      assert transaction.block_hash == block.hash
      assert :ets.info(module.cache_name(), :size) <= @limit
    end
  end

  defp expire_and_wait(module) do
    config = Application.fetch_env!(:explorer, module)
    Application.put_env(:explorer, module, Keyword.put(config, :cache_period, 20))
    send(module, :prune_cache)
    eventually(fn -> :ets.info(module.cache_name(), :size) == 0 end)
  end

  defp store_address(hash) do
    {:ok, hash} = Explorer.Chain.Hash.Address.cast(hash)
    Repo.get(Address, hash) || insert(:address, hash: hash)
  end

  defp store_transaction(params, block) do
    %Transaction{}
    |> Transaction.changeset(
      Map.merge(params, %{
        block_hash: block.hash,
        block_number: block.number,
        block_timestamp: block.timestamp,
        block_consensus: true
      })
    )
    |> Repo.insert!()
  end

  defp topic_address("0x" <> topic), do: "0x" <> String.slice(topic, -40, 40)

  defp eventually(fun, attempts \\ 200)
  defp eventually(fun, 0), do: assert(fun.())

  defp eventually(fun, attempts) do
    if fun.() do
      :ok
    else
      Process.sleep(10)
      eventually(fun, attempts - 1)
    end
  end
end
