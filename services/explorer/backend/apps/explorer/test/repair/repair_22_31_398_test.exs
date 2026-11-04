defmodule Explorer.Repair.Repair2231398Test do
  use ExUnit.Case, async: false

  import Ecto.Query

  alias EthereumJSONRPC.Block
  alias Explorer.Migrator.{MigrationStatus, SanitizeMissingBlockRanges}
  alias Explorer.Repo
  alias Explorer.Utility.MissingBlockRange

  @migration "sanitize_missing_ranges"
  @fixture Path.expand("../../../ethereum_jsonrpc/test/support/fixture/paxeer_x/consecutive_blocks.json", __DIR__)

  setup do
    original_repo = Repo.get_dynamic_repo()
    schema = "repair_2231398_#{System.unique_integer([:positive])}"
    Repo.query!("CREATE SCHEMA #{schema}")

    on_exit(fn ->
      Repo.put_dynamic_repo(original_repo)
      Repo.query!("DROP SCHEMA #{schema} CASCADE")
    end)

    # The startup sanitizer runs on the legacy queue before the exclusion
    # constraint can be installed. Recreate that schema without altering the
    # current queue or removing any of its constraints.
    Repo.query!("""
    CREATE TABLE #{schema}.missing_block_ranges (
      id serial PRIMARY KEY,
      from_number integer,
      to_number integer,
      priority smallint,
      UNIQUE (from_number, to_number)
    )
    """)

    Repo.query!("CREATE TABLE #{schema}.migrations_status (LIKE public.migrations_status INCLUDING ALL)")

    repo = start_supervised!({Repo, name: nil, parameters: [search_path: schema]})
    Ecto.Adapters.SQL.Sandbox.mode(repo, :auto)
    Repo.put_dynamic_repo(repo)

    [low, high] =
      @fixture
      |> File.read!()
      |> Jason.decode!()
      |> Map.values()
      |> Enum.map(&Map.fetch!(Block.to_elixir(&1), "number"))
      |> Enum.sort()

    assert high == low + 1

    # These are the real queue producer's inputs decoded from recorded blocks.
    :ok = MissingBlockRange.add_ranges_by_block_numbers([low, high])
    Repo.insert!(MissingBlockRange.changeset(%{from_number: high, to_number: high}))

    # A second, disjoint backlog below the recorded head exercises replacement
    # of more than one merged range and rollback after a successful insert.
    Repo.insert!(MissingBlockRange.changeset(%{from_number: low - 10, to_number: low - 12}))
    Repo.insert!(MissingBlockRange.changeset(%{from_number: low - 11, to_number: low - 13}))

    # A reversed legacy range and an untouched priority range must also survive.
    Repo.insert!(MissingBlockRange.changeset(%{from_number: low - 22, to_number: low - 20}))
    untouched = Repo.insert!(MissingBlockRange.changeset(%{from_number: low - 30, to_number: low - 31, priority: 1}))

    %{repo: repo, high: high, original: queue_rows(), heights: queued_heights(), untouched: untouched}
  end

  test "missing-range-crash-loss: killed replacement recovers every queued height on restart", context do
    reference = pause_replacement()
    {pid, monitor} = start_migrator(context.repo)

    assert_receive {:replacement_deleted, ^reference, ^pid, 4}, 10_000
    assert MigrationStatus.get_status(@migration) == "started"
    assert queued_heights() == context.heights

    error =
      assert_raise Postgrex.Error, fn ->
        Repo.query!("SELECT id FROM missing_block_ranges WHERE from_number = $1 FOR UPDATE NOWAIT", [context.high])
      end

    assert error.postgres.code == :lock_not_available

    Process.exit(pid, :kill)
    assert_receive {:DOWN, ^monitor, :process, ^pid, :killed}, 10_000
    :telemetry.detach(reference)

    assert queue_rows() == context.original
    assert MigrationStatus.get_status(@migration) == "started"

    assert_restart_recovers(context)
  end

  test "missing-range-crash-loss: later insert failure rolls back deletion and earlier replacement", context do
    Repo.query!("""
    CREATE FUNCTION repair_2231398_reject_insert() RETURNS trigger LANGUAGE plpgsql AS $$
    BEGIN
      IF NEW.from_number = #{context.high} THEN
        RAISE EXCEPTION 'repair_2231398 interrupted replacement';
      END IF;
      RETURN NEW;
    END;
    $$
    """)

    Repo.query!("""
    CREATE TRIGGER repair_2231398_reject_insert BEFORE INSERT ON missing_block_ranges
    FOR EACH ROW EXECUTE FUNCTION repair_2231398_reject_insert()
    """)

    reference = pause_replacement()
    {pid, monitor} = start_migrator(context.repo)
    assert_receive {:replacement_deleted, ^reference, ^pid, 4}, 10_000
    send(pid, {:resume_replacement, reference})

    assert_receive {:replacement_inserted, ^reference, ^pid}, 10_000
    assert_receive {:DOWN, ^monitor, :process, ^pid, {error, _stack}}, 10_000
    assert %Postgrex.Error{postgres: %{message: "repair_2231398 interrupted replacement"}} = error
    :telemetry.detach(reference)

    assert queue_rows() == context.original
    assert MigrationStatus.get_status(@migration) == "started"

    Repo.query!("DROP TRIGGER repair_2231398_reject_insert ON missing_block_ranges")
    assert_restart_recovers(context)
  end

  test "missing-range-crash-loss: successful replacement is durable and idempotent", context do
    assert_restart_recovers(context)
    sanitized = queue_rows()

    # Reopen a started migration after its queue transaction already committed.
    assert {:ok, _} = MigrationStatus.set_status(@migration, "started")
    assert_restart_recovers(context)
    assert queue_rows() == sanitized
  end

  defp start_migrator(repo) do
    owner = self()

    # Enter the actual startup GenServer with a dedicated connection pool for
    # the legacy schema. No outer sandbox transaction can mask a lost commit.
    pid =
      :proc_lib.spawn(fn ->
        Repo.put_dynamic_repo(repo)
        send(owner, {:migrator_ready, self()})

        receive do
          :start ->
            {:ok, state, continuation} = SanitizeMissingBlockRanges.init(:ok)
            :gen_server.enter_loop(SanitizeMissingBlockRanges, [], state, continuation)
        end
      end)

    on_exit(fn -> if Process.alive?(pid), do: Process.exit(pid, :kill) end)
    monitor = Process.monitor(pid)
    assert_receive {:migrator_ready, ^pid}, 10_000
    send(pid, :start)
    {pid, monitor}
  end

  defp pause_replacement do
    reference = make_ref()
    event = (Repo.config()[:telemetry_prefix] || [:explorer, :repo]) ++ [:query]
    :ok = :telemetry.attach(reference, event, &__MODULE__.observe_replacement/4, {self(), reference})
    on_exit(fn -> :telemetry.detach(reference) end)
    reference
  end

  def observe_replacement(_event, _measurements, metadata, {owner, reference}) do
    case metadata do
      %{query: query, result: {:ok, %{num_rows: count}}} ->
        cond do
          String.starts_with?(query, "DELETE FROM \"missing_block_ranges\"") and count > 0 ->
            send(owner, {:replacement_deleted, reference, self(), count})

            receive do
              {:resume_replacement, ^reference} -> :ok
            end

          String.starts_with?(query, "INSERT INTO \"missing_block_ranges\"") and count == 1 ->
            send(owner, {:replacement_inserted, reference, self()})

          true ->
            :ok
        end

      _ ->
        :ok
    end
  end

  defp assert_restart_recovers(context) do
    {pid, monitor} = start_migrator(context.repo)
    assert_receive {:DOWN, ^monitor, :process, ^pid, :normal}, 10_000
    assert MigrationStatus.get_status(@migration) == "completed"
    assert queued_heights() == context.heights
    assert Repo.get!(MissingBlockRange, context.untouched.id) == context.untouched

    rows = Repo.all(MissingBlockRange)
    assert length(rows) == 4
    assert Enum.all?(rows, &(&1.from_number >= &1.to_number))

    all_heights = Enum.flat_map(rows, &Enum.to_list(&1.from_number..&1.to_number))
    assert length(all_heights) == MapSet.size(context.heights)

    recovered = MissingBlockRange.get_latest_batch(MapSet.size(context.heights))
    assert recovered |> Enum.flat_map(&Enum.to_list/1) |> MapSet.new() == context.heights
  end

  defp queue_rows do
    MissingBlockRange |> order_by([r], r.id) |> Repo.all()
  end

  defp queued_heights do
    MissingBlockRange
    |> Repo.all()
    |> Enum.flat_map(&Enum.to_list(&1.from_number..&1.to_number))
    |> MapSet.new()
  end
end
