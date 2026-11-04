for file <- [
      "20261028000000_add_paxeer_x_price_observation_accepted.exs",
      "20261104000000_backfill_paxeer_x_price_acceptance.exs",
      "scripts/price_acceptance_backfill.exs"
    ] do
  Code.require_file(Path.expand("../../priv/repo/migrations/#{file}", __DIR__))
end

defmodule Explorer.Repair.Repair2231406Test do
  use ExUnit.Case, async: false

  import Ecto.Query
  import Explorer.Factory

  alias Explorer.Chain.Block
  alias Explorer.Chain.Cache.Counters.LastFetchedCounter
  alias Explorer.Chain.PaxeerX.{PriceFeed, PriceFeedScannedBlock, PriceMarket, PriceObservation}
  alias Explorer.PagingOptions
  alias Explorer.PaxeerX.CosmosRest
  alias Explorer.Repo

  alias Explorer.Repo.Migrations.{
    AddPaxeerXPriceObservationAccepted,
    BackfillPaxeerXPriceAcceptance,
    PriceAcceptanceBackfill
  }

  @version 20_261_104_000_000
  @accepted_version 20_261_028_000_000
  @height 37_715_713
  @fixtures Path.expand("../support/fixture/paxeer_x/price_feed", __DIR__)

  setup do
    assert :ok = Ecto.Migrator.down(Repo, @version, BackfillPaxeerXPriceAcceptance, log: false)

    body = fixture("block_txs.json")
    results = fixture("block_results.json")["txs_results"]
    transactions = %{"txs" => body["txs"], "raw" => body["block"]["data"]["txs"]}
    block = insert(:block, number: @height, consensus: true)

    on_exit(fn ->
      Repo.delete_all(from(b in Block, where: b.hash == ^block.hash))
      Repo.delete_all(from(s in PriceFeedScannedBlock, where: s.block_number == @height))
      Repo.delete_all(from(m in PriceMarket, where: m.market_id in [0, 1, 2, 17]))
      LastFetchedCounter.delete("paxeer_x_price_update_total")
      LastFetchedCounter.delete("paxeer_x_price_feed_scanned_through_#{@height}")
      Ecto.Migrator.up(Repo, @accepted_version, AddPaxeerXPriceObservationAccepted, log: false)
      Ecto.Migrator.up(Repo, @version, BackfillPaxeerXPriceAcceptance, log: false)
    end)

    %{block: block, transactions: transactions, results: results}
  end

  test "historical_price_acceptance_recovers_mid_block_after_restart", context do
    historical_price_acceptance_recovers_mid_block_after_restart(context)
  end

  def historical_price_acceptance_recovers_mid_block_after_restart(context) do
    for encode <- [&Function.identity/1, &Base.encode64/1] do
      results = refuse_market(context.results, 2, encode)
      assert {:ok, [update]} = decoded(context.transactions, results)
      assert update.status == 1
      assert Enum.find(update.observations, &(&1.market_id == 2)).accepted == false

      Repo.query!("ALTER TABLE paxeer_x_price_observations ALTER COLUMN accepted SET DEFAULT true")
      import_before_upgrade(context.block, update)
      assert Enum.all?(PriceFeed.block_updates(@height) |> hd() |> Map.fetch!(:observations), & &1.accepted)
      assert PriceFeed.compact_scanned_blocks(@height, 100) == @height
      assert {:ok, []} = PriceFeed.stream_unscanned_block_numbers([], &[&1 | &2], @height)

      PriceFeed.upsert_markets([
        %{market_id: 0, symbol: "BTC", chainflow_symbol: "BTC"},
        %{market_id: 2, symbol: "SOL", chainflow_symbol: "SOL"},
        %{market_id: 17, symbol: "BNB", chainflow_symbol: "BNB"}
      ])

      assert :ok = Ecto.Migrator.up(Repo, @version, BackfillPaxeerXPriceAcceptance, log: false)
      assert unknown_count() == 18
      assert PriceFeed.market_updates(2, %PagingOptions{}) == []
      assert Enum.all?(PriceFeed.latest_per_market(), fn {_market, observation} -> is_nil(observation) end)
      assert %{rows: [[nil, "YES"]]} = accepted_column()

      {:ok, first} = PriceAcceptanceBackfill.start_link()
      assert :ok = PriceAcceptanceBackfill.retain(first, @height, context.transactions, results)
      assert {:ok, 9} = PriceAcceptanceBackfill.step(first, 9)
      assert cursor() == [[0, 8, false, nil]]
      assert unknown_count() == 9
      assert PriceFeed.market_updates(2, %PagingOptions{}) == []
      assert [%PriceObservation{accepted: true}] = PriceFeed.market_updates(0, %PagingOptions{})
      assert PriceFeed.market_updates(17, %PagingOptions{}) == []
      assert PriceFeed.total_update_count() == 1

      assert :ok = GenServer.stop(first)
      refute Process.alive?(first)

      {:ok, reopened} = PriceAcceptanceBackfill.start_link()
      assert cursor() == [[0, 8, false, nil]]
      assert unknown_count() == 9
      assert {:ok, 9} = PriceAcceptanceBackfill.step(reopened, 9)
      assert cursor() == [[0, 17, true, nil]]
      assert :complete = PriceAcceptanceBackfill.step(reopened, 9)
      assert :ok = GenServer.stop(reopened)
      assert unknown_count() == 0
      assert PriceFeed.scanned_through(@height) == @height
      assert PriceFeed.total_update_count() == 1
      assert PriceFeed.market_updates(2, %PagingOptions{}) == []
      assert [%PriceObservation{accepted: true}] = PriceFeed.market_updates(17, %PagingOptions{})

      assert [
               {%PriceMarket{market_id: 0}, %PriceObservation{accepted: true}},
               {%PriceMarket{market_id: 2}, nil},
               {%PriceMarket{market_id: 17}, %PriceObservation{accepted: true}}
             ] = PriceFeed.latest_per_market()

      [stored] = PriceFeed.block_updates(@height)
      assert stored.status == 1
      assert stored.observation_count == 18
      assert Enum.find(stored.observations, &(&1.market_id == 2)).accepted == false

      assert :ok = Ecto.Migrator.down(Repo, @version, BackfillPaxeerXPriceAcceptance, log: false)
    end

    assert :ok = Ecto.Migrator.up(Repo, @version, BackfillPaxeerXPriceAcceptance, log: false)
  end

  test "historical_price_acceptance_upgrades_the_pre_column_schema", context do
    historical_price_acceptance_upgrades_the_pre_column_schema(context)
  end

  def historical_price_acceptance_upgrades_the_pre_column_schema(context) do
    assert :ok = Ecto.Migrator.down(Repo, @accepted_version, AddPaxeerXPriceObservationAccepted, log: false)
    assert {:ok, [update]} = decoded(context.transactions, context.results)
    import_before_upgrade(context.block, update)
    assert :ok = Ecto.Migrator.up(Repo, @accepted_version, AddPaxeerXPriceObservationAccepted, log: false)
    assert unknown_count() == 18
    assert %{rows: [[nil, "YES"]]} = accepted_column()
    assert :ok = Ecto.Migrator.up(Repo, @version, BackfillPaxeerXPriceAcceptance, log: false)

    {:ok, worker} = PriceAcceptanceBackfill.start_link()
    assert :ok = PriceAcceptanceBackfill.retain(worker, @height, context.transactions, context.results)
    assert {:ok, 18} = PriceAcceptanceBackfill.step(worker, 18)
    assert :complete = PriceAcceptanceBackfill.step(worker, 18)
    assert unknown_count() == 0
    assert :ok = GenServer.stop(worker)
  end

  test "historical_price_acceptance_rejects_mismatched_retained_data", context do
    historical_price_acceptance_rejects_mismatched_retained_data(context)
  end

  def historical_price_acceptance_rejects_mismatched_retained_data(context) do
    assert {:ok, [update]} = decoded(context.transactions, context.results)
    import_before_upgrade(context.block, update)
    assert :ok = Ecto.Migrator.up(Repo, @version, BackfillPaxeerXPriceAcceptance, log: false)
    {:ok, worker} = PriceAcceptanceBackfill.start_link()
    assert {:error, {:malformed, _}} = PriceAcceptanceBackfill.retain(worker, @height, context.transactions, [])
    assert cursor() == [[-1, -1, false, nil]]

    mismatched =
      put_in(
        context.transactions,
        ["txs", Access.at(0), "body", "messages", Access.at(0), "observations", Access.at(8), "sequence"],
        "1"
      )

    assert :ok = PriceAcceptanceBackfill.retain(worker, @height, mismatched, context.results)
    assert {:error, :retained_observation_mismatch} = PriceAcceptanceBackfill.step(worker, 18)
    assert cursor() == [[-1, -1, false, ":retained_observation_mismatch"]]
    assert unknown_count() == 18
    assert PriceFeed.market_updates(0, %PagingOptions{}) == []
    assert :ok = GenServer.stop(worker)

    {:ok, reopened} = PriceAcceptanceBackfill.start_link()
    assert cursor() == [[-1, -1, false, ":retained_observation_mismatch"]]
    assert :ok = GenServer.stop(reopened)
  end

  test "historical_price_acceptance_keeps_fetch_errors_retryable", context do
    historical_price_acceptance_keeps_fetch_errors_retryable(context)
  end

  def historical_price_acceptance_keeps_fetch_errors_retryable(context) do
    assert {:ok, [update]} = decoded(context.transactions, context.results)
    import_before_upgrade(context.block, update)
    assert :ok = Ecto.Migrator.up(Repo, @version, BackfillPaxeerXPriceAcceptance, log: false)
    {:ok, worker} = PriceAcceptanceBackfill.start_link()
    assert {:error, :no_urls} = PriceAcceptanceBackfill.step(worker, 18)
    assert cursor() == [[-1, -1, false, ":no_urls"]]
    assert unknown_count() == 18
    assert :ok = GenServer.stop(worker)

    {:ok, reopened} = PriceAcceptanceBackfill.start_link()
    assert :ok = PriceAcceptanceBackfill.retain(reopened, @height, context.transactions, context.results)
    assert {:ok, 18} = PriceAcceptanceBackfill.step(reopened, 18)
    assert cursor() == [[0, 17, true, nil]]
    assert :ok = GenServer.stop(reopened)
  end

  test "historical_price_acceptance_replays_failed_transactions", context do
    results = Enum.map(context.results, &Map.put(&1, "code", 5))
    assert {:ok, [update]} = decoded(context.transactions, results)
    assert update.status == 0
    import_before_upgrade(context.block, update)
    PriceFeed.upsert_markets([%{market_id: 0, symbol: "BTC", chainflow_symbol: "BTC"}])
    assert :ok = Ecto.Migrator.up(Repo, @version, BackfillPaxeerXPriceAcceptance, log: false)

    {:ok, worker} = PriceAcceptanceBackfill.start_link()
    assert :ok = PriceAcceptanceBackfill.retain(worker, @height, context.transactions, results)
    assert {:ok, 18} = PriceAcceptanceBackfill.step(worker, 18)
    assert unknown_count() == 0
    assert [%{observations: observations, status: 0}] = PriceFeed.block_updates(@height)
    assert length(observations) == 18
    assert Enum.all?(observations, &(&1.accepted == false))
    assert PriceFeed.market_updates(0, %PagingOptions{}) == []
    assert [{%PriceMarket{market_id: 0}, nil}] = PriceFeed.latest_per_market()
    assert :ok = GenServer.stop(worker)
  end

  defp import_before_upgrade(block, update) do
    legacy = Map.update!(update, :observations, &Enum.map(&1, fn observation -> Map.delete(observation, :accepted) end))
    assert {:ok, 1} = PriceFeed.replace_block_updates(block, [legacy])
  end

  defp decoded(transactions, results) do
    with {:ok, submissions} <- CosmosRest.decode_block_txs(transactions) do
      CosmosRest.decode_price_updates(submissions, results, length(transactions["raw"]))
    end
  end

  defp refuse_market([result], market_id, encode) do
    accepted =
      Enum.find(result["events"], fn event ->
        event["type"] == "pricefeed_observation" and
          Enum.any?(event["attributes"], fn attribute ->
            attribute["key"] == Base.encode64("market_id") and
              attribute["value"] == Base.encode64(Integer.to_string(market_id))
          end)
      end)

    attributes =
      Map.new(accepted["attributes"], fn attribute ->
        {Base.decode64!(attribute["key"]), Base.decode64!(attribute["value"])}
      end)

    refusal = %{
      "type" => "pricefeed_observation_refused",
      "attributes" =>
        Enum.map(
          [
            {"validator", attributes["validator"]},
            {"market_id", attributes["market_id"]},
            {"sequence", attributes["sequence"]},
            {"reason", "sequence"}
          ],
          fn {key, value} ->
            %{"key" => encode.(key), "value" => encode.(value), "index" => true}
          end
        )
    }

    [Map.update!(result, "events", &Enum.map(&1, fn event -> if event == accepted, do: refusal, else: event end))]
  end

  defp unknown_count do
    Repo.aggregate(from(o in PriceObservation, where: o.block_number == @height and is_nil(o.accepted)), :count)
  end

  defp cursor do
    Repo.query!(
      """
      SELECT through_index, through_market_id, completed, error
      FROM paxeer_x_price_acceptance_backfill WHERE block_number = $1
      """,
      [@height]
    ).rows
  end

  defp accepted_column do
    Repo.query!("""
    SELECT column_default, is_nullable FROM information_schema.columns
    WHERE table_schema = 'public' AND table_name = 'paxeer_x_price_observations' AND column_name = 'accepted'
    """)
  end

  defp fixture(name), do: @fixtures |> Path.join(name) |> File.read!() |> Jason.decode!()
end
