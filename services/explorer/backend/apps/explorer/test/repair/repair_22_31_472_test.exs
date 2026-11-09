for file <- [
      "20261104000000_backfill_paxeer_x_price_acceptance.exs",
      "scripts/price_acceptance_backfill.exs"
    ] do
  Code.require_file(Path.expand("../../priv/repo/migrations/#{file}", __DIR__))
end

defmodule Explorer.Repair.Repair2231472Test do
  use ExUnit.Case, async: false

  import Ecto.Query
  import Explorer.Factory

  alias Explorer.Chain.Block
  alias Explorer.Chain.Import.Runner.Blocks
  alias Explorer.Chain.Cache.Counters.LastFetchedCounter
  alias Explorer.Chain.PaxeerX.{PriceFeed, PriceFeedScannedBlock, PriceObservation, PriceUpdate}
  alias Explorer.PaxeerX.CosmosRest
  alias Explorer.Repo
  alias Explorer.Repo.Migrations.{BackfillPaxeerXPriceAcceptance, PriceAcceptanceBackfill}
  alias Indexer.Fetcher.PaxeerX.{PriceAcceptanceReplay, PriceFeedMaintenance, PriceUpdates}
  alias Indexer.Fetcher.PaxeerX.PriceUpdates.Supervisor, as: PriceUpdatesSupervisor

  @version 20_261_104_000_000
  @height 37_715_713
  @fixtures Path.expand("../support/fixture/paxeer_x/price_feed", __DIR__)

  defmodule RecordedNode do
    @behaviour Plug

    def init(options), do: options

    def call(conn, {body, results, observer, availability, role}) do
      send(observer, {:node_request, conn.request_path, conn.query_string})

      {status, response} =
        if Agent.get(availability, & &1) do
          case {role, conn.request_path} do
            {:rest, "/cosmos/tx/v1beta1/txs/block/37715713"} -> {200, body}
            {:rpc, "/block_results"} -> {200, results}
          end
        else
          {503, %{"message" => "recorded node temporarily unavailable"}}
        end

      conn
      |> Plug.Conn.put_resp_content_type("application/json")
      |> Plug.Conn.send_resp(status, Jason.encode!(response))
    end
  end

  setup do
    assert :ok = Ecto.Migrator.down(Repo, @version, BackfillPaxeerXPriceAcceptance, log: false)
    body = fixture("block_txs.json")
    result_body = fixture("block_results.json")
    transactions = %{"txs" => body["txs"], "raw" => body["block"]["data"]["txs"]}
    {:ok, submissions} = CosmosRest.decode_block_txs(transactions)

    {:ok, [update]} =
      CosmosRest.decode_price_updates(submissions, result_body["txs_results"], length(transactions["raw"]))

    block =
      insert(:block,
        number: @height,
        consensus: true,
        timestamp: DateTime.add(DateTime.utc_now(), -10 * 86_400, :second)
      )

    legacy = %{update | observations: Enum.map(update.observations, &Map.put(&1, :accepted, true))}
    assert {:ok, 1} = PriceFeed.replace_block_updates(block, [legacy])

    keys = [PriceUpdates, PriceUpdatesSupervisor]
    previous = Enum.map(keys, &{&1, Application.fetch_env(:indexer, &1)})

    on_exit(fn ->
      for {key, value} <- previous do
        case value do
          {:ok, config} -> Application.put_env(:indexer, key, config)
          :error -> Application.delete_env(:indexer, key)
        end
      end

      Repo.delete_all(from(b in Block, where: b.number >= @height and b.number <= @height + 30))

      Repo.delete_all(
        from(s in PriceFeedScannedBlock, where: s.block_number >= @height and s.block_number <= @height + 30)
      )

      LastFetchedCounter.delete("paxeer_x_price_update_total")
      LastFetchedCounter.delete("paxeer_x_price_feed_scanned_through_#{@height}")
      Ecto.Migrator.up(Repo, @version, BackfillPaxeerXPriceAcceptance, log: false)
    end)

    %{
      block: block,
      update: update,
      body: body,
      result_body: result_body,
      transactions: transactions,
      results: result_body["txs_results"]
    }
  end

  test "migration installs the queue without writing observation rows" do
    Repo.query!("UPDATE paxeer_x_price_observations SET accepted = false WHERE block_number = $1 AND market_id = 2", [
      @height
    ])

    Repo.query!("UPDATE paxeer_x_price_observations SET accepted = NULL WHERE block_number = $1 AND market_id = 17", [
      @height
    ])

    before_up = observations(@height)
    assert length(before_up) == 18

    assert Enum.map(before_up, fn [row, _xmin] -> row["accepted"] end) |> Enum.uniq() |> Enum.sort() == [
             false,
             nil,
             true
           ]

    migrate()

    assert observations(@height) == before_up
    assert cursor() == [[-1, -1, false, nil]]
    assert PriceAcceptanceBackfill.pending?()
  end

  test "one block resets only its observations and a restarted batch preserves prior decisions", context do
    other = insert(:block, number: @height + 1, consensus: true)
    other_update = %{context.update | hash: to_string(other.hash)}
    assert {:ok, 1} = PriceFeed.replace_block_updates(other, [other_update])
    other_before = observations(@height + 1)
    migrate()

    worker = start_supervised!({PriceAcceptanceBackfill, []})
    failed_results = Enum.map(context.results, &Map.put(&1, "code", 5))
    # The stored status must agree with the real failed-transaction decoder.
    Repo.query!("UPDATE paxeer_x_price_updates SET status = 0 WHERE block_hash = $1", [context.block.hash.bytes])
    assert :ok = PriceAcceptanceBackfill.retain(worker, @height, context.transactions, failed_results)
    assert {:ok, 9} = PriceAcceptanceBackfill.step(worker, 9)
    assert cursor() == [[0, 8, false, nil]]
    assert accepted_values() == List.duplicate(false, 9) ++ List.duplicate(nil, 9)
    assert observations(@height + 1) == other_before
    first_batch = observations(@height) |> Enum.take(9)

    stop_supervised!(PriceAcceptanceBackfill)
    reopened = start_supervised!({PriceAcceptanceBackfill, []})
    assert {:ok, 9} = PriceAcceptanceBackfill.step(reopened, 9)
    assert cursor() == [[0, 17, true, nil]]
    assert accepted_values() == List.duplicate(false, 18)
    assert Enum.take(observations(@height), 9) == first_batch
    assert observations(@height + 1) == other_before
  end

  test "supervisor runs configured endpoints, retries durably and stops replay at complete", context do
    migrate()
    {:ok, _} = Application.ensure_all_started(:plug_cowboy)
    availability = start_supervised!({Agent, fn -> false end})
    rest_url = serve(context, availability, :rest)
    rpc_url = serve(context, availability, :rpc)

    Application.put_env(:indexer, PriceUpdatesSupervisor, disabled?: false)

    Application.put_env(:indexer, PriceUpdates,
      start_block: @height,
      cosmos_rest_urls: [rest_url],
      cometbft_rpc_urls: [rpc_url],
      markets_refresh_seconds: 3600,
      acceptance_backfill_batch_size: 9,
      acceptance_backfill_interval: 1_000,
      acceptance_backfill_retry_interval: 25
    )

    supervisor = start_supervised!({PriceUpdatesSupervisor, [[json_rpc_named_arguments: []]]})

    {PriceAcceptanceReplay, runner, :worker, _} =
      List.keyfind(Supervisor.which_children(supervisor), PriceAcceptanceReplay, 0)

    assert is_pid(runner)
    monitor = Process.monitor(runner)
    assert_receive {:node_request, "/cosmos/tx/v1beta1/txs/block/37715713", _}, 5_000
    await(fn -> match?([[-1, -1, false, error]] when is_binary(error), cursor()) end)
    assert accepted_values() == List.duplicate(true, 18)
    assert PriceAcceptanceBackfill.pending?()

    Agent.update(availability, fn _ -> true end)
    assert_receive {:node_request, "/block_results", "height=37715713"}, 5_000
    await(fn -> cursor() == [[0, 8, false, nil]] end)
    assert Enum.count(accepted_values(), &is_nil/1) == 9
    first_batch = observations(@height) |> Enum.take(9)
    assert_receive {:DOWN, ^monitor, :process, ^runner, :normal}, 5_000
    assert cursor() == [[0, 17, true, nil]]
    assert Enum.take(observations(@height), 9) == first_batch
    refute PriceAcceptanceBackfill.pending?()

    await(fn ->
      {PriceAcceptanceReplay, :undefined, :worker, [PriceAcceptanceReplay]} in Supervisor.which_children(supervisor)
    end)

    assert Process.alive?(Process.whereis(PriceUpdates))
    assert Enum.all?(drain_node_requests(), fn {path, _query} -> path == "/cosmos/tx/v1beta1/txs/block/37715713" end)
    refute_receive {:node_request, _, _}, 350
  end

  test "retention waits for incomplete replay jobs and resumes after completion", context do
    migrate()
    newer = insert(:block, number: @height + 1, consensus: true)
    assert {:ok, 1} = PriceFeed.replace_block_updates(newer, [%{context.update | hash: to_string(newer.hash)}])
    assert %{expired: 0} = PriceFeedMaintenance.run_once(@height, 5)
    assert Repo.aggregate(PriceUpdate, :count) == 2
    assert Repo.aggregate(PriceObservation, :count) == 36

    worker = start_supervised!({PriceAcceptanceBackfill, []})
    assert :ok = PriceAcceptanceBackfill.retain(worker, @height, context.transactions, context.results)
    assert {:ok, 9} = PriceAcceptanceBackfill.step(worker, 9)
    assert %{expired: 0} = PriceFeedMaintenance.run_once(@height, 5)
    assert Repo.aggregate(PriceObservation, :count) == 36
    assert {:ok, 9} = PriceAcceptanceBackfill.step(worker, 9)
    assert :complete = PriceAcceptanceBackfill.step(worker, 9)
    assert %{expired: 1} = PriceFeedMaintenance.run_once(@height, 5)
    assert Repo.aggregate(PriceUpdate, :count) == 1
    assert Repo.aggregate(PriceObservation, :count) == 18
  end

  test "orphan_replay_retention_recovers_after_restart", context do
    orphan_replay_retention_recovers_after_restart(context)
  end

  def orphan_replay_retention_recovers_after_restart(context) do
    orphan = history(context, 10, 10)
    migrate()
    orphan_block(orphan)
    orphan_before = observations(orphan.number)
    history(context, 20, 10)
    history(context, 30, 0)

    options = replay_options(context)
    runner = start_supervised!({PriceAcceptanceReplay, options})
    worker = :sys.get_state(runner).worker
    assert :ok = PriceAcceptanceBackfill.retain(worker, orphan.number, context.transactions, context.results)
    payload = retained_payload(orphan.number)
    await(fn -> cursor() == [[0, 17, true, nil]] end)
    assert_receive {:node_request, "/cosmos/tx/v1beta1/txs/block/37715713", _}, 5_000
    assert_receive {:node_request, "/block_results", "height=37715713"}, 5_000
    assert :deferred = PriceAcceptanceBackfill.step(worker, 9)
    refute PriceAcceptanceBackfill.pending?()
    assert cursor(orphan.number) == [[-1, -1, false, nil]]
    assert observations(orphan.number) == orphan_before
    assert %{expired: 2} = PriceFeedMaintenance.run_once(@height, 5)
    assert observations(@height) == []
    assert observations(@height + 20) == []
    assert observations(orphan.number) == orphan_before
    refute_receive {:node_request, _, _}, 150
    assert Process.alive?(runner)

    stop_supervised!(PriceAcceptanceReplay)
    reopened = start_supervised!({PriceAcceptanceReplay, options})
    assert reopened != runner
    assert :deferred = PriceAcceptanceBackfill.step(:sys.get_state(reopened).worker, 9)
    history(context, 21, 10)
    assert %{expired: 1} = PriceFeedMaintenance.run_once(@height, 5)
    assert observations(@height + 21) == []
    assert observations(orphan.number) == orphan_before
    assert retained_payload(orphan.number) == payload
    assert cursor(orphan.number) == [[-1, -1, false, nil]]
    refute_receive {:node_request, _, _}, 150
    assert Process.alive?(reopened)
    stop_supervised!(PriceAcceptanceReplay)
  end

  test "migration_defers_preexisting_orphan_jobs", context do
    migration_defers_preexisting_orphan_jobs(context)
  end

  def migration_defers_preexisting_orphan_jobs(context) do
    orphan_block(context.block)
    before_up = observations(@height)
    migrate()
    assert observations(@height) == before_up
    assert cursor() == [[-1, -1, false, nil]]
    refute PriceAcceptanceBackfill.pending?()

    runner = start_supervised!({PriceAcceptanceReplay, replay_options(context)})
    worker = :sys.get_state(runner).worker
    assert :ok = PriceAcceptanceBackfill.retain(worker, @height, context.transactions, context.results)
    payload = retained_payload(@height)
    assert :deferred = PriceAcceptanceBackfill.step(worker, 9)
    history(context, 20, 10)
    history(context, 30, 0)
    assert %{expired: 1} = PriceFeedMaintenance.run_once(@height, 5)
    assert observations(@height + 20) == []
    assert observations(@height) == before_up
    assert length(before_up) == 18
    assert retained_payload(@height) == payload
    assert cursor() == [[-1, -1, false, nil]]
    refute_receive {:node_request, _, _}, 150
    assert Process.alive?(runner)
    stop_supervised!(PriceAcceptanceReplay)
  end

  test "orphan_replay_resumes_after_consensus_returns", context do
    orphan_replay_resumes_after_consensus_returns(context)
  end

  def orphan_replay_resumes_after_consensus_returns(context) do
    migrate()
    options = replay_options(context) |> Keyword.put(:acceptance_backfill_interval, 1_000)
    runner = start_supervised!({PriceAcceptanceReplay, options})
    await(fn -> cursor() == [[0, 8, false, nil]] end)
    :ok = :sys.suspend(runner)
    first_batch = Enum.take(observations(@height), 9)
    before_orphan = observations(@height)
    payload = retained_payload(@height)
    assert accepted_values() == List.duplicate(true, 9) ++ List.duplicate(nil, 9)
    orphan_block(context.block)
    assert :deferred = PriceAcceptanceBackfill.step(:sys.get_state(runner).worker, 9)
    refute PriceAcceptanceBackfill.pending?()
    assert observations(@height) == before_orphan
    assert cursor() == [[0, 8, false, nil]]
    assert_receive {:node_request, "/cosmos/tx/v1beta1/txs/block/37715713", _}, 5_000
    assert_receive {:node_request, "/block_results", "height=37715713"}, 5_000

    stop_supervised!(PriceAcceptanceReplay)
    reopened = start_supervised!({PriceAcceptanceReplay, options})
    assert reopened != runner
    assert :deferred = PriceAcceptanceBackfill.step(:sys.get_state(reopened).worker, 9)
    history(context, 20, 10)
    history(context, 30, 0)
    assert %{expired: 1} = PriceFeedMaintenance.run_once(@height, 5)
    assert observations(@height + 20) == []
    assert observations(@height) == before_orphan
    assert retained_payload(@height) == payload
    assert cursor() == [[0, 8, false, nil]]
    refute_receive {:node_request, _, _}, 150
    assert Process.alive?(reopened)

    :ok = :sys.suspend(reopened)
    import_consensus(context.block)
    assert PriceAcceptanceBackfill.pending?()
    assert %{expired: 0} = PriceFeedMaintenance.run_once(@height, 5)
    # Exercise source protection even when consensus changed after the guard ran.
    assert PriceFeed.delete_expired_updates(5, 100) == 0
    assert Repo.all(PriceFeed.oldest_updates_query(100)) == []
    assert observations(@height) == before_orphan
    assert cursor() == [[0, 8, false, nil]]
    monitor = Process.monitor(reopened)
    :ok = :sys.resume(reopened)
    assert_receive {:DOWN, ^monitor, :process, ^reopened, :normal}, 5_000
    assert cursor() == [[0, 17, true, nil]]
    assert Enum.take(observations(@height), 9) == first_batch
    assert accepted_values() == List.duplicate(true, 18)
    assert retained_payload(@height) == payload
    refute PriceAcceptanceBackfill.pending?()
    refute_receive {:node_request, _, _}, 150
    assert %{expired: 1} = PriceFeedMaintenance.run_once(@height, 5)
    assert observations(@height) == []
    assert length(observations(@height + 30)) == 18
  end

  defp history(context, offset, days) do
    block =
      insert(:block,
        number: @height + offset,
        consensus: true,
        timestamp: DateTime.add(DateTime.utc_now(), -days * 86_400, :second)
      )

    update = %{context.update | hash: to_string(block.hash)}
    assert {:ok, 1} = PriceFeed.replace_block_updates(block, [update])
    block
  end

  defp orphan_block(block) do
    competitor = insert(:block, number: block.number, parent_hash: block.parent_hash, consensus: false)
    import_consensus(competitor)
    refute Repo.get!(Block, block.hash).consensus
  end

  defp import_consensus(block) do
    params =
      params_for(:block,
        hash: block.hash,
        miner_hash: block.miner_hash,
        number: block.number,
        parent_hash: block.parent_hash,
        timestamp: block.timestamp,
        consensus: true
      )

    %Ecto.Changeset{valid?: true, changes: changes} = Block.changeset(%Block{}, params)
    now = DateTime.utc_now()

    assert {:ok, _} =
             Ecto.Multi.new()
             |> Blocks.run([changes], %{timestamps: %{inserted_at: now, updated_at: now}})
             |> Repo.transaction()

    assert Repo.get!(Block, block.hash).consensus
  end

  defp replay_options(context) do
    {:ok, _} = Application.ensure_all_started(:plug_cowboy)
    availability = start_supervised!({Agent, fn -> true end})

    [
      cosmos_rest_urls: [serve(context, availability, :rest)],
      cometbft_rpc_urls: [serve(context, availability, :rpc)],
      acceptance_backfill_batch_size: 9,
      acceptance_backfill_interval: 25,
      acceptance_backfill_retry_interval: 25
    ]
  end

  defp retained_payload(height) do
    %{rows: [[transactions, results]]} =
      Repo.query!(
        "SELECT transactions, results FROM paxeer_x_price_acceptance_backfill WHERE block_number = $1",
        [height]
      )

    assert is_map(transactions)
    assert is_map(results)
    {transactions, results}
  end

  defp migrate, do: assert(:ok = Ecto.Migrator.up(Repo, @version, BackfillPaxeerXPriceAcceptance, log: false))

  defp cursor(height \\ @height) do
    Repo.query!(
      "SELECT through_index, through_market_id, completed, error FROM paxeer_x_price_acceptance_backfill WHERE block_number = $1",
      [height]
    ).rows
  end

  defp observations(height) do
    Repo.query!(
      "SELECT to_jsonb(o), xmin::text FROM paxeer_x_price_observations o WHERE block_number = $1 ORDER BY market_id",
      [height]
    ).rows
  end

  defp accepted_values do
    Repo.all(from(o in PriceObservation, where: o.block_number == @height, order_by: o.market_id, select: o.accepted))
  end

  defp serve(context, availability, role) do
    ref = {RecordedNode, make_ref()}

    child =
      Plug.Cowboy.child_spec(
        scheme: :http,
        plug: {RecordedNode, {context.body, context.result_body, self(), availability, role}},
        options: [port: 0, ref: ref]
      )

    start_supervised!(%{child | id: ref})
    "http://127.0.0.1:#{:ranch.get_port(ref)}"
  end

  defp drain_node_requests do
    receive do
      {:node_request, path, query} -> [{path, query} | drain_node_requests()]
    after
      0 -> []
    end
  end

  defp await(condition, attempts \\ 500)
  defp await(condition, 0), do: assert(condition.())

  defp await(condition, attempts) do
    if condition.() do
      :ok
    else
      Process.sleep(10)
      await(condition, attempts - 1)
    end
  end

  defp fixture(name), do: @fixtures |> Path.join(name) |> File.read!() |> Jason.decode!()
end
