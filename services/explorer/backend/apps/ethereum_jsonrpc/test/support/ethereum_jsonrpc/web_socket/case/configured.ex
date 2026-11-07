defmodule EthereumJSONRPC.WebSocket.Case.Configured do
  @moduledoc false

  import ExUnit.Callbacks, only: [start_supervised!: 1]

  alias EthereumJSONRPC.{HTTP, WebSocket}
  alias EthereumJSONRPC.WebSocket.Supervisor, as: WebSocketSupervisor
  alias EthereumJSONRPC.WebSocket.WebSocketClient
  alias EthereumJSONRPC.WebSocket.WebSocketClient.Options

  def setup(module, block_interval) do
    config = Application.fetch_env!(:ethereum_jsonrpc, module)
    profile = Keyword.fetch!(config, :profile)
    chain_id = chain_id!(profile, config)
    http_url = Keyword.fetch!(config, :http_url)
    web_socket_url = Keyword.fetch!(config, :web_socket_url)
    validate_endpoints!(profile, http_url, web_socket_url)

    http_arguments = [
      transport: HTTP,
      transport_options: [
        http: HTTP.HTTPoison,
        http_options: [recv_timeout: 5_000, timeout: 5_000, pool: :ethereum_jsonrpc],
        urls: [http_url]
      ]
    ]

    verify_chain!(http_arguments, chain_id)

    transport_options = %WebSocket{
      web_socket: WebSocketClient,
      web_socket_options: %Options{web_socket: Indexer.Block.Realtime.WebSocket},
      url: web_socket_url,
      fallback_url: nil
    }

    start_supervised!({WebSocketSupervisor, transport_options})
    subscribe_arguments = [transport: WebSocket, transport_options: transport_options]
    verify_chain!(subscribe_arguments, chain_id)

    block = rpc!(http_arguments, "eth_getBlockByNumber", ["latest", false])

    unless is_map(block) and is_binary(block["hash"]) and is_binary(block["number"]) do
      raise ArgumentError, "configured HTTP endpoint did not return a block"
    end

    paired_block = rpc!(subscribe_arguments, "eth_getBlockByHash", [block["hash"], false])

    unless paired_block == block do
      raise ArgumentError, "configured HTTP and WebSocket endpoints disagree on the same block"
    end

    %{
      block_interval: block_interval,
      profile: profile,
      chain_id: chain_id,
      json_rpc_named_arguments: http_arguments,
      subscribe_named_arguments: subscribe_arguments
    }
  end

  defp chain_id!(:paxeer_x, config) do
    unless Keyword.get(config, :chain_id, 125) == 125 do
      raise ArgumentError, "Paxeer qualification requires chain id 125"
    end

    125
  end

  defp chain_id!(:compatibility, config) do
    case Keyword.fetch!(config, :chain_id) do
      chain_id when is_integer(chain_id) and chain_id > 0 -> chain_id
      _ -> raise ArgumentError, "compatibility profile requires a positive chain id"
    end
  end

  defp chain_id!(_, _), do: raise(ArgumentError, "explicit :paxeer_x or :compatibility profile required")

  defp validate_endpoints!(profile, http_url, web_socket_url) do
    http = endpoint!(http_url, ["http", "https"])
    web_socket = endpoint!(web_socket_url, ["ws", "wss"])

    unless http.host == web_socket.host do
      raise ArgumentError, "paired endpoints must declare the same node host"
    end

    if profile == :paxeer_x and http.host not in ["localhost", "127.0.0.1", "::1"] do
      raise ArgumentError, "Paxeer qualification requires a disposable loopback node"
    end
  end

  defp endpoint!(url, schemes) when is_binary(url) do
    uri = URI.parse(url)

    unless uri.scheme in schemes and is_binary(uri.host) and uri.host != "" and
             is_nil(uri.userinfo) and is_nil(uri.fragment) do
      raise ArgumentError, "explicit endpoint has an invalid scheme, host, credentials or fragment"
    end

    uri
  end

  defp endpoint!(_, _), do: raise(ArgumentError, "explicit endpoint must be a URL")

  defp verify_chain!(arguments, expected) do
    chain_id = rpc!(arguments, "eth_chainId", [])

    unless is_binary(chain_id) and EthereumJSONRPC.quantity_to_integer(chain_id) == expected do
      raise ArgumentError, "configured endpoint chain id does not match the declared profile"
    end
  end

  defp rpc!(arguments, method, params) do
    request = EthereumJSONRPC.request(%{id: 1, method: method, params: params})
    transport = Keyword.fetch!(arguments, :transport)

    case transport.json_rpc(request, Keyword.fetch!(arguments, :transport_options)) do
      {:ok, result} -> result
      {:error, _} -> raise ArgumentError, "configured endpoint failed #{method}"
    end
  end
end
