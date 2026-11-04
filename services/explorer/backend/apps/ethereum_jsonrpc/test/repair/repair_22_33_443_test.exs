defmodule EthereumJSONRPC.Repair2233443Test.Node do
  @moduledoc false
  @behaviour :cowboy_websocket

  @impl :cowboy_websocket
  def init(request, state) do
    if :cowboy_req.header("upgrade", request) == "websocket" do
      {:cowboy_websocket, request, state}
    else
      {:ok, body, request} = :cowboy_req.read_body(request)
      rpc = Jason.decode!(body)
      result = respond(rpc, state, :http)
      body = Jason.encode!(%{jsonrpc: "2.0", id: rpc["id"], result: result})
      {:ok, :cowboy_req.reply(200, %{"content-type" => "application/json"}, body, request), state}
    end
  end

  @impl :cowboy_websocket
  def websocket_handle({:text, text}, state) do
    request = Jason.decode!(text)
    result = respond(request, state, :websocket)
    frame = {:text, Jason.encode!(%{jsonrpc: "2.0", id: request["id"], result: result})}

    case request do
      %{"method" => "eth_subscribe", "params" => ["newHeads"]} ->
        send(self(), :publish_head)
        {:reply, frame, state}

      _ ->
        {:reply, frame, state}
    end
  end

  @impl :cowboy_websocket
  def websocket_info(:publish_head, state) do
    notification = %{
      jsonrpc: "2.0",
      method: "eth_subscription",
      params: %{subscription: "0x1", result: state.head}
    }

    {:reply, {:text, Jason.encode!(notification)}, state}
  end

  defp respond(%{"jsonrpc" => "2.0", "method" => method, "params" => params} = request, state, transport) do
    send(state.owner, {:node_request, transport, self(), request})

    case {method, params} do
      {"eth_chainId", []} ->
        Map.fetch!(state.chain_ids, transport)

      {"eth_getBlockByHash", [hash, false]} ->
        %{"hash" => ^hash} = state.block
        state.block

      {"eth_subscribe", ["newHeads"]} ->
        "0x1"

      {"eth_unsubscribe", ["0x1"]} ->
        true
    end
  end
end

defmodule EthereumJSONRPC.Repair2233443Test do
  use ExUnit.Case, async: false

  alias EthereumJSONRPC.Case.Nethermind.HTTPWebSocket
  alias EthereumJSONRPC.Repair2233443Test.Node
  alias EthereumJSONRPC.WebSocket
  alias EthereumJSONRPC.WebSocket.Supervisor, as: WebSocketSupervisor

  @fixture Path.expand("../support/fixture/paxeer_x/new_head_full.json", __DIR__)
  @listener __MODULE__.NodeListener

  setup do
    previous_adapter = Application.fetch_env(:tesla, :adapter)
    previous_config = Application.fetch_env(:ethereum_jsonrpc, HTTPWebSocket)
    Application.put_env(:tesla, :adapter, Tesla.Adapter.Mint)

    on_exit(fn ->
      restore(:tesla, :adapter, previous_adapter)
      restore(:ethereum_jsonrpc, HTTPWebSocket, previous_config)
    end)

    :ok
  end

  test "external-test-endpoint-142: explicit local pair verifies chain 125 and transports recorded blocks and heads" do
    {config, fixture} = start_node(%{http: "0x7d", websocket: "0x7d"})
    Application.put_env(:ethereum_jsonrpc, HTTPWebSocket, config)
    context = HTTPWebSocket.setup()
    assert context.json_rpc_named_arguments[:variant] == EthereumJSONRPC.PaxeerX
    assert context.json_rpc_named_arguments[:transport_options][:urls] == [config[:http_url]]
    assert context.subscribe_named_arguments[:transport_options].url == config[:websocket_url]
    assert_request(:http, "eth_chainId", [])
    assert_request(:websocket, "eth_chainId", [])

    block = fixture["block"]
    head = fixture["new_head"]
    request = EthereumJSONRPC.request(%{id: 42, method: "eth_getBlockByHash", params: [block["hash"], false]})
    assert {:ok, ^block} = EthereumJSONRPC.json_rpc(request, context.json_rpc_named_arguments)
    assert {:ok, ^block} = WebSocket.json_rpc(request, context.subscribe_named_arguments[:transport_options])
    assert_request(:http, "eth_getBlockByHash", [block["hash"], false])
    assert_request(:websocket, "eth_getBlockByHash", [block["hash"], false])
    assert {:ok, subscription} = EthereumJSONRPC.subscribe("newHeads", [], context.subscribe_named_arguments)
    assert_receive {^subscription, {:ok, ^head}}, 2_000
    assert_request(:websocket, "eth_subscribe", ["newHeads"])
    assert :ok = EthereumJSONRPC.unsubscribe(subscription)
    assert_request(:websocket, "eth_unsubscribe", ["0x1"])
  end

  test "external-test-endpoint-142: rejects a wrong HTTP chain before starting subscriptions" do
    {config, _fixture} = start_node(%{http: "0x1", websocket: "0x7d"})

    assert_raise ArgumentError, "http endpoint did not identify expected chain 125", fn ->
      HTTPWebSocket.setup(config)
    end

    assert_request(:http, "eth_chainId", [])
    refute Process.whereis(WebSocketSupervisor)
    refute_receive {:node_request, :websocket, _, _}
  end

  test "external-test-endpoint-142: rejects a mismatched WebSocket chain and closes the client" do
    {config, _fixture} = start_node(%{http: "0x7d", websocket: "0x1"})

    assert_raise ArgumentError, "websocket endpoint did not identify expected chain 125", fn ->
      HTTPWebSocket.setup(config)
    end

    assert_request(:http, "eth_chainId", [])
    assert_request(:websocket, "eth_chainId", [])
    refute Process.whereis(WebSocketSupervisor)
    refute Process.whereis(HTTPWebSocket.Client)
    refute_receive {:node_request, _, _, %{"method" => "eth_subscribe"}}
  end

  test "external-test-endpoint-142: another chain requires the explicit compatibility profile" do
    {config, _fixture} = start_node(%{http: "0x1", websocket: "0x1"})
    context = HTTPWebSocket.setup(config ++ [profile: :nethermind_compatibility, chain_id: 1])
    assert context.json_rpc_named_arguments[:variant] == EthereumJSONRPC.Nethermind
    assert_request(:http, "eth_chainId", [])
    assert_request(:websocket, "eth_chainId", [])
  end

  test "external-test-endpoint-142: missing, remote, unpaired and credential-bearing configuration fails closed" do
    Application.delete_env(:ethereum_jsonrpc, HTTPWebSocket)
    assert_raise ArgumentError, fn -> HTTPWebSocket.setup() end
    assert_raise KeyError, fn -> HTTPWebSocket.setup(http_url: "http://127.0.0.1:1") end

    for config <- [
          [http_url: "http://node.invalid:8545", websocket_url: "ws://node.invalid:8546"],
          [http_url: "http://127.0.0.1:1", websocket_url: "ws://localhost:2"],
          [http_url: "http://127.0.0.1:1", websocket_url: "wss://127.0.0.1:2"],
          [http_url: "http://user:password@127.0.0.1:1", websocket_url: "ws://127.0.0.1:2"],
          [http_url: "http://127.0.0.1:1", websocket_url: "ws://user:password@127.0.0.1:2"],
          [http_url: "/rpc", websocket_url: "ws://127.0.0.1:2"],
          [http_url: "http://127.0.0.1:1", websocket_url: "ws://127.0.0.1:2", profile: :unknown],
          [http_url: "http://127.0.0.1:1", websocket_url: "ws://127.0.0.1:2", profile: :nethermind_compatibility],
          [
            http_url: "http://127.0.0.1:1",
            websocket_url: "ws://127.0.0.1:2",
            profile: :nethermind_compatibility,
            chain_id: 0
          ]
        ] do
      exception =
        if config[:profile] == :nethermind_compatibility and not Keyword.has_key?(config, :chain_id),
          do: KeyError,
          else: ArgumentError

      assert_raise exception, fn -> HTTPWebSocket.setup(config) end
    end

    refute Process.whereis(WebSocketSupervisor)
  end

  defp start_node(chain_ids) do
    fixture = @fixture |> File.read!() |> Jason.decode!()
    state = %{owner: self(), chain_ids: chain_ids, block: fixture["block"], head: fixture["new_head"]}
    dispatch = :cowboy_router.compile([{:_, [{"/rpc", Node, state}]}])
    {:ok, _} = :cowboy.start_clear(@listener, [ip: {127, 0, 0, 1}, port: 0], %{env: %{dispatch: dispatch}})
    on_exit(fn -> :cowboy.stop_listener(@listener) end)
    port = :ranch.get_port(@listener)
    {[http_url: "http://127.0.0.1:#{port}/rpc", websocket_url: "ws://127.0.0.1:#{port}/rpc"], fixture}
  end

  defp assert_request(transport, method, params) do
    assert_receive {:node_request, ^transport, _, %{"jsonrpc" => "2.0", "method" => ^method, "params" => ^params}},
                   2_000
  end

  defp restore(app, key, {:ok, value}), do: Application.put_env(app, key, value)
  defp restore(app, key, :error), do: Application.delete_env(app, key)
end
