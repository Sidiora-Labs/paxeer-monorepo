defmodule Explorer.Repair.Repair2233381Test do
  @moduledoc """
  Task 22.33.381 repair record, audit 22.30.140.

  history-cache-publication is proven by the concurrent first-read cases (with
  recorded and empty history) and the expiry case below. They import recorded
  DIA daily prices through MarketHistory.bulk_insert/1, query the real schema,
  and consume the cache through Market.fetch_recent_history/1 and to_token/1.
  Recorded dates are shifted together to today to exercise the recent window.
  ConCache callbacks and Repo telemetry only pause real operations; neither
  replaces a producer, database, cache, or consumer.

  No out-of-path wiring is required. The runner's gate ledger records the
  verified revision and private evidence. Baseline verification was not run:
  the ordered instructions require full implementation before hf-build and
  hf-done and prohibit running the verify command separately. No pre-change
  failures are claimed. The task record is kept here because spec edits are
  outside the authorized scope.
  """

  use Explorer.DataCase, async: false

  alias Explorer.Chain.Cache.Counters.Helper
  alias Explorer.Market
  alias Explorer.Market.{MarketHistory, MarketHistoryCache, Source}

  setup do
    owner = self()
    child = {ConCache, MarketHistoryCache.cache_name()}
    :ok = Supervisor.terminate_child(Explorer.Supervisor, child)
    previous = :persistent_term.get(:market_history_fetcher_enabled, :unset)
    :persistent_term.put(:market_history_fetcher_enabled, true)

    state = start_supervised!({Agent, fn -> %{query: false, publication: false, queries: 0} end})

    start_supervised!(
      {ConCache,
       name: MarketHistoryCache.cache_name(),
       ttl_check_interval: false,
       callback: fn
         {:update, _, key, value} -> pause_once(state, :publication, owner, {:publication, key, value})
         _ -> :ok
       end}
    )

    handler = {__MODULE__, make_ref()}

    :ok =
      :telemetry.attach(
        handler,
        [:explorer, :repo, :query],
        fn _, _, metadata, _ ->
          if metadata[:source] == "market_history" and String.starts_with?(metadata.query, "SELECT") do
            Agent.update(state, &Map.update!(&1, :queries, fn count -> count + 1 end))
            pause_once(state, :query, owner, :query)
          end
        end,
        nil
      )

    on_exit(fn ->
      :telemetry.detach(handler)
      {:ok, _} = Supervisor.restart_child(Explorer.Supervisor, child)

      case previous do
        :unset -> :persistent_term.erase(:market_history_fetcher_enabled)
        value -> :persistent_term.put(:market_history_fetcher_enabled, value)
      end
    end)

    {:ok, state: state}
  end

  for empty? <- [false, true] do
    @empty empty?
    test "history-cache-publication concurrent first reads publish complete history (empty=#{empty?})", %{state: state} do
      records = if @empty, do: [], else: recorded_rows()
      assert {:ok, {count, _}} = MarketHistory.bulk_insert(records)
      assert count == length(records)
      expected = schema_rows()
      refute cache(MarketHistoryCache.data_key())
      refute cache(MarketHistoryCache.updated_at_key())
      Agent.update(state, &Map.put(&1, :queries, 0))

      arm(state, :publication)
      before = Helper.current_time()
      writer = Task.async(fn -> Market.fetch_recent_history() end)
      assert_receive {:paused, publisher, {:publication, key, published}}, 5_000
      assert key == MarketHistoryCache.data_key()
      assert published == expected
      assert cache(key) == expected
      refute cache(MarketHistoryCache.updated_at_key())

      readers = concurrent_readers()
      refute_receive {:read_complete, _, _}, 100
      send(publisher, :resume)

      assert Task.await(writer) == for_coin(expected, false)
      assert_readers(readers, expected)
      assert cache(MarketHistoryCache.data_key()) == expected
      assert cache(MarketHistoryCache.updated_at_key()) >= before
      assert Agent.get(state, & &1.queries) == 1

      assert Market.fetch_recent_history() == for_coin(expected, false)
      assert Market.fetch_recent_history(true) == for_coin(expected, true)
      assert Agent.get(state, & &1.queries) == 1
    end
  end

  test "history-cache-publication expiry preserves the last complete history until refresh publishes", %{state: state} do
    records = recorded_rows()
    assert {:ok, {count, _}} = MarketHistory.bulk_insert(records)
    assert count == length(records)
    old_history = schema_rows()
    assert Market.fetch_recent_history() == for_coin(old_history, false)

    # Replay a different recorded day's prices for the same market-history dates.
    replacement = recorded_rows(1)
    assert {:ok, {^count, _}} = MarketHistory.bulk_insert(replacement)
    expected = schema_rows()
    refute expected == old_history
    :ok = ConCache.put(MarketHistoryCache.cache_name(), MarketHistoryCache.updated_at_key(), 1)
    assert cache(MarketHistoryCache.data_key()) == old_history

    Agent.update(state, &Map.put(&1, :queries, 0))
    arm(state, :query)
    arm(state, :publication)
    before = Helper.current_time()
    writer = Task.async(fn -> Market.fetch_recent_history() end)
    assert_receive {:paused, querying, :query}, 5_000

    assert cache(MarketHistoryCache.data_key()) == old_history
    assert cache(MarketHistoryCache.updated_at_key()) == 1
    readers = concurrent_readers()
    refute_receive {:read_complete, _, _}, 100
    send(querying, :resume)

    assert_receive {:paused, publisher, {:publication, key, published}}, 5_000
    assert key == MarketHistoryCache.data_key()
    assert published == expected
    assert cache(key) == expected
    assert cache(MarketHistoryCache.updated_at_key()) == 1
    refute_receive {:read_complete, _, _}, 100
    send(publisher, :resume)

    assert Task.await(writer) == for_coin(expected, false)
    assert_readers(readers, expected)
    assert cache(MarketHistoryCache.data_key()) == expected
    assert cache(MarketHistoryCache.updated_at_key()) >= before
    assert Agent.get(state, & &1.queries) == 1
  end

  defp concurrent_readers do
    owner = self()

    readers =
      for index <- 1..12 do
        secondary? = rem(index, 2) == 0

        task =
          Task.async(fn ->
            send(owner, {:reading, self()})
            history = Market.fetch_recent_history(secondary?)
            send(owner, {:read_complete, self(), history})
            history
          end)

        {task, secondary?}
      end

    for {task, _} <- readers do
      pid = task.pid
      assert_receive {:reading, ^pid}, 5_000
    end

    readers
  end

  defp assert_readers(readers, expected) do
    for {task, secondary?} <- readers do
      history = Task.await(task)
      assert history == for_coin(expected, secondary?)

      for record <- history do
        assert %Explorer.Market.Token{fiat_value: price} = MarketHistory.to_token(record)
        assert Decimal.equal?(price, record.closing_price)
      end
    end
  end

  defp arm(state, phase), do: Agent.update(state, &Map.put(&1, phase, true))

  defp pause_once(state, phase, owner, event) do
    if Agent.get_and_update(state, fn current -> {Map.fetch!(current, phase), Map.put(current, phase, false)} end) do
      send(owner, {:paused, self(), event})

      receive do
        :resume -> :ok
      after
        5_000 -> raise "Test did not release #{inspect(event)}"
      end
    end
  end

  defp cache(key), do: ConCache.get(MarketHistoryCache.cache_name(), key)

  defp for_coin(records, secondary?), do: Enum.filter(records, &(&1.secondary_coin == secondary?))

  defp schema_rows do
    Repo.all(from(mh in MarketHistory, where: mh.date > ^Date.add(Date.utc_today(), -30), order_by: [desc: mh.date]))
  end

  defp recorded_rows(day_offset \\ 0) do
    for {coin, secondary?} <- [{"native", false}, {"secondary", true}], age <- [0, 1, 30] do
      %{"DataPoints" => [%{"Series" => [%{"values" => values}]}]} =
        __DIR__
        |> Path.join("../support/fixture/market/dia/#{coin}_coin_history.json")
        |> File.read!()
        |> Jason.decode!()

      days = values |> Enum.group_by(&String.slice(hd(&1), 0, 10)) |> Enum.sort(:desc)
      {_date, samples} = Enum.at(days, day_offset + rem(age, 2))
      samples = Enum.sort_by(samples, &hd/1)

      %{
        date: Date.add(Date.utc_today(), -age),
        opening_price: samples |> hd() |> List.last() |> Source.to_decimal(),
        closing_price: samples |> List.last() |> List.last() |> Source.to_decimal(),
        secondary_coin: secondary?
      }
    end
  end
end
