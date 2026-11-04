defmodule EthereumJSONRPC.Case.Geth.HTTPWebSocket do
  @moduledoc """
  Paired HTTP and WebSocket connections to an explicitly configured test node.

      config :ethereum_jsonrpc, EthereumJSONRPC.Case.Geth.HTTPWebSocket,
        http_url: "http://127.0.0.1:8545",
        websocket_url: "ws://127.0.0.1:8546",
        block_interval: 500

  The default profile qualifies a loopback Paxeer X node (chain 125). Other Geth chains require
  the opt-in `profile: :geth_compatibility` and an explicit `chain_id` in this
  same configuration. Both endpoints must belong to the declared node host;
  neither transport inherits a provider URL from another case module.
  """

  import ExUnit.Callbacks, only: [start_supervised!: 1, stop_supervised: 1]

  alias EthereumJSONRPC.{HTTP, WebSocket}
  alias EthereumJSONRPC.WebSocket.Supervisor, as: WebSocketSupervisor
  alias EthereumJSONRPC.WebSocket.WebSocketClient
  alias EthereumJSONRPC.WebSocket.WebSocketClient.Options

  @client __MODULE__.Client

  def setup do
    config = Application.fetch_env!(:ethereum_jsonrpc, __MODULE__)
    http_url = Keyword.fetch!(config, :http_url)
    websocket_url = Keyword.fetch!(config, :websocket_url)
    {chain_id, variant} = profile!(config)
    validate_pair!(http_url, websocket_url, variant)

    json_rpc_named_arguments = [
      transport: HTTP,
      transport_options: [
        http: HTTP.Tesla,
        http_options: [recv_timeout: 60_000, timeout: 60_000, pool: :ethereum_jsonrpc],
        urls: [http_url]
      ],
      variant: variant
    ]

    request = EthereumJSONRPC.request(%{id: 0, method: "eth_chainId", params: []})
    verify_chain_id!(:http, chain_id, EthereumJSONRPC.json_rpc(request, json_rpc_named_arguments))

    transport_options = %WebSocket{
      web_socket: WebSocketClient,
      web_socket_options: %Options{web_socket: @client},
      url: websocket_url
    }

    start_supervised!({WebSocketSupervisor, transport_options})

    try do
      verify_chain_id!(:websocket, chain_id, WebSocket.json_rpc(request, transport_options))
    rescue
      exception ->
        stop_supervised(WebSocketSupervisor)
        reraise exception, __STACKTRACE__
    end

    %{
      block_interval: Keyword.get(config, :block_interval, 500),
      json_rpc_named_arguments: json_rpc_named_arguments,
      subscribe_named_arguments: [transport: WebSocket, transport_options: transport_options]
    }
  end

  defp profile!(config) do
    case Keyword.get(config, :profile, :paxeer_x) do
      :paxeer_x ->
        {125, EthereumJSONRPC.PaxeerX}

      :geth_compatibility ->
        case Keyword.fetch!(config, :chain_id) do
          chain_id when is_integer(chain_id) and chain_id > 0 -> {chain_id, EthereumJSONRPC.Geth}
          _ -> raise ArgumentError, "geth_compatibility requires a positive chain_id"
        end

      _ ->
        raise ArgumentError, "unsupported HTTPWebSocket test profile"
    end
  end

  defp validate_pair!(http_url, websocket_url, variant) when is_binary(http_url) and is_binary(websocket_url) do
    http = URI.parse(http_url)
    websocket = URI.parse(websocket_url)

    unless {http.scheme, websocket.scheme} in [{"http", "ws"}, {"https", "wss"}] and
             is_binary(http.host) and http.host != "" and http.host == websocket.host and
             is_nil(http.userinfo) and is_nil(websocket.userinfo) and
             is_nil(http.fragment) and is_nil(websocket.fragment) and
             (variant != EthereumJSONRPC.PaxeerX or http.host in ["localhost", "127.0.0.1", "::1"]) do
      raise ArgumentError, "configure paired HTTP and WebSocket URLs for the same test node host"
    end
  end

  defp validate_pair!(_, _, _) do
    raise ArgumentError, "configure paired HTTP and WebSocket URLs for the same test node host"
  end

  defp verify_chain_id!(transport, expected, {:ok, quantity}) when is_binary(quantity) do
    unless EthereumJSONRPC.quantity_to_integer(quantity) == expected do
      raise ArgumentError, "#{transport} test endpoint must report chain id #{expected}"
    end
  end

  defp verify_chain_id!(transport, _expected, _response) do
    raise ArgumentError, "#{transport} test endpoint failed eth_chainId"
  end
end
