defmodule Indexer.Repair2233512Test do
  use ExUnit.Case, async: false

  import Ecto.Query
  import ExUnit.CaptureLog

  alias Ecto.Adapters.SQL.Sandbox
  alias EthereumJSONRPC.{Blocks, Logs, Receipts}
  alias Explorer.Chain.{Block, TokenTransfer}
  alias Explorer.Chain.Import.Runner.TokenTransfers, as: TransferRunner
  alias Explorer.Repo
  alias Explorer.Utility.MissingBlockRange
  alias Indexer.Memory.Monitor, as: MemoryMonitor
  alias Indexer.TokenTransferBlockConsensusSanitizer, as: Sanitizer
  alias Indexer.Transform.TokenTransfers

  @moduletag :capture_log

  test "participant / dead-worker-log: terminated participant diagnostics preserve monitor cleanup" do
    monitor =
      start_supervised!({MemoryMonitor, [%{limit: 1, timer_interval: 3_600_000}, [name: Repair2233512MemoryMonitor]]})

    owner = self()

    {:ok, participant} =
      Task.start(fn ->
        Process.register(self(), Repair2233512Participant)
        :ok = MemoryMonitor.shrinkable(monitor)
        send(owner, {:registered, self()})

        receive do
          :stop -> :ok
        end
      end)

    assert_receive {:registered, ^participant}

    assert IO.iodata_to_binary(Indexer.Logger.process(participant)) ==
             "#{inspect(participant)} (Repair2233512Participant)"

    :ok = :sys.suspend(monitor)
    reference = Process.monitor(participant)
    send(monitor, :check)
    send(participant, :stop)
    assert_receive {:DOWN, ^reference, :process, ^participant, :normal}
    assert IO.iodata_to_binary(Indexer.Logger.process(participant)) == inspect(participant)

    log =
      capture_log(fn ->
        :ok = :sys.resume(monitor)
        assert :sys.get_state(monitor).shrinkable_set == MapSet.new()
        Logger.flush()
      end)

    assert log =~ "#{inspect(participant)} exited during memory pressure handling"
    assert Process.alive?(monitor)
    send(monitor, :check)
    assert :sys.get_state(monitor).shrinkable_set == MapSet.new()
  end

  test "participant / dead-worker-log: registered live queue reaches the memory monitor diagnostic" do
    :ok = Sandbox.checkout(Repo)
    Sandbox.mode(Repo, {:shared, self()})
    start_supervised!({Task.Supervisor, name: Repair2233512TaskSupervisor})

    monitor =
      start_supervised!({MemoryMonitor, [%{limit: 1, timer_interval: 3_600_000}, [name: Repair2233512MemoryMonitor]]})

    queue =
      start_supervised!(
        {Indexer.BufferedTask,
         [
           {Indexer.Fetcher.Token,
            state: [],
            task_supervisor: Repair2233512TaskSupervisor,
            flush_interval: 3_600_000,
            max_batch_size: 1,
            max_concurrency: 1,
            poll: false,
            memory_monitor: Repair2233512MemoryMonitor},
           [name: Repair2233512LiveQueue]
         ]}
      )

    await_initialized(queue)

    assert IO.iodata_to_binary(Indexer.Logger.process(queue)) ==
             "#{inspect(queue)} (Repair2233512LiveQueue)"

    log =
      capture_log(fn ->
        send(monitor, :check)
        assert MapSet.member?(:sys.get_state(monitor).shrinkable_set, queue)
        Logger.flush()
      end)

    assert log =~ "#{inspect(queue)} (Repair2233512LiveQueue)"
    assert log =~ "Asking process to shrink"
    assert Process.alive?(monitor)
    assert Process.alive?(queue)
  end

  describe "consensus-scan" do
    setup do
      schema = "repair_2233512_#{System.unique_integer([:positive])}"
      original_repo = Repo.get_dynamic_repo()
      original_config = Application.fetch_env!(:indexer, Sanitizer)
      Application.put_env(:indexer, Sanitizer, Keyword.put(original_config, :interval, 3_600_000))

      on_exit(fn ->
        Repo.put_dynamic_repo(original_repo)
        Sandbox.unboxed_run(Repo, fn -> Repo.query!("DROP SCHEMA IF EXISTS #{schema} CASCADE") end)
        Application.put_env(:indexer, Sanitizer, original_config)
      end)

      repo =
        start_supervised!(
          {Repo, name: nil, pool: DBConnection.ConnectionPool, pool_size: 4, parameters: [search_path: schema]}
        )

      Repo.put_dynamic_repo(repo)
      Repo.query!("CREATE SCHEMA #{schema}")

      for table <- ["blocks", "tokens", "token_transfers", "missing_block_ranges"] do
        Repo.query!("CREATE TABLE #{schema}.#{table} (LIKE public.#{table} INCLUDING ALL)")
      end

      # Independent connections and committed writes expose the durable state
      # across worker death; there is no outer Sandbox transaction here.
      numbers = Enum.map(0..204, &(&1 * 2))
      seed_recorded_history(numbers)
      reference = make_ref()
      event = (Repo.config()[:telemetry_prefix] || [:explorer, :repo]) ++ [:query]
      :ok = :telemetry.attach(reference, event, &__MODULE__.observe_scan/4, {self(), reference})
      on_exit(fn -> :telemetry.detach(reference) end)
      %{repo: repo, numbers: numbers, reference: reference}
    end

    test "bounded keyset batches preserve committed refetch markers across interruption and reopen", context do
      %{repo: repo, numbers: numbers, reference: reference} = context
      [first_batch, second_batch, last_batch] = Enum.chunk_every(numbers, 100)
      worker = open_scanner(repo)
      send(worker, :sanitize)
      assert_batch(reference, worker, first_batch, -1)
      send(worker, {:resume_scan, reference})
      assert_deadline_and_update(reference, worker, first_batch)
      assert_batch(reference, worker, second_batch, List.last(first_batch))
      assert marked_numbers() == first_batch ++ [1_002]
      assert_refetch_ranges(first_batch)
      committed = marked_rows()

      monitor = Process.monitor(worker)
      Process.exit(worker, :kill)
      assert_receive {:DOWN, ^monitor, :process, ^worker, :killed}, 5_000
      assert marked_rows() == committed
      assert_refetch_ranges(first_batch)

      reopened = open_scanner(repo)
      assert reopened != worker
      send(reopened, :sanitize)
      assert_batch(reference, reopened, second_batch, -1)
      assert marked_rows() == committed
      send(reopened, {:resume_scan, reference})
      assert_deadline_and_update(reference, reopened, second_batch)
      assert_batch(reference, reopened, last_batch, List.last(second_batch))
      send(reopened, {:resume_scan, reference})
      assert_deadline_and_update(reference, reopened, last_batch)
      assert_batch(reference, reopened, [], List.last(last_batch))
      send(reopened, {:resume_scan, reference})
      assert :sys.get_state(reopened).cursor == -1
      assert marked_numbers() == numbers ++ [1_002]
      assert_refetch_ranges(numbers)
      assert Enum.all?(committed, &(&1 in marked_rows()))
      refute Repo.get_by!(Block, number: 1_000).refetch_needed

      # A later mismatch behind the old cursor is discovered on the next sweep.
      {1, _} = Repo.update_all(from(b in Block, where: b.number == 1_000), set: [consensus: false])
      assert :ok = GenServer.stop(reopened)
      recovered = open_scanner(repo)
      send(recovered, :sanitize)
      assert_batch(reference, recovered, [1_000], -1)
      send(recovered, {:resume_scan, reference})
      assert_deadline_and_update(reference, recovered, [1_000])
      assert_batch(reference, recovered, [], 1_000)
      send(recovered, {:resume_scan, reference})
      assert :sys.get_state(recovered).cursor == -1
      assert marked_numbers() == numbers ++ [1_000, 1_002]
      assert_refetch_ranges(numbers ++ [1_000])
      refute_received {:refetch_update, ^reference, ^recovered, _}
    end

    test "completed markers are idempotent on a fresh sweep and unmarked lower heights are revisited", context do
      %{repo: repo, numbers: numbers, reference: reference} = context
      worker = open_scanner(repo)
      send(worker, :sanitize)

      Enum.reduce(Enum.chunk_every(numbers, 100), -1, fn batch, cursor ->
        assert_batch(reference, worker, batch, cursor)
        send(worker, {:resume_scan, reference})
        assert_deadline_and_update(reference, worker, batch)
        List.last(batch)
      end)

      assert_batch(reference, worker, [], List.last(numbers))
      send(worker, {:resume_scan, reference})
      assert :sys.get_state(worker).cursor == -1
      previous = marked_rows()
      send(worker, :sanitize)
      assert_batch(reference, worker, [], -1)
      send(worker, {:resume_scan, reference})
      assert :sys.get_state(worker).cursor == -1
      assert marked_rows() == previous
      refute_received {:refetch_update, ^reference, ^worker, _}

      # The refetch consumer may clear a marker while the mismatch still exists.
      {1, _} = Repo.update_all(from(b in Block, where: b.number == 0), set: [refetch_needed: false])
      send(worker, :sanitize)
      assert_batch(reference, worker, [0], -1)
      send(worker, {:resume_scan, reference})
      assert_deadline_and_update(reference, worker, [0])
      assert_batch(reference, worker, [], 0)
      send(worker, {:resume_scan, reference})
      assert :sys.get_state(worker).cursor == -1
      assert marked_numbers() == numbers ++ [1_002]
      assert_refetch_ranges(numbers)
    end
  end

  def observe_scan(_event, _measurements, metadata, {owner, reference}) do
    if Process.whereis(Sanitizer) == self() do
      case metadata do
        %{query: "SELECT DISTINCT" <> _ = query, params: params, result: {:ok, %{rows: rows}}} ->
          send(owner, {:scan_batch, reference, self(), List.flatten(rows), query, params})

          receive do
            {:resume_scan, ^reference} -> :ok
          after
            10_000 -> exit(:scan_observer_timeout)
          end

        %{query: "SET LOCAL statement_timeout" <> _} ->
          %{rows: [[deadline]]} = Repo.query!("SHOW statement_timeout")
          send(owner, {:statement_deadline, reference, self(), deadline})

        %{query: "UPDATE \"blocks\"" <> _, result: {:ok, %{rows: rows}}} ->
          send(owner, {:refetch_update, reference, self(), Enum.sort(List.flatten(rows))})

        _ ->
          :ok
      end
    end
  end

  defp open_scanner(repo) do
    owner = self()

    pid =
      :proc_lib.spawn(fn ->
        Repo.put_dynamic_repo(repo)
        Process.register(self(), Sanitizer)
        {:ok, state} = Sanitizer.init(:ok)
        send(owner, {:scanner_started, self()})
        :gen_server.enter_loop(Sanitizer, [], state)
      end)

    on_exit(fn ->
      if Process.alive?(pid) do
        monitor = Process.monitor(pid)
        Process.exit(pid, :kill)
        assert_receive {:DOWN, ^monitor, :process, ^pid, _}, 5_000
      end
    end)

    assert_receive {:scanner_started, ^pid}, 5_000
    pid
  end

  defp assert_batch(reference, worker, expected, cursor) do
    assert_receive {:scan_batch, ^reference, ^worker, actual, query, params}, 5_000
    assert actual == expected
    assert length(actual) <= 100
    assert query =~ "ORDER BY"
    assert query =~ "LIMIT"
    assert query =~ "refetch_needed"
    assert params == [cursor, 100]
  end

  defp assert_deadline_and_update(reference, worker, expected) do
    assert_receive {:statement_deadline, ^reference, ^worker, "15s"}, 5_000
    assert_receive {:refetch_update, ^reference, ^worker, actual}, 5_000
    assert actual == expected
    assert length(actual) <= 100
  end

  defp marked_rows do
    Repo.all(from(b in Block, where: b.refetch_needed == true, order_by: b.number, select: {b.number, b.updated_at}))
  end

  defp marked_numbers, do: Enum.map(marked_rows(), &elem(&1, 0))

  defp assert_refetch_ranges(expected) do
    actual = MissingBlockRange.get_latest_batch(1_000) |> Enum.flat_map(&Enum.to_list/1) |> Enum.sort()
    assert actual == expected
  end

  defp seed_recorded_history(numbers) do
    raw_block = recorded_response("block_importer_import_1_pending.json")
    decoded = Blocks.from_responses([%{id: 0, result: raw_block}], %{0 => %{number: raw_block["number"]}})
    assert decoded.errors == []
    [block_params] = decoded.blocks_params
    receipt = recorded_response("import_receipt_perform_1.json")

    # Expand the committed receipt/block shapes to a gapped, multi-batch
    # history. Only identities/heights and consensus flags vary between copies.
    raw_log = %{
      hd(receipt["logs"])
      | "topics" => [
          TokenTransfer.constant(),
          "0x000000000000000000000000556813d9cc20acfe8388af029a679d34a63388db",
          "0x00000000000000000000000092148dd870fa1b7c4700f2bd7f44238821c26f73"
        ]
    }

    transfers =
      for number <- numbers ++ [1_000, 1_002], reduce: [] do
        acc ->
          hash = "0x" <> Base.encode16(<<number + 1::256>>, case: :lower)

          block =
            %Block{}
            |> Block.changeset(
              Map.merge(block_params, %{
                number: number,
                hash: hash,
                parent_hash: "0x" <> Base.encode16(<<number::256>>, case: :lower),
                consensus: rem(number, 4) == 0,
                refetch_needed: number == 1_002
              })
            )
            |> Repo.insert!()

          log = Map.merge(raw_log, %{"blockHash" => hash, "blockNumber" => EthereumJSONRPC.integer_to_quantity(number)})

          logs =
            [%{receipt | "logs" => [log]}]
            |> Receipts.to_elixir()
            |> Receipts.elixir_to_logs()
            |> Logs.elixir_to_params()

          %{token_transfers: [transfer]} = TokenTransfers.parse(logs)
          consensus = if number == 1_000, do: block.consensus, else: not block.consensus
          transfer = Map.put(transfer, :block_consensus, consensus)
          assert TokenTransfer.changeset(%TokenTransfer{}, transfer).valid?
          duplicate = Map.put(transfer, :log_index, transfer.log_index + 1)
          [transfer, duplicate | acc]
      end

    changes = Enum.map(transfers, &TokenTransfer.changeset(%TokenTransfer{}, &1).changes)
    timestamps = %{inserted_at: DateTime.utc_now(), updated_at: DateTime.utc_now()}
    multi = TransferRunner.run(Ecto.Multi.new(), changes, %{timestamps: timestamps})
    assert {:ok, %{token_transfers: imported}} = Repo.transaction(multi)
    assert length(imported) == 2 * (length(numbers) + 2)
  end

  defp recorded_response(name) do
    [record] =
      Path.expand("../../../explorer/test/support/fixture/vcr_cassettes/#{name}", __DIR__)
      |> File.read!()
      |> Jason.decode!()

    record["response"]["body"] |> Jason.decode!() |> Map.fetch!("result")
  end

  defp await_initialized(queue, attempts \\ 100) do
    if :sys.get_state(queue).init_task != :complete do
      assert attempts > 0, "live buffered queue did not finish initialization"
      Process.sleep(10)
      await_initialized(queue, attempts - 1)
    end
  end
end
