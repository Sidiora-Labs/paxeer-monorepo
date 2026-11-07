defmodule EthereumJSONRPC.WebSocket.Registration do
  @moduledoc """
  When a caller registers for responses to asynchronous frame responses.
  """

  alias EthereumJSONRPC.{Subscription, Transport}

  @enforce_keys ~w(from request type)a
  defstruct from: nil,
            request: nil,
            type: nil,
            deadline: nil,
            monitor: nil,
            timer: nil,
            subscription_reference: nil

  @type deadline :: integer() | nil

  @doc "The explicit read-only JSON-RPC methods allowed to cross a reconnect."
  def replayable?(%__MODULE__{type: :subscribe}), do: true

  def replayable?(%__MODULE__{type: :json_rpc, request: %{method: method}}) do
    method in ~w(eth_blockNumber eth_chainId eth_call eth_estimateGas eth_gasPrice eth_getBalance
      eth_getBlockByHash eth_getBlockByNumber eth_getBlockTransactionCountByHash
      eth_getBlockTransactionCountByNumber eth_getCode eth_getLogs eth_getStorageAt
      eth_getTransactionByHash eth_getTransactionByBlockHashAndIndex eth_getTransactionByBlockNumberAndIndex
      eth_getTransactionCount eth_getTransactionReceipt eth_syncing net_version net_listening web3_clientVersion)
  end

  def replayable?(_), do: false

  @typedoc """
  What kind of request will be issued by the caller

    * `:json_rpc` - a generic JSONRPC request that just needs to be returned to the caller based on `id` matching.
    * `:subscribe` - an `eth_subscribe` request will be issued by the caller.  Its response need to be returned to
      caller **AND** the client needs to `EthereumJSONRPC.Subscription.publish/2` any `eth_subscription` messages to
      the caller until the `EthereumJSONRPC.WebSocket.Client.unsubscribe/1` is called.
    * `:unsubscribe` - an `eth_unsubscribe` request will be issued by the caller.  Its response needs to be returned to
      caller **AND** the client needs to stop tracking the subscription.
  """
  @type type :: :json_rpc | :subscribe | :unsubscribe

  @typedoc """
  `"eth_subscribe"`
  """
  @type subscribe :: Transport.method()

  @typedoc """
  The event to `t:subscribe/0` to.
  """
  @type event :: String.t()

  @typedoc """
  Parameters unique to `t:event/0` that customize the `t:subscribe`.
  """
  @type event_param :: term()

  @typedoc """
  Parameters to `t:subscribe/0` `t:EthereumJSONRPC.Transport.request/0`.

  A list that start with the `t:event/0` followed by zero or more `t:event_param/0`.
  """
  @type subscribe_params :: [event | event_param, ...]

  @typedoc """
  `"eth_unsubscribe"`
  """
  @type unsubscribe :: Transport.method()

  @typedoc """
  A list containing the `t:EthereumJSONRPC.Subscription.id/0` that is being unsubscribed.
  """
  @type unsubscribe_params :: [Subscription.id(), ...]

  @typedoc """
   * `from` - used to `GenServer.reply/2` to caller
   * `type` - the `t:type/0` of request
   * `request` - the request sent to the server.  Used to replay the request on disconnect.
  """
  @type t :: %__MODULE__{
          from: GenServer.from(),
          type: type(),
          request: Transport.request(),
          deadline: deadline(),
          monitor: reference() | nil,
          timer: reference() | nil,
          subscription_reference: reference() | nil
        }
end
