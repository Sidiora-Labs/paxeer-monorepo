defmodule EthereumJSONRPC.WebSocket.RetryWorker do
  @moduledoc """
  Owns subscription intent independently of the socket process and retries the primary endpoint.
  Intent survives unexpected socket exits; connection-local IDs and pending calls do not.
  """

  use GenServer

  alias EthereumJSONRPC.WebSocket.Supervisor, as: WebSocketSupervisor

  def start_link(options) do
    GenServer.start_link(__MODULE__, options, name: __MODULE__)
  end

  def checkpoint(ws_state) do
    if Process.whereis(__MODULE__), do: GenServer.call(__MODULE__, {:checkpoint, self(), ws_state})
    ws_state
  end

  def activate(ws_state) do
    GenServer.cast(__MODULE__, {:activate, self(), ws_state})
  end

  def deactivate, do: GenServer.cast(__MODULE__, :deactivate)

  def init(options) do
    schedule_next_retry()
    {:ok, %{active?: false, ws_state: nil, client: nil, monitor: nil, probe: nil, options: Map.new(options)}}
  end

  def handle_call({:checkpoint, pid, ws_state}, _from, state) do
    if participant?(pid, state) do
      if state.monitor, do: Process.demonitor(state.monitor, [:flush])
      if state.probe, do: Process.demonitor(elem(state.probe, 1), [:flush])

      {:reply, :ok,
       %{state | ws_state: snapshot(ws_state, state), client: pid, monitor: Process.monitor(pid), probe: nil}}
    else
      {:reply, {:error, :unknown_participant}, state}
    end
  end

  def handle_cast({:activate, pid, ws_state}, state) do
    if participant?(pid, state) do
      state =
        if state.client in [nil, pid],
          do: %{state | ws_state: snapshot(ws_state, state)},
          else: state

      {:noreply, %{state | active?: true}}
    else
      {:noreply, state}
    end
  end

  def handle_cast(:deactivate, state), do: {:noreply, %{state | active?: false}}

  def handle_info({:DOWN, ref, :process, _pid, _reason}, %{monitor: ref} = state) do
    {:noreply, %{state | active?: true, client: nil, monitor: nil}}
  end

  def handle_info({:DOWN, ref, :process, _pid, _reason}, %{probe: {_probe_pid, ref}} = state) do
    {:noreply, %{state | active?: true, probe: nil}}
  end

  def handle_info(:retry, %{active?: true, ws_state: ws_state, probe: nil} = state) when not is_nil(ws_state) do
    # Drop intent whose consumer died during an outage before opening a new socket.
    subscriptions =
      Map.filter(ws_state.subscription_reference_to_subscription, fn {_, subscription} ->
        Process.alive?(subscription.subscriber_pid)
      end)

    ws_state = %{ws_state | subscription_reference_to_subscription: subscriptions, retry: true}
    module = Map.get(state.options, :web_socket, EthereumJSONRPC.WebSocket.WebSocketClient)

    probe =
      case WebSocketSupervisor.start_client(ws_state, module) do
        {:ok, pid} -> {pid, Process.monitor(pid)}
        _ -> nil
      end

    schedule_next_retry()
    {:noreply, %{state | ws_state: ws_state, probe: probe}}
  end

  def handle_info(:retry, state) do
    schedule_next_retry()
    {:noreply, state}
  end

  defp participant?(pid, %{options: %{name: name}}) do
    pid == Process.whereis(name) or pid == Process.whereis(Indexer.Block.Realtime.WebSocketCopy)
  end

  defp participant?(_pid, _state), do: false

  defp snapshot(ws_state, state) do
    primary = state.ws_state || ws_state

    %{
      primary
      | connected: false,
        retry: true,
        fallback?: false,
        request_id_to_registration: %{},
        subscriber_monitors: %{},
        subscription_id_to_subscription_reference: %{},
        subscription_reference_to_subscription_id: %{},
        subscription_reference_to_subscription: ws_state.subscription_reference_to_subscription
    }
  end

  defp schedule_next_retry do
    interval = Application.get_env(:ethereum_jsonrpc, __MODULE__, [])[:retry_interval] || 1_000
    Process.send_after(self(), :retry, interval)
  end
end
