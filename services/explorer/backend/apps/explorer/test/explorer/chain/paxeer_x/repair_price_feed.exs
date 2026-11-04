Code.require_file("repair_transport.exs", __DIR__)

defmodule Explorer.Chain.PaxeerX.Repair2233523.PriceFeedCases do
  import ExUnit.Assertions
  import ExUnit.Callbacks
  import Ecto.Query
  import Explorer.Factory
  alias Explorer.Repo
  alias Explorer.Chain.PaxeerX.{PriceFeed, PriceFeedScannedBlock}
  alias Explorer.Chain.Cache.Counters.LastFetchedCounter
  alias Explorer.Chain.PaxeerX.Repair2233523.Transport
  alias Indexer.Fetcher.PaxeerX.PriceUpdates
  alias Indexer.BufferedTask

  @fixtures Path.expand("../../../support/fixture/paxeer_x/price_feed", __DIR__)

  def price_feed_concurrent_compaction do
    start = 5_233_523
    counter = "paxeer_x_price_feed_scanned_through_#{start}"
    {:ok, coordinator} = Repo.start_link(name: nil, pool: DBConnection.ConnectionPool, pool_size: 1)
    previous = Repo.put_dynamic_repo(coordinator)

    try do
      Repo.insert_all(PriceFeedScannedBlock, Enum.map(start..(start + 5), &%{block_number: &1, status: "scanned"}))
      assert :ok = PriceFeed.mark_unscannable(start + 6, "failed", "recorded node refusal")
      Repo.query!("SELECT pg_advisory_lock(hashtextextended($1, 0))", [counter])
      parent = self()

      workers =
        for _ <- 1..2 do
          Task.async(fn ->
            {:ok, connection} = Repo.start_link(name: nil, pool: DBConnection.ConnectionPool, pool_size: 1)
            Repo.put_dynamic_repo(connection)

            try do
              %{rows: [[backend]]} = Repo.query!("SELECT pg_backend_pid()")
              send(parent, {:connected, self(), backend})

              receive do
                :compact -> :ok
              after
                5_000 -> flunk("compaction was not released")
              end

              result = PriceFeed.compact_scanned_blocks(start, 100)
              assert result == start + 6
              assert PriceFeed.scanned_any?()
              {backend, result}
            after
              Supervisor.stop(connection)
            end
          end)
        end

      backends =
        for worker <- workers do
          assert_receive {:connected, pid, backend}, 5_000
          assert pid in Enum.map(workers, & &1.pid)
          send(pid, :compact)
          {pid, backend}
        end

      assert backends |> Enum.map(&elem(&1, 1)) |> Enum.uniq() |> length() == 2
      backend_ids = Enum.map(backends, &elem(&1, 1))

      try do
        await(fn ->
          %{rows: [[waiting]]} =
            Repo.query!("SELECT count(*) FROM pg_locks WHERE pid = ANY($1) AND locktype = 'advisory' AND NOT granted", [
              backend_ids
            ])

          waiting == 2
        end)
      after
        Repo.query!("SELECT pg_advisory_unlock(hashtextextended($1, 0))", [counter])
      end

      assert Enum.map(workers, &Task.await(&1, 10_000)) |> Enum.map(&elem(&1, 1)) == [start + 6, start + 6]
      assert PriceFeed.advance_scanned_through(start, start + 1) == start + 6
      assert PriceFeed.scanned_through(start) == start + 6

      refute Repo.exists?(
               from(row in PriceFeedScannedBlock,
                 where: row.block_number >= ^start and row.block_number <= ^(start + 5)
               )
             )

      assert Repo.get!(PriceFeedScannedBlock, start + 6).status == "failed"
    after
      Repo.delete_all(
        from(row in PriceFeedScannedBlock, where: row.block_number >= ^start and row.block_number <= ^(start + 6))
      )

      Repo.delete_all(from(row in LastFetchedCounter, where: row.counter_type == ^counter))
      Repo.put_dynamic_repo(previous)
      Supervisor.stop(coordinator)
    end
  end

  def price_feed_restarted_fetcher do
    start = 523_000
    Enum.each(start..(start + 2), &insert(:block, number: &1))

    for number <- start..(start + 1) do
      assert {:ok, 0} = PriceFeed.replace_block_updates(Repo.get_by!(Explorer.Chain.Block, number: number), [])
    end

    assert :ok = PriceFeed.mark_unscannable(start + 2, "failed", "recorded node refusal")
    assert PriceFeed.compact_scanned_blocks(start, 100) == start + 2
    assert PriceFeed.scanned_any?()
    refute Repo.get(PriceFeedScannedBlock, start)
    server = Bypass.open()
    parent = self()
    {:ok, mode} = Agent.start_link(fn -> :refuse end)
    empty = Path.join(@fixtures, "block_txs_empty.json") |> File.read!()

    Bypass.expect(server, "GET", "/cosmos/tx/v1beta1/txs/block/#{start + 2}", fn conn ->
      current = Agent.get(mode, & &1)
      send(parent, {:price_scan, current})
      assert conn.query_string == "pagination.offset=0&pagination.limit=100"

      if current == :refuse,
        do: Plug.Conn.resp(conn, 400, ~s({"code":13,"message":"internal"})),
        else: Plug.Conn.resp(conn, 400, empty)
    end)

    state = %{
      json_rpc_named_arguments: Transport.arguments("http://127.0.0.1:#{server.port}/"),
      cosmos_rest_urls: ["http://127.0.0.1:#{server.port}"],
      cometbft_rpc_urls: ["http://127.0.0.1:#{server.port}"],
      start_block: start,
      precompile: "0x0000000000000000000000000000000000001020",
      markets_refresh_seconds: 300,
      markets_attempted_at: PriceUpdates.refresh_clock()
    }

    task_supervisor = start_supervised!({Task.Supervisor, name: __MODULE__.Tasks})

    options = [
      {PriceUpdates,
       state: state,
       poll: false,
       flush_interval: 20,
       max_batch_size: 1,
       max_concurrency: 1,
       task_supervisor: task_supervisor},
      [name: PriceUpdates]
    ]

    config = Application.get_env(:indexer, Indexer.Supervisor)
    fetcher_config = Application.get_env(:indexer, PriceUpdates.Supervisor)
    Application.put_env(:indexer, Indexer.Supervisor, enabled: true)
    Application.put_env(:indexer, PriceUpdates.Supervisor, disabled?: false)

    try do
      first = start_supervised!({BufferedTask, options})
      assert_receive {:price_scan, :refuse}, 5_000
      assert_receive {:price_scan, :refuse}, 5_000

      await(fn ->
        %{task_ref_to_batch: tasks, init_task: initial} = GenServer.call(first, :state)
        initial == :complete and map_size(tasks) == 0
      end)

      assert Repo.get!(PriceFeedScannedBlock, start + 2).status == "failed"
      assert :ok = stop_supervised(BufferedTask)
      refute Process.alive?(first)
      Agent.update(mode, fn _ -> :serve end)
      restarted = start_supervised!({BufferedTask, options})
      refute restarted == first
      assert_receive {:price_scan, :serve}, 5_000
      await(fn -> Repo.get!(PriceFeedScannedBlock, start + 2).status == "scanned" end)
      assert PriceFeed.compact_scanned_blocks(start, 100) == start + 2
      assert PriceFeed.scanned_through(start) == start + 2
      assert PriceFeed.scanned_any?()
      assert PriceUpdates.init([], &[&1 | &2], state) == []

      refute Repo.exists?(
               from(row in PriceFeedScannedBlock,
                 where: row.block_number >= ^start and row.block_number <= ^(start + 2)
               )
             )

      response =
        BlockScoutWeb.API.V2.PaxeerX.CapabilitiesController.capabilities(
          Plug.Test.conn(:get, "/api/v2/paxeer-x/capabilities"),
          %{}
        )

      assert response.status == 200
      assert Jason.decode!(response.resp_body)["price_feed"] == true
      assert :ok = stop_supervised(BufferedTask)
    after
      Agent.stop(mode)
      Transport.restore(:indexer, Indexer.Supervisor, config)
      Transport.restore(:indexer, PriceUpdates.Supervisor, fetcher_config)
    end
  end

  defp await(predicate, remaining \\ 500) do
    cond do
      predicate.() ->
        :ok

      remaining == 0 ->
        flunk("production component did not reach its expected state")

      true ->
        Process.sleep(10)
        await(predicate, remaining - 1)
    end
  end
end
