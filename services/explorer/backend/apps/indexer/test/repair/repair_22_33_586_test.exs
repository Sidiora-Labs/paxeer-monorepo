defmodule Indexer.Repair2233586Test do
  use Explorer.DataCase, async: false

  alias Explorer.Chain.Block
  alias Explorer.Utility.{MassiveBlock, MissingBlockRange}
  alias Indexer.Block.Catchup.{BoundIntervalSupervisor, Fetcher, MassiveBlocksFetcher}
  alias Indexer.Block.Catchup.TaskSupervisor

  @moduledoc """
  Repair record 22.33.586:
  * catchup-monitor-cleanup: task results settle their monitors and schedule one replacement.
  * massive-range-handoff: interrupted handoff retains durable ownership and recovers after restart.
  * massive-retry-discard: RPC errors and imports of another height survive restart until a successful retry.

  The task's implementation-before-qualification gate order precludes an unchanged-tree verification;
  no baseline result is claimed. The previous attempt reported six passing cases and a shutdown in
  "massive-retry-discard: wrong_height survives restart until the requested block is imported".
  No out-of-path wiring is required.
  """

  @fixture Path.expand("../../../ethereum_jsonrpc/test/support/fixture/paxeer_x/consecutive_blocks.json", __DIR__)
  @external_resource @fixture
  @first 24_203_005
  @last 24_203_006
  @moduletag capture_log: true

  setup do
    preserve_env(:indexer, [:block_ranges, :json_rpc_named_arguments, Fetcher])
    Application.put_env(:indexer, :block_ranges, "#{@first}..#{@last}")
    Application.put_env(:indexer, Fetcher, batch_size: 2, concurrency: 1)
    start_supervised!({Task.Supervisor, name: TaskSupervisor})

    recorded = @fixture |> File.read!() |> Jason.decode!()
    blocks = Map.new(recorded, fn {quantity, block} -> {EthereumJSONRPC.quantity_to_integer(quantity), block} end)
    responses = start_supervised!({Agent, fn -> :success end})
    server = Bypass.open()
    owner = self()

    Bypass.stub(server, "POST", "/", fn conn ->
      {:ok, body, conn} = Plug.Conn.read_body(conn)
      requests = Jason.decode!(body)
      outcome = Agent.get(responses, & &1)

      replies =
        Enum.map(requests, fn %{"jsonrpc" => "2.0", "id" => id, "method" => method, "params" => params} ->
          case {method, params} do
            {"eth_getBlockByNumber", [quantity, true]} ->
              number = EthereumJSONRPC.quantity_to_integer(quantity)
              assert number in @first..@last

              case outcome do
                :rpc_error ->
                  %{"jsonrpc" => "2.0", "id" => id, "error" => %{"code" => -32000, "message" => "header not found"}}

                :wrong_height ->
                  %{"jsonrpc" => "2.0", "id" => id, "result" => Map.fetch!(blocks, @last)}

                :success ->
                  %{"jsonrpc" => "2.0", "id" => id, "result" => Map.fetch!(blocks, number)}
              end

            {"trace_block", [_quantity]} ->
              %{"jsonrpc" => "2.0", "id" => id, "result" => []}

            {"eth_getBalance", [_address, _quantity]} ->
              %{"jsonrpc" => "2.0", "id" => id, "result" => "0x0"}
          end
        end)

      send(owner, {:rpc_exchange, outcome, requests, replies})

      conn
      |> Plug.Conn.put_resp_content_type("application/json")
      |> Plug.Conn.resp(200, Jason.encode!(replies))
    end)

    arguments = [
      transport: EthereumJSONRPC.HTTP,
      transport_options: [
        http: EthereumJSONRPC.HTTP.HTTPoison,
        urls: ["http://127.0.0.1:#{server.port}/"],
        http_options: [timeout: 2_000, recv_timeout: 2_000]
      ],
      variant: EthereumJSONRPC.Nethermind
    ]

    Application.put_env(:indexer, :json_rpc_named_arguments, arguments)

    block_fetcher = %Indexer.Block.Fetcher{
      callback_module: Fetcher,
      json_rpc_named_arguments: arguments,
      receipts_batch_size: 10,
      receipts_concurrency: 1,
      task_supervisor: TaskSupervisor
    }

    %{arguments: arguments, block_fetcher: block_fetcher, responses: responses, blocks: blocks}
  end

  for reason <- [:etimedout, :enetunreach, :econnrefused] do
    @reason reason
    test "catchup-monitor-cleanup: #{@reason} task results settle their monitors and schedule one replacement",
         context do
      owner = self()

      pid =
        start_supervised!({BoundIntervalSupervisor, [%{block_fetcher: context.block_fetcher, block_interval: 60_000}]})

      eventually(fn -> assert :sys.get_state(pid).task == nil end)

      debug = fn state, event, _name ->
        if event == {:in, :catchup_index}, do: send(owner, :replacement)
        state
      end

      :ok = :sys.install(pid, {debug, nil})
      state = :sys.replace_state(pid, &put_fixture_task/1)
      %Task{pid: task_pid, ref: ref} = state.task
      monitor = Process.monitor(task_pid)
      :ok = :sys.suspend(pid)
      send(task_pid, {:finish, {:error, @reason}})
      assert_receive {:DOWN, ^monitor, :process, ^task_pid, :normal}, 2_000

      eventually(fn ->
        {:messages, messages} = Process.info(pid, :messages)
        assert {ref, {:error, @reason}} in messages
        assert {:DOWN, ref, :process, task_pid, :normal} in messages
      end)

      :ok = :sys.resume(pid)

      assert_receive :replacement, 2_000
      eventually(fn -> assert :sys.get_state(pid).task == nil end)
      refute_receive :replacement, 100
      assert Process.alive?(pid)
      {:monitors, monitors} = Process.info(pid, :monitors)
      refute {:process, task_pid} in monitors

      send(pid, {ref, {:error, @reason}})
      send(pid, {:DOWN, ref, :process, task_pid, :normal})
      assert :sys.get_state(pid).task == nil
      refute_receive :replacement, 100

      current = :sys.replace_state(pid, &put_fixture_task/1)
      send(pid, {ref, {:error, @reason}})
      send(pid, {:DOWN, ref, :process, task_pid, :normal})
      send(pid, :catchup_index)
      assert :sys.get_state(pid).task == current.task
      assert Process.alive?(pid)
      assert :ok = Supervisor.terminate_child(pid, :task)
    end
  end

  test "massive-range-handoff: interrupted handoff retains durable ownership and recovers after restart", context do
    MissingBlockRange.save_batch([@last..@first])
    install_timeout_trigger()

    sql!("""
    CREATE FUNCTION pg_temp.repair_2233586_handoff() RETURNS trigger AS $$
    BEGIN
      RAISE EXCEPTION 'interrupted handoff';
    END;
    $$ LANGUAGE plpgsql
    """)

    sql!("""
    CREATE TRIGGER repair_2233586_handoff BEFORE DELETE ON missing_block_ranges
    FOR EACH ROW EXECUTE FUNCTION pg_temp.repair_2233586_handoff()
    """)

    first = run_catchup(context.block_fetcher)
    refute Process.alive?(first)
    assert MissingBlockRange.get_latest_batch(2) == [@last..@first]
    assert Enum.sort(Repo.all(from(b in MassiveBlock, select: b.number))) == [@first, @last]
    assert Repo.all(Block) == []
    assert_receive {:rpc_exchange, :success, _, _}

    sql!("DROP TRIGGER repair_2233586_handoff ON missing_block_ranges")
    reopened = run_catchup(context.block_fetcher)
    assert reopened != first
    assert MissingBlockRange.get_latest_batch(2) == []
    assert Enum.sort(Repo.all(from(b in MassiveBlock, select: b.number))) == [@first, @last]

    sql!("DROP TRIGGER repair_2233586_timeout ON blocks")
    start_import_peers(context.arguments)
    pid = start_massive()

    eventually(fn ->
      assert Repo.all(MassiveBlock) == []
      assert Enum.sort(Repo.all(from(b in Block, select: b.number))) == [@first, @last]
    end)

    assert Process.alive?(pid)
  end

  test "massive-range-handoff: destination insertion failure keeps the finite source range", context do
    MissingBlockRange.save_batch([@last..@first])
    install_timeout_trigger()
    sql!("ALTER TABLE massive_blocks ADD CONSTRAINT repair_2233586_destination CHECK (number < #{@first})")

    first = run_catchup(context.block_fetcher)
    refute Process.alive?(first)
    assert MissingBlockRange.get_latest_batch(2) == [@last..@first]
    assert Repo.all(MassiveBlock) == []

    sql!("ALTER TABLE massive_blocks DROP CONSTRAINT repair_2233586_destination")
    reopened = run_catchup(context.block_fetcher)
    assert reopened != first
    assert MissingBlockRange.get_latest_batch(2) == []
    assert Enum.sort(Repo.all(from(b in MassiveBlock, select: b.number))) == [@first, @last]
  end

  for outcome <- [:rpc_error, :wrong_height] do
    @outcome outcome
    test "massive-retry-discard: #{@outcome} survives restart until the requested block is imported", context do
      start_import_peers(context.arguments)
      MassiveBlock.insert_block_numbers([@first])
      Agent.update(context.responses, fn _ -> @outcome end)
      pid = start_massive()

      assert_receive {:rpc_exchange, @outcome, requests, replies}, 5_000
      assert [%{"method" => "eth_getBlockByNumber", "params" => [quantity, true]}] = requests
      assert EthereumJSONRPC.quantity_to_integer(quantity) == @first

      if @outcome == :rpc_error do
        assert [%{"error" => %{"code" => -32000, "message" => "header not found"}}] = replies
      else
        assert [%{"result" => %{"number" => "0x1714efe"}}] = replies
      end

      eventually(fn -> assert :sys.get_state(pid).low_priority_blocks == [@first] end)
      assert Repo.get!(MassiveBlock, @first)
      refute Repo.get_by(Block, number: @first)
      assert MissingBlockRange.get_latest_batch(2) == []
      monitor = Process.monitor(pid)
      Process.exit(pid, :shutdown)
      assert_receive {:DOWN, ^monitor, :process, ^pid, :shutdown}, 2_000
      refute Process.alive?(pid)
      assert Repo.get!(MassiveBlock, @first)

      {:ok, supervisor} = ExUnit.fetch_test_supervisor()

      eventually(fn ->
        assert [{:massive, reopened, :worker, [MassiveBlocksFetcher]}] =
                 Enum.filter(Supervisor.which_children(supervisor), &(elem(&1, 0) == :massive))

        assert is_pid(reopened)
        assert reopened != pid
        assert Process.alive?(reopened)
      end)

      [{:massive, reopened, :worker, [MassiveBlocksFetcher]}] =
        Enum.filter(Supervisor.which_children(supervisor), &(elem(&1, 0) == :massive))

      assert reopened != pid
      eventually(fn -> assert :sys.get_state(reopened).low_priority_blocks == [@first] end)
      assert Repo.get!(MassiveBlock, @first)
      Agent.update(context.responses, fn _ -> :success end)

      eventually(fn ->
        refute Repo.get(MassiveBlock, @first)
        block = Repo.get_by!(Block, number: @first)
        assert to_string(block.hash) == context.blocks[@first]["hash"]
        assert block.consensus
      end)

      assert :sys.get_state(reopened).low_priority_blocks == []
    end
  end

  defp put_fixture_task(state) do
    task =
      Task.Supervisor.async_nolink(TaskSupervisor, fn ->
        receive do
          {:finish, result} -> result
        end
      end)

    %{state | task: task}
  end

  defp run_catchup(block_fetcher) do
    task = Task.Supervisor.async_nolink(TaskSupervisor, Fetcher, :task, [%Fetcher{block_fetcher: block_fetcher}])
    monitor = Process.monitor(task.pid)
    assert %{missing_block_count: 2} = Task.await(task, 10_000)
    assert_receive {:DOWN, ^monitor, :process, _, :normal}, 2_000
    task.pid
  end

  defp start_massive do
    {MassiveBlocksFetcher, [%{task_supervisor: TaskSupervisor}]}
    |> Supervisor.child_spec(id: :massive)
    |> start_supervised!()
  end

  defp install_timeout_trigger do
    sql!("""
    CREATE FUNCTION pg_temp.repair_2233586_timeout() RETURNS trigger AS $$
    BEGIN
      RAISE EXCEPTION 'canceling statement due to user request';
    END;
    $$ LANGUAGE plpgsql
    """)

    sql!("""
    CREATE TRIGGER repair_2233586_timeout BEFORE INSERT ON blocks
    FOR EACH ROW EXECUTE FUNCTION pg_temp.repair_2233586_timeout()
    """)
  end

  defp start_import_peers(arguments) do
    preserve_env(:indexer, [:last_block])
    Application.put_env(:indexer, :last_block, 0)
    start_supervised!({Indexer.Fetcher.OnDemand.ContractCreator, []})

    for module <- [
          Indexer.Fetcher.CoinBalance.Catchup.Supervisor,
          Indexer.Fetcher.InternalTransaction.Supervisor,
          Indexer.Fetcher.ContractCode.Supervisor,
          Indexer.Fetcher.Token.Supervisor,
          Indexer.Fetcher.TokenBalance.Historical.Supervisor,
          Indexer.Fetcher.TokenBalance.Current.Supervisor,
          Indexer.Fetcher.ReplacedTransaction.Supervisor
        ] do
      start_supervised!({module, [[json_rpc_named_arguments: arguments, flush_interval: 60_000]]})
    end

    start_supervised!(
      {Indexer.Fetcher.UncleBlock.Supervisor,
       [
         [
           block_fetcher: %Indexer.Block.Fetcher{json_rpc_named_arguments: arguments, task_supervisor: TaskSupervisor},
           flush_interval: 60_000
         ]
       ]}
    )
  end

  defp sql!(statement), do: Ecto.Adapters.SQL.query!(Repo, statement, [])

  defp eventually(assertion, attempts \\ 200)
  defp eventually(assertion, 0), do: assertion.()

  defp eventually(assertion, attempts) do
    assertion.()
  rescue
    _error in [ExUnit.AssertionError, Ecto.NoResultsError] ->
      Process.sleep(10)
      eventually(assertion, attempts - 1)
  end

  defp preserve_env(application, keys) do
    previous = Enum.map(keys, &{&1, Application.fetch_env(application, &1)})

    on_exit(fn ->
      for {key, value} <- previous do
        case value do
          {:ok, original} -> Application.put_env(application, key, original)
          :error -> Application.delete_env(application, key)
        end
      end
    end)
  end
end
