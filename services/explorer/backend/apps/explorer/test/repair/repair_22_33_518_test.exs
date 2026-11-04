for file <- [
      "20260301000000_hot_tables_storage_parameters.exs",
      "20260301000100_internal_transactions_block_number_brin_index.exs",
      "20260301000200_hot_tables_block_number_statistics.exs"
    ] do
  Code.require_file(Path.expand("../../priv/repo/migrations/#{file}", __DIR__))
end

defmodule Explorer.Repair.Repair2233518Test do
  use ExUnit.Case, async: false

  alias Explorer.Chain.{Block, InternalTransaction}
  alias Explorer.Repo

  alias Explorer.Repo.Migrations.{
    HotTablesBlockNumberStatistics,
    HotTablesStorageParameters,
    InternalTransactionsBlockNumberBrinIndex
  }

  @storage_version 20_260_301_000_000
  @brin_version 20_260_301_000_100
  @statistics_version 20_260_301_000_200
  @index "internal_transactions_block_number_brin_index"
  @tables ~w(blocks transactions logs token_transfers internal_transactions address_coin_balances)
  @fixtures Path.expand("../../../ethereum_jsonrpc/test/support/fixture/paxeer_x", __DIR__)

  setup do
    schema = "repair_2233518_#{System.unique_integer([:positive])}"
    original_repo = Repo.get_dynamic_repo()
    Repo.query!("CREATE SCHEMA #{schema}")
    pid = open_repo(schema)
    Repo.put_dynamic_repo(pid)

    on_exit(fn ->
      if Process.alive?(pid), do: Supervisor.stop(pid)
      Repo.put_dynamic_repo(original_repo)
      Repo.query!("DROP SCHEMA #{schema} CASCADE")
    end)

    for table <- @tables do
      Repo.query!("CREATE TABLE #{table} (LIKE public.#{table} INCLUDING ALL EXCLUDING INDEXES)")
    end

    Ecto.Migration.SchemaMigration.ensure_schema_migrations_table!(Repo, Repo.config(), [])
    {block, traces} = import_recorded_chain()
    %{schema: schema, repo: pid, block: block, traces: traces}
  end

  test "hot-table-lock-budget: refusal rolls back all storage changes and permits retry", context do
    assert_lock_budget(context, @storage_version, HotTablesStorageParameters, &storage_state/0)

    assert %{rows: [["85"]]} =
             Repo.query!(
               "SELECT option_value FROM pg_options_to_table((SELECT reloptions FROM pg_class WHERE oid = 'transactions'::regclass)) WHERE option_name = 'fillfactor'"
             )

    assert_recorded_chain(context)
  end

  test "statistics-lock-budget: refusal rolls back all statistics changes and permits retry", context do
    assert_lock_budget(context, @statistics_version, HotTablesBlockNumberStatistics, &statistics_state/0)
    assert Enum.all?(statistics_state(), fn [_table, _column, target] -> target == 1000 end)
    assert_recorded_chain(context)
  end

  test "brin-invalid-index-resume: interrupted migration recovers after repository restart", context do
    # A real open writer makes CREATE INDEX CONCURRENTLY wait after publishing
    # its invalid catalog entry. PostgreSQL cancels it at statement_timeout.
    blocker = hold_lock(context.repo, "internal_transactions", "ROW EXCLUSIVE")
    first = open_repo(context.schema, "1s")
    Repo.put_dynamic_repo(first)

    try do
      error =
        assert_raise Postgrex.Error, fn -> migrate(:up, @brin_version, InternalTransactionsBlockNumberBrinIndex) end

      assert error.postgres.code == :query_canceled
      assert %{rows: [[old_oid, false, false]]} = index_state()
      refute applied?(@brin_version)
      release_lock(blocker)

      monitor = Process.monitor(first)
      Supervisor.stop(first)
      assert_receive {:DOWN, ^monitor, :process, ^first, :normal}

      reopened = open_repo(context.schema)
      Repo.put_dynamic_repo(reopened)

      try do
        assert %{rows: [[^old_oid, false, false]]} = index_state()
        assert :ok = migrate(:up, @brin_version, InternalTransactionsBlockNumberBrinIndex)
        assert %{rows: [[new_oid, true, true]]} = index_state()
        refute new_oid == old_oid
        assert applied?(@brin_version)
        assert_brin_reads_recorded_traces(context)
        assert :already_up = migrate(:up, @brin_version, InternalTransactionsBlockNumberBrinIndex)
        assert %{rows: [[^new_oid, true, true]]} = index_state()
        assert :ok = migrate(:down, @brin_version, InternalTransactionsBlockNumberBrinIndex)
        assert %{rows: []} = index_state()
      after
        Repo.put_dynamic_repo(context.repo)
        Supervisor.stop(reopened)
      end
    after
      if Process.alive?(blocker.pid), do: release_lock(blocker)
      if Process.alive?(first), do: Supervisor.stop(first)
      Repo.put_dynamic_repo(context.repo)
    end
  end

  test "brin-invalid-index-resume: valid same-name index is preserved", context do
    assert :ok = migrate(:up, @brin_version, InternalTransactionsBlockNumberBrinIndex)
    assert %{rows: [[oid, true, true]]} = index_state()
    Repo.query!("DELETE FROM schema_migrations WHERE version = $1", [@brin_version])
    assert :ok = migrate(:up, @brin_version, InternalTransactionsBlockNumberBrinIndex)
    assert %{rows: [[^oid, true, true]]} = index_state()
    assert_brin_reads_recorded_traces(context)
  end

  test "brin-invalid-index-resume: same name on another table cannot qualify the migration" do
    Repo.query!("CREATE INDEX #{@index} ON logs USING brin (block_number)")

    assert_raise Ecto.MigrationError, ~r/not a valid index on internal_transactions/, fn ->
      migrate(:up, @brin_version, InternalTransactionsBlockNumberBrinIndex)
    end

    refute applied?(@brin_version)
    assert %{rows: [[true]]} = Repo.query!("SELECT indisvalid FROM pg_index WHERE indexrelid = '#{@index}'::regclass")
  end

  defp assert_lock_budget(context, version, migration, snapshot) do
    initial = snapshot.()

    for direction <- [:up, :down] do
      before = snapshot.()
      was_applied = applied?(version)
      blocker = hold_lock(context.repo, "logs", "ACCESS EXCLUSIVE")
      started = System.monotonic_time(:millisecond)

      try do
        error = assert_raise Postgrex.Error, fn -> migrate(direction, version, migration) end
        assert error.postgres.code == :lock_not_available
        elapsed = System.monotonic_time(:millisecond) - started
        assert elapsed >= 4_500
        assert elapsed < 10_000
        assert snapshot.() == before
        assert applied?(version) == was_applied

        # Earlier table locks must have been released with the transaction.
        assert {:ok, _} = Repo.transaction(fn -> Repo.query!("LOCK TABLE blocks IN ACCESS EXCLUSIVE MODE NOWAIT") end)
      after
        release_lock(blocker)
      end

      assert :ok = migrate(direction, version, migration)
      assert applied?(version) == (direction == :up)
      assert snapshot.() != before
      assert_clean_sessions(context.repo)
    end

    assert snapshot.() == initial
    assert :ok = migrate(:up, version, migration)
    assert_recorded_chain(context)
  end

  defp open_repo(schema, statement_timeout \\ "15s") do
    {:ok, pid} =
      Repo.start_link(
        name: nil,
        pool: DBConnection.ConnectionPool,
        pool_size: 5,
        after_connect: fn connection ->
          Postgrex.query!(connection, "SET search_path TO #{schema}, public", [])
          Postgrex.query!(connection, "SET statement_timeout = '#{statement_timeout}'", [])
          Postgrex.query!(connection, "SET lock_timeout = '0'", [])
        end
      )

    Process.unlink(pid)
    pid
  end

  defp hold_lock(repo, table, mode) do
    owner = self()

    task =
      Task.async(fn ->
        Repo.put_dynamic_repo(repo)

        Repo.transaction(fn ->
          Repo.query!("LOCK TABLE #{table} IN #{mode} MODE")
          send(owner, {:locked, self()})

          receive do
            :release -> :ok
          after
            20_000 -> raise "lock holder was not released"
          end
        end)
      end)

    assert_receive {:locked, pid} when pid == task.pid, 5_000
    task
  end

  defp release_lock(task) do
    send(task.pid, :release)
    assert {:ok, :ok} = Task.await(task, 5_000)
  end

  defp assert_clean_sessions(repo) do
    owner = self()

    tasks =
      for _ <- 1..5 do
        Task.async(fn ->
          Repo.put_dynamic_repo(repo)

          Repo.checkout(fn ->
            assert %{rows: [["0"]]} = Repo.query!("SHOW lock_timeout")
            send(owner, {:checked_out, self()})

            receive do
              :release -> :ok
            after
              5_000 -> raise "session check was not released"
            end
          end)
        end)
      end

    for task <- tasks, do: assert_receive({:checked_out, pid} when pid == task.pid, 5_000)
    for task <- tasks, do: send(task.pid, :release)
    for task <- tasks, do: Task.await(task, 5_000)
  end

  defp migrate(direction, version, module), do: apply(Ecto.Migrator, direction, [Repo, version, module, [log: false]])

  defp applied?(version) do
    %{rows: [[applied]]} = Repo.query!("SELECT EXISTS (SELECT 1 FROM schema_migrations WHERE version = $1)", [version])
    applied
  end

  defp storage_state do
    Repo.query!("""
    SELECT c.relname, c.reloptions, toast.reloptions
    FROM pg_class c
    LEFT JOIN pg_class toast ON toast.oid = c.reltoastrelid
    WHERE c.oid IN (#{Enum.map_join(@tables, ",", &"'#{&1}'::regclass")})
    ORDER BY c.relname
    """).rows
  end

  defp statistics_state do
    Repo.query!("""
    SELECT c.relname, a.attname, a.attstattarget
    FROM pg_attribute a JOIN pg_class c ON c.oid = a.attrelid
    WHERE (c.oid = 'blocks'::regclass AND a.attname IN ('number', 'timestamp'))
       OR (c.oid IN (#{Enum.map_join(tl(@tables), ",", &"'#{&1}'::regclass")}) AND a.attname = 'block_number')
    ORDER BY c.relname, a.attname
    """).rows
  end

  defp index_state do
    Repo.query!("""
    SELECT c.oid::bigint, i.indisvalid, i.indisready
    FROM pg_index i JOIN pg_class c ON c.oid = i.indexrelid
    WHERE c.relname = '#{@index}' AND i.indrelid = 'internal_transactions'::regclass
    """)
  end

  defp import_recorded_chain do
    [recorded | _] =
      @fixtures |> Path.join("consecutive_blocks.json") |> File.read!() |> Jason.decode!() |> Map.values()

    params =
      recorded
      |> EthereumJSONRPC.Block.to_elixir()
      |> EthereumJSONRPC.Block.elixir_to_params()
      |> Map.put(:consensus, true)

    changeset = Block.changeset(%Block{}, params)
    assert changeset.valid?, inspect(changeset.errors)
    block = Repo.insert!(changeset, returning: true)

    previous = Application.get_env(:ethereum_jsonrpc, EthereumJSONRPC.Geth)
    Application.put_env(:ethereum_jsonrpc, EthereumJSONRPC.Geth, tracer: "call_tracer")

    try do
      %{"result" => [%{"txHash" => hash, "result" => trace}]} =
        @fixtures |> Path.join("call_tracer_block.json") |> File.read!() |> Jason.decode!()

      traces =
        trace
        |> EthereumJSONRPC.Geth.prepare_calls()
        |> Enum.with_index()
        |> Enum.map(fn {call, index} ->
          Map.merge(call, %{
            "blockNumber" => block.number,
            "transactionIndex" => 0,
            "transactionHash" => hash,
            "index" => index
          })
        end)
        |> EthereumJSONRPC.Geth.Calls.to_internal_transactions_params()

      inserted =
        Enum.map(traces, fn params ->
          params = Map.merge(params, %{block_hash: to_string(block.hash), block_index: params.index})
          changeset = InternalTransaction.changeset(%InternalTransaction{}, params)
          assert changeset.valid?, inspect(changeset.errors)
          Repo.insert!(changeset, returning: true)
        end)

      {block, inserted}
    after
      Application.put_env(:ethereum_jsonrpc, EthereumJSONRPC.Geth, previous)
    end
  end

  defp assert_recorded_chain(context) do
    assert Repo.get_by!(Block, hash: context.block.hash) == context.block
    assert Enum.sort_by(Repo.all(InternalTransaction), & &1.index) == Enum.sort_by(context.traces, & &1.index)
  end

  defp assert_brin_reads_recorded_traces(context) do
    assert %{rows: [["brin", ["pages_per_range=128", "autosummarize=on"]]]} =
             Repo.query!(
               "SELECT am.amname, c.reloptions FROM pg_class c JOIN pg_am am ON am.oid = c.relam WHERE c.oid = '#{@index}'::regclass"
             )

    assert {:ok, _} =
             Repo.transaction(fn ->
               Repo.query!("SET LOCAL enable_seqscan = off")

               plan =
                 Repo.query!("EXPLAIN SELECT * FROM internal_transactions WHERE block_number = $1", [
                   context.block.number
                 ])

               assert Enum.any?(plan.rows, fn [line] -> String.contains?(line, @index) end)

               assert %{rows: [[2]]} =
                        Repo.query!("SELECT count(*) FROM internal_transactions WHERE block_number = $1", [
                          context.block.number
                        ])
             end)

    assert_recorded_chain(context)
  end
end
