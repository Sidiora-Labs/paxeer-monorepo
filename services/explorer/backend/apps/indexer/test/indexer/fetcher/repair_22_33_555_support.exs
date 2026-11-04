defmodule Indexer.Fetcher.Repair2233555 do
  @moduledoc false

  import ExUnit.Assertions
  import ExUnit.Callbacks
  import Explorer.Factory

  alias Explorer.{Chain, Repo}
  alias Explorer.Chain.{Block, PendingTransactionOperation}
  alias Indexer.BufferedTask
  alias Indexer.Fetcher.{BlockReward, EmptyBlocksSanitizer, InternalTransaction, Withdrawal}
  alias Indexer.Fetcher.CoinBalance.Catchup

  @fixtures Path.join(__DIR__, "fixtures/repair_22_33_555")
  @trace_fixture Path.expand("../block/realtime/fixtures/chain_125_batch.json", __DIR__)

  def wait_until(timeout, producer) do
    poll(System.monotonic_time(:millisecond) + timeout, producer)
  end

  defp poll(deadline, producer) do
    if producer.() do
      :ok
    else
      remaining = deadline - System.monotonic_time(:millisecond)
      assert remaining > 0, "completion predicate did not succeed before timeout"
      Process.sleep(min(10, remaining))
      poll(deadline, producer)
    end
  end

  def configure_sanitizer do
    original = Application.fetch_env!(:indexer, EmptyBlocksSanitizer)
    on_exit(fn -> Application.put_env(:indexer, EmptyBlocksSanitizer, original) end)
    Application.put_env(:indexer, EmptyBlocksSanitizer, Keyword.put(original, :head_offset, 1))
    :ok
  end

  def put_env(app, key, value) do
    original = Application.fetch_env(app, key)

    on_exit(fn ->
      case original do
        {:ok, previous} -> Application.put_env(app, key, previous)
        :error -> Application.delete_env(app, key)
      end
    end)

    Application.put_env(app, key, value)
  end

  def fixture(name), do: @fixtures |> Path.join(name <> ".json") |> File.read!() |> Jason.decode!()

  def serve(responder, http \\ EthereumJSONRPC.HTTP.HTTPoison) do
    server = Bypass.open()
    owner = self()

    Bypass.expect(server, "POST", "/", fn conn ->
      {:ok, body, conn} = Plug.Conn.read_body(conn)
      requests = Jason.decode!(body)
      send(owner, {:rpc_requests, requests})

      case responder.(requests) do
        {:http_error, status} ->
          Plug.Conn.resp(conn, status, "recorded peer unavailable")

        responses ->
          conn
          |> Plug.Conn.put_resp_content_type("application/json")
          |> Plug.Conn.resp(200, Jason.encode!(responses))
      end
    end)

    [
      transport: EthereumJSONRPC.HTTP,
      transport_options: [
        http: http,
        urls: ["http://127.0.0.1:#{server.port}"],
        http_options: [timeout: 5_000, recv_timeout: 5_000]
      ]
    ]
  end

  def catalog_arguments(http) do
    %{"address" => address, "results" => results} = fixture("catalog")

    if http == EthereumJSONRPC.HTTP.Tesla do
      put_env(:tesla, :adapter, Tesla.Adapter.Mint)
    end

    serve(
      fn requests ->
        Enum.map(requests, fn %{
                                "id" => id,
                                "jsonrpc" => "2.0",
                                "method" => "eth_call",
                                "params" => [%{"data" => data, "to" => ^address}, "latest"]
                              } ->
          %{jsonrpc: "2.0", id: id, result: Map.fetch!(results, data)}
        end)
      end,
      http
    )
  end

  def assert_catalog(http) do
    arguments = catalog_arguments(http)
    put_env(:explorer, :json_rpc_named_arguments, arguments)
    address = insert(:address, hash: fixture("catalog")["address"])

    token =
      insert(:token,
        contract_address: address,
        name: nil,
        symbol: nil,
        total_supply: nil,
        decimals: nil,
        cataloged: false
      )

    assert :ok = Indexer.Fetcher.Token.run([token.contract_address_hash], arguments)
    assert {:ok, cataloged} = Chain.token_from_address_hash(token.contract_address_hash)
    assert cataloged.cataloged
    assert cataloged.name == "Bancor"
    assert cataloged.symbol == "BNT"
    assert cataloged.total_supply == Decimal.new(1_000_000_000_000_000_000)
    assert cataloged.decimals == Decimal.new(18)

    assert_receive {:rpc_requests, requests}, 5_000

    assert Enum.sort(Enum.map(requests, &get_in(&1, ["params", Access.at(0), "data"]))) ==
             Enum.sort(Map.keys(fixture("catalog")["results"]))
  end

  def assert_reward_completion do
    [raw | _] = fixture("withdrawals")
    number = EthereumJSONRPC.quantity_to_integer(raw["number"])
    quantity = EthereumJSONRPC.integer_to_quantity(number)

    arguments =
      serve(fn [%{"id" => id, "jsonrpc" => "2.0", "method" => "trace_block", "params" => [^quantity]}] ->
        [
          %{
            jsonrpc: "2.0",
            id: id,
            result: [
              %{
                "action" => %{"author" => raw["miner"], "rewardType" => "external", "value" => "0x1"},
                "blockHash" => raw["hash"],
                "blockNumber" => number,
                "result" => nil,
                "subtraces" => 0,
                "traceAddress" => [],
                "transactionHash" => nil,
                "transactionPosition" => nil,
                "type" => "reward"
              }
            ]
          }
        ]
      end)
      |> Keyword.put(:variant, EthereumJSONRPC.Nethermind)

    start_balance_peer(arguments)
    miner = insert(:address, hash: raw["miner"])
    insert(:block, hash: raw["hash"], number: number, miner: miner)
    put_env(:indexer, BlockReward.Supervisor, disabled?: false)
    BlockReward.Supervisor.Case.start_supervised!(json_rpc_named_arguments: arguments)

    wait_until(5_000, fn ->
      Repo.aggregate(Chain.Block.Reward, :count) == 1 and
        :sys.get_state(BlockReward).init_task == :complete and
        BufferedTask.debug_count(BlockReward, false) == %{buffer: 0, tasks: 0}
    end)

    [reward] = Repo.all(Chain.Block.Reward)
    assert to_string(reward.block_hash) == raw["hash"]
    assert to_string(reward.address_hash) == raw["miner"]
    assert Decimal.equal?(reward.reward.value, 1)
    assert Repo.aggregate(Chain.Address.CoinBalance, :count) == 1
    assert_receive {:rpc_requests, [%{"method" => "trace_block", "params" => [^quantity]}]}, 5_000
  end

  defp trace_capture do
    capture = @trace_fixture |> File.read!() |> Jason.decode!()
    assert capture["capture"]["kind"] == "recorded"
    assert capture["capture"]["chain_id"] == 125
    assert capture["capture"]["client_version"] =~ "Geth/"
    capture
  end

  def assert_trace_queue_selection(variant) do
    capture = trace_capture()

    raw_block =
      capture["exchanges"]
      |> Enum.find(fn item ->
        item["request"]["method"] == "eth_getBlockByNumber" and
          Enum.any?(item["response"]["result"]["transactions"], &is_map/1)
      end)
      |> require_record!("recorded block for pending-queue migration")
      |> get_in(["response", "result"])

    [raw_transaction] = raw_block["transactions"]
    block = insert(:block, hash: raw_block["hash"], number: EthereumJSONRPC.quantity_to_integer(raw_block["number"]))
    transaction = insert(:transaction, hash: raw_transaction["hash"]) |> with_block(block, index: 0)
    insert(:pending_transaction_operation, transaction_hash: transaction.hash)
    arguments = [variant: variant]
    put_env(:explorer, :json_rpc_named_arguments, arguments)

    put_env(
      :indexer,
      InternalTransaction,
      Keyword.put(Application.get_env(:indexer, InternalTransaction), :lookback_blocks, nil)
    )

    geth = Application.get_env(:ethereum_jsonrpc, EthereumJSONRPC.Geth)

    for block_traceable? <- [true, false] do
      put_env(:ethereum_jsonrpc, EthereumJSONRPC.Geth, Keyword.put(geth, :block_traceable?, block_traceable?))
      assert variant in InternalTransaction.block_traceable_variants() == block_traceable?
      assert :ok = Chain.PendingOperationsHelper.maybe_transfuse_data()

      if block_traceable? do
        assert Chain.PendingOperationsHelper.pending_operations_type() == "blocks"
        assert Chain.PendingOperationsHelper.actual_entity() == Chain.PendingBlockOperation
        assert Repo.get!(Chain.PendingBlockOperation, block.hash).block_number == block.number
        assert Repo.aggregate(PendingTransactionOperation, :count) == 0
        assert InternalTransaction.init([], &[&1 | &2], arguments) == [block.number]
      else
        assert Chain.PendingOperationsHelper.pending_operations_type() == "transactions"
        assert Chain.PendingOperationsHelper.actual_entity() == PendingTransactionOperation
        assert Repo.get!(PendingTransactionOperation, transaction.hash).transaction_hash == transaction.hash
        assert Repo.aggregate(Chain.PendingBlockOperation, :count) == 0
        assert [%{hash: hash, block_number: number, index: 0}] = InternalTransaction.init([], &[&1 | &2], arguments)
        assert hash == transaction.hash
        assert number == block.number
      end
    end
  end

  def assert_trace_recovery(variant) do
    capture = trace_capture()

    exchange =
      capture["exchanges"]
      |> Enum.find(&(&1["request"]["method"] == "debug_traceTransaction"))
      |> require_record!("recorded debug_traceTransaction exchange")

    [hash, options] = exchange["request"]["params"]

    raw_block =
      capture["exchanges"]
      |> Enum.find(fn item ->
        item["request"]["method"] == "eth_getBlockByNumber" and
          Enum.any?(item["response"]["result"]["transactions"], &is_map/1)
      end)
      |> require_record!("recorded eth_getBlockByNumber exchange containing the traced transaction")
      |> get_in(["response", "result"])

    [raw_transaction] = raw_block["transactions"]
    node = start_supervised!({Agent, fn -> %{mode: :failure, calls: 0} end})

    arguments =
      serve(fn requests ->
        Enum.map(requests, fn %{
                                "jsonrpc" => "2.0",
                                "id" => id,
                                "method" => "debug_traceTransaction",
                                "params" => [^hash, ^options]
                              } ->
          mode = Agent.get_and_update(node, &{&1.mode, %{&1 | calls: &1.calls + 1}})

          case mode do
            :failure -> %{jsonrpc: "2.0", id: id, error: %{code: -32_000, message: "execution timeout"}}
            :success -> Map.put(exchange["response"], "id", id)
          end
        end)
      end)
      |> Keyword.put(:variant, variant)

    geth = Application.get_env(:ethereum_jsonrpc, EthereumJSONRPC.Geth)

    put_env(
      :ethereum_jsonrpc,
      EthereumJSONRPC.Geth,
      Keyword.merge(geth, block_traceable?: false, tracer: "call_tracer", debug_trace_timeout: "5s")
    )

    put_env(:explorer, :json_rpc_named_arguments, arguments)
    assert Chain.PendingOperationsHelper.pending_operations_type() == "transactions"

    put_env(
      :indexer,
      InternalTransaction,
      Keyword.put(Application.get_env(:indexer, InternalTransaction), :lookback_blocks, nil)
    )

    start_balance_peer(arguments)

    block = insert(:block, hash: raw_block["hash"], number: EthereumJSONRPC.quantity_to_integer(raw_block["number"]))
    from_address = insert(:address, hash: raw_transaction["from"])
    to_address = insert(:address, hash: raw_transaction["to"])

    transaction =
      insert(:transaction,
        hash: hash,
        from_address: from_address,
        to_address: to_address,
        input: raw_transaction["input"]
      )
      |> with_block(block,
        index: 0,
        status: :ok,
        gas_used: EthereumJSONRPC.quantity_to_integer(exchange["response"]["result"]["gasUsed"])
      )

    insert(:pending_transaction_operation, transaction_hash: transaction.hash)
    put_env(:indexer, InternalTransaction.Supervisor, disabled?: false)

    # Drive each failed attempt explicitly so stopping the fetcher cannot abort
    # an HTTP request that the loopback peer is still completing.
    InternalTransaction.Supervisor.Case.start_supervised!(
      json_rpc_named_arguments: arguments,
      flush_interval: :infinity
    )

    wait_until(5_000, fn -> Agent.get(node, & &1.calls) > 0 end)

    wait_until(5_000, fn ->
      :sys.get_state(InternalTransaction).init_task == :complete and
        BufferedTask.debug_count(InternalTransaction) == %{buffer: 1, tasks: 0}
    end)

    assert Agent.get(node, & &1.calls) == 1
    send(InternalTransaction, :flush)

    wait_until(5_000, fn ->
      Agent.get(node, & &1.calls) == 2 and
        BufferedTask.debug_count(InternalTransaction) == %{buffer: 1, tasks: 0}
    end)

    assert :ok = stop_supervised(InternalTransaction.Supervisor)

    Repo.get(PendingTransactionOperation, transaction.hash)
    |> require_record!("pending transaction operation #{hash} after the failed fetcher stops")

    assert Repo.aggregate(Chain.InternalTransaction, :count) == 0

    Agent.update(node, &%{&1 | mode: :success, calls: 0})
    recovered = InternalTransaction.init([], &[&1 | &2], arguments)

    recovered
    |> Enum.find(&(&1.hash == transaction.hash))
    |> require_record!("pending trace #{hash} recovered from the durable transaction queue")

    assert [%{hash: recovered_hash, block_number: recovered_number, index: 0}] = recovered
    assert recovered_hash == transaction.hash
    assert recovered_number == block.number

    params = [%{hash_data: hash, block_number: block.number, transaction_index: 0}]
    assert {:ok, [producer_trace]} = EthereumJSONRPC.fetch_internal_transactions(params, arguments)
    assert producer_trace.trace_address == []
    assert EthereumJSONRPC.quantity_to_integer(exchange["response"]["result"]["value"]) == 0
    assert producer_trace.value == nil

    InternalTransaction.Supervisor.Case.start_supervised!(json_rpc_named_arguments: arguments)
    wait_until(5_000, fn -> is_nil(Repo.get(PendingTransactionOperation, transaction.hash)) end)
    wait_until(5_000, fn -> BufferedTask.debug_count(InternalTransaction) == %{buffer: 0, tasks: 0} end)
    wait_until(5_000, fn -> :sys.get_state(InternalTransaction).init_task == :complete end)
    assert :ok = stop_supervised(InternalTransaction.Supervisor)
    assert Agent.get(node, & &1.calls) > 0

    trace =
      Repo.one(Chain.InternalTransaction)
      |> require_record!("persisted internal transaction #{hash} after restart")

    assert trace.transaction_hash == transaction.hash
    assert trace.block_number == block.number
    assert trace.from_address_hash == from_address.hash
    assert trace.to_address_hash == to_address.hash
    assert trace.type == :call
    assert trace.call_type_enum == :call
    assert trace.index == 0
    # The import runner intentionally clears this transport-only path when
    # assigning the persisted block/transaction indices.
    assert trace.trace_address == nil
    assert to_string(trace.input) == raw_transaction["input"]

    assert Decimal.equal?(
             trace.gas_used,
             EthereumJSONRPC.quantity_to_integer(exchange["response"]["result"]["gasUsed"])
           )

    assert trace.value == producer_trace.value
    assert trace.value == nil

    calls = Agent.get(node, & &1.calls)
    InternalTransaction.Supervisor.Case.start_supervised!(json_rpc_named_arguments: arguments)
    wait_until(5_000, fn -> :sys.get_state(InternalTransaction).init_task == :complete end)
    assert InternalTransaction.init([], &[&1 | &2], arguments) == []
    assert Agent.get(node, & &1.calls) == calls
    assert Repo.aggregate(Chain.InternalTransaction, :count) == 1
    assert :ok = stop_supervised(InternalTransaction.Supervisor)
  end

  def assert_withdrawal_recovery do
    [raw_a, raw_b] = fixture("withdrawals")
    numbers = Enum.map([raw_a, raw_b], &EthereumJSONRPC.quantity_to_integer(&1["number"]))

    Enum.each(
      [raw_a, raw_b],
      &insert(:block, hash: &1["hash"], number: EthereumJSONRPC.quantity_to_integer(&1["number"]))
    )

    node = start_supervised!({Agent, fn -> %{mode: :failure, quantities: []} end})

    arguments =
      serve(fn [%{"id" => id, "jsonrpc" => "2.0", "method" => "eth_getBlockByNumber", "params" => [quantity, true]}] ->
        mode = Agent.get_and_update(node, &{&1.mode, %{&1 | quantities: &1.quantities ++ [quantity]}})
        block = Enum.find([raw_a, raw_b], &(&1["number"] == String.downcase(quantity)))
        assert block

        if mode == :failure and block == raw_b do
          {:http_error, 503}
        else
          [%{jsonrpc: "2.0", id: id, result: block}]
        end
      end)

    put_env(:indexer, Withdrawal, first_block: Integer.to_string(hd(numbers)))
    opts = [json_rpc_named_arguments: arguments, interval: 60_000, max_batch_size: 1, max_concurrency: 1]
    pid = start_supervised!({Withdrawal, [opts]})
    assert Enum.sort(:sys.get_state(pid).blocks_to_fetch) == numbers
    send(pid, :fetch_withdrawals)
    assert :sys.get_state(pid).blocks_to_fetch == [List.last(numbers)]
    assert_withdrawal(raw_a)
    assert Repo.get(Chain.Withdrawal, 2) == nil
    assert :ok = stop_supervised(Withdrawal)

    Agent.update(node, &%{&1 | mode: :success, quantities: []})
    pid = start_supervised!({Withdrawal, [opts]})
    assert :sys.get_state(pid).blocks_to_fetch == [List.last(numbers)]
    send(pid, :fetch_withdrawals)
    assert :sys.get_state(pid).blocks_to_fetch == []
    assert_withdrawal(raw_a)
    assert_withdrawal(raw_b)
    assert Repo.aggregate(Chain.Withdrawal, :count) == 2
    assert Agent.get(node, & &1.quantities) == [EthereumJSONRPC.integer_to_quantity(List.last(numbers))]
    monitor = Process.monitor(pid)
    send(pid, :fetch_withdrawals)
    assert_receive {:DOWN, ^monitor, :process, ^pid, :normal}, 5_000
    {:ok, supervisor} = ExUnit.fetch_test_supervisor()
    assert :ok = Supervisor.delete_child(supervisor, Withdrawal)

    Agent.update(node, &%{&1 | quantities: []})
    pid = start_supervised!({Withdrawal, [opts]})
    assert :sys.get_state(pid).blocks_to_fetch == []
    monitor = Process.monitor(pid)
    send(pid, :fetch_withdrawals)
    assert_receive {:DOWN, ^monitor, :process, ^pid, :normal}, 5_000
    assert Agent.get(node, & &1.quantities) == []
    assert_withdrawal(raw_a)
    assert_withdrawal(raw_b)
  end

  def assert_withdrawal(block) do
    [raw] = block["withdrawals"]
    row = Repo.get!(Chain.Withdrawal, EthereumJSONRPC.quantity_to_integer(raw["index"]))
    assert to_string(row.block_hash) == block["hash"]
    assert Repo.get!(Block, row.block_hash).number == EthereumJSONRPC.quantity_to_integer(block["number"])
    assert row.validator_index == EthereumJSONRPC.quantity_to_integer(raw["validatorIndex"])
    assert to_string(row.address_hash) == raw["address"]
    assert Decimal.equal?(Chain.Wei.to(row.amount, :gwei), EthereumJSONRPC.quantity_to_integer(raw["amount"]))
  end

  def start_balance_peer(arguments) do
    put_env(:indexer, Catchup.Supervisor, disabled?: false)

    Catchup.Supervisor.Case.start_supervised!(
      json_rpc_named_arguments: arguments,
      flush_interval: :infinity,
      poll: false
    )

    wait_until(5_000, fn -> :sys.get_state(Catchup).init_task == :complete end)
  end

  defp require_record!(record, name) do
    assert record != nil, "missing record: #{name}"
    record
  end
end
