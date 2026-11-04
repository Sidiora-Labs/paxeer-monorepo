defmodule Explorer.Repair.Repair2233463Test do
  use ExUnit.Case, async: false

  import Ecto.Query

  alias EthereumJSONRPC.Block
  alias Explorer.Chain.{Address, Hash}
  alias Explorer.Chain.Fetcher.LookUpSmartContractSourcesOnDemand
  alias Explorer.Repo
  alias Explorer.Utility.{Hammer, MissingBlockRange, RateLimiter}

  @fixture Path.expand("../../../ethereum_jsonrpc/test/support/fixture/paxeer_x/consecutive_blocks.json", __DIR__)

  setup do
    owner = Ecto.Adapters.SQL.Sandbox.start_owner!(Repo, shared: true)
    on_exit(fn -> Ecto.Adapters.SQL.Sandbox.stop_owner(owner) end)

    blocks = @fixture |> File.read!() |> Jason.decode!() |> Map.values() |> Enum.map(&Block.to_elixir/1)
    [low, high] = Enum.sort_by(blocks, &Map.fetch!(&1, "number"))
    assert high["number"] == low["number"] + 1
    assert high["parentHash"] == low["hash"]

    %{head: high["number"], miner: high["miner"]}
  end

  describe "missing-range-allocation" do
    setup do
      Repo.delete_all(MissingBlockRange)
      :ok
    end

    test "large recorded catch-up gap stores only intervals and preserves priority batches", %{head: head} do
      # Recorded block heights enter through the same schema producer used for
      # prioritized contract-creation fetches. The catch-up backlog spans genesis
      # to the recorded head, but has only two priority islands.
      :ok = MissingBlockRange.add_ranges_by_block_numbers([head - 2, head - 3], 1)
      :ok = MissingBlockRange.add_ranges_by_block_numbers([head - 10, head - 11, head - 12], 1)
      priority_rows = Repo.all(from(r in MissingBlockRange, order_by: r.id))
      MissingBlockRange.save_batch([5..9, (head + 2)..(head + 1)])

      owner = self()

      {pid, monitor} =
        :erlang.spawn_opt(
          fn ->
            result = MissingBlockRange.save_batch(10..head)
            send(owner, {:catch_up_saved, self(), result})
          end,
          [:monitor, {:max_heap_size, %{size: 1_000_000, kill: true, error_logger: false}}]
        )

      # A per-height list/MapSet for this 24-million-height recorded gap cannot
      # fit this worker's heap. The real schema/Repo path must finish within it.
      assert_receive {:catch_up_saved, ^pid, [{:ok, :ok}]}, 10_000
      assert_receive {:DOWN, ^monitor, :process, ^pid, :normal}, 10_000

      assert queue_intervals() == [
               {head + 2, head - 1, nil},
               {head - 2, head - 3, 1},
               {head - 4, head - 9, nil},
               {head - 10, head - 12, 1},
               {head - 13, 5, nil}
             ]

      assert Repo.all(from(r in MissingBlockRange, where: not is_nil(r.priority), order_by: r.id)) == priority_rows
      assert Repo.aggregate(MissingBlockRange, :count) == 5

      # The indexer consumes priority heights before the higher normal backlog.
      batch = MissingBlockRange.get_latest_batch(8)
      assert batch == [(head - 2)..(head - 3)//-1, (head - 10)..(head - 12)//-1, (head + 2)..head//-1]
      MissingBlockRange.clear_batch(batch)
      assert MissingBlockRange.get_latest_batch(2) == [(head - 1)..(head - 1)//-1, (head - 4)..(head - 4)//-1]
      assert Repo.aggregate(MissingBlockRange, :count) == 3
    end

    test "subtraction preserves inclusive boundaries, single gaps and adjacent priority islands", %{head: head} do
      # These small windows around the recorded head cover empty, single and
      # multiple complements without changing any priority record's identity.
      for {islands, expected} <- [
            {[{18, 20}, {0, 2}], [{20, 18, 1}, {17, 3, nil}, {2, 0, 1}]},
            {[{2, 5}], [{20, 6, nil}, {5, 2, 1}, {1, 0, nil}]},
            {[{0, 19}], [{20, 20, nil}, {19, 0, 1}]},
            {[{1, 20}], [{20, 1, 1}, {0, 0, nil}]},
            {[{0, 9}, {10, 20}], [{20, 10, 1}, {9, 0, 1}]},
            {[{0, 20}], [{20, 0, 1}]},
            {[], [{20, 0, nil}]}
          ] do
        Repo.delete_all(MissingBlockRange)
        base = head - 20

        originals =
          Enum.map(islands, fn {low, high} ->
            Repo.insert!(MissingBlockRange.changeset(%{from_number: base + high, to_number: base + low, priority: 1}))
          end)

        assert [{:ok, _}] = MissingBlockRange.save_batch(base..head)

        assert queue_intervals() ==
                 Enum.map(expected, fn {high, low, priority} -> {base + high, base + low, priority} end)

        Enum.each(originals, fn original -> assert Repo.get!(MissingBlockRange, original.id) == original end)
      end
    end
  end

  describe "rate-limit-expiry-process" do
    setup %{miner: miner} do
      original = Application.get_env(:explorer, RateLimiter)

      config =
        original
        |> Keyword.merge(enabled: true, storage: :ets, hammer_backend_module: Hammer.ETS)
        |> Keyword.put(:on_demand,
          time_interval_limit: 1_000,
          limit_by_ip: 1,
          exp_timeout_coeff: 1,
          max_ban_interval: 60_000,
          limitation_period: 100
        )

      Application.put_env(:explorer, RateLimiter, config)
      on_exit(fn -> Application.put_env(:explorer, RateLimiter, original) end)

      limiter = ensure_started(RateLimiter)
      if :ets.whereis(Hammer.ETS) == :undefined, do: start_supervised!({Hammer.ETS, []})
      fetcher = ensure_started(LookUpSmartContractSourcesOnDemand)
      :erlang.trace(fetcher, true, [:receive])
      on_exit(fn -> if Process.alive?(fetcher), do: :erlang.trace(fetcher, false, [:receive]) end)

      # Use the address actually recorded as miner. An address without deployed
      # code is consumed by the real fetcher's eligibility path without RPC.
      {:ok, hash} = Hash.Address.cast(miner)
      Repo.insert!(%Address{hash: hash})
      identifier = "#{miner}_repair_#{System.unique_integer([:positive])}"
      key = "#{Application.get_env(:block_scout_web, :chain_id)}_#{identifier}_on_demand"
      %{identifier: identifier, key: key, fetcher: fetcher, limiter: limiter}
    end

    test "request exit cannot retain ban history and the next ban starts fresh", context do
      assert request(context, 2) == [:ok, :ignore]
      assert_consumed(context)
      assert [{_, value}] = :ets.lookup(:rate_limiter, context.key)
      assert String.ends_with?(value, ":1")

      assert_eventually(fn -> :ets.lookup(:rate_limiter, context.key) == [] end, 5_000)
      refute Map.has_key?(:sys.get_state(context.limiter), context.key)

      assert request(context, 2) == [:ok, :ignore]
      assert_consumed(context)
      assert [{_, new_value}] = :ets.lookup(:rate_limiter, context.key)
      assert String.ends_with?(new_value, ":1")
    end

    test "renewed ban survives an older expiry and its own timer clears all history", context do
      update_action(time_interval_limit: 60_000, limitation_period: 3_000)
      assert request(context, 2) == [:ok, :ignore]
      assert_consumed(context)
      [{_, first_value}] = :ets.lookup(:rate_limiter, context.key)
      [try_after, "1"] = String.split(first_value, ":")
      {old_timer, old_token} = Map.fetch!(:sys.get_state(context.limiter), context.key)

      assert_eventually(fn -> System.system_time(:second) > String.to_integer(try_after) end, 3_000)
      assert request(context, 1) == [:ignore]
      [{_, renewed_value}] = :ets.lookup(:rate_limiter, context.key)
      assert String.ends_with?(renewed_value, ":2")
      {new_timer, new_token} = Map.fetch!(:sys.get_state(context.limiter), context.key)
      refute new_token == old_token
      refute new_timer == old_timer
      assert Process.read_timer(old_timer) == false

      # A cancelled timer can already be in the owner's mailbox. Deliver that
      # exact old generation to exercise this race through the real GenServer.
      send(context.limiter, {:delete, context.key, old_token})
      assert Map.fetch!(:sys.get_state(context.limiter), context.key) == {new_timer, new_token}
      assert :ets.lookup(:rate_limiter, context.key) == [{context.key, renewed_value}]

      assert_eventually(fn -> :ets.lookup(:rate_limiter, context.key) == [] end, 7_000)
      refute Map.has_key?(:sys.get_state(context.limiter), context.key)
    end

    test "an old unversioned expiry cannot delete a current ban", context do
      assert request(context, 2) == [:ok, :ignore]
      assert_consumed(context)
      entries = :ets.lookup(:rate_limiter, context.key)
      send(context.limiter, {:delete, context.key})
      :sys.get_state(context.limiter)
      assert :ets.lookup(:rate_limiter, context.key) == entries
    end
  end

  defp queue_intervals do
    Repo.all(
      from(r in MissingBlockRange, order_by: [desc: r.from_number], select: {r.from_number, r.to_number, r.priority})
    )
  end

  defp ensure_started(module) do
    Process.whereis(module) || start_supervised!({module, []})
  end

  defp request(context, count) do
    # Keep the request burst (and a subsequent backoff renewal) away from a
    # fixed-window boundary in the real Hammer backend.
    interval = Application.get_env(:explorer, RateLimiter)[:on_demand][:time_interval_limit]
    margin = min(div(interval, 2), 5_000)
    assert_eventually(fn -> interval - rem(System.system_time(:millisecond), interval) > margin end, margin + 1_000)
    owner = self()

    {pid, monitor} =
      spawn_monitor(fn ->
        results =
          Enum.map(1..count, fn _ ->
            LookUpSmartContractSourcesOnDemand.trigger_fetch(context.identifier, context.miner)
          end)

        send(owner, {:request_finished, self(), results})
      end)

    assert_receive {:request_finished, ^pid, results}, 5_000
    assert_receive {:DOWN, ^monitor, :process, ^pid, :normal}, 5_000
    results
  end

  defp assert_consumed(%{fetcher: fetcher, miner: miner}) do
    assert_receive {:trace, ^fetcher, :receive, {:"$gen_cast", {:check_eligibility, ^miner}}}, 5_000
    assert %{current_concurrency: 0} = :sys.get_state(fetcher)
  end

  defp update_action(options) do
    config = Application.get_env(:explorer, RateLimiter)
    Application.put_env(:explorer, RateLimiter, Keyword.update!(config, :on_demand, &Keyword.merge(&1, options)))
  end

  defp assert_eventually(predicate, timeout) do
    deadline = System.monotonic_time(:millisecond) + timeout
    await(predicate, deadline)
  end

  defp await(predicate, deadline) do
    if predicate.() do
      :ok
    else
      assert System.monotonic_time(:millisecond) < deadline, "condition did not become true before its deadline"
      Process.sleep(10)
      await(predicate, deadline)
    end
  end
end
