defmodule EthereumJSONRPC.WebSocket.Case.Nethermind do
  @moduledoc """
  Explicitly configured Nethermind compatibility or disposable Paxeer node.

  Configure this module in `:ethereum_jsonrpc` with `:http_url`,
  `:web_socket_url` and `profile: :paxeer_x` for local chain-125 qualification.
  Other chains require `profile: :compatibility` and an explicit `:chain_id`.
  """

  def setup do
    EthereumJSONRPC.WebSocket.Case.Configured.setup(__MODULE__, 5_000)
  end
end
