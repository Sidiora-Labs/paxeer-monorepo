for file <- [
      "20261028000000_add_paxeer_x_price_observation_accepted.exs",
      "20261104000000_backfill_paxeer_x_price_acceptance.exs",
      "20261115000000_relax_paxeer_x_price_observation_accepted.exs",
      "scripts/price_acceptance_backfill.exs"
    ] do
  Code.require_file(Path.expand("../../priv/repo/migrations/#{file}", __DIR__))
end

defmodule Explorer.Repair.PriceAcceptanceColumnTest do
  use ExUnit.Case, async: false

  import Ecto.Query
  import Explorer.Factory

  alias Explorer.Chain.Block
  alias Explorer.Chain.Cache.Counters.LastFetchedCounter
  alias Explorer.Chain.PaxeerX.{PriceFeed, PriceFeedScannedBlock, PriceObservation}
  alias Explorer.PaxeerX.CosmosRest
  alias Explorer.Repo

  alias Explorer.Repo.Migrations.{
    BackfillPaxeerXPriceAcceptance,
    PriceAcceptanceBackfill,
    RelaxPaxeerXPriceObservationAccepted
  }

  @queue_version 20_261_104_000_000
  @version 20_261_115_000_000
  @height 37_715_713
  @fixtures Path.expand("../support/fixture/paxeer_x/price_feed", __DIR__)
  @index "paxeer_x_price_acceptance_backfill_next_job_index"

  setup do
    Explorer.TestHelper.start_migration_owner!()
    assert :ok = Ecto.Migrator.down(Repo, @version, RelaxPaxeerXPriceObservationAccepted, log: false)
    assert :ok = Ecto.Migrator.down(Repo, @queue_version, BackfillPaxeerXPriceAcceptance, log: false)

    body = fixture("block_txs.json")
    results = refuse_market(fixture("block_results.json")["txs_results"], 2)
    transactions = %{"txs" => body["txs"], "raw" => body["block"]["data"]["txs"]}
    {:ok, submissions} = CosmosRest.decode_block_txs(transactions)

    {:ok, [update]} =
      CosmosRest.decode_price_updates(submissions, results, length(transactions["raw"]))

    block = insert(:block, number: @height, consensus: true)

    on_exit(fn ->
      Repo.delete_all(from(b in Block, where: b.hash == ^block.hash))
      Repo.delete_all(from(s in PriceFeedScannedBlock, where: s.block_number == @height))
      LastFetchedCounter.delete("paxeer_x_price_update_total")
      Ecto.Migrator.up(Repo, @version, RelaxPaxeerXPriceObservationAccepted, log: false)
      Ecto.Migrator.up(Repo, @queue_version, BackfillPaxeerXPriceAcceptance, log: false)
    end)

    %{block: block, transactions: transactions, results: results, update: update}
  end

  test "accepted is nullable without a default and replay keeps node decisions", context do
    assert {:ok, :ok} =
             Repo.transaction(fn ->
               Repo.query!("ALTER TABLE paxeer_x_price_observations ALTER COLUMN accepted SET DEFAULT true")

               Repo.query!("ALTER TABLE paxeer_x_price_observations ALTER COLUMN accepted SET NOT NULL")
               :ok
             end)

    legacy =
      Map.update!(
        context.update,
        :observations,
        &Enum.map(&1, fn observation -> Map.delete(observation, :accepted) end)
      )

    assert {:ok, 1} = PriceFeed.replace_block_updates(context.block, [legacy])
    assert Enum.all?(Repo.all(from(o in PriceObservation, select: o.accepted)), &(&1 == true))

    assert :ok = Ecto.Migrator.up(Repo, @queue_version, BackfillPaxeerXPriceAcceptance, log: false)
    before_migration = observation_rows()

    assert :ok = Ecto.Migrator.up(Repo, @version, RelaxPaxeerXPriceObservationAccepted, log: false)
    assert observation_rows() == before_migration
    assert %{rows: [[nil, "YES"]]} = accepted_column()
    assert_index()

    state = {accepted_column(), index_state()}
    assert :already_up = Ecto.Migrator.up(Repo, @version, RelaxPaxeerXPriceObservationAccepted, log: false)
    assert {accepted_column(), index_state()} == state

    worker = start_supervised!({PriceAcceptanceBackfill, []})
    assert :ok = PriceAcceptanceBackfill.retain(worker, @height, context.transactions, context.results)
    assert {:ok, 18} = PriceAcceptanceBackfill.step(worker, 18)
    assert :ok = stop_supervised(PriceAcceptanceBackfill)

    assert %{rows: [[true]]} =
             Repo.query!("SELECT completed FROM paxeer_x_price_acceptance_backfill WHERE block_number = $1", [@height])

    assert %{rows: [[true], [false]]} =
             Repo.query!(
               """
               SELECT accepted FROM paxeer_x_price_observations
               WHERE block_number = $1 AND market_id IN (0, 2)
               ORDER BY market_id
               """,
               [@height]
             )
  end

  defp accepted_column do
    Repo.query!("""
    SELECT column_default, is_nullable
    FROM information_schema.columns
    WHERE table_schema = 'public'
      AND table_name = 'paxeer_x_price_observations'
      AND column_name = 'accepted'
    """)
  end

  defp index_state do
    Repo.query!(
      """
      SELECT i.indkey::text, pg_get_indexdef(i.indexrelid), pg_get_expr(i.indpred, i.indrelid)
      FROM pg_index i
      JOIN pg_class c ON c.oid = i.indexrelid
      WHERE c.relname = $1
      """,
      [@index]
    ).rows
  end

  defp assert_index do
    assert [["0 2 1", definition, predicate]] = index_state()
    assert definition =~ ~r/error IS NOT NULL.*block_number.*block_hash/s
    assert predicate =~ "completed = false"
  end

  defp observation_rows do
    Repo.query!(
      """
      SELECT market_id, accepted, xmin::text
      FROM paxeer_x_price_observations
      WHERE block_number = $1
      ORDER BY market_id
      """,
      [@height]
    ).rows
  end

  defp refuse_market([result], market_id) do
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
            %{"key" => Base.encode64(key), "value" => Base.encode64(value), "index" => true}
          end
        )
    }

    [Map.update!(result, "events", &Enum.map(&1, fn event -> if event == accepted, do: refusal, else: event end))]
  end

  defp fixture(name), do: @fixtures |> Path.join(name) |> File.read!() |> Jason.decode!()
end
