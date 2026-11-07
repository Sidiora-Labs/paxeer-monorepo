Code.require_file("test_config.exs", __DIR__)

defmodule Indexer.Block.Realtime.PaxeerBatchCase.Transcript do
  @moduledoc false
  @behaviour :cowboy_handler

  @impl true
  def init(request, %{node: node, recordings: recordings} = state) do
    "POST" = :cowboy_req.method(request)
    {:ok, body, request} = :cowboy_req.read_body(request)
    payload = Jason.decode!(body)

    head =
      Agent.get_and_update(node, fn state ->
        {state.head, %{state | calls: state.calls ++ [payload]}}
      end)

    responses = Enum.map(List.wrap(payload), &reply(&1, head, recordings))
    response = if is_list(payload), do: responses, else: hd(responses)
    request = :cowboy_req.reply(200, %{"content-type" => "application/json"}, Jason.encode!(response), request)
    {:ok, request, state}
  end

  defp reply(%{"id" => id, "jsonrpc" => "2.0", "method" => method, "params" => params}, head, recordings) do
    params =
      case {method, params} do
        {"eth_getBlockByNumber", ["latest", full]} -> [EthereumJSONRPC.integer_to_quantity(head), full]
        _ -> params
      end

    response = Map.fetch!(recordings, key(method, params))
    Map.put(response, "id", id)
  end

  def key(method, params) do
    {method, Enum.map(params, &normalize/1)}
  end

  defp normalize("0x" <> value), do: "0x" <> String.downcase(value)
  defp normalize(value), do: value
end

defmodule Indexer.Block.Realtime.PaxeerBatchCase do
  @moduledoc false
  use ExUnit.CaseTemplate

  import Ecto.Query

  alias Explorer.Chain.{Block, Transaction}
  alias Explorer.Repo
  alias Indexer.Block.Realtime.{Fetcher, TestConfig}
  alias Indexer.Block.Realtime.PaxeerBatchCase.Transcript
  alias Indexer.Fetcher.{ContractCode, ReplacedTransaction}

  using do
    quote do
      import Indexer.Block.Realtime.PaxeerBatchCase
    end
  end

  setup context do
    fixture = __DIR__ |> Path.join("fixtures/chain_125_batch.json") |> File.read!() |> Jason.decode!()

    blocks =
      for %{
            "request" => %{"method" => "eth_getBlockByNumber", "params" => [_, true]},
            "response" => %{"result" => block}
          } <- fixture["exchanges"],
          do: block

    first = EthereumJSONRPC.quantity_to_integer(hd(blocks)["number"])
    node = start_supervised!({Agent, fn -> %{head: first, calls: []} end})

    recordings =
      Map.new(fixture["exchanges"], fn %{"request" => request, "response" => response} ->
        {Transcript.key(request["method"], request["params"]), response}
      end)

    listener = make_ref()
    dispatch = :cowboy_router.compile([{:_, [{"/", Transcript, %{node: node, recordings: recordings}}]}])
    {:ok, _} = :cowboy.start_clear(listener, [ip: {127, 0, 0, 1}, port: 0], %{env: %{dispatch: dispatch}})
    on_exit(fn -> :cowboy.stop_listener(listener) end)
    url = "http://127.0.0.1:#{:ranch.get_port(listener)}"

    arguments = [
      transport: EthereumJSONRPC.HTTP,
      transport_options: [
        http: EthereumJSONRPC.HTTP.HTTPoison,
        http_options: [timeout: 5_000, recv_timeout: 5_000],
        urls: [url],
        fallback_urls: [],
        trace_urls: [url],
        fallback_trace_urls: [],
        method_to_url: [debug_traceTransaction: :trace, debug_traceBlockByNumber: :trace]
      ],
      variant: Map.get(context, :json_rpc_variant, EthereumJSONRPC.PaxeerX)
    ]

    TestConfig.preserve_env(
      TestConfig.keys() ++
        [
          {:indexer, Fetcher},
          {:indexer, Indexer.Fetcher.CoinBalance.Realtime.Supervisor},
          {:ethereum_jsonrpc, EthereumJSONRPC.Geth}
        ]
    )

    Application.put_env(:indexer, Fetcher, batch_size: 2, max_lag: 10, polling_period: :timer.hours(1))
    Application.put_env(:indexer, :fetch_rewards_way, nil)
    Application.put_env(:ethereum_jsonrpc, EthereumJSONRPC.Geth, tracer: "call_tracer", debug_trace_timeout: "5s")

    for supervisor <- [
          Indexer.Fetcher.InternalTransaction.Supervisor,
          Indexer.Fetcher.UncleBlock.Supervisor,
          Indexer.Fetcher.CoinBalance.Realtime.Supervisor,
          Indexer.Fetcher.Celo.EpochBlockOperations.Supervisor,
          Indexer.Fetcher.TokenInstance.Realtime.Supervisor
        ],
        do: Application.put_env(:indexer, supervisor, disabled?: true)

    start_supervised!({Task.Supervisor, name: Indexer.TaskSupervisor})
    start_supervised!({Task.Supervisor, name: Indexer.Block.Realtime.TaskSupervisor})
    ContractCode.Supervisor.Case.start_supervised!(json_rpc_named_arguments: arguments)
    ReplacedTransaction.Supervisor.Case.start_supervised!()
    start_supervised!({Indexer.Fetcher.OnDemand.ContractCreator, [[], []]})

    %{
      fixture: fixture,
      blocks: blocks,
      first: first,
      node: node,
      arguments: arguments,
      block_fetcher: TestConfig.block_fetcher(arguments)
    }
  end

  def assert_batch_recovery(context) do
    assert context.fixture["capture"]["kind"] == "recorded"
    assert context.fixture["capture"]["chain_id"] == 125
    assert {:ok, 125} = EthereumJSONRPC.fetch_chain_id(context.arguments)
    assert Enum.any?(context.blocks, &(&1["transactions"] != []))

    for [parent, child] <- Enum.chunk_every(context.blocks, 2, 1, :discard) do
      assert child["parentHash"] == parent["hash"]
    end

    first_pid = start_fetcher(context.block_fetcher)
    poll(first_pid)
    assert_imported(Enum.take(context.blocks, 1))

    Agent.update(context.node, &%{&1 | head: context.first + 3})
    poll(first_pid)
    assert_imported(Enum.take(context.blocks, 4))
    full_batches = full_block_batches(context.node)
    assert Enum.sort(full_batches) == [[context.first], [context.first + 1, context.first + 2], [context.first + 3]]

    before_restart = persisted_identity()
    assert :ok = stop_supervised(:paxeer_realtime_fetcher)
    refute Process.alive?(first_pid)
    assert persisted_identity() == before_restart

    reopened_pid = start_fetcher(context.block_fetcher)
    assert reopened_pid != first_pid
    assert :sys.get_state(reopened_pid).previous_number == nil
    poll(reopened_pid)
    assert_imported(Enum.take(context.blocks, 4))
    assert persisted_identity() == before_restart
    assert full_block_batches(context.node) == full_batches ++ [[context.first + 3]]

    Agent.update(context.node, &%{&1 | head: context.first + 4})
    poll(reopened_pid)
    assert_imported(context.blocks)
    assert :sys.get_state(reopened_pid).previous_number == context.first + 4
    assert Enum.all?(full_block_batches(context.node), &(length(&1) <= 2))
    :ok
  end

  def assert_imported(blocks) do
    expected_numbers = Enum.map(blocks, &EthereumJSONRPC.quantity_to_integer(&1["number"]))
    await(fn -> Repo.all(from(b in Block, order_by: b.number, select: b.number)) == expected_numbers end)
    await_tasks()
    actual_blocks = Repo.all(from(b in Block, order_by: b.number))
    assert length(actual_blocks) == length(blocks)

    for {actual, recorded} <- Enum.zip(actual_blocks, blocks) do
      assert actual.consensus
      assert to_string(actual.hash) == recorded["hash"]
      assert to_string(actual.parent_hash) == recorded["parentHash"]

      assert actual.base_fee_per_gas.value ==
               Decimal.new(EthereumJSONRPC.quantity_to_integer(recorded["baseFeePerGas"]))

      assert actual.gas_used == Decimal.new(EthereumJSONRPC.quantity_to_integer(recorded["gasUsed"]))
    end

    recorded_transactions = Enum.flat_map(blocks, & &1["transactions"])
    actual_transactions = Repo.all(Transaction)

    assert Enum.sort(Enum.map(actual_transactions, &to_string(&1.hash))) ==
             Enum.sort(Enum.map(recorded_transactions, & &1["hash"]))

    for actual <- actual_transactions do
      recorded = Enum.find(recorded_transactions, &(&1["hash"] == to_string(actual.hash)))
      assert to_string(actual.block_hash) == recorded["blockHash"]
      assert actual.block_number == EthereumJSONRPC.quantity_to_integer(recorded["blockNumber"])
      assert actual.index == EthereumJSONRPC.quantity_to_integer(recorded["transactionIndex"])
      assert actual.status == :ok
      assert actual.gas_used == Decimal.new(0x52A8)
      assert actual.input.bytes == Base.decode16!(String.upcase(String.slice(recorded["input"], 2..-1//1)))
    end
  end

  def calls(node), do: Agent.get(node, & &1.calls)

  defp full_block_batches(node) do
    for batch <- calls(node),
        is_list(batch),
        requests = Enum.filter(batch, &match?(%{"method" => "eth_getBlockByNumber", "params" => [_, true]}, &1)),
        requests != [] do
      Enum.map(requests, &EthereumJSONRPC.quantity_to_integer(hd(&1["params"])))
    end
  end

  defp start_fetcher(block_fetcher) do
    start_supervised!({Fetcher, [%{block_fetcher: block_fetcher, subscribe_named_arguments: nil}, []]},
      id: :paxeer_realtime_fetcher
    )
  end

  defp poll(pid) do
    send(pid, :poll_latest_block_number)
    :sys.get_state(pid)
    await_tasks()
  end

  defp await_tasks do
    await(fn -> Task.Supervisor.children(Indexer.Block.Realtime.TaskSupervisor) == [] end)
    await(fn -> Task.Supervisor.children(Indexer.TaskSupervisor) == [] end)
  end

  defp persisted_identity do
    {Repo.all(from(b in Block, order_by: b.number, select: {b.number, b.hash, b.consensus})),
     Repo.all(
       from(t in Transaction, order_by: t.hash, select: {t.hash, t.block_hash, t.block_number, t.index, t.status})
     )}
  end

  defp await(condition, attempts \\ 300)

  defp await(condition, attempts) when attempts > 0 do
    if condition.(),
      do: :ok,
      else:
        (
          Process.sleep(20)
          await(condition, attempts - 1)
        )
  end

  defp await(_condition, 0), do: flunk("recorded Paxeer import did not finish within 6000ms")
end
