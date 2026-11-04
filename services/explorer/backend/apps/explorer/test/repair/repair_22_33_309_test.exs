Code.require_file("../explorer/market/market_history_cache_test.exs", __DIR__)

defmodule Explorer.Repair.Repair2233309Test do
  @moduledoc """
  Task 22.33.309, audit 22.30.135: market-cache-teardown is proved by
  "market-cache-teardown: a subsequent caller cannot read rolled-back rows" and
  "market-cache-teardown: teardown preserves the original rows and timestamp".
  The original cache tests also execute. The regression invokes their actual
  ExUnit setup and on_exit callbacks around a separate database sandbox, drives
  MarketHistory.bulk_insert through the production schema and cache, rolls the
  sandbox back, and checks a subsequent caller for both native and secondary
  coins. Recorded CryptoCompare daily opening/closing prices (1.1/2.2 and
  2.2/3.3) use today's date to remain inside the real cache's recent-day window.

  Baseline: no previous gate ledger or scoped changes were present. No unchanged
  tree verification ran because the ordered instructions require implementation
  before hf-build and hf-done. No baseline failures are claimed. The gate runner
  records the verified revision, command, exit code and private evidence log.
  No out-of-path wiring is required. This module is the scoped task record;
  spec and GOTCHA.kvx edits are outside the authorized paths.

  Qualification attempt 1 at 71a2969b8f52aa15f4cffc10bcc8f4ed863f39a2
  failed the two new cases with DBConnection.OwnershipError: running DataCase's
  shared-mode setup discarded the observer's independent sandbox. The named
  cache setup now runs directly inside the producer's own checkout, preserving
  the observer's transaction and all teardown and subsequent-caller assertions.
  """

  use ExUnit.Case, async: false

  alias Ecto.Adapters.SQL.Sandbox
  alias Explorer.Chain.Cache.Blocks
  alias Explorer.Market.{MarketHistory, MarketHistoryCache, MarketHistoryCacheTest}
  alias Explorer.Repo

  setup do
    :ok = Sandbox.checkout(Repo)
    cache_name = MarketHistoryCache.cache_name()
    original_entries = :ets.tab2list(ConCache.ets(cache_name))
    restart_cache()

    on_exit(fn ->
      restart_cache()
      Enum.each(original_entries, fn {key, value} -> :ok = ConCache.put(cache_name, key, value) end)
    end)

    :ok
  end

  test "market-cache-teardown: a subsequent caller cannot read rolled-back rows", context do
    exercise_teardown(false, context)
  end

  test "market-cache-teardown: teardown preserves the original rows and timestamp", context do
    exercise_teardown(true, context)
  end

  defp exercise_teardown(populated?, context) do
    records = recorded_prices()

    if populated? do
      assert {:ok, {2, _}} = MarketHistory.bulk_insert(records)
      assert_prices(MarketHistoryCache.fetch(), hd(records))
      assert_prices(MarketHistoryCache.fetch(true), List.last(records))
    end

    cache_name = MarketHistoryCache.cache_name()
    original_entries = :ets.tab2list(ConCache.ets(cache_name)) |> Enum.sort()
    original_rows = Repo.all(MarketHistory) |> Enum.sort()
    observer = self()

    {worker, monitor} =
      spawn_monitor(fn ->
        :ok = ExUnit.OnExitHandler.register(self())
        :ok = Sandbox.checkout(Repo)
        :ok = MarketHistoryCacheTest.isolate_market_history_cache(context)

        isolated_records =
          Enum.map(records, fn record ->
            %{
              record
              | date: Date.add(record.date, -1),
                opening_price: Decimal.new("3.3"),
                closing_price: Decimal.new("4.4")
            }
          end)

        assert {:ok, {2, _}} = MarketHistory.bulk_insert(isolated_records)
        assert_prices(MarketHistoryCache.fetch(), hd(isolated_records))
        assert_prices(MarketHistoryCache.fetch(true), List.last(isolated_records))

        block_cache_pid = child_pid(Blocks.child_id())
        send(observer, {:populated, self(), Process.whereis(cache_name), block_cache_pid})

        :ok = Sandbox.checkin(Repo)
      end)

    assert_receive {:populated, ^worker, isolated_cache_pid, block_cache_pid}, 5_000
    assert_receive {:DOWN, ^monitor, :process, ^worker, :normal}, 5_000

    # The producer's transaction is gone before its registered teardown runs.
    assert Enum.sort(Repo.all(MarketHistory)) == original_rows

    assert :ok = ExUnit.OnExitHandler.run(worker, 5_000)
    refute Process.alive?(isolated_cache_pid)
    assert child_pid(Blocks.child_id()) == block_cache_pid
    assert :ets.tab2list(ConCache.ets(cache_name)) |> Enum.sort() == original_entries

    unless populated? do
      assert {:ok, {2, _}} = MarketHistory.bulk_insert(records)
    end

    assert_prices(MarketHistoryCache.fetch(), hd(records))
    assert_prices(MarketHistoryCache.fetch(true), List.last(records))
  end

  defp recorded_prices do
    for secondary_coin <- [false, true] do
      %{
        date: Date.utc_today(),
        opening_price: Decimal.new(if(secondary_coin, do: "2.2", else: "1.1")),
        closing_price: Decimal.new(if(secondary_coin, do: "3.3", else: "2.2")),
        secondary_coin: secondary_coin
      }
    end
  end

  defp assert_prices([%MarketHistory{} = actual], expected) do
    assert actual.date == expected.date
    assert actual.secondary_coin == expected.secondary_coin
    assert Decimal.equal?(actual.opening_price, expected.opening_price)
    assert Decimal.equal?(actual.closing_price, expected.closing_price)
  end

  defp restart_cache do
    child_id = {ConCache, MarketHistoryCache.cache_name()}
    assert :ok = Supervisor.terminate_child(Explorer.Supervisor, child_id)
    assert {:ok, _} = Supervisor.restart_child(Explorer.Supervisor, child_id)
  end

  defp child_pid(child_id) do
    {^child_id, pid, _, _} = Enum.find(Supervisor.which_children(Explorer.Supervisor), &(elem(&1, 0) == child_id))
    pid
  end
end
