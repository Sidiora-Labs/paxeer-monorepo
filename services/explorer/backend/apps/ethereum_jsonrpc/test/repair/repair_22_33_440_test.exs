defmodule EthereumJSONRPC.Repair2233440Test.RecordedNode do
  @moduledoc false
  @behaviour :cowboy_handler

  @impl :cowboy_handler
  def init(request, state) do
    {:ok, body, request} = :cowboy_req.read_body(request)
    requests = Jason.decode!(body)

    responses =
      Enum.map(requests, fn %{"jsonrpc" => "2.0", "id" => id} = rpc ->
        send(state.owner, {:node_request, rpc})
        %{"jsonrpc" => "2.0", "id" => id, "result" => result(rpc, state.fixture)}
      end)

    request =
      :cowboy_req.reply(200, %{"content-type" => "application/json"}, Jason.encode!(responses), request)

    {:ok, request, state}
  end

  defp result(%{"method" => "debug_traceTransaction", "params" => [hash, options]}, fixture) do
    ^hash = fixture["tx"]["hash"]

    case options do
      %{"tracer" => "callTracer"} -> fixture["callTracer"]
      %{"enableMemory" => true, "disableStack" => false, "disableStorage" => true} -> fixture["result"]
    end
  end

  defp result(%{"method" => "eth_getTransactionReceipt", "params" => [hash]}, fixture) do
    ^hash = fixture["tx"]["hash"]
    fixture["receipt"]
  end

  defp result(%{"method" => "eth_getTransactionByHash", "params" => [hash]}, fixture) do
    ^hash = fixture["tx"]["hash"]
    fixture["tx"]
  end

  defp result(%{"method" => "eth_getCode", "params" => [address, block]}, fixture) do
    block_number = EthereumJSONRPC.quantity_to_integer(block)
    ^block_number = EthereumJSONRPC.quantity_to_integer(fixture["receipt"]["blockNumber"])
    Map.fetch!(fixture["codes"], address)
  end
end

defmodule EthereumJSONRPC.Repair2233440Test do
  use ExUnit.Case, async: false

  alias EthereumJSONRPC.Geth
  alias EthereumJSONRPC.Geth.Tracer
  alias EthereumJSONRPC.Repair2233440Test.RecordedNode

  @fixtures Path.expand("../../lib/ethereum_jsonrpc/geth/fixtures/repair_22_33_440", __DIR__)

  setup do
    previous = Application.get_env(:ethereum_jsonrpc, Geth)

    on_exit(fn ->
      if is_nil(previous) do
        Application.delete_env(:ethereum_jsonrpc, Geth)
      else
        Application.put_env(:ethereum_jsonrpc, Geth, previous)
      end
    end)

    :ok
  end

  test "native-create-stack: minimal CREATE and CREATE2 operands produce constructor code through HTTP import" do
    for {name, opcode, arity} <- [{"minimal_create", "CREATE", 3}, {"minimal_create2", "CREATE2", 4}] do
      fixture = fixture(name)
      log = Enum.find(fixture["result"]["structLogs"], &(&1["op"] == opcode))
      assert length(log["stack"]) == arity

      [creation] = prove_creation_path(fixture)
      assert creation["type"] == String.downcase(opcode)
      assert creation["init"] == "0x63deadbeef6000526003601df3"
      assert creation["createdContractCode"] == "0xadbeef"
      assert creation["value"] == "0x7"
      assert EthereumJSONRPC.quantity_to_integer(creation["gasUsed"]) == 618
    end
  end

  test "native-create-stack: unrelated stack entries never become creation output offsets" do
    fixture = fixture("extra_stack_create")
    log = Enum.find(fixture["result"]["structLogs"], &(&1["op"] == "CREATE"))
    assert ["0x55", "0x66", "0xd", "0x5", "0x7"] == log["stack"]
    [creation] = prove_creation_path(fixture)
    assert creation["createdContractCode"] == "0xadbeef"
  end

  test "native-create-stack: nested constructors retain their own return memory and creator addresses" do
    [outer, inner] = "nested_create" |> fixture() |> prove_creation_path()
    assert outer["traceAddress"] == [0]
    assert inner["traceAddress"] == [0, 0]
    assert outer["createdContractCode"] == "0x1122"
    assert inner["createdContractCode"] == "0xadbeef"
    assert inner["from"] == outer["createdContractAddressHash"]
    assert inner["type"] == "create2"
    assert inner["value"] == "0x0"
  end

  test "native-create-stack: empty constructors complete without a depth change or memory field" do
    for name <- ["empty_create", "empty_create2"] do
      fixture = fixture(name)
      assert Enum.all?(fixture["result"]["structLogs"], &(&1["depth"] == 1))
      refute Enum.any?(fixture["result"]["structLogs"], &Map.has_key?(&1, "memory"))
      [creation] = prove_creation_path(fixture)
      assert creation["init"] == "0x"
      assert creation["createdContractCode"] == "0x"
      assert creation["gasUsed"] == "0x0"
      refute Map.has_key?(creation, "error")
    end
  end

  test "native-create-stack: uninitialized creation memory is zero-filled before constructor execution" do
    fixture = fixture("zero_initialized_create")
    log = Enum.find(fixture["result"]["structLogs"], &(&1["op"] == "CREATE"))
    refute Map.has_key?(log, "memory")
    [creation] = prove_creation_path(fixture)
    assert creation["init"] == "0x00"
    assert creation["createdContractCode"] == "0x"
  end

  test "native-create-stack: reverted exceptional and rejected-code creations publish no deployed code" do
    for name <- ["reverted_create", "invalid_create2", "rejected_code_create"] do
      fixture = fixture(name)
      [creation] = prove_creation_path(fixture)
      assert is_binary(creation["error"])
      assert creation["createdContractAddressHash"] == nil
      assert creation["createdContractCode"] == "0x"

      if name == "reverted_create" do
        assert creation["error"] == "execution reverted"
      end
    end
  end

  defp fixture(name) do
    @fixtures |> Path.join(name <> ".json") |> File.read!() |> Jason.decode!()
  end

  defp prove_creation_path(fixture) do
    [_top | creations] = Tracer.replay(fixture["result"], fixture["receipt"], fixture["tx"])
    expected = flatten_creations(fixture["callTracer"])
    assert length(creations) == length(expected)

    for {creation, recorded} <- Enum.zip(creations, expected) do
      assert creation["type"] == String.downcase(recorded["type"])
      assert creation["init"] == recorded["input"]
      assert creation["from"] == recorded["from"]
      assert creation["value"] == recorded["value"]
      assert creation["gas"] == recorded["gas"]
      assert creation["gasUsed"] == recorded["gasUsed"]
      refute Map.has_key?(creation, "outputOffset")
      refute Map.has_key?(creation, "outputLength")
      refute Map.has_key?(creation, "parentGas")

      if Map.has_key?(recorded, "error") do
        assert is_binary(creation["error"])
        assert creation["createdContractAddressHash"] == nil
        assert creation["createdContractCode"] == "0x"
      else
        assert creation["createdContractAddressHash"] == recorded["to"]
        assert creation["createdContractCode"] == Map.get(recorded, "output", "0x")
        refute Map.has_key?(creation, "error")
      end
    end

    listener = make_ref()
    state = %{owner: self(), fixture: fixture}
    dispatch = :cowboy_router.compile([{:_, [{"/", RecordedNode, state}]}])
    {:ok, _} = :cowboy.start_clear(listener, [ip: {127, 0, 0, 1}, port: 0], %{env: %{dispatch: dispatch}})
    on_exit(fn -> :cowboy.stop_listener(listener) end)
    url = "http://127.0.0.1:#{:ranch.get_port(listener)}"

    arguments = [
      transport: EthereumJSONRPC.HTTP,
      transport_options: [
        http: EthereumJSONRPC.HTTP.HTTPoison,
        urls: [url],
        http_options: [timeout: 5_000, recv_timeout: 5_000]
      ]
    ]

    transaction = %{
      block_number: EthereumJSONRPC.quantity_to_integer(fixture["receipt"]["blockNumber"]),
      transaction_index: EthereumJSONRPC.quantity_to_integer(fixture["receipt"]["transactionIndex"]),
      hash_data: fixture["tx"]["hash"]
    }

    Application.put_env(:ethereum_jsonrpc, Geth, tracer: "opcode", debug_trace_timeout: "5s")
    assert {:ok, imported} = Geth.fetch_internal_transactions([transaction], arguments)
    imported = Enum.sort_by(imported, & &1.index)
    assert length(imported) == length(creations) + 1

    for {creation, params} <- Enum.zip(creations, tl(imported)) do
      assert params.type == creation["type"]
      assert params.init == creation["init"]
      assert params.trace_address == creation["traceAddress"]
      assert params.from_address_hash == creation["from"]
      assert params.transaction_hash == transaction.hash_data
      assert params.gas == EthereumJSONRPC.quantity_to_integer(creation["gas"])
      assert params.gas_used == EthereumJSONRPC.quantity_to_integer(creation["gasUsed"])

      if Map.has_key?(creation, "error") do
        assert params.error == creation["error"]
        refute Map.has_key?(params, :created_contract_address_hash)
        refute Map.has_key?(params, :created_contract_code)
      else
        assert params.created_contract_address_hash == creation["createdContractAddressHash"]
        assert params.created_contract_code == creation["createdContractCode"]
      end
    end

    assert_receive {:node_request, %{"method" => "debug_traceTransaction", "params" => [_, options]}}
    assert options["enableMemory"] == true
    refute Map.has_key?(options, "tracer")
    assert_receive {:node_request, %{"method" => "eth_getTransactionReceipt"}}
    assert_receive {:node_request, %{"method" => "eth_getTransactionByHash"}}

    for {address, _code} <- fixture["codes"] do
      assert_receive {:node_request, %{"method" => "eth_getCode", "params" => [^address, _]}}
    end

    Application.put_env(:ethereum_jsonrpc, Geth, tracer: "call_tracer", debug_trace_timeout: "5s")
    assert {:ok, [_top | call_tracer_creations]} = Geth.fetch_internal_transactions([transaction], arguments)
    assert length(call_tracer_creations) == length(expected)

    for {params, recorded} <- Enum.zip(call_tracer_creations, expected) do
      assert params.type == String.downcase(recorded["type"])
      assert params.init == recorded["input"]
      assert params.gas == EthereumJSONRPC.quantity_to_integer(recorded["gas"])
      assert params.gas_used == EthereumJSONRPC.quantity_to_integer(recorded["gasUsed"])

      if Map.has_key?(recorded, "error") do
        assert params.error == recorded["error"]
      else
        assert params.created_contract_address_hash == recorded["to"]
        assert params.created_contract_code == Map.get(recorded, "output", "0x")
      end
    end

    assert_receive {:node_request,
                    %{"method" => "debug_traceTransaction", "params" => [_, %{"tracer" => "callTracer"}]}}

    assert :ok = :cowboy.stop_listener(listener)
    creations
  end

  defp flatten_creations(call) do
    Enum.flat_map(Map.get(call, "calls", []), fn child -> [child | flatten_creations(child)] end)
  end
end
