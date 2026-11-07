defmodule EthereumJSONRPC.Case.Nethermind.HTTPWebSocket do
  @moduledoc """
  Paired HTTP and WebSocket integration endpoints supplied through
  `config :ethereum_jsonrpc, #{__MODULE__}, http_url: ..., websocket_url: ...`.

  The default `:paxeer_local` profile requires a disposable loopback node on
  chain 125. Other chains require `profile: :nethermind_compatibility` and an
  explicit `chain_id`; no compatibility endpoint is enabled by default.
  Both transports must report the expected chain before setup returns.
  """

  import ExUnit.Callbacks, only: [start_supervised!: 1, stop_supervised: 1]

  alias EthereumJSONRPC.WebSocket
  alias EthereumJSONRPC.WebSocket.Supervisor, as: WebSocketSupervisor
  alias EthereumJSONRPC.WebSocket.WebSocketClient
  alias EthereumJSONRPC.WebSocket.WebSocketClient.Options

  def setup do
    :ethereum_jsonrpc
    |> Application.fetch_env!(__MODULE__)
    |> setup()
  end

  def setup(config) when is_list(config) do
    http_url = Keyword.fetch!(config, :http_url)
    websocket_url = Keyword.fetch!(config, :websocket_url)
    {variant, chain_id} = profile!(config)
    validate_endpoints!(http_url, websocket_url, variant)

    json_rpc_named_arguments = [
      transport: EthereumJSONRPC.HTTP,
      transport_options: [
        http: EthereumJSONRPC.HTTP.Tesla,
        http_options: [recv_timeout: 60_000, timeout: 60_000, pool: :ethereum_jsonrpc],
        urls: [http_url]
      ],
      variant: variant
    ]

    verify_chain!(EthereumJSONRPC.fetch_chain_id(json_rpc_named_arguments), chain_id, :http)

    transport_options = %WebSocket{
      url: websocket_url,
      web_socket: WebSocketClient,
      web_socket_options: %Options{web_socket: __MODULE__.Client}
    }

    start_supervised!({WebSocketSupervisor, transport_options})

    try do
      request = EthereumJSONRPC.request(%{id: 0, method: "eth_chainId", params: []})

      result =
        case WebSocket.json_rpc(request, transport_options) do
          {:ok, quantity} -> {:ok, EthereumJSONRPC.quantity_to_integer(quantity)}
          error -> error
        end

      verify_chain!(result, chain_id, :websocket)
    rescue
      error ->
        stop_supervised(WebSocketSupervisor)
        reraise error, __STACKTRACE__
    end

    %{
      block_interval: 5_000,
      json_rpc_named_arguments: json_rpc_named_arguments,
      subscribe_named_arguments: [transport: WebSocket, transport_options: transport_options]
    }
  end

  defp profile!(config) do
    case Keyword.get(config, :profile, :paxeer_local) do
      :paxeer_local ->
        {EthereumJSONRPC.PaxeerX, 125}

      :nethermind_compatibility ->
        case Keyword.fetch!(config, :chain_id) do
          chain_id when is_integer(chain_id) and chain_id > 0 -> {EthereumJSONRPC.Nethermind, chain_id}
          _ -> raise ArgumentError, "compatibility profile requires a positive chain_id"
        end

      _ ->
        raise ArgumentError, "unknown integration profile"
    end
  end

  defp validate_endpoints!(http_url, websocket_url, variant) do
    http = endpoint!(http_url, ["http", "https"])
    websocket = endpoint!(websocket_url, ["ws", "wss"])

    unless http.host == websocket.host and
             {http.scheme, websocket.scheme} in [{"http", "ws"}, {"https", "wss"}] do
      raise ArgumentError, "HTTP and WebSocket must be a pair on the same declared node"
    end

    if variant == EthereumJSONRPC.PaxeerX and http.host not in ["localhost", "127.0.0.1", "::1"] do
      raise ArgumentError, "paxeer_local requires a disposable loopback node"
    end
  end

  defp endpoint!(url, schemes) when is_binary(url) do
    uri = URI.parse(url)

    unless uri.scheme in schemes and is_binary(uri.host) and uri.host != "" and
             is_nil(uri.userinfo) and is_nil(uri.fragment) do
      raise ArgumentError, "integration endpoint must be an absolute credential-free HTTP or WebSocket URL"
    end

    uri
  end

  defp endpoint!(_url, _schemes), do: raise(ArgumentError, "integration endpoint must be a URL")

  defp verify_chain!({:ok, chain_id}, chain_id, _transport), do: :ok

  defp verify_chain!(_result, expected, transport) do
    raise ArgumentError, "#{transport} endpoint did not identify expected chain #{expected}"
  end
end
