defmodule EthereumJSONRPC.WebSocket.Supervisor do
  @moduledoc """
  Supervises the processes related to `EthereumJSONRPC.WebSocket`.
  """

  use Supervisor

  alias EthereumJSONRPC.WebSocket.RetryWorker

  def start_link(transport_options) do
    Supervisor.start_link(__MODULE__, transport_options, name: __MODULE__)
  end

  def start_client(ws_state, web_socket_module \\ EthereumJSONRPC.WebSocket.WebSocketClient) do
    name =
      if Process.whereis(ws_state.web_socket),
        do: Indexer.Block.Realtime.WebSocketCopy,
        else: ws_state.web_socket

    Supervisor.start_child(__MODULE__, client_spec(web_socket_module, name, ws_state.url, nil, ws_state))
  end

  def stop_other_client(pid, name \\ Indexer.Block.Realtime.WebSocket) do
    __MODULE__
    |> Supervisor.which_children()
    |> Enum.reject(fn {child_id, child_pid, _type, _modules} -> child_pid == pid or child_id == RetryWorker end)
    |> Enum.each(fn {child_id, child_pid, _type, _modules} ->
      if is_pid(child_pid), do: stop_client(child_pid)
      Supervisor.delete_child(__MODULE__, child_id)
    end)

    if Process.whereis(Indexer.Block.Realtime.WebSocketCopy) == pid do
      Process.unregister(Indexer.Block.Realtime.WebSocketCopy)
      Process.register(pid, name)
    end
  end

  @doc "Stops a socket through its system callback so subscribers receive the termination envelope."
  def stop_client(pid) when is_pid(pid), do: :sys.terminate(pid, :shutdown)

  def init(%{
        url: url,
        fallback_url: fallback_url,
        web_socket: web_socket_module,
        web_socket_options: %{web_socket: web_socket}
      }) do
    children = [
      {RetryWorker, %{web_socket: web_socket_module, name: web_socket}},
      client_spec(web_socket_module, web_socket, url, fallback_url)
    ]

    Supervisor.init(children, strategy: :one_for_one)
  end

  # Keep endpoint credentials out of supervisor child-failure reports, which print the start MFA.
  def start_client_link(start) when is_function(start, 0), do: start.()

  defp client_spec(web_socket_module, name, url, fallback_url, init_state \\ nil) do
    %{
      id: name,
      start:
        {__MODULE__, :start_client_link,
         [fn -> web_socket_module.start_link(url, name: name, fallback_url: fallback_url, init_state: init_state) end]},
      restart: :temporary
    }
  end
end
