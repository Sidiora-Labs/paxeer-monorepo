for file <- [
      "20261104000000_backfill_paxeer_x_price_acceptance.exs",
      "scripts/price_acceptance_backfill.exs"
    ] do
  Code.require_file(Path.expand("../../priv/repo/migrations/#{file}", __DIR__))
end

defmodule Explorer.Repair.Repair2231490Test do
  use ExUnit.Case, async: false

  import Ecto.Query
  import ExUnit.CaptureLog
  import Explorer.Factory

  alias Explorer.Chain.Block
  alias Explorer.Chain.Cache.Counters.LastFetchedCounter
  alias Explorer.Chain.PaxeerX.{PriceFeed, PriceFeedScannedBlock, PriceObservation}
  alias Explorer.PaxeerX.CosmosRest
  alias Explorer.Repo
  alias Explorer.Repo.Migrations.{BackfillPaxeerXPriceAcceptance, PriceAcceptanceBackfill}
  alias Indexer.Fetcher.PaxeerX.PriceAcceptanceReplay

  @version 20_261_104_000_000
  @height 37_715_713
  @first @height - 2
  @held @height - 1
  @fixtures Path.expand("../support/fixture/paxeer_x/price_feed", __DIR__)

  defmodule RecordedNode do
    @behaviour Plug

    def init(options), do: options

    def call(conn, {observer, role, bodies, results, held}) do
      conn = Plug.Conn.fetch_query_params(conn)

      {kind, height} =
        case conn.path_info do
          ["block_results"] -> {:results, String.to_integer(conn.query_params["height"])}
          ["cosmos", "tx", "v1beta1", "txs", "block", height] -> {:transactions, String.to_integer(height)}
        end

      {status, response} =
        cond do
          role == :error or (kind == :results and height in held) ->
            {200, %{"code" => -32603, "message" => "height #{height} is not available"}}

          kind == :results ->
            {200, %{"result" => Map.fetch!(results, height), "code" => 0, "message" => ""}}

          true ->
            body = Map.fetch!(bodies, height)
            {if(Map.has_key?(body, "code"), do: 400, else: 200), body}
        end

      conn =
        conn
        |> Plug.Conn.put_resp_content_type("application/json")
        |> Plug.Conn.send_resp(status, Jason.encode!(response))

      send(observer, {:node_request, role, kind, height})
      conn
    end
  end

  setup tags do
    {:ok, _} = Application.ensure_all_started(:plug_cowboy)
    body = fixture("block_txs.json")
    result = fixture("block_results.json")
    empty_body = fixture("block_txs_empty.json")
    empty_result = fixture("block_results_empty.json")

    context = %{
      body: body,
      result: result,
      bodies: %{@first => empty_body, @held => empty_body, @height => body},
      results: %{@first => empty_result, @held => empty_result, @height => result}
    }

    if tags[:queue], do: seed_queue(context)
    context
  end

  test "block results fail over from a node error envelope to recorded execution results", context do
    first = serve(context, :error)
    second = serve(context, :available)

    assert {:ok, results} = CosmosRest.fetch_block_results([first, second], @height)
    assert results == context.result["txs_results"]
    assert results != []
    assert_receive {:node_request, :error, :results, @height}
    assert_receive {:node_request, :available, :results, @height}
  end

  test "block transactions fail over on the same envelope and preserve raw transaction bytes", context do
    first = serve(context, :error)
    second = serve(context, :available)

    assert {:ok, transactions} = CosmosRest.fetch_block_txs([first, second], @height)
    assert transactions == %{"txs" => context.body["txs"], "raw" => context.body["block"]["data"]["txs"]}
    assert_receive {:node_request, :error, :transactions, @height}
    assert_receive {:node_request, :available, :transactions, @height}
  end

  test "node errors retain their URL and message only after every endpoint has failed", context do
    first = serve(context, :error)
    second = serve(context, :available)
    message = "height #{@held} is not available"

    assert {:error, {:node_error, ^first, ^message}} = CosmosRest.fetch_block_results([first, second], @held)

    for _ <- 1..2 do
      assert_receive {:node_request, :error, :results, @held}
      assert_receive {:node_request, :available, :results, @held}
    end
  end

  @tag :queue
  test "replay advances beyond a held middle block, preserves its error and throttles held warnings", context do
    first = serve(context, :error)
    second = serve(context, :available)
    options = [cosmos_rest_urls: [first, second], cometbft_rpc_urls: [first, second]]
    worker = start_supervised!({PriceAcceptanceBackfill, options})

    assert PriceAcceptanceBackfill.held_jobs() == nil
    assert {:ok, 0} = PriceAcceptanceBackfill.step(worker, 18)
    assert {:error, {:node_error, endpoint, message}} = PriceAcceptanceBackfill.step(worker, 18)
    assert endpoint in [first, second]
    assert message == "height #{@held} is not available"
    assert [[@first, true, nil], [@held, false, error], [@height, false, nil]] = queue()
    assert error =~ endpoint
    assert error =~ message
    assert PriceAcceptanceBackfill.held_jobs() == {1, @held, error}

    assert {:ok, 18} = PriceAcceptanceBackfill.step(worker, 18)
    assert queue() == [[@first, true, nil], [@held, false, error], [@height, true, nil]]
    assert Repo.aggregate(from(o in PriceObservation, where: o.block_number == @height), :count) == 18

    assert Repo.aggregate(from(o in PriceObservation, where: o.block_number == @height and is_nil(o.accepted)), :count) ==
             0

    assert [%{observations: observations}] = PriceFeed.block_updates(@height)
    assert Enum.all?(observations, & &1.accepted)
    assert :ok = stop_supervised(PriceAcceptanceBackfill)

    log =
      capture_log([level: :warning], fn ->
        runner =
          start_supervised!(
            {PriceAcceptanceReplay,
             options ++ [acceptance_backfill_batch_size: 18, acceptance_backfill_retry_interval: 60_000]}
          )

        state = :sys.get_state(runner)
        assert is_integer(state.last_held_warning_at)

        for _ <- 1..2 do
          send(runner, :replay)
          assert :sys.get_state(runner).last_held_warning_at == state.last_held_warning_at
        end

        assert queue() == [[@first, true, nil], [@held, false, error], [@height, true, nil]]
        assert :ok = stop_supervised(PriceAcceptanceReplay)
      end)

    assert length(Regex.scan(~r/price acceptance replay held:/, log)) == 1
    assert log =~ "[warning]"
    assert log =~ "1 rows; lowest block #{@held}"
    assert log =~ message
    assert log =~ endpoint

    reopened = start_supervised!({PriceAcceptanceBackfill, options})
    assert :ok = PriceAcceptanceBackfill.retain(reopened, @held, %{"txs" => [], "raw" => []}, [])
    assert {:ok, 0} = PriceAcceptanceBackfill.step(reopened, 18)
    assert :complete = PriceAcceptanceBackfill.step(reopened, 18)
    assert PriceAcceptanceBackfill.held_jobs() == nil
    assert queue() == [[@first, true, nil], [@held, true, nil], [@height, true, nil]]
  end

  @tag :queue
  test "replay_database_outage_recovers", context do
    replay_database_outage_recovers(context)
  end

  def replay_database_outage_recovers(context) do
    {options, error} = prepare_held_replay(context)
    handler = observe_replay_queries()
    before_outage = durable_replay_state()

    log =
      capture_log([level: :warning], fn ->
        {runner, state, monitors} =
          with_queue_unavailable(fn ->
            runner = start_supervised!({PriceAcceptanceReplay, options})
            state = :sys.get_state(runner)
            assert state.last_held_warning_at == nil
            monitors = monitor_replay(runner, state.worker)
            assert_outage_retries(handler, runner, state, monitors, before_outage)
            {runner, state, monitors}
          end)

        finish_recovered_replay(handler, runner, state, monitors, error, 6)
      end)

    assert_outage_log(log, error, 1)
  end

  @tag :queue
  test "replay_held_warnings_survive_database_outage", context do
    replay_held_warnings_survive_database_outage(context)
  end

  def replay_held_warnings_survive_database_outage(context) do
    {options, error} = prepare_held_replay(context)
    handler = observe_replay_queries()

    log =
      capture_log([level: :warning], fn ->
        runner = start_supervised!({PriceAcceptanceReplay, options})
        state = await_held_warning(handler, runner, error)
        monitors = monitor_replay(runner, state.worker)
        assert accepted_observations() == 6
        assert [[@first, true, nil], [@held, false, ^error], [@height, false, nil]] = queue()
        before_outage = durable_replay_state()

        due_state =
          :sys.replace_state(runner, fn current ->
            %{current | last_held_warning_at: current.last_held_warning_at - 60_000}
          end)

        with_queue_unavailable(fn ->
          assert_outage_retries(handler, runner, due_state, monitors, before_outage)
        end)

        finish_recovered_replay(handler, runner, due_state, monitors, error, 12)
      end)

    assert_outage_log(log, error, 2)
  end

  defp prepare_held_replay(context) do
    first = serve(context, :error)
    second = serve(context, :available)

    options = [
      cosmos_rest_urls: [first, second],
      cometbft_rpc_urls: [first, second],
      acceptance_backfill_batch_size: 6,
      acceptance_backfill_interval: 250,
      acceptance_backfill_retry_interval: 400
    ]

    worker = start_supervised!({PriceAcceptanceBackfill, options})
    assert {:ok, 0} = PriceAcceptanceBackfill.step(worker, 18)
    assert {:error, {:node_error, endpoint, message}} = PriceAcceptanceBackfill.step(worker, 18)
    assert endpoint in [first, second]
    assert message == "height #{@held} is not available"
    assert [[@first, true, nil], [@held, false, error], [@height, false, nil]] = queue()
    assert PriceAcceptanceBackfill.held_jobs() == {1, @held, error}
    assert accepted_observations() == 0
    assert :ok = stop_supervised(PriceAcceptanceBackfill)
    {options, error}
  end

  defp with_queue_unavailable(fun) do
    Repo.query!("ALTER TABLE paxeer_x_price_acceptance_backfill RENAME TO price_acceptance_outage")

    try do
      fun.()
    after
      Repo.query!("ALTER TABLE price_acceptance_outage RENAME TO paxeer_x_price_acceptance_backfill")
    end
  end

  defp observe_replay_queries do
    handler = {__MODULE__, make_ref()}
    prefix = Repo.config()[:telemetry_prefix] || [:explorer, :repo]
    :ok = :telemetry.attach(handler, prefix ++ [:query], &__MODULE__.observe_replay_query/4, {self(), handler})
    on_exit(fn -> :telemetry.detach(handler) end)
    handler
  end

  def observe_replay_query(_event, _measurements, metadata, {owner, handler}) do
    kind =
      cond do
        String.contains?(metadata.query, "SELECT j.block_hash, j.block_number, j.transactions, j.results") -> :step
        String.contains?(metadata.query, "SELECT count(*) OVER (), j.block_number, j.error") -> :diagnostic
        true -> nil
      end

    if kind do
      send(owner, {handler, kind, self(), System.monotonic_time(:millisecond), metadata.result})
    end
  end

  defp monitor_replay(runner, worker), do: {Process.monitor(runner), Process.monitor(worker)}

  defp assert_same_replay(runner, state, {runner_monitor, worker_monitor}) do
    worker = state.worker
    assert Process.alive?(runner)
    assert Process.alive?(worker)
    assert :sys.get_state(runner) == state
    refute_receive {:DOWN, ^runner_monitor, :process, ^runner, _}, 0
    refute_receive {:DOWN, ^worker_monitor, :process, ^worker, _}, 0
  end

  defp assert_outage_retries(handler, runner, state, monitors, before_outage) do
    worker = state.worker

    attempts =
      for _ <- 1..3 do
        assert_receive {^handler, :step, ^worker, at, {:error, %Postgrex.Error{postgres: %{code: :undefined_table}}}},
                       3_000

        assert_receive {^handler, :diagnostic, ^runner, _,
                        {:error, %Postgrex.Error{postgres: %{code: :undefined_table}}}},
                       3_000

        assert_same_replay(runner, state, monitors)
        assert durable_replay_state("price_acceptance_outage") == before_outage
        at
      end

    for [previous, current] <- Enum.chunk_every(attempts, 2, 1, :discard) do
      assert current - previous >= state.retry_interval
    end

    refute_receive {^handler, :step, ^worker, _,
                    {:error,
                     %Postgrex.Error{
                       query:
                         "SELECT j.block_hash, j.block_number, j.transactions, j.results\nFROM paxeer_x_price_acceptance_backfill " <>
                           _
                     }}},
                   div(state.retry_interval, 2)

    assert_same_replay(runner, state, monitors)
    assert durable_replay_state("price_acceptance_outage") == before_outage
  end

  defp await_held_warning(handler, runner, error) do
    assert_receive {^handler, :diagnostic, ^runner, _, {:ok, %{rows: [[1, @held, ^error]]}}}, 3_000
    state = :sys.get_state(runner)
    assert is_integer(state.last_held_warning_at)
    state
  end

  defp finish_recovered_replay(handler, runner, previous_state, monitors, error, recovered_count) do
    state = await_held_warning(handler, runner, error)
    assert state.last_held_warning_at != previous_state.last_held_warning_at
    assert_same_replay(runner, state, monitors)
    assert accepted_observations() == recovered_count
    assert [[@first, true, nil], [@held, false, ^error], [@height, false, nil]] = queue()
    assert PriceAcceptanceBackfill.held_jobs() == {1, @held, error}
    worker = state.worker

    for _ <- 1..2 do
      assert_receive {^handler, :step, ^worker, _, {:ok, %{rows: [[_, @held, nil, nil]]}}}, 3_000
      assert_same_replay(runner, state, monitors)
      assert PriceAcceptanceBackfill.held_jobs() == {1, @held, error}
    end

    assert queue() == [[@first, true, nil], [@held, false, error], [@height, true, nil]]
    assert accepted_observations() == 18

    assert Repo.aggregate(from(o in PriceObservation, where: o.block_number == @height and is_nil(o.accepted)), :count) ==
             0

    assert :ok = PriceAcceptanceBackfill.retain(worker, @held, %{"txs" => [], "raw" => []}, [])
    {runner_monitor, worker_monitor} = monitors
    assert_receive {:DOWN, ^runner_monitor, :process, ^runner, :normal}, 3_000
    assert_receive {:DOWN, ^worker_monitor, :process, ^worker, :normal}, 3_000
    assert PriceAcceptanceBackfill.held_jobs() == nil
    assert queue() == [[@first, true, nil], [@held, true, nil], [@height, true, nil]]
    assert accepted_observations() == 18
  end

  defp assert_outage_log(log, error, warnings) do
    assert length(Regex.scan(~r/price acceptance replay held-job diagnostic unavailable:/, log)) == 3
    assert log =~ "price_acceptance_backfill"
    assert length(Regex.scan(~r/price acceptance replay held:/, log)) == warnings
    assert log =~ "1 rows; lowest block #{@held}: #{error}"
    assert log =~ "[warning]"
  end

  defp durable_replay_state(table \\ "paxeer_x_price_acceptance_backfill") do
    {
      Repo.query!("SELECT * FROM #{table} ORDER BY block_number, block_hash").rows,
      Repo.query!("SELECT * FROM paxeer_x_price_observations ORDER BY update_hash, market_id").rows
    }
  end

  defp accepted_observations do
    Repo.aggregate(from(o in PriceObservation, where: o.block_number == @height and o.accepted == true), :count)
  end

  @tag :queue
  test "a replay mismatch holds its row while the next error-free row completes", context do
    worker = start_supervised!({PriceAcceptanceBackfill, []})
    transactions = %{"txs" => context.body["txs"], "raw" => context.body["block"]["data"]["txs"]}

    assert :ok = PriceAcceptanceBackfill.retain(worker, @height, transactions, context.result["txs_results"])
    Repo.query!("UPDATE paxeer_x_price_updates SET status = 0 WHERE block_number = $1", [@height])

    Repo.query!("UPDATE paxeer_x_price_acceptance_backfill SET error = 'earlier hold' WHERE block_number < $1", [
      @height
    ])

    assert {:error, :retained_submission_mismatch} = PriceAcceptanceBackfill.step(worker, 18)
    assert PriceAcceptanceBackfill.held_jobs() == {3, @first, "earlier hold"}

    assert :ok = PriceAcceptanceBackfill.retain(worker, @held, %{"txs" => [], "raw" => []}, [])
    assert {:ok, 0} = PriceAcceptanceBackfill.step(worker, 18)

    assert queue() == [
             [@first, false, "earlier hold"],
             [@held, true, nil],
             [@height, false, ":retained_submission_mismatch"]
           ]
  end

  defp seed_queue(context) do
    assert :ok = Ecto.Migrator.down(Repo, @version, BackfillPaxeerXPriceAcceptance, log: false)
    blocks = for height <- @first..@height, do: insert(:block, number: height, consensus: true)

    on_exit(fn ->
      hashes = Enum.map(blocks, & &1.hash)
      Repo.delete_all(from(b in Block, where: b.hash in ^hashes))
      Repo.delete_all(from(s in PriceFeedScannedBlock, where: s.block_number in ^Enum.to_list(@first..@height)))
      LastFetchedCounter.delete("paxeer_x_price_update_total")
      Ecto.Migrator.up(Repo, @version, BackfillPaxeerXPriceAcceptance, log: false)
    end)

    transactions = %{"txs" => context.body["txs"], "raw" => context.body["block"]["data"]["txs"]}
    assert {:ok, submissions} = CosmosRest.decode_block_txs(transactions)
    assert {:ok, [update]} = CosmosRest.decode_price_updates(submissions, context.result["txs_results"], 1)
    legacy = Map.update!(update, :observations, &Enum.map(&1, fn observation -> Map.delete(observation, :accepted) end))
    assert {:ok, 1} = PriceFeed.replace_block_updates(List.last(blocks), [legacy])
    assert :ok = Ecto.Migrator.up(Repo, @version, BackfillPaxeerXPriceAcceptance, log: false)

    for block <- Enum.take(blocks, 2) do
      Repo.query!(
        "INSERT INTO paxeer_x_price_acceptance_backfill (block_hash, block_number) VALUES ($1, $2)",
        [block.hash.bytes, block.number]
      )
    end
  end

  defp queue do
    Repo.query!("""
    SELECT block_number, completed, error FROM paxeer_x_price_acceptance_backfill
    ORDER BY block_number, block_hash
    """).rows
  end

  defp serve(context, role) do
    ref = {RecordedNode, make_ref()}

    child =
      Plug.Cowboy.child_spec(
        scheme: :http,
        plug: {RecordedNode, {self(), role, context.bodies, context.results, [@held]}},
        options: [port: 0, ref: ref, ip: {127, 0, 0, 1}]
      )

    start_supervised!(%{child | id: ref})
    "http://127.0.0.1:#{:ranch.get_port(ref)}"
  end

  defp fixture(name), do: @fixtures |> Path.join(name) |> File.read!() |> Jason.decode!()
end
