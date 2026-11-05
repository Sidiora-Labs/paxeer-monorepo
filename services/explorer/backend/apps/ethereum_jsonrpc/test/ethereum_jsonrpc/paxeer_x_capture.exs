defmodule EthereumJSONRPC.PaxeerXCapture.HTTPReplay do
  @moduledoc false
  @behaviour :cowboy_handler

  @impl :cowboy_handler
  def init(request, state) do
    "POST" = :cowboy_req.method(request)
    {:ok, body, request} = :cowboy_req.read_body(request)
    decoded = Jason.decode!(body)
    send(state.owner, {:capture_http_request, decoded})

    response =
      if is_list(decoded) do
        Enum.map(decoded, &Map.fetch!(state.recordings, &1))
      else
        Map.fetch!(state.recordings, decoded)
      end

    request = :cowboy_req.reply(200, %{"content-type" => "application/json"}, Jason.encode!(response), request)
    {:ok, request, state}
  end
end

defmodule EthereumJSONRPC.PaxeerXCapture.WebSocketReplay do
  @moduledoc false
  @behaviour :cowboy_websocket

  @impl :cowboy_websocket
  def init(request, state) do
    case :cowboy_req.header("upgrade", request) do
      "websocket" -> {:cowboy_websocket, request, state}
      _ -> EthereumJSONRPC.PaxeerXCapture.HTTPReplay.init(request, state)
    end
  end

  @impl :cowboy_websocket
  def websocket_handle({:text, text}, state) do
    request = Jason.decode!(text)
    # Request IDs belong to the client; the captured subscription ID and payload stay intact.
    true = Map.delete(request, "id") == Map.delete(state.capture["subscribe_request"], "id")
    send(state.owner, {:capture_ws_request, self(), request})
    response = Map.replace!(state.capture["subscribe_response"], "id", Map.fetch!(request, "id"))
    {:reply, {:text, Jason.encode!(response)}, state}
  end

  @impl :cowboy_websocket
  def websocket_info(:publish_capture, state) do
    {:reply, {:text, Jason.encode!(state.capture["notification"])}, state}
  end
end

defmodule EthereumJSONRPC.PaxeerXCapture do
  @moduledoc false

  import ExUnit.Assertions
  import ExUnit.Callbacks, only: [on_exit: 1, start_supervised!: 1]

  alias EthereumJSONRPC.{Blocks, Geth, HTTP, PaxeerX, WebSocket}
  alias EthereumJSONRPC.PaxeerXCapture.WebSocketReplay
  alias EthereumJSONRPC.WebSocket.{Supervisor, WebSocketClient}
  alias EthereumJSONRPC.WebSocket.WebSocketClient.Options

  @capture_names ~w(new_heads new_heads_previous pax_block pax_count synced syncing trace_block trace_historical_refusal)
  for name <- @capture_names do
    @external_resource Path.join([__DIR__, "captures", name <> ".json"])
  end

  @captures Map.new(@capture_names, fn name ->
              path = Path.join([__DIR__, "captures", name <> ".json"])
              {name, path |> File.read!() |> Jason.decode!()}
            end)
  @legacy_fixtures Path.expand("../support/fixture/paxeer_x", __DIR__)
  @producer_revision "2da69275051f6feb074a8b552977d37197735d86"
  @client __MODULE__.Client

  def capture(name), do: Map.fetch!(@captures, name)

  def assert_recorded_head_fields do
    {head, body, arguments} = delivered_head()

    for field <- ~w(number hash logsBloom gasUsed transactionsRoot receiptsRoot parentHash) do
      assert Map.has_key?(head, field), field
    end

    assert {:ok, %Blocks{blocks_params: [imported], errors: []}} =
             PaxeerX.fetch_block_by_new_head(head, arguments)

    assert_http([capture("new_heads")["block_by_hash"]["request"]])
    assert_imported_header(imported, body)
    assert imported.hash == head["hash"]
    assert imported.number == EthereumJSONRPC.quantity_to_integer(head["number"])
    assert imported.receipts_root == head["receiptsRoot"]
    assert imported.parent_hash == head["parentHash"]
    refute PaxeerX.receipts_root_verifiable?()
  end

  @formatter_capture @captures["new_heads"]["formatter_capture"]

  def assert_recorded_head_parity do
    recording = @formatter_capture
    source = recording["source"]
    assert source["kind"] == "recorded"
    assert source["node_revision"] == "aa5e0820b3652890f3812a220f4b91f4f8a45387"
    assert source["formatter_revision"] == "4af5959eb28ba0c7269a509c5dfb2ea13b022b1c"
    assert source["environment"] == "in-process committed node fixture; production HTTP and WebSocket servers"
    assert source["consensus_transactions"] == 1

    {head, body, arguments} = delivered_head(recording)
    assert body["number"] == source["block_number"]
    assert body["hash"] == source["block_hash"]
    assert body["transactions"] == []
    refute head["transactionsRoot"] == source["consensus_data_hash"]
    assert head == Map.drop(body, ~w(transactions uncles totalDifficulty))

    # Retain the public producer's historical mismatch without correcting either payload.
    public = capture("new_heads")
    public_head = public["notification"]["params"]["result"]
    public_body = public["block_by_hash"]["response"]["result"]
    assert public_head["hash"] == public_body["hash"]
    assert public_head["transactionsRoot"] == "0xf340f955cbacca0cd10f65b4b62ebc426932c74f93fd9f93048a67287dcf3492"
    assert public_body["transactionsRoot"] == "0x56e81f171bcc55a6ff8345e692c0f86e5b48e01b996cadc001622fb5e363b421"
    refute public_head["transactionsRoot"] == public_body["transactionsRoot"]

    previous = capture("new_heads_previous")
    assert_source(previous["source"])
    previous_head = previous["notification"]["params"]["result"]
    previous_body = previous["block_by_hash"]["response"]["result"]
    assert previous_head["hash"] == previous_body["hash"]
    refute previous_head["transactionsRoot"] == previous_body["transactionsRoot"]

    assert {:ok, %Blocks{blocks_params: [imported], errors: []}} =
             PaxeerX.fetch_block_by_new_head(head, arguments)

    assert_http([recording["block_by_hash"]["request"]])
    assert_imported_header(imported, body)

    for field <- ~w(number hash logsBloom gasUsed transactionsRoot receiptsRoot parentHash) do
      assert Map.fetch!(head, field) == Map.fetch!(body, field), field
    end

    assert imported.transactions_root == head["transactionsRoot"]
    assert imported.logs_bloom == head["logsBloom"]
    assert imported.gas_used == EthereumJSONRPC.quantity_to_integer(head["gasUsed"])
  end

  def assert_recorded_head_mutations do
    {head, body, arguments} = delivered_head()
    recording = capture("new_heads")

    for field <- ~w(parentHash receiptsRoot) do
      "0x" <> hex = Map.fetch!(head, field)
      <<first, rest::binary>> = Base.decode16!(hex, case: :mixed)
      changed = "0x" <> Base.encode16(<<Bitwise.bxor(first, 1), rest::binary>>, case: :lower)
      changed_head = Map.replace!(head, field, changed)

      assert {:ok, %Blocks{blocks_params: [imported], errors: []}} =
               PaxeerX.fetch_block_by_new_head(changed_head, arguments)

      assert_http([recording["block_by_hash"]["request"]])
      assert_http([recording["block_by_number"]["request"]])
      assert_imported_header(imported, body)
      refute PaxeerX.new_head_matches_block?(changed_head, imported)
    end

    assert {:error, {:invalid_new_head, :number}} =
             PaxeerX.fetch_block_by_new_head(Map.delete(head, "number"), arguments)

    refute_receive {:capture_http_request, _}
  end

  def assert_recorded_syncing(name) when name in ["synced", "syncing"] do
    recording = capture(name)

    case name do
      "synced" ->
        assert_source(recording["source"])

      "syncing" ->
        source = recording["source"]
        assert source["kind"] == "recorded"
        assert source["node_revision_observed_after"] == @producer_revision
        assert source["application_version"]["git_commit"] == @producer_revision
        assert source["environment"] == "public node; disposable-node provenance not available"
        refute Map.has_key?(source, "node_revision_before")
    end

    arguments = endpoint([recording])
    result = recording["response"]["result"]

    expected =
      case name do
        "synced" ->
          assert result == false
          {:ok, :synced}

        "syncing" ->
          assert Map.keys(result) |> Enum.sort() == ~w(currentBlock highestBlock startingBlock)
          starting = EthereumJSONRPC.quantity_to_integer(result["startingBlock"])
          current = EthereumJSONRPC.quantity_to_integer(result["currentBlock"])
          highest = EthereumJSONRPC.quantity_to_integer(result["highestBlock"])
          assert starting <= current
          assert current < highest
          {:ok, {:syncing, current, highest}}
      end

    assert PaxeerX.fetch_syncing_status(arguments) == expected
    assert_http(recording["request"])
  end

  def assert_syncing_transport_error do
    # Close a real listening socket: no transport or adapter is replaced.
    {:ok, socket} = :gen_tcp.listen(0, [:binary, ip: {127, 0, 0, 1}, active: false])
    {:ok, {_address, port}} = :inet.sockname(socket)
    :ok = :gen_tcp.close(socket)
    arguments = http_arguments("http://127.0.0.1:#{port}")
    request = EthereumJSONRPC.request(%{id: 0, method: "eth_syncing", params: []})

    assert {:error, :econnrefused} = HTTP.json_rpc(request, arguments[:transport_options])
    assert {:error, :econnrefused} = PaxeerX.fetch_syncing_status(arguments)
  end

  def assert_recorded_trace do
    recording = capture("trace_block")
    assert_source(recording["source"])
    configure_tracer()
    arguments = endpoint([recording])
    number = EthereumJSONRPC.quantity_to_integer(recording["request"]["params"] |> hd())

    # This recorded block has no eth-visible transactions; it is not internal-call evidence.
    assert recording["block"]["transactions"] == []
    assert recording["response"]["result"] == []
    assert {:ok, []} = PaxeerX.fetch_block_internal_transactions([number], arguments)
    assert_http([recording["request"]])
  end

  def assert_recorded_trace_refusal do
    recording = capture("trace_historical_refusal")
    assert_source(recording["source"])
    configure_tracer()
    arguments = endpoint([recording])
    number = EthereumJSONRPC.quantity_to_integer(hd(recording["request"]["params"]))
    error = recording["response"]["error"]
    assert error["code"] == -32_000
    assert error["message"] == "block number 24203024 is beyond max lookback of 10000"

    assert {:error, [%{code: -32_000, message: message}]} =
             PaxeerX.fetch_block_internal_transactions([number], arguments)

    assert message == error["message"]
    assert_http([recording["request"]])
  end

  def assert_recorded_counts do
    fixture = @legacy_fixtures |> Path.join("tx_count_parity.json") |> File.read!() |> Jason.decode!()
    assert fixture["capture"]["kind"] == "recorded"
    assert fixture["capture"]["node_revision_before"] == @producer_revision
    assert fixture["capture"]["node_revision_after"] == @producer_revision
    assert fixture["capture"]["namespace"] == "eth"

    entries =
      Enum.map(~w(block count shell_logs), fn name ->
        %{"request" => fixture[name <> "_request"], "response" => fixture[name <> "_response"]}
      end)

    arguments = endpoint(entries)
    wire_block = fixture["block_response"]["result"]
    number = EthereumJSONRPC.quantity_to_integer(wire_block["number"])

    assert {:ok, %Blocks{blocks_params: [block], transactions_params: transactions, errors: []}} =
             EthereumJSONRPC.fetch_blocks_by_numbers([number], arguments)

    assert_http([fixture["block_request"]])
    assert {:ok, quantity} = rpc(fixture["count_request"], arguments)
    assert_http(fixture["count_request"])
    assert quantity == fixture["count_response"]["result"]
    count = EthereumJSONRPC.quantity_to_integer(quantity)
    assert block.hash == wire_block["hash"]
    assert block.number == number
    assert count == length(wire_block["transactions"])
    assert count == length(transactions)
    assert count == 0
    assert transactions == []

    assert {:ok, logs} = rpc(fixture["shell_logs_request"], arguments)
    assert_http(fixture["shell_logs_request"])
    assert length(logs) == 15

    for log <- logs do
      assert log["blockHash"] == block.hash
      assert EthereumJSONRPC.quantity_to_integer(log["blockNumber"]) == number
      refute log["transactionHash"] in Enum.map(transactions, & &1.hash)
    end
  end

  def assert_recorded_pax_refusals do
    entries = Enum.map(~w(pax_block pax_count), &capture/1)
    arguments = endpoint(entries)

    for recording <- entries do
      assert_source(recording["source"])
      expected = recording["response"]["error"]
      assert expected["code"] == -32_601
      assert expected["data"] == "legacy_pax_deprecated"

      assert {:error, %{code: -32_601, data: "legacy_pax_deprecated", message: message}} =
               rpc(recording["request"], arguments)

      assert message == expected["message"]
      assert_http(recording["request"])
    end
  end

  defp delivered_head(recording \\ capture("new_heads")) do
    assert_source(recording["source"])

    if Map.has_key?(recording["source"], "endpoint") do
      assert URI.parse(recording["source"]["endpoint"]).path in [nil, "/"]
    end

    assert recording["chain_id"]["request"] == %{
             "jsonrpc" => "2.0",
             "id" => 0,
             "method" => "eth_chainId",
             "params" => []
           }

    assert recording["chain_id"]["response"]["result"] == "0x7d"
    assert recording["block_by_hash"]["response"]["result"] == recording["block_by_number"]["response"]["result"]
    arguments = endpoint([recording["block_by_hash"], recording["block_by_number"]], recording)
    url = arguments[:transport_options][:url]

    transport = %WebSocket{
      url: String.replace_prefix(url, "http://", "ws://"),
      web_socket: WebSocketClient,
      web_socket_options: %Options{web_socket: @client}
    }

    start_supervised!({Supervisor, transport})
    assert {:ok, subscription} = WebSocket.subscribe("newHeads", [], transport)
    assert_receive {:capture_ws_request, connection, request}, 5_000
    assert Map.delete(request, "id") == Map.delete(recording["subscribe_request"], "id")
    send(connection, :publish_capture)
    assert_receive {^subscription, {:ok, head}}, 5_000
    assert head == recording["notification"]["params"]["result"]
    assert recording["notification"]["params"]["subscription"] == recording["subscribe_response"]["result"]
    {head, recording["block_by_hash"]["response"]["result"], arguments}
  end

  defp endpoint(entries, websocket_capture \\ nil) do
    {:ok, _} = Application.ensure_all_started(:cowboy)
    {:ok, _} = Application.ensure_all_started(:httpoison)
    recordings = Map.new(entries, &{&1["request"], &1["response"]})
    state = %{owner: self(), recordings: recordings, capture: websocket_capture}
    routes = [{"/", WebSocketReplay, state}]
    dispatch = :cowboy_router.compile([{:_, routes}])
    listener = make_ref()
    {:ok, _} = :cowboy.start_clear(listener, [ip: {127, 0, 0, 1}, port: 0], %{env: %{dispatch: dispatch}})
    on_exit(fn -> :cowboy.stop_listener(listener) end)
    http_arguments("http://127.0.0.1:#{:ranch.get_port(listener)}")
  end

  defp http_arguments(url) do
    [
      transport: HTTP,
      throttle_timeout: 5_000,
      transport_options: [
        url: url,
        urls: [url],
        trace_urls: [url],
        fallback_urls: [],
        http: HTTP.HTTPoison,
        http_options: [timeout: 5_000, recv_timeout: 5_000]
      ]
    ]
  end

  defp assert_source(%{"formatter_revision" => _} = source) do
    assert source == @formatter_capture["source"]
  end

  defp assert_source(source) do
    assert source["kind"] == "recorded"
    assert source["node_revision_before"] == @producer_revision
    assert source["node_revision_after"] == @producer_revision
    assert source["application_version"]["git_commit"] == @producer_revision
    assert source["environment"] == "public node; disposable-node provenance not available"
  end

  defp assert_imported_header(imported, body) do
    for {key, wire} <- [
          hash: "hash",
          parent_hash: "parentHash",
          receipts_root: "receiptsRoot",
          transactions_root: "transactionsRoot",
          logs_bloom: "logsBloom",
          state_root: "stateRoot"
        ] do
      assert Map.fetch!(imported, key) == Map.fetch!(body, wire), wire
    end

    for {key, wire} <- [number: "number", gas_used: "gasUsed", gas_limit: "gasLimit", base_fee_per_gas: "baseFeePerGas"] do
      assert Map.fetch!(imported, key) == EthereumJSONRPC.quantity_to_integer(Map.fetch!(body, wire)), wire
    end
  end

  defp assert_http(request) do
    assert_receive {:capture_http_request, received}, 1_000
    assert received == request
  end

  defp rpc(request, arguments) do
    %{id: request["id"], method: request["method"], params: request["params"]}
    |> EthereumJSONRPC.request()
    |> EthereumJSONRPC.json_rpc(arguments)
  end

  defp configure_tracer do
    previous = Application.get_env(:ethereum_jsonrpc, Geth)

    on_exit(fn ->
      if is_nil(previous),
        do: Application.delete_env(:ethereum_jsonrpc, Geth),
        else: Application.put_env(:ethereum_jsonrpc, Geth, previous)
    end)

    Application.put_env(:ethereum_jsonrpc, Geth, tracer: "call_tracer", debug_trace_timeout: "5s")
  end
end
