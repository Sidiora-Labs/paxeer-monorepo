defmodule EthereumJSONRPC.Repair2233441Test.Transcript do
  @moduledoc false
  @behaviour :cowboy_handler

  @impl true
  def init(request, %{owner: owner, recordings: recordings} = state) do
    "POST" = :cowboy_req.method(request)
    {:ok, body, request} = :cowboy_req.read_body(request)
    decoded = Jason.decode!(body)
    send(owner, {:recorded_request, decoded})

    response =
      if is_list(decoded) do
        Enum.map(decoded, &Map.fetch!(recordings, &1))
      else
        Map.fetch!(recordings, decoded)
      end

    request =
      :cowboy_req.reply(200, %{"content-type" => "application/json"}, Jason.encode!(response), request)

    {:ok, request, state}
  end
end

defmodule EthereumJSONRPC.Repair2233441Test do
  use ExUnit.Case, async: false

  alias EthereumJSONRPC.{Blocks, HTTP}
  alias EthereumJSONRPC.Repair2233441Test.Transcript

  @fixture_dir Path.expand("../support/fixture/paxeer_x", __DIR__)
  @node_revision "2da69275051f6feb074a8b552977d37197735d86"
  @height "0x289221e"
  @request_height "0x289221E"
  @block_hash "0x548a3e34789d5437b05ddf5c74cdda661852a887973ce50f1b74651ca59b90f8"

  setup do
    fixture = fixture("tx_count_parity.json")

    recordings =
      Map.new(~w(block count shell_logs client_version chain_id), fn name ->
        {Map.fetch!(fixture, name <> "_request"), Map.fetch!(fixture, name <> "_response")}
      end)

    listener = make_ref()
    dispatch = :cowboy_router.compile([{:_, [{"/", Transcript, %{owner: self(), recordings: recordings}}]}])

    {:ok, _} = :cowboy.start_clear(listener, [ip: {127, 0, 0, 1}, port: 0], %{env: %{dispatch: dispatch}})
    on_exit(fn -> :cowboy.stop_listener(listener) end)
    url = "http://127.0.0.1:#{:ranch.get_port(listener)}"

    arguments = [
      transport: HTTP,
      transport_options: [
        url: url,
        urls: [url],
        fallback_urls: [],
        http: HTTP.HTTPoison,
        http_options: [timeout: 5_000, recv_timeout: 5_000]
      ]
    ]

    {:ok, fixture: fixture, arguments: arguments}
  end

  test "mixed-count-fixture-shape: paired eth captures traverse HTTP and the block importer", %{
    fixture: fixture,
    arguments: arguments
  } do
    capture = fixture["capture"]
    assert capture["kind"] == "recorded"
    assert capture["namespace"] == "eth"
    assert capture["height"] == @height
    assert capture["node_revision_before"] == @node_revision
    assert capture["node_revision_after"] == @node_revision
    assert capture["application_version"]["git_commit"] == @node_revision
    assert capture["application_version"]["version"] == "v6.12.0"

    block = fixture["block_response"]["result"]
    number = EthereumJSONRPC.quantity_to_integer(@height)

    assert fixture["block_request"] == %{
             "jsonrpc" => "2.0",
             "id" => fixture["block_response"]["id"],
             "method" => "eth_getBlockByNumber",
             "params" => [@request_height, true]
           }

    assert fixture["count_request"] == %{
             "jsonrpc" => "2.0",
             "id" => fixture["count_response"]["id"],
             "method" => "eth_getBlockTransactionCountByNumber",
             "params" => [@request_height]
           }

    assert {:ok, %Blocks{blocks_params: [imported], transactions_params: transactions, errors: []}} =
             EthereumJSONRPC.fetch_blocks_by_numbers([number], arguments)

    expected_request = [fixture["block_request"]]
    assert_receive {:recorded_request, ^expected_request}

    assert {:ok, quantity} = rpc(fixture["count_request"], arguments)
    expected_request = fixture["count_request"]
    assert_receive {:recorded_request, ^expected_request}

    assert quantity == fixture["count_response"]["result"]
    assert quantity == "0x0"
    count = EthereumJSONRPC.quantity_to_integer(quantity)
    assert imported.number == number
    assert imported.hash == @block_hash
    assert block["number"] == @height
    assert block["hash"] == @block_hash
    assert length(block["transactions"]) == count
    assert length(transactions) == count
    assert transactions == []

    assert {:ok, "0x7d"} = rpc(fixture["chain_id_request"], arguments)
    assert {:ok, "Geth/linux-amd64/go1.25.6"} = rpc(fixture["client_version_request"], arguments)
  end

  test "mixed-count-fixture-shape: Cosmos shell logs do not become eth block transactions", %{
    fixture: fixture,
    arguments: arguments
  } do
    assert {:ok, logs} = rpc(fixture["shell_logs_request"], arguments)
    assert logs == fixture["shell_logs_response"]["result"]
    assert length(logs) == 15
    expected_request = fixture["shell_logs_request"]
    assert_receive {:recorded_request, ^expected_request}

    number = EthereumJSONRPC.quantity_to_integer(@height)

    assert {:ok, %Blocks{blocks_params: [block], transactions_params: [], errors: []}} =
             EthereumJSONRPC.fetch_blocks_by_numbers([number], arguments)

    assert block.hash == @block_hash
    assert fixture["count_response"]["result"] == "0x0"

    for log <- logs do
      assert log["blockNumber"] == @height
      assert log["blockHash"] == block.hash
      assert log["address"] == "0x0000000000000000000000000000000000001015"
      assert is_binary(log["transactionHash"])
      refute log["transactionHash"] in fixture["block_response"]["result"]["transactions"]
    end
  end

  test "synthetic Cosmos compatibility remains explicitly separate from the node capture" do
    synthetic = fixture("synthetic_cosmos_tx_count_parity.json")
    %{"block_response" => %{"id" => id, "result" => block}} = synthetic
    number = EthereumJSONRPC.quantity_to_integer(block["number"])

    assert %Blocks{blocks_params: [imported], transactions_params: [cosmos, evm], errors: []} =
             Blocks.from_responses([%{id: id, result: block}], %{id => %{number: number}})

    assert synthetic["count_response"]["result"] == "0x2"
    assert imported.number == number
    assert imported.hash == block["hash"]
    assert [cosmos.index, evm.index] == [0, 1]
    assert [cosmos.hash, evm.hash] == Enum.map(block["transactions"], & &1["hash"])
    [cosmos_wire, evm_wire] = block["transactions"]
    assert is_nil(cosmos_wire["value"])
    assert is_nil(cosmos_wire["gasPrice"])
    assert cosmos.value == 0
    assert cosmos.gas_price == 0
    assert cosmos.input == cosmos_wire["input"]
    assert evm.value == EthereumJSONRPC.quantity_to_integer(evm_wire["value"])
    assert evm.gas_price == EthereumJSONRPC.quantity_to_integer(evm_wire["gasPrice"])

    for transaction <- [cosmos, evm] do
      assert transaction.block_number == number
      assert transaction.block_hash == imported.hash
    end
  end

  defp fixture(name), do: @fixture_dir |> Path.join(name) |> File.read!() |> Jason.decode!()

  defp rpc(request, arguments) do
    %{id: request["id"], method: request["method"], params: request["params"]}
    |> EthereumJSONRPC.request()
    |> EthereumJSONRPC.json_rpc(arguments)
  end
end
