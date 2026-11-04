defmodule Explorer.ThirdPartyIntegrations.UniversalProxy.TargetClient do
  @moduledoc """
  A WebSocket client that connects to a target WebSocket server and forwards messages to/from a
  parent process.
  """
  use WebSockex

  @doc """
  Starts a WebSocket client that connects to the target URL and forwards messages to the parent process.

  ## Parameters
  - `parent_pid`: The PID of the parent process that will receive forwarded messages.
  - `url`: The WebSocket URL to connect to.

  ## Returns
  - `{:ok, pid}` on successful connection
  - `{:error, reason}` if the connection fails
  """
  @spec start_link(parent_pid :: pid(), url :: String.t()) :: {:ok, pid()} | {:error, term()}
  def start_link(parent_pid, url) do
    WebSockex.start_link(url, __MODULE__, %{
      parent: parent_pid
    })
  end

  ## Callbacks

  @impl true
  def handle_frame({:text, msg}, %{parent: parent} = state) do
    send(parent, {:from_target, msg, :type, :text})
    {:ok, state}
  end

  @impl true
  def handle_frame({:binary, msg}, %{parent: parent} = state) do
    send(parent, {:from_target, msg, :type, :binary})
    {:ok, state}
  end

  @doc """
  Forwards a text or binary frame to the target WebSocket server.

  ## Parameters
  - `pid`: The PID of the TargetClient process.
  - `frame`: The frame to send, preserving its type and payload.

  ## Returns
  - `:ok` on success
  - `{:error, reason}` if the message cannot be sent
  """
  @spec forward(pid :: pid(), frame :: {:text | :binary, binary()}) :: :ok | {:error, term()}
  def forward(pid, {type, _msg} = frame) when type in [:text, :binary] do
    WebSockex.send_frame(pid, frame)
  end
end
