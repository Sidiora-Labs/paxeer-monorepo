defmodule EthereumJSONRPC.Repair2233431Test.NodeWebSocket do
  @moduledoc false
  @behaviour :cowboy_websocket

  @impl :cowboy_websocket
  def init(request, options) do
    {mode, number} =
      Agent.get_and_update(options.store, fn state ->
        {{state.mode, state.connections + 1}, %{state | connections: state.connections + 1}}
      end)

    send(options.owner, {:node_opening, options.label, self(), number})

    if mode == :hold do
      receive do
        :accept -> :ok
      after
        5_000 -> exit(:handshake_not_released)
      end
    end

    state =
      Map.merge(options, %{id: "0x" <> Integer.to_string(number, 16), subscriptions: MapSet.new(), opening_pid: self()})

    {:cowboy_websocket, request, state}
  end

  @impl :cowboy_websocket
  def websocket_init(state) do
    send(state.owner, {:node_connected, state.label, state.opening_pid, self()})
    {:ok, state}
  end

  @impl :cowboy_websocket
  def websocket_handle({:text, text}, state) do
    %{"jsonrpc" => "2.0", "id" => id} = request = Jason.decode!(text)
    Agent.update(state.store, fn transcript -> %{transcript | requests: transcript.requests ++ [request]} end)
    send(state.owner, {:node_request, state.label, self(), request})

    if Agent.get(state.store, & &1.mode) == :withhold do
      {:ok, state}
    else
      {result, state} = respond(request, state)
      {:reply, {:text, Jason.encode!(%{jsonrpc: "2.0", id: id, result: result})}, state}
    end
  end

  defp respond(%{"method" => "eth_subscribe", "params" => ["newHeads"]}, state) do
    {state.id, %{state | subscriptions: MapSet.put(state.subscriptions, state.id)}}
  end

  defp respond(%{"method" => "eth_unsubscribe", "params" => [id]}, state) do
    {MapSet.member?(state.subscriptions, id), %{state | subscriptions: MapSet.delete(state.subscriptions, id)}}
  end

  defp respond(%{"method" => "eth_getBlockByHash", "params" => [hash, false]}, %{block: %{"hash" => hash}} = state) do
    {state.block, state}
  end

  @impl :cowboy_websocket
  def websocket_info(:close, state), do: {:reply, {:close, 1000, "recorded outage"}, state}

  def websocket_info(:new_head, state) do
    frames =
      Enum.map(state.subscriptions, fn id ->
        {:text,
         Jason.encode!(%{jsonrpc: "2.0", method: "eth_subscription", params: %{subscription: id, result: state.head}})}
      end)

    send(state.owner, {:node_published, state.label, self(), length(frames)})
    {:reply, frames, state}
  end
end

defmodule EthereumJSONRPC.Repair2233431Test do
  use ExUnit.Case, async: false

  import ExUnit.CaptureLog

  alias EthereumJSONRPC.Repair2233431Test.NodeWebSocket
  alias EthereumJSONRPC.WebSocket
  alias EthereumJSONRPC.WebSocket.{RetryWorker, WebSocketClient}
  alias EthereumJSONRPC.WebSocket.Supervisor, as: WebSocketSupervisor
  alias EthereumJSONRPC.WebSocket.WebSocketClient.Options

  @client __MODULE__.Client
  @fixture Path.expand("../support/fixture/paxeer_x/new_head_full.json", __DIR__)

  setup do
    previous = Application.get_env(:ethereum_jsonrpc, RetryWorker)
    Application.put_env(:ethereum_jsonrpc, RetryWorker, retry_interval: 60_000)

    on_exit(fn ->
      if previous,
        do: Application.put_env(:ethereum_jsonrpc, RetryWorker, previous),
        else: Application.delete_env(:ethereum_jsonrpc, RetryWorker)
    end)

    {:ok, fixture: @fixture |> File.read!() |> Jason.decode!()}
  end

  test "ws-crash-recovery preserves public handles after an unexpected client exit", %{fixture: fixture} do
    node = start_node(:primary, fixture)
    named = start_connection(node)
    assert {:ok, subscription} = EthereumJSONRPC.subscribe("newHeads", [], named)
    connection = subscribed(:primary)
    assert_head(connection, :primary, subscription, fixture)
    owner = Process.whereis(RetryWorker)
    snapshot = :sys.get_state(RetryWorker).ws_state
    assert {:error, :unknown_participant} = GenServer.call(RetryWorker, {:checkpoint, self(), snapshot})
    assert :sys.get_state(RetryWorker).client == Process.whereis(@client)

    for _ <- 1..3 do
      old = Process.whereis(@client)
      monitor = Process.monitor(old)
      Process.exit(old, :kill)
      assert_receive {:DOWN, ^monitor, :process, ^old, :killed}
      await(fn -> :sys.get_state(RetryWorker).active? end)
      send(RetryWorker, :retry)
      connection = subscribed(:primary)
      await(fn -> is_pid(Process.whereis(@client)) and Process.whereis(@client) != old end)
      assert Process.whereis(RetryWorker) == owner
      assert_handle_state(subscription)
      assert_head(connection, :primary, subscription, fixture)
    end

    assert :ok = EthereumJSONRPC.unsubscribe(subscription)
    assert_empty_subscription_state()
    old = Process.whereis(@client)
    Process.exit(old, :kill)
    await(fn -> :sys.get_state(RetryWorker).active? end)
    send(RetryWorker, :retry)
    await(fn -> is_pid(Process.whereis(@client)) and Process.whereis(@client) != old end)
    await(fn -> :sys.get_state(@client).connected end)
    assert_empty_subscription_state()
    assert Enum.count(requests(node), &(&1["method"] == "eth_subscribe")) == 4
  end

  test "ws-old-server-ids stay bounded through repeated transport reconnects", %{fixture: fixture} do
    node = start_node(:primary, fixture)
    named = start_connection(node)
    assert {:ok, subscription} = EthereumJSONRPC.subscribe("newHeads", [], named)
    first = subscribed(:primary)
    pid = Process.whereis(@client)

    Enum.reduce(1..5, first, fn _, connection ->
      previous_id = :sys.get_state(pid).subscription_reference_to_subscription_id[subscription.reference]
      mode(node, :hold)
      send(connection, :close)
      assert_receive {:node_opening, :primary, next, _}, 5_000
      await(fn -> not :sys.get_state(pid).connected end)
      state = :sys.get_state(pid)
      assert state.subscription_id_to_subscription_reference == %{}
      assert state.subscription_reference_to_subscription_id == %{}
      assert map_size(state.subscription_reference_to_subscription) == 1
      mode(node, :normal)
      send(next, :accept)
      assert_receive {:node_connected, :primary, ^next, websocket}, 2_000
      assert subscribed(:primary, websocket) == websocket
      assert_handle_state(subscription)
      refute Map.has_key?(:sys.get_state(pid).subscription_id_to_subscription_reference, previous_id)
      assert Process.whereis(@client) == pid
      assert_head(websocket, :primary, subscription, fixture)
      websocket
    end)

    assert :ok = EthereumJSONRPC.unsubscribe(subscription)
    assert_empty_subscription_state()
  end

  test "ws-unbounded-pending expires live callers, removes dead callers and replays only live reads", %{
    fixture: fixture
  } do
    node = start_node(:primary, fixture)
    named = start_connection(node)
    assert {:ok, subscription} = EthereumJSONRPC.subscribe("newHeads", [], named)
    connection = subscribed(:primary)
    mode(node, :withhold)
    read = request("eth_getBlockByHash", [fixture["block"]["hash"], false])

    expired = caller(:expired, fn -> WebSocketClient.json_rpc(@client, read, 150) end)

    expired_subscription =
      caller(:expired_subscription, fn -> WebSocketClient.subscribe(@client, "newHeads", [], 150) end)

    dead = caller(:dead, fn -> WebSocketClient.json_rpc(@client, read, 5_000) end)
    await(fn -> map_size(:sys.get_state(@client).request_id_to_registration) == 3 end)
    registrations = Map.values(:sys.get_state(@client).request_id_to_registration)
    assert Enum.all?(registrations, &(is_integer(&1.deadline) and is_reference(&1.monitor) and is_reference(&1.timer)))
    assert_receive {:caller_result, :expired, {:error, :timeout}}, 2_000
    assert_receive {:caller_result, :expired_subscription, {:error, :timeout}}, 2_000
    assert Process.alive?(expired)
    assert Process.alive?(expired_subscription)
    await(fn -> map_size(:sys.get_state(@client).request_id_to_registration) == 1 end)
    Process.exit(dead, :kill)
    await(fn -> :sys.get_state(@client).request_id_to_registration == %{} end)

    request_count = length(requests(node))
    safe = caller(:safe, fn -> WebSocketClient.json_rpc(@client, read, 5_000) end)
    unsafe_request = request("eth_sendRawTransaction", ["0xdeadbeef"])
    caller(:unsafe, fn -> WebSocketClient.json_rpc(@client, unsafe_request, 5_000) end)
    await(fn -> map_size(:sys.get_state(@client).request_id_to_registration) == 2 end)
    await(fn -> length(requests(node)) == request_count + 2 end)
    original = requests(node)
    mode(node, :hold)
    send(connection, :close)
    assert_receive {:node_opening, :primary, next, _}, 5_000
    assert_receive {:caller_result, :unsafe, {:error, :disconnected}}, 2_000
    assert map_size(:sys.get_state(@client).request_id_to_registration) == 1

    caller(:queued_timeout, fn -> WebSocketClient.json_rpc(@client, read, 100) end)
    queued_dead = caller(:queued_dead, fn -> WebSocketClient.subscribe(@client, "newHeads", [], 5_000) end)
    assert_receive {:caller_result, :queued_timeout, {:error, :timeout}}, 2_000
    Process.exit(queued_dead, :kill)
    mode(node, :normal)
    send(next, :accept)
    assert_receive {:node_connected, :primary, ^next, websocket}, 2_000
    assert subscribed(:primary, websocket) == websocket
    assert_receive {:caller_result, :safe, {:ok, block}}, 2_000
    assert block == fixture["block"]
    assert Process.alive?(safe)
    await(fn -> :sys.get_state(@client).request_id_to_registration == %{} end)
    assert_handle_state(subscription)
    assert_head(websocket, :primary, subscription, fixture)

    later = Enum.drop(requests(node), length(original))
    assert Enum.count(later, &(&1["method"] == "eth_subscribe")) == 1
    assert Enum.count(later, &(&1["method"] == "eth_getBlockByHash")) == 2
    refute Enum.any?(later, &(&1["method"] == "eth_sendRawTransaction"))
    assert :ok = EthereumJSONRPC.unsubscribe(subscription)
  end

  test "ws-termination-envelope delivers tagged errors on terminal exit and failback", %{fixture: fixture} do
    primary = start_node(:primary, fixture)
    fallback = start_node(:fallback, fixture)
    named = start_connection(primary, fallback)
    assert {:ok, subscription} = EthereumJSONRPC.subscribe("newHeads", [], named)
    subscribed(:primary)
    old = Process.whereis(@client)
    stop_node(primary)
    connection = subscribed(:fallback)
    assert_head(connection, :fallback, subscription, fixture)
    reopen_node(primary)
    monitor = Process.monitor(old)
    send(RetryWorker, :retry)
    next = subscribed(:primary)
    assert_receive {:DOWN, ^monitor, :process, ^old, :shutdown}, 5_000
    assert_receive {^subscription, {:error, {:websocket_terminated, :shutdown}}}, 2_000
    assert_handle_state(subscription)
    assert_head(next, :primary, subscription, fixture)
    assert :ok = WebSocketSupervisor.stop_client(Process.whereis(@client))
    assert_receive {^subscription, {:error, {:websocket_terminated, :shutdown}}}, 2_000
    refute_receive {^subscription, :shutdown}
  end

  test "ws-url-logging redacts userinfo, query, fragment and credential paths for primary fallback and retry", %{
    fixture: fixture
  } do
    primary = start_node(:primary, fixture)
    fallback = start_node(:fallback, fixture)
    primary_url = endpoint(primary.port, "primary")
    fallback_url = endpoint(fallback.port, "fallback")

    log =
      capture_log(fn ->
        named = start_connection(primary, fallback, primary_url, fallback_url)
        assert {:ok, subscription} = EthereumJSONRPC.subscribe("newHeads", [], named)
        subscribed(:primary)
        stop_node(primary)
        connection = subscribed(:fallback)
        assert_head(connection, :fallback, subscription, fixture)
        old_connections = Agent.get(primary.store, & &1.connections)
        send(RetryWorker, :retry)
        :sys.get_state(RetryWorker)

        await(fn ->
          state = :sys.get_state(RetryWorker)
          state.probe == nil and state.active?
        end)

        assert Agent.get(primary.store, & &1.connections) == old_connections
        stop_node(fallback)
        await(fn -> :sys.get_state(RetryWorker).client == nil end)
        # A barrier makes all asynchronous retry warnings visible before capture ends.
        assert :sys.get_state(RetryWorker).active?
        Logger.flush()
      end)

    assert log =~ "switching to fallback"
    assert log =~ "is still unavailable"
    assert log =~ "WS fallback endpoint"
    assert log =~ "127.0.0.1:#{primary.port}/[redacted]"
    assert log =~ "127.0.0.1:#{fallback.port}/[redacted]"

    for label <- ["primary", "fallback"],
        secret <- ["user", "password", "pathsecret", "querysecret", "fragmentsecret"] do
      refute log =~ "#{label}_#{secret}"
    end

    assert WebSocketClient.redact_endpoint("invalid_primary_password") == "[invalid endpoint]"
  end

  defp start_node(label, fixture) do
    store = start_supervised!({Agent, fn -> %{mode: :normal, connections: 0, requests: []} end}, id: make_ref())
    options = %{owner: self(), label: label, store: store, head: fixture["new_head"], block: fixture["block"]}
    listener = {__MODULE__, make_ref()}
    dispatch = :cowboy_router.compile([{:_, [{"/[...]", NodeWebSocket, options}]}])
    {:ok, _} = :cowboy.start_clear(listener, [ip: {127, 0, 0, 1}, port: 0], %{env: %{dispatch: dispatch}})
    on_exit(fn -> :cowboy.stop_listener(listener) end)
    %{listener: listener, port: :ranch.get_port(listener), dispatch: dispatch, store: store}
  end

  defp stop_node(node), do: assert(:ok == :cowboy.stop_listener(node.listener))

  defp reopen_node(node) do
    assert {:ok, _} =
             :cowboy.start_clear(node.listener, [ip: {127, 0, 0, 1}, port: node.port], %{
               env: %{dispatch: node.dispatch}
             })
  end

  defp start_connection(primary, fallback \\ nil, primary_url \\ nil, fallback_url \\ nil) do
    options = %WebSocket{
      url: primary_url || "ws://127.0.0.1:#{primary.port}",
      fallback_url: fallback_url || if(fallback, do: "ws://127.0.0.1:#{fallback.port}"),
      web_socket: WebSocketClient,
      web_socket_options: %Options{web_socket: @client}
    }

    start_supervised!({WebSocketSupervisor, options})
    await(fn -> :sys.get_state(@client).connected end)
    assert_receive {:node_opening, :primary, _, _}, 2_000
    [transport: WebSocket, transport_options: options]
  end

  defp subscribed(label) do
    assert_receive {:node_request, ^label, connection, %{"method" => "eth_subscribe", "params" => ["newHeads"]}}, 5_000
    await(fn -> map_size(:sys.get_state(@client).subscription_id_to_subscription_reference) == 1 end)
    connection
  end

  defp subscribed(label, connection) do
    assert_receive {:node_request, ^label, ^connection, %{"method" => "eth_subscribe", "params" => ["newHeads"]}}, 5_000
    await(fn -> map_size(:sys.get_state(@client).subscription_id_to_subscription_reference) == 1 end)
    connection
  end

  defp assert_head(connection, label, subscription, fixture) do
    send(connection, :new_head)
    assert_receive {:node_published, ^label, ^connection, 1}, 2_000
    assert_receive {^subscription, {:ok, head}}, 2_000
    assert head == fixture["new_head"]
    assert {:ok, block} = WebSocketClient.json_rpc(@client, request("eth_getBlockByHash", [head["hash"], false]))
    assert block == fixture["block"]
    assert head["hash"] == block["hash"]
    assert head["number"] == block["number"]
  end

  defp assert_handle_state(subscription) do
    state = :sys.get_state(@client)
    assert state.subscription_reference_to_subscription == %{subscription.reference => subscription}
    assert map_size(state.subscription_reference_to_subscription_id) == 1
    assert map_size(state.subscription_id_to_subscription_reference) == 1
    id = Map.fetch!(state.subscription_reference_to_subscription_id, subscription.reference)
    assert state.subscription_id_to_subscription_reference == %{id => subscription.reference}
  end

  defp assert_empty_subscription_state do
    state = :sys.get_state(@client)
    assert state.subscription_reference_to_subscription == %{}
    assert state.subscription_reference_to_subscription_id == %{}
    assert state.subscription_id_to_subscription_reference == %{}
    assert state.subscriber_monitors == %{}
    assert :sys.get_state(RetryWorker).ws_state.subscription_reference_to_subscription == %{}
  end

  defp request(method, params), do: EthereumJSONRPC.request(%{id: 1, method: method, params: params})
  defp requests(node), do: Agent.get(node.store, & &1.requests)
  defp mode(node, value), do: Agent.update(node.store, &%{&1 | mode: value})

  defp caller(label, function) do
    owner = self()

    pid =
      spawn(fn ->
        result =
          try do
            function.()
          catch
            :exit, {:timeout, _} -> {:error, :timeout}
          end

        send(owner, {:caller_result, label, result})

        receive do
          :stop -> :ok
        end
      end)

    on_exit(fn -> if Process.alive?(pid), do: Process.exit(pid, :kill) end)
    pid
  end

  defp endpoint(port, label) do
    "ws://#{label}_user:#{label}_password@127.0.0.1:#{port}/v2/#{label}_pathsecret?token=#{label}_querysecret##{label}_fragmentsecret"
  end

  defp await(condition, attempts \\ 250)
  defp await(_condition, 0), do: flunk("WebSocket lifecycle did not reach the expected state")

  defp await(condition, attempts) do
    unless condition.() do
      Process.sleep(20)
      await(condition, attempts - 1)
    end
  end
end
