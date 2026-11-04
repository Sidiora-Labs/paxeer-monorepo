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
