defmodule BlockScoutWeb.Repair223512Test do
  use BlockScoutWeb.ConnCase

  alias Explorer.Chain.Cache.Counters.{
    AddressesCount,
    AverageBlockTime,
    NewPendingTransactionsCount,
    Transactions24hCount,
    TransactionsCount
  }

  alias Explorer.Chain.PaxeerX.PriceFeed
  alias Explorer.Chain.Transaction
  alias Explorer.PaxeerX.CosmosRest
  alias Explorer.Repo

  @fixtures Path.expand("../../../explorer/test/support/fixture/paxeer_x/price_feed", __DIR__)
  @count_fields ~w(transactions_count_24h price_updates_count_24h total_price_updates_count total_transactions_count)

  setup do
    body = @fixtures |> Path.join("block_txs.json") |> File.read!() |> Jason.decode!()
    results = @fixtures |> Path.join("block_results.json") |> File.read!() |> Jason.decode!()
    raw = body["block"]["data"]["txs"]

    {:ok, submissions} = CosmosRest.decode_block_txs(%{"txs" => body["txs"], "raw" => raw})
    {:ok, [update]} = CosmosRest.decode_price_updates(submissions, results["txs_results"], length(raw))

    average_block_time_config = Application.fetch_env!(:explorer, AverageBlockTime)
    Application.put_env(:explorer, AverageBlockTime, Keyword.put(average_block_time_config, :enabled, true))

    on_exit(fn ->
      Application.put_env(:explorer, AverageBlockTime, average_block_time_config)
      Supervisor.terminate_child(Explorer.Supervisor, TransactionsCount.child_id())
      Supervisor.restart_child(Explorer.Supervisor, TransactionsCount.child_id())
    end)

    start_supervised!(AddressesCount)
    start_supervised!(AverageBlockTime)

    %{update: update}
  end

  test "transaction statistics return zero integer strings for an empty chain", %{conn: conn} do
    consolidate_counts()

    response = conn |> get("/api/v2/transactions/stats") |> json_response(200)

    assert Map.take(response, @count_fields) == %{
             "transactions_count_24h" => "0",
             "price_updates_count_24h" => "0",
             "total_price_updates_count" => "0",
             "total_transactions_count" => "0"
           }
  end

  test "transaction statistics include price updates in the day and lifetime counts", %{conn: conn, update: update} do
    now = DateTime.utc_now()
    recent = insert(:block, number: 40, timestamp: DateTime.add(now, -23 * 3600, :second))
    old = insert(:block, number: 41, timestamp: DateTime.add(now, -25 * 3600, :second))

    for _ <- 1..2 do
      insert(:transaction, gas_price: 2) |> with_block(recent, gas_used: 21_000, status: :ok)
    end

    insert(:transaction, gas_price: 100) |> with_block(old, gas_used: 21_000, status: :ok)
    insert(:transaction)

    assert {:ok, 2} = PriceFeed.replace_block_updates(recent, [update, another_update(update)])
    assert {:ok, 1} = PriceFeed.replace_block_updates(old, [another_update(update)])
    consolidate_counts()

    response = conn |> get("/api/v2/transactions/stats") |> json_response(200)

    assert Map.take(response, @count_fields) == %{
             "transactions_count_24h" => "4",
             "price_updates_count_24h" => "2",
             "total_price_updates_count" => "3",
             "total_transactions_count" => "7"
           }

    assert response["pending_transactions_count"] == "1"
    assert Decimal.equal?(Decimal.new(response["transaction_fees_sum_24h"]), 84_000)
    assert Decimal.equal?(Decimal.new(response["transaction_fees_avg_24h"]), 42_000)

    stats = conn |> get("/api/v2/stats") |> json_response(200)
    assert stats["total_transactions"] == "7"
    assert response["total_transactions_count"] == stats["total_transactions"]
  end

  test "lifetime statistics retain price updates removed by retention", %{conn: conn, update: update} do
    now = DateTime.utc_now()
    recent = insert(:block, number: 41, timestamp: DateTime.add(now, -3600, :second))
    old = insert(:block, number: 40, timestamp: DateTime.add(now, -40 * 86_400, :second))

    assert {:ok, 1} = PriceFeed.replace_block_updates(old, [update])
    assert {:ok, 1} = PriceFeed.replace_block_updates(recent, [another_update(update)])
    assert PriceFeed.delete_expired_updates(30, 10) == 1
    consolidate_counts()

    response = conn |> get("/api/v2/transactions/stats") |> json_response(200)

    assert Map.take(response, @count_fields) == %{
             "transactions_count_24h" => "1",
             "price_updates_count_24h" => "1",
             "total_price_updates_count" => "2",
             "total_transactions_count" => "2"
           }

    assert (conn |> get("/api/v2/stats") |> json_response(200))["total_transactions"] == "2"
  end

  defp consolidate_counts do
    Transactions24hCount.consolidate()
    NewPendingTransactionsCount.consolidate()
    TransactionsCount.set_count(Repo.aggregate(Transaction, :count, :hash))
  end

  defp another_update(update) do
    %{update | hash: "0x" <> Base.encode16(:crypto.strong_rand_bytes(32), case: :lower), index: update.index + 1}
  end
end
