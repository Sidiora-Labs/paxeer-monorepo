defmodule EthereumJSONRPC.Repair2233442Test.Node do
  @moduledoc false
  @behaviour :cowboy_websocket

  @impl :cowboy_websocket
  def init(request, %{transport: :websocket} = state) do
    {:cowboy_websocket, request, state}
  end

  def init(request, %{transport: :http} = state) do
    {:ok, body, request} = :cowboy_req.read_body(request)
    decoded = Jason.decode!(body)

    response =
      if is_list(decoded) do
        Enum.map(decoded, &respond(&1, state))
      else
        respond(decoded, state)
      end

    request = :cowboy_req.reply(200, %{"content-type" => "application/json"}, Jason.encode!(response), request)
    {:ok, request, state}
  end

  @impl :cowboy_websocket
  def websocket_handle({:text, text}, state) do
    response = text |> Jason.decode!() |> respond(state)
    {:reply, {:text, Jason.encode!(response)}, state}
  end

  @impl :cowboy_websocket
  def websocket_info(:publish_head, state) do
    notification = %{
      jsonrpc: "2.0",
      method: "eth_subscription",
      params: %{subscription: "0x125", result: state.fixture["new_head"]}
    }

    {:reply, {:text, Jason.encode!(notification)}, state}
  end

  defp respond(%{"jsonrpc" => "2.0", "id" => id} = request, state) do
    send(state.owner, {:node_request, state.transport, self(), request})
    Map.merge(%{jsonrpc: "2.0", id: id}, result(request, state))
  end

  defp result(%{"method" => "eth_chainId", "params" => []}, state), do: state.chain_response

  defp result(%{"method" => "eth_getBlockByHash", "params" => [hash, true]}, state) do
    ^hash = state.fixture["block"]["hash"]
    %{result: state.fixture["block"]}
  end

  defp result(%{"method" => "eth_getBlockByNumber", "params" => ["latest", false]}, state) do
    %{result: state.fixture["block"]}
  end

  defp result(%{"method" => "eth_subscribe", "params" => ["newHeads"]}, _state), do: %{result: "0x125"}
  defp result(%{"method" => "eth_unsubscribe", "params" => ["0x125"]}, _state), do: %{result: true}
end

defmodule EthereumJSONRPC.Repair2233442Test do
  use ExUnit.Case, async: false

  alias EthereumJSONRPC.Case.Geth.HTTPWebSocket
  alias EthereumJSONRPC.Repair2233442Test.Node
  alias EthereumJSONRPC.WebSocket.Supervisor, as: WebSocketSupervisor

  @fixture Path.expand("../support/fixture/paxeer_x/new_head_full.json", __DIR__)
  @listener __MODULE__.Listener

  setup_all do
    {:ok, _} = Application.ensure_all_started(:cowboy)
    :ok
  end

  setup do
    previous = Application.fetch_env(:ethereum_jsonrpc, HTTPWebSocket)
    previous_adapter = Application.fetch_env(:tesla, :adapter)

    on_exit(fn ->
      case previous do
        {:ok, value} -> Application.put_env(:ethereum_jsonrpc, HTTPWebSocket, value)
        :error -> Application.delete_env(:ethereum_jsonrpc, HTTPWebSocket)
      end

      case previous_adapter do
        {:ok, value} -> Application.put_env(:tesla, :adapter, value)
        :error -> Application.delete_env(:tesla, :adapter)
      end
    end)

    Application.put_env(:tesla, :adapter, Tesla.Adapter.Mint)
    %{fixture: @fixture |> File.read!() |> Jason.decode!()}
  end

  test "external-test-endpoint-140: configured chain-125 pair carries recorded heads into the real block consumer",
       ctx do
    config = start_node(ctx.fixture)
    Application.put_env(:ethereum_jsonrpc, HTTPWebSocket, config)
    context = HTTPWebSocket.setup()
    assert context.block_interval == 500
    assert context.json_rpc_named_arguments[:variant] == EthereumJSONRPC.PaxeerX
    assert context.json_rpc_named_arguments[:transport_options][:urls] == [config[:http_url]]
    assert context.subscribe_named_arguments[:transport_options].url == config[:websocket_url]
    assert_request(:http, "eth_chainId", [])
    assert_request(:websocket, "eth_chainId", [])

    assert {:ok, subscription} = EthereumJSONRPC.subscribe("newHeads", [], context.subscribe_named_arguments)
    connection = assert_request(:websocket, "eth_subscribe", ["newHeads"])
    send(connection, :publish_head)
    assert_receive {^subscription, {:ok, head}}, 2_000
    assert head == ctx.fixture["new_head"]

    assert {:ok, %EthereumJSONRPC.Blocks{blocks_params: [block], errors: []}} =
             EthereumJSONRPC.PaxeerX.fetch_block_by_new_head(head, context.json_rpc_named_arguments)

    assert_request(:http, "eth_getBlockByHash", [head["hash"], true])
    assert block.hash == ctx.fixture["block"]["hash"]
    assert block.number == EthereumJSONRPC.quantity_to_integer(head["number"])
    assert block.parent_hash == head["parentHash"]
    assert block.receipts_root == head["receiptsRoot"]
    assert block.state_root == head["stateRoot"]
    assert block.transactions_root == head["transactionsRoot"]
    assert :ok = EthereumJSONRPC.unsubscribe(subscription)
    assert_request(:websocket, "eth_unsubscribe", ["0x125"])
  end

  test "external-test-endpoint-140: absent or incomplete explicit configuration never inherits provider endpoints" do
    Application.delete_env(:ethereum_jsonrpc, HTTPWebSocket)
    assert_raise ArgumentError, fn -> HTTPWebSocket.setup() end

    for config <- [[http_url: "http://127.0.0.1:8545"], [websocket_url: "ws://127.0.0.1:8546"]] do
      Application.put_env(:ethereum_jsonrpc, HTTPWebSocket, config)
      assert_raise KeyError, fn -> HTTPWebSocket.setup() end
    end
  end

  test "external-test-endpoint-140: invalid endpoint pairs fail before opening a transport" do
    for {http, websocket} <- [
          {"http://127.0.0.1:8545", "ws://localhost:8546"},
          {"ws://127.0.0.1:8545", "ws://127.0.0.1:8546"},
          {"http://127.0.0.1:8545", "http://127.0.0.1:8546"},
          {"http://127.0.0.1:8545", "wss://127.0.0.1:8546"},
          {"http://node.invalid:8545", "ws://node.invalid:8546"},
          {"http://user:password@127.0.0.1:8545", "ws://127.0.0.1:8546"},
          {"http://127.0.0.1:8545", "ws://user:password@127.0.0.1:8546"},
          {nil, "ws://127.0.0.1:8546"}
        ] do
      Application.put_env(:ethereum_jsonrpc, HTTPWebSocket, http_url: http, websocket_url: websocket)
      assert_raise ArgumentError, ~r/paired HTTP and WebSocket/, fn -> HTTPWebSocket.setup() end
    end

    refute_receive {:node_request, _, _, _}
  end

  test "external-test-endpoint-140: HTTP chain mismatch refuses the node before subscription", ctx do
    config = start_node(ctx.fixture, %{result: "0x1"}, %{result: "0x7d"})
    # A chain_id entry alone must not opt out of the merged-network requirement.
    Application.put_env(:ethereum_jsonrpc, HTTPWebSocket, Keyword.put(config, :chain_id, 1))
    assert_raise ArgumentError, "http test endpoint must report chain id 125", fn -> HTTPWebSocket.setup() end
    assert_request(:http, "eth_chainId", [])
    refute_receive {:node_request, :websocket, _, _}
    refute_receive {:node_request, _, _, %{"method" => "eth_getBlockByHash"}}
  end

  test "external-test-endpoint-140: WebSocket chain mismatch refuses a split pair and closes the client", ctx do
    config = start_node(ctx.fixture, %{result: "0x7d"}, %{result: "0x1"})
    Application.put_env(:ethereum_jsonrpc, HTTPWebSocket, config)
    assert_raise ArgumentError, "websocket test endpoint must report chain id 125", fn -> HTTPWebSocket.setup() end
    assert_request(:http, "eth_chainId", [])
    assert_request(:websocket, "eth_chainId", [])
    assert Process.whereis(HTTPWebSocket.Client) == nil
    assert Process.whereis(WebSocketSupervisor) == nil
    refute_receive {:node_request, _, _, %{"method" => "eth_subscribe"}}
    refute_receive {:node_request, _, _, %{"method" => "eth_getBlockByHash"}}
  end

  for transport <- [:http, :websocket] do
    @transport transport
    test "external-test-endpoint-140: #{@transport} chain RPC error fails closed", ctx do
      error = %{error: %{code: -32601, message: "Method not found"}}
      http = if @transport == :http, do: error, else: %{result: "0x7d"}
      websocket = if @transport == :websocket, do: error, else: %{result: "0x7d"}
      config = start_node(ctx.fixture, http, websocket)
      Application.put_env(:ethereum_jsonrpc, HTTPWebSocket, config)

      assert_raise ArgumentError, "#{@transport} test endpoint failed eth_chainId", fn -> HTTPWebSocket.setup() end
      assert_request(@transport, "eth_chainId", [])
      assert Process.whereis(HTTPWebSocket.Client) == nil
      assert Process.whereis(WebSocketSupervisor) == nil
      refute_receive {:node_request, _, _, %{"method" => "eth_subscribe"}}
    end
  end

  test "external-test-endpoint-140: other-client compatibility requires explicit profile and chain id", ctx do
    config = start_node(ctx.fixture, %{result: "0x1"}, %{result: "0x1"})
    compatibility = Keyword.put(config, :profile, :geth_compatibility)
    Application.put_env(:ethereum_jsonrpc, HTTPWebSocket, compatibility)
    assert_raise KeyError, fn -> HTTPWebSocket.setup() end

    Application.put_env(:ethereum_jsonrpc, HTTPWebSocket, Keyword.put(compatibility, :chain_id, 1))
    context = HTTPWebSocket.setup()
    assert context.json_rpc_named_arguments[:variant] == EthereumJSONRPC.Geth
    assert_request(:http, "eth_chainId", [])
    assert_request(:websocket, "eth_chainId", [])

    assert {:ok, number} = EthereumJSONRPC.fetch_block_number_by_tag("latest", context.json_rpc_named_arguments)
    assert number == EthereumJSONRPC.quantity_to_integer(ctx.fixture["block"]["number"])
    assert_request(:http, "eth_getBlockByNumber", ["latest", false])
  end

  defp start_node(fixture, http_chain \\ %{result: "0x7d"}, websocket_chain \\ %{result: "0x7d"}) do
    state = %{owner: self(), fixture: fixture}

    dispatch =
      :cowboy_router.compile([
        {:_,
         [
           {"/rpc", Node, Map.merge(state, %{transport: :http, chain_response: http_chain})},
           {"/ws", Node, Map.merge(state, %{transport: :websocket, chain_response: websocket_chain})}
         ]}
      ])

    {:ok, _} = :cowboy.start_clear(@listener, [ip: {127, 0, 0, 1}, port: 0], %{env: %{dispatch: dispatch}})
    on_exit(fn -> :cowboy.stop_listener(@listener) end)
    port = :ranch.get_port(@listener)
    [http_url: "http://127.0.0.1:#{port}/rpc", websocket_url: "ws://127.0.0.1:#{port}/ws"]
  end

  defp assert_request(transport, method, params) do
    assert_receive {:node_request, ^transport, connection,
                    %{"jsonrpc" => "2.0", "method" => ^method, "params" => ^params}},
                   2_000

    connection
  end
end
