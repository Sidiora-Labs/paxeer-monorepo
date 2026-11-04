defmodule Explorer.Repair.Repair2233495Test do
  @moduledoc """
  Task 22.33.495 / audit 22.30.151 repair record:
  unbounded-token-holders -> "unbounded-token-holders: schema counters bound recorded holders and recompute after eviction"
  unbounded-token-holders -> "unbounded-token-holders: scheduled expiry preserves the persisted fallback"
  unbounded-token-transfers -> "unbounded-token-transfers: schema counters bound recorded transfers and recompute after eviction"
  unbounded-token-transfers -> "unbounded-token-transfers: scheduled expiry preserves the persisted fallback"
  Both findings -> "refreshes coordinate with pruning and concurrent requests without retaining extra entries"

  No out-of-path wiring is required: both counters use the existing BoundedCache
  :max_entries configuration (default 10_000) and their existing :cache_period.
  Baseline: the initial tree contained only runner bookkeeping and no gate ledger.
  The explicit implementation-first gate order prohibits the older step's baseline
  verify, so no unchanged-tree result is claimed. hf-build and hf-done record the
  verified revision, exact commands, exit codes and private evidence locations.
  """

  use Explorer.DataCase, async: false

  alias Explorer.Chain.{Address, Token, TokenTransfer, Transaction}
  alias Explorer.Chain.Address.CurrentTokenBalance
  alias Explorer.Chain.Cache.Counters.{TokenHoldersCount, TokenTransfersCount}

  @recording Path.expand("../../../ethereum_jsonrpc/test/support/fixture/geth/trace/struct_logger.json", __DIR__)
  @limit 4
  @counters [TokenHoldersCount, TokenTransfersCount]

  setup do
    for module <- @counters do
      config = Application.get_env(:explorer, module)
      Application.put_env(:explorer, module, Keyword.merge(config || [], max_entries: @limit, cache_period: 60_000))

      on_exit(fn ->
        if config, do: Application.put_env(:explorer, module, config), else: Application.delete_env(:explorer, module)
      end)

      if pid = Process.whereis(module) do
        :ok = Supervisor.terminate_child(Explorer.Supervisor, module)
        on_exit(fn -> Supervisor.restart_child(Explorer.Supervisor, module) end)
        refute Process.alive?(pid)
      end

      start_supervised!(module)
    end

    recording = @recording |> File.read!() |> Jason.decode!()
    tx = recording["tx"] |> EthereumJSONRPC.Transaction.to_elixir() |> EthereumJSONRPC.Transaction.elixir_to_params()
    receipt = recording["receipt"] |> EthereumJSONRPC.Receipt.to_elixir() |> EthereumJSONRPC.Receipt.elixir_to_params()
    params = Map.merge(tx, Map.take(receipt, [:gas_used, :cumulative_gas_used, :status]))
    block = insert(:block, hash: params.block_hash, number: params.block_number)
    for hash <- [params.from_address_hash, params.to_address_hash], do: store_address(hash)

    transaction =
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

    token = store_token(log.address_hash)
    store_transfer(transfer_params)
    store_balance(token, to, transfer_params)
    {:ok, token: token, transfer_params: transfer_params}
  end

  test "unbounded-token-holders: schema counters bound recorded holders and recompute after eviction", ctx do
    assert Token.fetch_token_counters(ctx.token.contract_address_hash, :infinity) == {1, 1}
    add_recorded_activity(ctx)
    assert Token.fetch_token_counters(ctx.token.contract_address_hash, :infinity) == {1, 1}
    churn_tokens(ctx)
    refute :ets.member(TokenHoldersCount.cache_name(), to_string(ctx.token.contract_address_hash))
    assert TokenHoldersCount.fetch_count_from_cache(ctx.token.contract_address_hash) == 1
    assert Token.fetch_token_counters(ctx.token.contract_address_hash, :infinity) == {2, 2}
    assert Repo.get!(Token, ctx.token.contract_address_hash).holder_count == 2
    assert TokenHoldersCount.fetch_count_from_cache(ctx.token.contract_address_hash) == 2
    assert_bounded()
  end

  test "unbounded-token-transfers: schema counters bound recorded transfers and recompute after eviction", ctx do
    assert Token.fetch_token_counters(ctx.token.contract_address_hash, :infinity) == {1, 1}
    add_recorded_activity(ctx)
    assert Token.fetch_token_counters(ctx.token.contract_address_hash, :infinity) == {1, 1}
    churn_tokens(ctx)
    refute :ets.member(TokenTransfersCount.cache_name(), to_string(ctx.token.contract_address_hash))
    assert TokenTransfersCount.fetch_count_from_cache(ctx.token.contract_address_hash) == 1
    assert Token.fetch_token_counters(ctx.token.contract_address_hash, :infinity) == {2, 2}
    assert Repo.get!(Token, ctx.token.contract_address_hash).transfer_count == 2
    assert TokenTransfersCount.fetch_count_from_cache(ctx.token.contract_address_hash) == 2
    assert_bounded()
  end

  test "unbounded-token-holders: scheduled expiry preserves the persisted fallback", ctx do
    prove_expiry(ctx, TokenHoldersCount, :holder_count)
  end

  test "unbounded-token-transfers: scheduled expiry preserves the persisted fallback", ctx do
    prove_expiry(ctx, TokenTransfersCount, :transfer_count)
  end

  test "refreshes coordinate with pruning and concurrent requests without retaining extra entries", ctx do
    churn_tokens(ctx)

    for {module, column} <- [{TokenHoldersCount, "holder_count"}, {TokenTransfersCount, "transfer_count"}] do
      observer = self()
      handler = {__MODULE__, module, make_ref()}
      latch = :atomics.new(1, [])

      :ok =
        :telemetry.attach(
          handler,
          [:explorer, :repo, :query],
          fn _, _, metadata, {observer, latch} ->
            if String.contains?(metadata.query, "UPDATE \"tokens\"") and String.contains?(metadata.query, column) do
              send(observer, {:refresh, module})

              if :atomics.compare_exchange(latch, 1, 0, 1) == :ok do
                send(observer, {:refresh_started, self()})

                receive do
                  :release_refresh -> :ok
                after
                  10_000 -> raise "recorded token refresh was not released"
                end
              end
            end
          end,
          {observer, latch}
        )

      on_exit(fn -> :telemetry.detach(handler) end)

      request = Task.async(fn -> module.fetch(ctx.token.contract_address_hash) end)
      assert_receive {:refresh_started, producer}, 5000
      on_exit(fn -> send(producer, :release_refresh) end)

      followers =
        Task.async(fn ->
          1..16
          |> Task.async_stream(fn _ -> module.fetch(ctx.token.contract_address_hash) end, max_concurrency: 16)
          |> Enum.to_list()
        end)

      send(module, :prune_cache)
      :sys.get_state(module)
      assert_bounded()
      send(producer, :release_refresh)
      assert Task.await(request, 5000) == 1
      assert Task.await(followers, 10_000) == List.duplicate({:ok, 1}, 16)
      assert_receive {:refresh, ^module}
      refute_receive {:refresh, ^module}, 50
      assert module.fetch_count_from_cache(ctx.token.contract_address_hash) == 1
      assert_bounded()
      :ok = :telemetry.detach(handler)
    end
  end

  defp prove_expiry(ctx, module, column) do
    hash = ctx.token.contract_address_hash
    assert Token.fetch_token_counters(hash, :infinity) == {1, 1}
    add_recorded_activity(ctx)
    config = Application.fetch_env!(:explorer, module)
    Application.put_env(:explorer, module, Keyword.put(config, :cache_period, 20))
    send(module, :prune_cache)
    eventually(fn -> :ets.info(module.cache_name(), :size) == 0 end)
    assert module.fetch_count_from_cache(hash) == 1
    assert Map.fetch!(Repo.get!(Token, hash), column) == 1
    Application.put_env(:explorer, module, config)
    assert Token.fetch_token_counters(hash, :infinity) == if(module == TokenHoldersCount, do: {1, 2}, else: {2, 1})
    assert Map.fetch!(Repo.get!(Token, hash), column) == 2
    assert_bounded()
  end

  defp churn_tokens(ctx) do
    for index <- 1..24 do
      address = insert(:address)
      token = store_token(address.hash)

      params = %{
        ctx.transfer_params
        | token_contract_address_hash: address.hash,
          log_index: ctx.transfer_params.log_index + index
      }

      store_transfer(params)
      store_balance(token, params.to_address_hash, params)
      assert Token.fetch_token_counters(token.contract_address_hash, :infinity) == {1, 1}
      assert_bounded()
    end
  end

  defp add_recorded_activity(ctx) do
    params = %{ctx.transfer_params | log_index: ctx.transfer_params.log_index + 100}
    store_transfer(params)
    store_balance(ctx.token, params.from_address_hash, params)
  end

  defp store_token(hash) do
    %Token{}
    |> Token.changeset(%{contract_address_hash: hash, type: "ERC-20", decimals: 18})
    |> Repo.insert!()
  end

  defp store_transfer(params), do: %TokenTransfer{} |> TokenTransfer.changeset(params) |> Repo.insert!()

  defp store_balance(token, address, params) do
    %CurrentTokenBalance{}
    |> CurrentTokenBalance.changeset(%{
      address_hash: address,
      block_number: params.block_number,
      token_contract_address_hash: token.contract_address_hash,
      token_type: params.token_type,
      value: params.amount
    })
    |> Repo.insert!()
  end

  defp store_address(hash) do
    {:ok, hash} = Explorer.Chain.Hash.Address.cast(hash)
    Repo.get(Address, hash) || insert(:address, hash: hash)
  end

  defp assert_bounded do
    for module <- @counters, do: assert(:ets.info(module.cache_name(), :size) <= @limit)
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
