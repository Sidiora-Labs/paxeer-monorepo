defmodule EthereumJSONRPC.WebSocket.Lifecycle.Node do
  @moduledoc false
  @behaviour :cowboy_websocket

  @impl :cowboy_websocket
  def init(request, options) do
    subscription_id = "0x" <> Integer.to_string(System.unique_integer([:positive, :monotonic]), 16)
    state = Map.merge(options, %{subscription_id: subscription_id, subscriptions: MapSet.new()})
    {:cowboy_websocket, request, state}
  end

  @impl :cowboy_websocket
  def websocket_handle({:text, text}, state) do
    %{"jsonrpc" => "2.0", "id" => id} = request = Jason.decode!(text)
    send(state.owner, {:node_request, state.phase, self(), request})
    {result, state} = respond(request, state)
    {:reply, {:text, Jason.encode!(%{jsonrpc: "2.0", id: id, result: result})}, state}
  end

  defp respond(%{"method" => "eth_subscribe", "params" => ["newHeads"]}, state) do
    send(state.owner, {:node_subscription, state.phase, self(), state.subscription_id})
    {state.subscription_id, %{state | subscriptions: MapSet.put(state.subscriptions, state.subscription_id)}}
  end

  defp respond(%{"method" => "eth_unsubscribe", "params" => [subscription_id]}, state) do
    result = MapSet.member?(state.subscriptions, subscription_id)
    send(state.owner, {:node_cancelled, state.phase, self(), subscription_id, result})
    {result, %{state | subscriptions: MapSet.delete(state.subscriptions, subscription_id)}}
  end

  defp respond(%{"method" => "eth_getBlockByHash", "params" => [hash, false]}, %{block: %{"hash" => hash}} = state) do
    {state.block, state}
  end

  @impl :cowboy_websocket
  def websocket_info(:new_head, state) do
    frames =
      Enum.map(state.subscriptions, fn subscription_id ->
        {:text,
         Jason.encode!(%{
           jsonrpc: "2.0",
           method: "eth_subscription",
           params: %{subscription: subscription_id, result: state.head}
         })}
      end)

    send(state.owner, {:node_published, state.phase, self(), length(frames)})
    {:reply, frames, state}
  end

  def websocket_info(:restart_connection, state) do
    {:reply, {:close, 1012, "service restart"}, state}
  end
end

defmodule EthereumJSONRPC.WebSocket.Lifecycle do
  @moduledoc false

  import ExUnit.Assertions
  import ExUnit.Callbacks, only: [on_exit: 1, start_supervised!: 1]

  alias EthereumJSONRPC.WebSocket
  alias EthereumJSONRPC.WebSocket.{RetryWorker, WebSocketClient}
  alias EthereumJSONRPC.WebSocket.Supervisor, as: WebSocketSupervisor
  alias EthereumJSONRPC.WebSocket.WebSocketClient.Options

  @client Indexer.Block.Realtime.WebSocket
  @primary_listener __MODULE__.Primary
  @fallback_listener __MODULE__.Fallback
  @tls_listener __MODULE__.TLS
  @fixture Path.expand("../../support/fixture/paxeer_x/new_head_full.json", __DIR__)
  @cert Path.join(__DIR__, "fixtures/localhost-cert.pem")
  @key Path.join(__DIR__, "fixtures/localhost-key.pem")
  @ca Path.join(__DIR__, "fixtures/ca-cert.pem")

  def assert_primary_recovery do
    fixture = fixture()
    primary_port = start_node(@primary_listener, :primary, fixture)
    fallback_port = start_node(@fallback_listener, :fallback, fixture)
    configure_runtime()

    transport_options = transport("ws://127.0.0.1:#{primary_port}/websocket")
    transport_options = %{transport_options | fallback_url: "ws://127.0.0.1:#{fallback_port}/websocket"}

    Application.put_env(:indexer, :subscribe_named_arguments,
      transport: WebSocket,
      transport_options: transport_options |> Map.from_struct() |> Map.to_list()
    )

    # The production supervisor supplies start_link(url, options), including the stable owner name.
    start_supervised!({WebSocketSupervisor, transport_options})
    initial_pid = Process.whereis(@client)
    await(fn -> :sys.get_state(initial_pid).connected end)
    assert {:ok, subscription} = subscribe(transport_options)
    assert subscription.transport_options.web_socket_options.web_socket == @client
    {primary_connection, primary_id} = assert_subscribed(:primary)
    assert_subscription(initial_pid, subscription, primary_id)
    assert_head(subscription, transport_options, primary_connection, :primary, fixture)

    assert :ok = :cowboy.stop_listener(@primary_listener)
    {fallback_connection, fallback_id} = assert_subscribed(:fallback)
    refute fallback_id == primary_id
    await(fn -> :sys.get_state(initial_pid).fallback? end)
    assert_subscription(initial_pid, subscription, fallback_id)
    assert_head(subscription, transport_options, fallback_connection, :fallback, fixture)

    assert start_node(@primary_listener, :recovered, fixture, primary_port) == primary_port
    monitor = Process.monitor(initial_pid)
    send(RetryWorker, :retry)
    assert_receive {:DOWN, ^monitor, :process, ^initial_pid, :shutdown}, 15_000
    {recovered_connection, recovered_id} = assert_subscribed(:recovered)
    recovered_pid = Process.whereis(@client)
    assert is_pid(recovered_pid)
    refute recovered_pid == initial_pid
    refute Process.alive?(initial_pid)
    refute recovered_id in [primary_id, fallback_id]
    assert_subscription(recovered_pid, subscription, recovered_id)
    refute :sys.get_state(recovered_pid).fallback?
    assert_head(subscription, transport_options, recovered_connection, :recovered, fixture)

    # The pre-repair handle captured this PID; cancellation through it must fail after replacement.
    assert {:noproc, {GenServer, :call, [^initial_pid, {:unsubscribe, ^subscription}, _]}} =
             catch_exit(WebSocketClient.unsubscribe(initial_pid, subscription))

    assert_cancelled(subscription, transport_options, recovered_pid, recovered_connection, :recovered, recovered_id)
    refute_receive {:node_request, :fallback, _, %{"method" => "eth_unsubscribe"}}
  end

  def assert_tls_reconnect do
    fixture = fixture()
    port = start_node(@tls_listener, :tls, fixture, 0, :tls)
    configure_runtime()

    # These are real control-plane peers used by the production WebSocket callbacks.
    start_supervised!(%{
      id: WebSocketSupervisor,
      start: {Supervisor, :start_link, [[], [name: WebSocketSupervisor, strategy: :one_for_one]]}
    })

    start_supervised!({RetryWorker, []})
    transport_options = transport("wss://localhost:#{port}/websocket")

    # The public WebSockex constructor allows fixture CA trust; production start_link pins certifi's roots.
    options = [
      name: @client,
      async: true,
      handle_initial_conn_failure: true,
      insecure: false,
      ssl_options: [
        verify: :verify_peer,
        cacertfile: String.to_charlist(@ca),
        server_name_indication: ~c"localhost",
        customize_hostname_check: [match_fun: :public_key.pkix_verify_hostname_match_fun(:https)]
      ]
    ]

    state = %WebSocketClient{url: transport_options.url, web_socket: @client}

    pid =
      start_supervised!(%{
        id: @client,
        start: {WebSockex, :start_link, [transport_options.url, WebSocketClient, state, options]},
        restart: :temporary
      })

    await(fn -> :sys.get_state(pid).connected end)
    assert {:ok, subscription} = subscribe(transport_options)
    assert subscription.transport_options.url == transport_options.url
    {connection, original_id} = assert_subscribed(:tls)
    assert_subscription(pid, subscription, original_id)
    assert_head(subscription, transport_options, connection, :tls, fixture)

    send(connection, :restart_connection)
    {reconnected, replacement_id} = assert_subscribed(:tls)
    refute reconnected == connection
    refute replacement_id == original_id
    assert Process.whereis(@client) == pid
    assert_subscription(pid, subscription, replacement_id)
    assert_head(subscription, transport_options, reconnected, :tls, fixture)
    assert_cancelled(subscription, transport_options, pid, reconnected, :tls, replacement_id)
  end

  defp fixture do
    %{"new_head" => head, "block" => block} = @fixture |> File.read!() |> Jason.decode!()
    %{owner: self(), head: head, block: block}
  end

  defp start_node(listener, phase, fixture, port \\ 0, mode \\ :clear) do
    options = Map.put(fixture, :phase, phase)
    dispatch = :cowboy_router.compile([{:_, [{"/websocket", __MODULE__.Node, options}]}])
    socket_options = [ip: {127, 0, 0, 1}, port: port]
    protocol_options = %{env: %{dispatch: dispatch}}

    {:ok, _} =
      case mode do
        :clear ->
          :cowboy.start_clear(listener, socket_options, protocol_options)

        :tls ->
          :cowboy.start_tls(
            listener,
            socket_options ++ [certfile: String.to_charlist(@cert), keyfile: String.to_charlist(@key)],
            protocol_options
          )
      end

    on_exit(fn -> :cowboy.stop_listener(listener) end)
    :ranch.get_port(listener)
  end

  defp configure_runtime do
    for {app, key} <- [
          {:ethereum_jsonrpc, RetryWorker},
          {:indexer, :subscribe_named_arguments},
          {:indexer, :realtime_overrides}
        ] do
      previous = Application.fetch_env(app, key)

      on_exit(fn ->
        case previous do
          {:ok, value} -> Application.put_env(app, key, value)
          :error -> Application.delete_env(app, key)
        end
      end)
    end

    Application.put_env(:ethereum_jsonrpc, RetryWorker, retry_interval: 60_000)
    Application.put_env(:indexer, :realtime_overrides, [])
  end

  defp transport(url) do
    %WebSocket{url: url, web_socket: WebSocketClient, web_socket_options: %Options{web_socket: @client}}
  end

  defp subscribe(options) do
    EthereumJSONRPC.subscribe("newHeads", [], transport: WebSocket, transport_options: options)
  end

  defp rpc(options, method, params) do
    request = EthereumJSONRPC.request(%{id: 1, method: method, params: params})
    WebSocket.json_rpc(request, options)
  end

  defp assert_subscribed(phase) do
    assert_receive {:node_request, ^phase, connection,
                    %{"jsonrpc" => "2.0", "method" => "eth_subscribe", "params" => ["newHeads"]}},
                   15_000

    assert_receive {:node_subscription, ^phase, ^connection, subscription_id}, 1_000
    assert is_binary(subscription_id)
    {connection, subscription_id}
  end

  defp assert_subscription(pid, subscription, server_id) do
    await(fn ->
      state = :sys.get_state(pid)
      state.connected and state.subscription_reference_to_subscription_id[subscription.reference] == server_id
    end)

    state = :sys.get_state(pid)
    assert state.subscription_reference_to_subscription[subscription.reference] == subscription
    assert state.subscription_id_to_subscription_reference[server_id] == subscription.reference
  end

  defp assert_head(subscription, options, connection, phase, fixture) do
    send(connection, :new_head)
    assert_receive {:node_published, ^phase, ^connection, 1}, 1_000
    assert_receive {^subscription, {:ok, head}}, 10_000
    assert head == fixture.head
    assert {:ok, block} = rpc(options, "eth_getBlockByHash", [head["hash"], false])
    assert block == fixture.block

    for field <- ~w(hash number parentHash stateRoot receiptsRoot transactionsRoot) do
      assert Map.fetch!(head, field) == Map.fetch!(block, field)
    end
  end

  defp assert_cancelled(subscription, options, pid, connection, phase, server_id) do
    assert :ok = EthereumJSONRPC.unsubscribe(subscription)

    assert_receive {:node_request, ^phase, ^connection,
                    %{"jsonrpc" => "2.0", "method" => "eth_unsubscribe", "params" => [^server_id]}},
                   1_000

    assert_receive {:node_cancelled, ^phase, ^connection, ^server_id, true}, 1_000
    state = :sys.get_state(pid)
    refute Map.has_key?(state.subscription_reference_to_subscription, subscription.reference)
    refute Map.has_key?(state.subscription_reference_to_subscription_id, subscription.reference)
    refute Map.has_key?(state.subscription_id_to_subscription_reference, server_id)
    assert Enum.empty?(state.request_id_to_registration)
    assert {:ok, false} = rpc(options, "eth_unsubscribe", [server_id])
    assert_receive {:node_cancelled, ^phase, ^connection, ^server_id, false}, 1_000
    assert {:error, :not_found} = EthereumJSONRPC.unsubscribe(subscription)
    send(connection, :new_head)
    assert_receive {:node_published, ^phase, ^connection, 0}, 1_000
    refute_receive {^subscription, {:ok, _}}, 1_000
  end

  defp await(condition, attempts \\ 300)
  defp await(_condition, 0), do: flunk("WebSocket lifecycle did not reach the expected state")

  defp await(condition, attempts) do
    unless condition.() do
      Process.sleep(50)
      await(condition, attempts - 1)
    end
  end
end
