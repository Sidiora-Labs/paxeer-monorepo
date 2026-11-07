defmodule EthereumJSONRPC.Repair2233444Test.Node do
  @moduledoc false
  @behaviour :cowboy_websocket

  @impl :cowboy_websocket
  def init(request, %{transport: :web_socket} = state) do
    {:cowboy_websocket, request, Map.put(state, :subscribed?, false)}
  end

  def init(request, %{transport: :http} = state) do
    {:ok, body, request} = :cowboy_req.read_body(request)
    rpc = Jason.decode!(body)
    send(state.owner, {:node_request, :http, rpc})
    response = response(rpc, state)
    request = :cowboy_req.reply(200, %{"content-type" => "application/json"}, Jason.encode!(response), request)
    {:ok, request, state}
  end

  @impl :cowboy_websocket
  def websocket_handle({:text, text}, state) do
    rpc = Jason.decode!(text)
    send(state.owner, {:node_request, :web_socket, rpc})
    response = response(rpc, state)

    state =
      case rpc do
        %{"method" => "eth_subscribe", "params" => ["newHeads"]} ->
          send(self(), :publish_head)
          %{state | subscribed?: true}

        %{"method" => "eth_unsubscribe", "params" => ["0x125"]} ->
          %{state | subscribed?: false}

        _ ->
          state
      end

    {:reply, {:text, Jason.encode!(response)}, state}
  end

  @impl :cowboy_websocket
  def websocket_info(:publish_head, %{subscribed?: true} = state) do
    notification = %{
      jsonrpc: "2.0",
      method: "eth_subscription",
      params: %{subscription: "0x125", result: state.head}
    }

    {:reply, {:text, Jason.encode!(notification)}, state}
  end

  defp response(%{"jsonrpc" => "2.0", "id" => id} = rpc, state) do
    case result(rpc, state) do
      :rpc_error -> %{jsonrpc: "2.0", id: id, error: %{code: -32603, message: "Internal error"}}
      result -> %{jsonrpc: "2.0", id: id, result: result}
    end
  end

  defp result(%{"method" => "eth_chainId", "params" => []}, state) do
    Map.fetch!(state, state.transport)
  end

  defp result(%{"method" => "eth_getBlockByNumber", "params" => ["latest", false]}, state), do: state.block

  defp result(%{"method" => "eth_getBlockByHash", "params" => [hash, false]}, state) do
    if hash == state.block["hash"], do: state.paired_block, else: nil
  end

  defp result(%{"method" => "eth_subscribe", "params" => ["newHeads"]}, _state), do: "0x125"

  defp result(%{"method" => "eth_unsubscribe", "params" => ["0x125"]}, state), do: state.subscribed?
end

defmodule EthereumJSONRPC.Repair2233444Test do
  use ExUnit.Case, async: false

  alias EthereumJSONRPC.Repair2233444Test.Node
  alias EthereumJSONRPC.WebSocket.Case.{Geth, Nethermind}

  @fixture Path.expand("../support/fixture/paxeer_x/new_head_full.json", __DIR__)
  @listener __MODULE__.Listener

  setup_all do
    {:ok, _} = Application.ensure_all_started(:cowboy)
    :ok
  end

  setup do
    previous = for module <- [Geth, Nethermind], do: {module, Application.fetch_env(:ethereum_jsonrpc, module)}

    on_exit(fn ->
      for {module, value} <- previous do
        case value do
          {:ok, config} -> Application.put_env(:ethereum_jsonrpc, module, config)
          :error -> Application.delete_env(:ethereum_jsonrpc, module)
        end
      end
    end)

    %{"new_head" => head, "block" => block} = @fixture |> File.read!() |> Jason.decode!()
    %{head: head, block: block}
  end

  for {module, finding, interval} <- [
        {Geth, "external-test-endpoint-146", 25_000},
        {Nethermind, "external-test-endpoint-148", 5_000}
      ] do
    @case_module module
    @interval interval

    test "#{finding}: configured local chain-125 HTTP and WebSocket consume the same recorded head", fixture do
      config = start_node(fixture)
      configure(@case_module, config)
      context = @case_module.setup()

      assert context.profile == :paxeer_x
      assert context.chain_id == 125
      assert context.block_interval == @interval
      assert context.json_rpc_named_arguments[:transport_options][:urls] == [config[:http_url]]
      assert context.subscribe_named_arguments[:transport_options].url == config[:web_socket_url]
      assert_validated_pair(fixture.block)

      assert {:ok, subscription} = EthereumJSONRPC.subscribe("newHeads", [], context.subscribe_named_arguments)
      assert_request(:web_socket, "eth_subscribe", ["newHeads"])
      assert_receive {^subscription, {:ok, head}}, 5_000
      assert head == fixture.head

      request = EthereumJSONRPC.request(%{id: 22, method: "eth_getBlockByHash", params: [head["hash"], false]})
      assert {:ok, block} = EthereumJSONRPC.json_rpc(request, context.json_rpc_named_arguments)
      assert block == fixture.block
      assert_request(:http, "eth_getBlockByHash", [head["hash"], false])

      for field <- ~w(hash number parentHash stateRoot receiptsRoot transactionsRoot) do
        assert Map.fetch!(head, field) == Map.fetch!(block, field)
      end

      assert :ok = EthereumJSONRPC.unsubscribe(subscription)
      assert_request(:web_socket, "eth_unsubscribe", ["0x125"])
    end

    test "#{finding}: missing configuration never selects an embedded endpoint", fixture do
      Application.delete_env(:ethereum_jsonrpc, @case_module)
      assert_raise ArgumentError, fn -> @case_module.setup() end

      config = start_node(fixture)

      for key <- [:http_url, :web_socket_url, :profile] do
        configure(@case_module, Keyword.delete(config, key))
        assert_raise KeyError, fn -> @case_module.setup() end
      end

      refute_receive {:node_request, _, _}
    end

    test "#{finding}: invalid pairs and implicit other-chain profiles are rejected before IO", fixture do
      config = start_node(fixture)

      for overrides <- [
            [http_url: "ws://127.0.0.1/rpc"],
            [web_socket_url: "http://127.0.0.1/ws"],
            [web_socket_url: "ws://localhost/ws"],
            [http_url: "http://node.invalid/rpc", web_socket_url: "ws://node.invalid/ws"],
            [http_url: "http://user:password@127.0.0.1/rpc"],
            [web_socket_url: nil],
            [profile: :geth],
            [chain_id: 1],
            [profile: :compatibility, chain_id: 0]
          ] do
        configure(@case_module, Keyword.merge(config, overrides))
        assert_raise ArgumentError, fn -> @case_module.setup() end
      end

      configure(@case_module, Keyword.put(config, :profile, :compatibility))
      assert_raise KeyError, fn -> @case_module.setup() end
      refute_receive {:node_request, _, _}
    end

    for transport <- [:http, :web_socket] do
      @transport transport

      test "#{finding}: #{@transport} wrong chain is rejected before block data", fixture do
        config = start_node(fixture, %{@transport => "0x1"})
        configure(@case_module, config)

        assert_raise ArgumentError, "configured endpoint chain id does not match the declared profile", fn ->
          @case_module.setup()
        end

        assert_request(:http, "eth_chainId", [])
        if @transport == :web_socket, do: assert_request(:web_socket, "eth_chainId", [])
        refute_receive {:node_request, _, %{"method" => "eth_getBlockByNumber"}}
        refute_receive {:node_request, _, %{"method" => "eth_getBlockByHash"}}
        refute_receive {:node_request, _, %{"method" => "eth_subscribe"}}
      end

      test "#{finding}: #{@transport} RPC failure cannot bypass chain validation", fixture do
        config = start_node(fixture, %{@transport => :rpc_error})
        configure(@case_module, config)

        assert_raise ArgumentError, "configured endpoint failed eth_chainId", fn -> @case_module.setup() end
        assert_request(:http, "eth_chainId", [])
        if @transport == :web_socket, do: assert_request(:web_socket, "eth_chainId", [])
        refute_receive {:node_request, _, %{"method" => "eth_getBlockByNumber"}}
        refute_receive {:node_request, _, %{"method" => "eth_subscribe"}}
      end
    end

    test "#{finding}: matching chain ids cannot hide mismatched node block data", fixture do
      block = Map.put(fixture.block, "receiptsRoot", fixture.block["stateRoot"])
      config = start_node(fixture, %{paired_block: block})
      configure(@case_module, config)

      assert_raise ArgumentError, "configured HTTP and WebSocket endpoints disagree on the same block", fn ->
        @case_module.setup()
      end

      assert_validated_pair(fixture.block)
      refute_receive {:node_request, _, %{"method" => "eth_subscribe"}}
    end

    test "#{finding}: compatibility is explicitly configured and retains paired transport validation", fixture do
      config = start_node(fixture)
      configure(@case_module, Keyword.merge(config, profile: :compatibility, chain_id: 125))
      context = @case_module.setup()
      assert context.profile == :compatibility
      assert context.chain_id == 125
      assert_validated_pair(fixture.block)
    end
  end

  defp configure(module, config), do: Application.put_env(:ethereum_jsonrpc, module, config)

  defp start_node(fixture, overrides \\ %{}) do
    state =
      fixture
      |> Map.merge(%{owner: self(), http: "0x7d", web_socket: "0x7d", paired_block: fixture.block})
      |> Map.merge(overrides)

    dispatch =
      :cowboy_router.compile([
        {:_,
         [
           {"/rpc", Node, Map.put(state, :transport, :http)},
           {"/ws", Node, Map.put(state, :transport, :web_socket)}
         ]}
      ])

    {:ok, _} = :cowboy.start_clear(@listener, [ip: {127, 0, 0, 1}, port: 0], %{env: %{dispatch: dispatch}})
    on_exit(fn -> :cowboy.stop_listener(@listener) end)
    port = :ranch.get_port(@listener)
    [profile: :paxeer_x, http_url: "http://127.0.0.1:#{port}/rpc", web_socket_url: "ws://127.0.0.1:#{port}/ws"]
  end

  defp assert_validated_pair(block) do
    assert_request(:http, "eth_chainId", [])
    assert_request(:web_socket, "eth_chainId", [])
    assert_request(:http, "eth_getBlockByNumber", ["latest", false])
    assert_request(:web_socket, "eth_getBlockByHash", [block["hash"], false])
  end

  defp assert_request(transport, method, params) do
    assert_receive {:node_request, ^transport,
                    %{"jsonrpc" => "2.0", "id" => id, "method" => ^method, "params" => ^params} = request},
                   5_000

    assert is_integer(id)
    assert Enum.sort(Map.keys(request)) == ~w(id jsonrpc method params)
  end
end
