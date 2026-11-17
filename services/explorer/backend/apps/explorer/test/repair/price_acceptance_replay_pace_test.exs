for file <- [
      "20261104000000_backfill_paxeer_x_price_acceptance.exs",
      "scripts/price_acceptance_backfill.exs"
    ] do
  Code.require_file(Path.expand("../../priv/repo/migrations/#{file}", __DIR__))
end

defmodule Explorer.Repair.PriceAcceptanceReplayPaceTest do
  use ExUnit.Case, async: false

  import Ecto.Query
  import Explorer.Factory

  alias Explorer.Chain.Block
  alias Explorer.PaxeerX.CosmosRest
  alias Explorer.Repo
  alias Explorer.Repo.Migrations.{BackfillPaxeerXPriceAcceptance, PriceAcceptanceBackfill}

  @version 20_261_104_000_000
  @base 60_000_000

  setup do
    Explorer.MigrationCase.start_owner!()
    assert Ecto.Migrator.down(Repo, @version, BackfillPaxeerXPriceAcceptance, log: false) in [:ok, :already_down]
    assert :ok = Ecto.Migrator.up(Repo, @version, BackfillPaxeerXPriceAcceptance, log: false)

    on_exit(fn ->
      Repo.delete_all(from(b in Block, where: b.number >= @base and b.number < @base + 16))
      assert Ecto.Migrator.down(Repo, @version, BackfillPaxeerXPriceAcceptance, log: false) in [:ok, :already_down]
      assert Ecto.Migrator.up(Repo, @version, BackfillPaxeerXPriceAcceptance, log: false) in [:ok, :already_up]
      Ecto.Adapters.SQL.Sandbox.mode(Repo, :auto)
    end)

    :ok
  end

  test "a base height envelope falls through to the next node" do
    first = Bypass.open()
    second = Bypass.open()
    message = "height is not available (requested height: 1000, base height: 1050)"

    Bypass.expect(first, "GET", "/block_results", fn conn ->
      Plug.Conn.resp(conn, 200, Jason.encode!(%{"code" => 2, "message" => message, "data" => %{}}))
    end)

    Bypass.expect(second, "GET", "/block_results", fn conn ->
      Plug.Conn.resp(conn, 200, Jason.encode!(%{"result" => %{"txs_results" => []}}))
    end)

    assert {:ok, []} = CosmosRest.fetch_block_results([url(first), url(second)], 1000)
  end

  test "all nodes with a base height envelope report unavailable" do
    first = Bypass.open()
    second = Bypass.open()
    message = "height is not available (requested height: 1000, base height: 1050)"
    body = Jason.encode!(%{"code" => 2, "message" => message, "data" => %{}})

    for bypass <- [first, second] do
      Bypass.expect(bypass, "GET", "/block_results", fn conn -> Plug.Conn.resp(conn, 200, body) end)
    end

    assert {:error, {:height_unavailable, _}} = CosmosRest.fetch_block_results([url(first), url(second)], 1000)
  end

  test "a step completes at most the configured number of blocks" do
    blocks = seed_jobs(6)
    worker = start_supervised!({PriceAcceptanceBackfill, [acceptance_backfill_concurrency: 3]})

    assert {:ok, 0} = PriceAcceptanceBackfill.step(worker, 18)
    assert completed_numbers() == Enum.map(Enum.take(blocks, 3), & &1.number)
  end

  test "a failed block is held while the other blocks in the step complete" do
    blocks = seed_jobs(6, 2)
    worker = start_supervised!({PriceAcceptanceBackfill, [acceptance_backfill_concurrency: 3]})

    assert {:error, :malformed_retained_block} = PriceAcceptanceBackfill.step(worker, 18)
    assert completed_numbers() == [hd(blocks).number, Enum.at(blocks, 2).number]
    assert held_numbers() == [Enum.at(blocks, 1).number]
  end

  test "simultaneous steps do not replay a block twice" do
    seed_jobs(6)
    worker = start_supervised!({PriceAcceptanceBackfill, [acceptance_backfill_concurrency: 3]})

    tasks = for _ <- 1..2, do: Task.async(fn -> PriceAcceptanceBackfill.step(worker, 18) end)
    assert Enum.map(tasks, &Task.await(&1, 5_000)) == [{:ok, 0}, {:ok, 0}]
    assert Repo.aggregate(from(j in "paxeer_x_price_acceptance_backfill", where: j.completed == true), :count) == 6
    assert Repo.query!("SELECT count(*) FROM paxeer_x_price_acceptance_backfill WHERE error IS NOT NULL").rows == [[0]]
  end

  test "a step reads blocks side by side up to the replay concurrency" do
    blocks = seed_jobs(16, nil, false)
    node = slow_node(25)

    serial =
      start_supervised!({PriceAcceptanceBackfill, [acceptance_backfill_concurrency: 1, cometbft_rpc_urls: [url(node)]]})

    serial_elapsed =
      elapsed(fn ->
        for _ <- 1..8 do
          assert {:ok, 0} = PriceAcceptanceBackfill.step(serial, 18)
        end
      end)

    serial_payloads = payloads(Enum.take(blocks, 8))
    stop_supervised!(PriceAcceptanceBackfill)

    parallel =
      start_supervised!({PriceAcceptanceBackfill, [acceptance_backfill_concurrency: 8, cometbft_rpc_urls: [url(node)]]})

    parallel_elapsed = elapsed(fn -> assert {:ok, 0} = PriceAcceptanceBackfill.step(parallel, 18) end)

    assert parallel_elapsed * 2 < serial_elapsed
    assert payloads(Enum.take(blocks, -8)) == serial_payloads
  end

  test "a failed node read is held while the other reads in the step apply" do
    blocks = seed_jobs(3, 2, false)
    failed_height = Enum.at(blocks, 1).number
    node = slow_node(1, failed_height)

    worker =
      start_supervised!({PriceAcceptanceBackfill, [acceptance_backfill_concurrency: 3, cometbft_rpc_urls: [url(node)]]})

    assert {:error, {:malformed, _}} = PriceAcceptanceBackfill.step(worker, 18)
    assert completed_numbers() == [hd(blocks).number, Enum.at(blocks, 2).number]
    assert held_numbers() == [failed_height]
  end

  defp seed_jobs(count, malformed \\ nil, retained \\ true) do
    for offset <- 0..(count - 1) do
      number = @base + offset
      block = insert(:block, number: number, consensus: true)

      transactions =
        cond do
          not retained -> nil
          offset + 1 == malformed -> %{"unexpected" => []}
          true -> %{"txs" => [], "raw" => []}
        end

      results = if retained, do: %{"txs_results" => []}, else: nil

      Repo.query!(
        "INSERT INTO paxeer_x_price_acceptance_backfill (block_hash, block_number, transactions, results) VALUES ($1, $2, $3, $4)",
        [block.hash.bytes, number, transactions, results]
      )

      block
    end
  end

  defp completed_numbers do
    Repo.query!(
      "SELECT block_number FROM paxeer_x_price_acceptance_backfill WHERE completed = true ORDER BY block_number"
    ).rows
    |> List.flatten()
  end

  defp held_numbers do
    Repo.query!(
      "SELECT block_number FROM paxeer_x_price_acceptance_backfill WHERE completed = false AND error IS NOT NULL ORDER BY block_number"
    ).rows
    |> List.flatten()
  end

  defp payloads(blocks) do
    Enum.map(blocks, fn block ->
      %{rows: [[transactions, results]]} =
        Repo.query!(
          "SELECT transactions, results FROM paxeer_x_price_acceptance_backfill WHERE block_number = $1",
          [block.number]
        )

      {transactions, results}
    end)
  end

  defp slow_node(delay, failed_height \\ nil) do
    bypass = Bypass.open()

    Bypass.expect(bypass, fn conn ->
      Process.sleep(delay)
      height = conn.query_string |> URI.decode_query() |> Map.fetch!("height") |> String.to_integer()

      response =
        if conn.request_path == "/block" and height == failed_height do
          %{"unexpected" => []}
        else
          case conn.request_path do
            "/block" -> %{"block" => %{"data" => %{"txs" => nil}}}
            "/block_results" -> %{"result" => %{"txs_results" => []}}
          end
        end

      Plug.Conn.resp(conn, 200, Jason.encode!(response))
    end)

    bypass
  end

  defp elapsed(fun) do
    started = System.monotonic_time(:millisecond)
    fun.()
    System.monotonic_time(:millisecond) - started
  end

  defp url(bypass), do: "http://localhost:#{bypass.port}"
end
