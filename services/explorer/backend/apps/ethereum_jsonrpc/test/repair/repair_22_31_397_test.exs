defmodule EthereumJSONRPC.Repair2231397Test.NodeWebSocket do
  @moduledoc false
  @behaviour :cowboy_websocket

  @impl :cowboy_websocket
  def init(request, options) do
    {:cowboy_websocket, request, Map.put(options, :subscriptions, MapSet.new())}
  end

  @impl :cowboy_websocket
  def websocket_handle({:text, text}, state) do
    %{"jsonrpc" => "2.0", "id" => id} = request = Jason.decode!(text)
    send(state.owner, {:node_request, state.phase, self(), request})
    {result, state} = respond(request, state)
    {:reply, {:text, Jason.encode!(%{jsonrpc: "2.0", id: id, result: result})}, state}
  end

  defp respond(%{"method" => "eth_subscribe", "params" => ["newHeads"]}, state) do
    {state.subscription_id, %{state | subscriptions: MapSet.put(state.subscriptions, state.subscription_id)}}
  end

  defp respond(%{"method" => "eth_unsubscribe", "params" => [subscription_id]}, state) do
    {MapSet.member?(state.subscriptions, subscription_id),
     %{state | subscriptions: MapSet.delete(state.subscriptions, subscription_id)}}
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

    send(state.owner, {:node_published, state.phase, length(frames)})
    {:reply, frames, state}
  end
end

defmodule EthereumJSONRPC.Repair2231397Test do
  use ExUnit.Case, async: false

  alias EthereumJSONRPC.Repair2231397Test.NodeWebSocket
  alias EthereumJSONRPC.WebSocket
  alias EthereumJSONRPC.WebSocket.{RetryWorker, WebSocketClient}
  alias EthereumJSONRPC.WebSocket.Supervisor, as: WebSocketSupervisor
  alias EthereumJSONRPC.WebSocket.WebSocketClient.Options

  @client Indexer.Block.Realtime.WebSocket
  @primary_listener __MODULE__.Primary
  @fallback_listener __MODULE__.Fallback
  @fixture Path.expand("../support/fixture/paxeer_x/new_head_full.json", __DIR__)
  @primary_id "0xa6f9c33d92741a00bdc4ff02dd328147"
  @fallback_id "0xb7fa044e03852b11ced50013ee439258"
  @recovered_id "0xc80b155f14963c22dfe61124ff54a369"

  @tag timeout: 60_000
  test "original subscription remains cancellable through primary outage, fallback and primary recovery" do
    %{"new_head" => head, "block" => block} = @fixture |> File.read!() |> Jason.decode!()
    fixture = %{owner: self(), head: head, block: block}
    primary_port = start_node(@primary_listener, :primary, @primary_id, fixture)
    fallback_port = start_node(@fallback_listener, :fallback, @fallback_id, fixture)

    previous_retry = Application.get_env(:ethereum_jsonrpc, RetryWorker)
    previous_subscribe = Application.get_env(:indexer, :subscribe_named_arguments)
    previous_overrides = Application.get_env(:indexer, :realtime_overrides)

    on_exit(fn ->
      restore_env(:ethereum_jsonrpc, RetryWorker, previous_retry)
      restore_env(:indexer, :subscribe_named_arguments, previous_subscribe)
      restore_env(:indexer, :realtime_overrides, previous_overrides)
    end)

    Application.put_env(:ethereum_jsonrpc, RetryWorker, retry_interval: 60_000)
    Application.put_env(:indexer, :realtime_overrides, [])

    transport_options = %WebSocket{
      url: "ws://127.0.0.1:#{primary_port}",
      fallback_url: "ws://127.0.0.1:#{fallback_port}",
      web_socket: WebSocketClient,
      web_socket_options: %Options{web_socket: @client}
    }

    named_arguments = [transport: WebSocket, transport_options: transport_options]

    Application.put_env(:indexer, :subscribe_named_arguments,
      transport: WebSocket,
      transport_options: transport_options |> Map.from_struct() |> Map.to_list()
    )

    start_supervised!({WebSocketSupervisor, transport_options})

    initial_pid = Process.whereis(@client)
    await(fn -> :sys.get_state(initial_pid).connected end)
    assert {:ok, subscription} = EthereumJSONRPC.subscribe("newHeads", [], named_arguments)
    assert subscription.transport_options.web_socket_options.web_socket == @client
    primary_connection = assert_subscribe_request(:primary)
    assert subscription_id(initial_pid, subscription) == @primary_id
    assert_node_head(subscription, initial_pid, primary_connection, :primary, head)

    assert :ok = :cowboy.stop_listener(@primary_listener)
    fallback_connection = assert_subscribe_request(:fallback)

    await(fn ->
      state = :sys.get_state(initial_pid)

      state.connected and state.fallback? and
        state.subscription_reference_to_subscription_id[subscription.reference] == @fallback_id
    end)

    assert_node_head(subscription, initial_pid, fallback_connection, :fallback, head)
    assert start_node(@primary_listener, :recovered, @recovered_id, fixture, primary_port) == primary_port
    monitor = Process.monitor(initial_pid)
    send(RetryWorker, :retry)
    assert_receive {:DOWN, ^monitor, :process, ^initial_pid, :shutdown}, 15_000

    recovered_connection = assert_subscribe_request(:recovered)

    await(fn ->
      pid = Process.whereis(@client)
      is_pid(pid) and pid != initial_pid
    end)

    recovered_pid = Process.whereis(@client)
    assert is_pid(recovered_pid)
    refute recovered_pid == initial_pid
    refute Process.alive?(initial_pid)

    await(fn ->
      state = :sys.get_state(recovered_pid)

      state.connected and not state.fallback? and
        state.subscription_reference_to_subscription_id[subscription.reference] == @recovered_id
    end)

    assert_node_head(subscription, recovered_pid, recovered_connection, :recovered, head)

    assert :ok = EthereumJSONRPC.unsubscribe(subscription)

    assert_receive {:node_request, :recovered, ^recovered_connection,
                    %{"jsonrpc" => "2.0", "method" => "eth_unsubscribe", "params" => [@recovered_id]}},
                   1_000

    state = :sys.get_state(recovered_pid)
    refute Map.has_key?(state.subscription_reference_to_subscription, subscription.reference)
    refute Map.has_key?(state.subscription_reference_to_subscription_id, subscription.reference)
    refute Map.has_key?(state.subscription_id_to_subscription_reference, @recovered_id)

    # The server owning the replacement ID confirms it no longer has the subscription.
    assert {:ok, false} = rpc(recovered_pid, "eth_unsubscribe", [@recovered_id])
    assert {:error, :not_found} = EthereumJSONRPC.unsubscribe(subscription)
    send(recovered_connection, :new_head)
    assert_receive {:node_published, :recovered, 0}
    refute_receive {^subscription, {:ok, _}}, 1_500
    refute_receive {:node_request, :fallback, _, %{"method" => "eth_unsubscribe"}}
  end

  defp start_node(listener, phase, subscription_id, fixture, port \\ 0) do
    options = Map.merge(fixture, %{phase: phase, subscription_id: subscription_id})
    dispatch = :cowboy_router.compile([{:_, [{"/", NodeWebSocket, options}]}])

    {:ok, _} =
      :cowboy.start_clear(listener, [ip: {127, 0, 0, 1}, port: port], %{env: %{dispatch: dispatch}})

    on_exit(fn -> :cowboy.stop_listener(listener) end)
    :ranch.get_port(listener)
  end

  defp assert_subscribe_request(phase) do
    assert_receive {:node_request, ^phase, connection,
                    %{"jsonrpc" => "2.0", "method" => "eth_subscribe", "params" => ["newHeads"]}},
                   15_000

    connection
  end

  defp subscription_id(pid, subscription) do
    Map.fetch!(:sys.get_state(pid).subscription_reference_to_subscription_id, subscription.reference)
  end

  defp assert_node_head(subscription, pid, connection, phase, expected_head) do
    send(connection, :new_head)
    assert_receive {:node_published, ^phase, 1}
    assert_receive {^subscription, {:ok, %{"hash" => hash} = head}}, 10_000
    assert head == expected_head
    assert {:ok, block} = rpc(pid, "eth_getBlockByHash", [hash, false])
    assert is_map(block)

    for field <- ~w(hash number parentHash stateRoot receiptsRoot transactionsRoot) do
      assert Map.fetch!(head, field) == Map.fetch!(block, field)
    end
  end

  defp rpc(pid, method, params) do
    WebSocketClient.json_rpc(pid, EthereumJSONRPC.request(%{id: 1, method: method, params: params}))
  end

  defp await(condition, attempts \\ 300)
  defp await(_condition, 0), do: flunk("WebSocket/node state did not become ready")

  defp await(condition, attempts) do
    unless condition.() do
      Process.sleep(50)
      await(condition, attempts - 1)
    end
  end

  defp restore_env(app, key, nil), do: Application.delete_env(app, key)
  defp restore_env(app, key, value), do: Application.put_env(app, key, value)
end
