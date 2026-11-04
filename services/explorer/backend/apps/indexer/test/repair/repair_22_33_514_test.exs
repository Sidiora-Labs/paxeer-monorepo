Code.require_file(
  Path.expand("../../../explorer/priv/blackfort/migrations/20240910112251_add_blackfort_validators.exs", __DIR__)
)

defmodule Indexer.Repair2233514Test do
  use ExUnit.Case, async: false

  import ExUnit.CaptureLog

  alias Ecto.Adapters.SQL.Sandbox
  alias Explorer.Chain.Blackfort.Validator
  alias Explorer.Repo
  alias Explorer.Repo.Blackfort.Migrations.AddBlackfortValidators
  alias Indexer.Fetcher.Blackfort.Validator, as: Fetcher

  @snapshots Path.expand("../../lib/indexer/fetcher/blackfort/repair_22_33_514_snapshots.json", __DIR__)

  setup do
    schema = "repair_2233514_#{System.unique_integer([:positive])}"
    original_repo = Repo.get_dynamic_repo()
    validator_config = Application.get_env(:explorer, Validator)
    http_client = Application.fetch_env!(:explorer, :http_client)
    adapter = Application.get_env(:tesla, :adapter)

    on_exit(fn ->
      Repo.put_dynamic_repo(original_repo)
      Sandbox.unboxed_run(Repo, fn -> Repo.query!("DROP SCHEMA IF EXISTS #{schema} CASCADE") end)
      restore_env(:explorer, Validator, validator_config)
      restore_env(:explorer, :http_client, http_client)
      restore_env(:tesla, :adapter, adapter)
    end)

    # Independent, committed connections expose transaction boundaries that an
    # outer Sandbox transaction would otherwise conceal from a second reader.
    repo =
      start_supervised!(
        {Repo, name: nil, pool: DBConnection.ConnectionPool, pool_size: 4, parameters: [search_path: schema]}
      )

    Repo.put_dynamic_repo(repo)
    Repo.query!("CREATE SCHEMA #{schema}")
    assert :ok = Ecto.Migrator.up(Repo, 20_240_910_112_251, AddBlackfortValidators, prefix: schema, log: false)

    snapshots = @snapshots |> File.read!() |> Jason.decode!()
    response = start_supervised!({Agent, fn -> {:initial, 200, Jason.encode!(snapshots["initial"])} end})
    server = Bypass.open()
    owner = self()

    Bypass.expect(server, "GET", "/validators", fn conn ->
      {label, status, body} = Agent.get(response, & &1)
      send(owner, {:snapshot_requested, label})

      conn
      |> Plug.Conn.put_resp_content_type("application/json")
      |> Plug.Conn.resp(status, body)
    end)

    Application.put_env(:explorer, Validator, api_url: "http://127.0.0.1:#{server.port}/validators")
    Application.put_env(:explorer, :http_client, Explorer.HttpClient.Tesla)
    Application.put_env(:tesla, :adapter, Tesla.Adapter.Hackney)

    {pid, _monitor} = start_fetcher(repo)
    assert_receive {:snapshot_requested, :initial}, 5_000
    assert %{retry_count: 0, retry_timer: nil} = :sys.get_state(pid)
    assert_snapshot(snapshots["initial"])

    %{repo: repo, response: response, snapshots: snapshots, pid: pid, original: rows()}
  end

  test "validator-replacement: database failure rolls back and the scheduled retry commits the full snapshot",
       context do
    replacement = context.snapshots["replacement"]
    duplicate = replacement ++ [hd(replacement)]

    log =
      capture_log(fn ->
        serve(context.response, :duplicate, duplicate)
        assert :ok = Fetcher.trigger_update_validators_list()
        assert_receive {:snapshot_requested, :duplicate}, 5_000
        assert %{retry_count: 1, retry_timer: {timer, _token}} = :sys.get_state(context.pid)
        assert is_integer(Process.read_timer(timer))
        assert rows() == context.original
        assert_snapshot(context.snapshots["initial"])

        serve(context.response, :replacement, replacement)
        assert_receive {:snapshot_requested, :replacement}, 5_000
        assert %{retry_count: 0, retry_timer: nil} = :sys.get_state(context.pid)
        assert_snapshot(replacement)
      end)

    assert log =~ "Blackfort validator snapshot update failed"
    assert log =~ "ON CONFLICT DO UPDATE command cannot affect row a second time"
    assert log =~ "Retrying Blackfort validator snapshot update 1/3 in 1000ms"
    assert_retained_inserted_at(context.original)
  end

  test "validator-replacement: killed replacement preserves the snapshot and restart recovers", context do
    reference = make_ref()
    event = (Repo.config()[:telemetry_prefix] || [:explorer, :repo]) ++ [:query]
    :ok = :telemetry.attach(reference, event, &__MODULE__.observe_replacement/4, {self(), reference})
    on_exit(fn -> :telemetry.detach(reference) end)

    serve(context.response, :replacement, context.snapshots["replacement"])
    assert :ok = Fetcher.trigger_update_validators_list()
    pid = context.pid
    assert_receive {:snapshot_requested, :replacement}, 5_000
    assert_receive {:replacement_deleted, ^reference, ^pid, 1}, 5_000

    # This production schema read uses another connection while the fetcher's
    # connection has already executed DELETE and has not yet executed INSERT.
    assert rows() == context.original
    monitor = Process.monitor(pid)
    Process.exit(pid, :kill)
    assert_receive {:DOWN, ^monitor, :process, ^pid, :killed}, 5_000
    :telemetry.detach(reference)
    assert rows() == context.original

    {restarted, _monitor} = start_fetcher(context.repo)
    assert restarted != pid
    assert_receive {:snapshot_requested, :replacement}, 5_000
    assert %{retry_count: 0, retry_timer: nil} = :sys.get_state(restarted)
    assert_snapshot(context.snapshots["replacement"])
    assert_retained_inserted_at(context.original)

    monitor = Process.monitor(restarted)
    assert :ok = GenServer.stop(restarted)
    assert_receive {:DOWN, ^monitor, :process, ^restarted, :normal}, 5_000
    {reopened, _monitor} = start_fetcher(context.repo)
    assert_receive {:snapshot_requested, :replacement}, 5_000
    assert %{retry_count: 0, retry_timer: nil} = :sys.get_state(reopened)
    assert_snapshot(context.snapshots["replacement"])
  end

  test "validator-replacement: fetch failures exhaust bounded retries and a new trigger recovers", context do
    log =
      capture_log(fn ->
        serve(context.response, :unavailable, %{"error" => "temporarily unavailable"}, 503)
        assert :ok = Fetcher.trigger_update_validators_list()
        assert_receive {:snapshot_requested, :unavailable}, 5_000
        assert %{retry_count: 1, retry_timer: {timer, _token}} = :sys.get_state(context.pid)
        remaining = Process.read_timer(timer)
        assert is_integer(remaining) and remaining > 0 and remaining <= 1_000
        assert rows() == context.original

        assert_receive {:snapshot_requested, :unavailable}, 5_000
        assert %{retry_count: 2, retry_timer: {timer, _token}} = :sys.get_state(context.pid)
        remaining = Process.read_timer(timer)
        assert is_integer(remaining) and remaining > 0 and remaining <= 2_000
        assert rows() == context.original

        assert_receive {:snapshot_requested, :unavailable}, 5_000
        assert %{retry_count: 3, retry_timer: {timer, _token}} = :sys.get_state(context.pid)
        remaining = Process.read_timer(timer)
        assert is_integer(remaining) and remaining > 0 and remaining <= 4_000
        assert rows() == context.original

        assert_receive {:snapshot_requested, :unavailable}, 6_000
        assert %{retry_count: 3, retry_timer: nil} = :sys.get_state(context.pid)
        refute_receive {:snapshot_requested, :unavailable}, 1_100
        assert rows() == context.original

        serve(context.response, :replacement, context.snapshots["replacement"])
        assert :ok = Fetcher.trigger_update_validators_list()
        assert_receive {:snapshot_requested, :replacement}, 5_000
        assert %{retry_count: 0, retry_timer: nil} = :sys.get_state(context.pid)
        assert_snapshot(context.snapshots["replacement"])
      end)

    assert log =~ "Blackfort validator snapshot update failed: :error"
    assert log =~ "Blackfort validator snapshot update exhausted 3 retries"
    assert length(Regex.scan(~r/Retrying Blackfort validator snapshot update/, log)) == 3
  end

  test "validator-replacement: successful replacement and empty snapshot survive restart", context do
    serve(context.response, :replacement, context.snapshots["replacement"])
    assert :ok = Fetcher.trigger_update_validators_list()
    assert_receive {:snapshot_requested, :replacement}, 5_000
    assert %{retry_count: 0, retry_timer: nil} = :sys.get_state(context.pid)
    assert_snapshot(context.snapshots["replacement"])
    assert_retained_inserted_at(context.original)

    serve(context.response, :empty, [])
    assert :ok = Fetcher.trigger_update_validators_list()
    assert_receive {:snapshot_requested, :empty}, 5_000
    assert %{retry_count: 0, retry_timer: nil} = :sys.get_state(context.pid)
    assert rows() == []
    assert :ok = GenServer.stop(context.pid)

    {restarted, _monitor} = start_fetcher(context.repo)
    assert_receive {:snapshot_requested, :empty}, 5_000
    assert %{retry_count: 0, retry_timer: nil} = :sys.get_state(restarted)
    assert rows() == []
  end

  def observe_replacement(_event, _measurements, metadata, {owner, reference}) do
    case metadata do
      %{query: query, result: {:ok, %{num_rows: count}}} when count > 0 ->
        if String.starts_with?(query, "DELETE FROM \"validators_blackfort\"") do
          send(owner, {:replacement_deleted, reference, self(), count})

          receive do
            {:resume_replacement, ^reference} -> :ok
          end
        end

      _ ->
        :ok
    end
  end

  defp start_fetcher(repo) do
    owner = self()

    # Ecto dynamic repositories are process-local. Enter the real GenServer
    # with the isolated pool so both the worker and independent reader use the
    # production schema, SQL, callbacks and startup reconciliation.
    pid =
      :proc_lib.spawn(fn ->
        Repo.put_dynamic_repo(repo)
        Process.register(self(), Fetcher)
        {:ok, state} = Fetcher.init([])
        send(owner, {:fetcher_started, self()})
        :gen_server.enter_loop(Fetcher, [], state)
      end)

    on_exit(fn ->
      if Process.alive?(pid) do
        monitor = Process.monitor(pid)
        Process.exit(pid, :kill)
        assert_receive {:DOWN, ^monitor, :process, ^pid, _reason}, 5_000
      end
    end)

    monitor = Process.monitor(pid)
    assert_receive {:fetcher_started, ^pid}, 5_000
    {pid, monitor}
  end

  defp serve(response, label, body, status \\ 200) do
    Agent.update(response, fn _ -> {label, status, Jason.encode!(body)} end)
  end

  defp rows do
    Enum.sort_by(Validator.get_all_validators(), & &1.address_hash.bytes)
  end

  defp assert_snapshot(snapshot) do
    actual = rows()
    assert Enum.map(actual, &to_string(&1.address_hash)) == Enum.sort(Enum.map(snapshot, & &1["address"]))

    for recorded <- snapshot do
      validator = Enum.find(actual, &(to_string(&1.address_hash) == recorded["address"]))
      assert validator.name == recorded["name"]
      assert validator.commission == String.to_integer(recorded["commission"])
      assert Decimal.equal?(validator.self_bonded_amount, Decimal.new(recorded["self_bonded_amount"]))
      assert Decimal.equal?(validator.delegated_amount, Decimal.new(recorded["delegated_amount"]))
      assert validator.slashing_status_is_slashed == recorded["slashing_status"]["is_slashed"]
      assert validator.slashing_status_by_block == recorded["slashing_status"]["by_block"]
      assert validator.slashing_status_multiplier == recorded["slashing_status"]["multiplier"]
      assert %DateTime{} = validator.inserted_at
      assert %DateTime{} = validator.updated_at
    end
  end

  defp assert_retained_inserted_at(original) do
    current = rows()

    for previous <- original, retained <- current, previous.address_hash == retained.address_hash do
      assert retained.inserted_at == previous.inserted_at
    end
  end

  defp restore_env(app, key, nil), do: Application.delete_env(app, key)
  defp restore_env(app, key, value), do: Application.put_env(app, key, value)
end
