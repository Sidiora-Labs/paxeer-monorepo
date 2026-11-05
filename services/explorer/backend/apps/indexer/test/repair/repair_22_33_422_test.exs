defmodule Indexer.Repair2233422Test do
  use ExUnit.Case, async: false

  import ExUnit.CaptureLog

  alias Indexer.BufferedTask
  alias Indexer.Memory.{Monitor, Shrinkable}

  @moduletag :capture_log
  @receive_timeout 5_000
  @blocks_fixture Path.expand(
                    "../../../ethereum_jsonrpc/test/support/fixture/paxeer_x/consecutive_blocks.json",
                    __DIR__
                  )

  defmodule RecordedBlocks do
    @behaviour BufferedTask

    @impl BufferedTask
    def init(initial, reducer, %{entries: entries}) do
      Enum.reduce(entries, initial, reducer)
    end

    @impl BufferedTask
    def run(batch, %{owner: owner}) do
      send(owner, {:running, self(), batch})

      receive do
        :release -> :ok
      end
    end
  end

  setup do
    task_supervisor = start_supervised!({Task.Supervisor, []})

    monitor =
      start_supervised!(
        {Monitor, [%{limit: 1, timer_interval: :timer.hours(1)}, [name: __MODULE__.MemoryMonitor]]},
        restart: :temporary
      )

    block_numbers =
      @blocks_fixture
      |> File.read!()
      |> Jason.decode!()
      |> Map.values()
      |> Enum.map(&EthereumJSONRPC.quantity_to_integer(&1["number"]))
      |> Enum.sort()

    assert block_numbers == [24_203_005, 24_203_006]

    %{monitor: monitor, task_supervisor: task_supervisor, block_numbers: block_numbers}
  end

  for {stage, request, occurrence} <- [
        {"before queue inspection", :queue_size, 1},
        {"between inspection and shrink", :shrink, 1},
        {"after shrink before queue inspection", :queue_size, 2}
      ] do
    test "participant: surviving queue is serviced when a participant exits #{stage}", context do
      assert_survivor_serviced(
        context,
        unquote(request),
        unquote(occurrence)
      )
    end
  end

  test "participant: normal and supervised shutdowns are reported by both queue operations", context do
    for reason <- [:normal, :shutdown, {:shutdown, :restart}], operation <- [:queue_size, :shrink] do
      participant = start_buffered(context, 40)
      :ok = :sys.suspend(participant.pid)

      call =
        Task.Supervisor.async_nolink(context.task_supervisor, fn ->
          case operation do
            :queue_size -> Shrinkable.queued_entry_count(participant.pid)
            :shrink -> Shrinkable.shrink(participant.pid)
          end
        end)

      wait_for_call(participant.pid, operation)
      :ok = :sys.terminate(participant.pid, reason)
      assert {:error, :not_alive} = Task.await(call, @receive_timeout)
    end
  end

  test "participant: a dead PID with zero memory is removed before division", context do
    participant = start_buffered(context, 40)
    ref = Process.monitor(participant.pid)

    :ok = :sys.suspend(context.monitor)
    send(context.monitor, :check)
    Process.exit(participant.pid, :kill)
    assert_receive {:DOWN, ^ref, :process, _, :killed}, @receive_timeout
    :ok = :sys.resume(context.monitor)

    state = :sys.get_state(context.monitor)
    assert Process.alive?(context.monitor)
    assert state.shrunk?
    assert MapSet.size(state.shrinkable_set) == 0
    assert {:error, :not_alive} = Shrinkable.queued_entry_count(participant.pid)
    assert {:error, :not_alive} = Shrinkable.shrink(participant.pid)
  end

  test "participant: reaching minimum size still advances to the next queue", context do
    full = start_buffered(context, 4_000)
    survivor = start_buffered(context, 40)

    shrink_to_minimum(full.pid)
    assert {:error, :minimum_size} = Shrinkable.shrink(full.pid)
    assert_memory_order(full.pid, survivor.pid)

    send(context.monitor, :check)
    state = :sys.get_state(context.monitor)

    assert state.shrinkable_set == MapSet.new([full.pid, survivor.pid])
    assert Shrinkable.queued_entry_count(survivor.pid) == div(survivor.queued, 2)
    assert Shrinkable.shrunk?(survivor.pid)
  end

  test "participant: unexpected participant faults remain visible", context do
    participant = start_buffered(context, 40)

    :ok = :sys.suspend(participant.pid)

    call =
      Task.Supervisor.async_nolink(context.task_supervisor, fn ->
        Shrinkable.queued_entry_count(participant.pid)
      end)

    wait_for_call(participant.pid, :queue_size)
    :ok = :sys.terminate(participant.pid, :unexpected_participant_fault)

    assert {:exit, {:unexpected_participant_fault, {GenServer, :call, _}}} =
             Task.yield(call, @receive_timeout)
  end

  test "participant: an unresponsive live queue still reports a timeout", context do
    participant = start_buffered(context, 40)
    :ok = :sys.suspend(participant.pid)

    try do
      assert {:timeout, {GenServer, :call, _}} = catch_exit(Shrinkable.queued_entry_count(participant.pid))
      assert Process.alive?(participant.pid)
    after
      :ok = :sys.resume(participant.pid)
    end
  end

  defp assert_survivor_serviced(context, request, occurrence) do
    participant = start_buffered(context, 4_000)
    survivor = start_buffered(context, 40)
    pause_at_call(participant.pid, request, occurrence)
    assert_memory_order(participant.pid, survivor.pid)
    ref = Process.monitor(participant.pid)

    log =
      capture_log(fn ->
        send(context.monitor, :check)
        assert_receive {:at_call, pid, ^request, ^occurrence}, @receive_timeout
        assert pid == participant.pid

        Process.exit(pid, :kill)

        assert_receive {:DOWN, ^ref, :process, ^pid, :killed}, @receive_timeout
        state = :sys.get_state(context.monitor)
        assert Process.alive?(context.monitor)
        assert state.shrunk?
        assert state.shrinkable_set == MapSet.new([survivor.pid])
        Logger.flush()
      end)

    assert log =~ "exited during memory pressure handling; continuing with remaining queues"
    assert log =~ "shed #{survivor.queued - div(survivor.queued, 2)} queued entries"
    assert Shrinkable.queued_entry_count(survivor.pid) == div(survivor.queued, 2)
    assert Shrinkable.shrunk?(survivor.pid)

    send(survivor.worker, :release)
    assert_receive {:running, next_worker, batch}, @receive_timeout
    assert next_worker != survivor.worker
    assert batch == context.block_numbers |> Stream.cycle() |> Stream.drop(2) |> Enum.take(2)

    :sys.replace_state(context.monitor, &%{&1 | limit: :erlang.memory(:total) * 10})
    send(context.monitor, :check)
    state = :sys.get_state(context.monitor)
    refute state.shrunk?
    refute Shrinkable.shrunk?(survivor.pid)
    assert state.shrinkable_set == MapSet.new([survivor.pid])
  end

  defp start_buffered(context, count) do
    entries = context.block_numbers |> Stream.cycle() |> Enum.take(count)

    pid =
      start_supervised!(
        {BufferedTask,
         [
           {RecordedBlocks,
            state: %{owner: self(), entries: entries},
            task_supervisor: context.task_supervisor,
            flush_interval: :timer.hours(1),
            max_batch_size: 2,
            max_concurrency: 1,
            poll: false,
            memory_monitor: context.monitor},
           []
         ]},
        id: make_ref(),
        restart: :temporary
      )

    assert_receive {:running, worker, batch}, @receive_timeout
    assert batch == Enum.take(entries, 2)
    wait_for_queue(pid, count - 2)

    %{pid: pid, worker: worker, queued: count - 2}
  end

  defp wait_for_queue(pid, count, attempts \\ 100) do
    case Shrinkable.queued_entry_count(pid) do
      ^count ->
        :ok

      _ when attempts > 0 ->
        Process.sleep(10)
        wait_for_queue(pid, count, attempts - 1)

      actual ->
        flunk("expected #{count} queued entries, got #{inspect(actual)}")
    end
  end

  defp pause_at_call(pid, request, occurrence) do
    owner = self()

    :ok =
      :sys.install(
        pid,
        {fn seen, event, _process ->
           case event do
             {:in, {:"$gen_call", _, ^request}} ->
               seen = seen + 1

               if seen == occurrence do
                 send(owner, {:at_call, self(), request, occurrence})

                 receive do
                   :continue -> :ok
                 end
               end

               seen

             _ ->
               seen
           end
         end, 0}
      )
  end

  defp wait_for_call(pid, request, attempts \\ 100) do
    {:messages, messages} = Process.info(pid, :messages)

    if Enum.any?(messages, &match?({:"$gen_call", _, ^request}, &1)) do
      :ok
    else
      assert attempts > 0, "participant did not receive #{inspect(request)}"
      Process.sleep(10)
      wait_for_call(pid, request, attempts - 1)
    end
  end

  defp assert_memory_order(first, second) do
    {:memory, first_memory} = Process.info(first, :memory)
    {:memory, second_memory} = Process.info(second, :memory)
    assert first_memory > second_memory
  end

  defp shrink_to_minimum(pid) do
    case Shrinkable.shrink(pid) do
      :ok -> shrink_to_minimum(pid)
      {:error, :minimum_size} -> :ok
    end
  end
end
