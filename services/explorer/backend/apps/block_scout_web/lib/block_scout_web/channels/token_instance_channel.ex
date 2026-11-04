defmodule BlockScoutWeb.TokenInstanceChannel do
  @moduledoc """
  Establishes pub/sub channel for live updates of token instances events.
  """
  use BlockScoutWeb, :channel

  alias BlockScoutWeb.Endpoint
  alias Phoenix.Socket.Broadcast

  def join("token_instances_old:" <> token_contract_address_hash, _params, socket) do
    :ok = Endpoint.subscribe("token_instances:" <> token_contract_address_hash)
    {:ok, %{}, socket}
  end

  def join("token_instances:" <> _token_contract_address_hash, _params, socket) do
    {:ok, %{}, socket}
  end

  def handle_info(
        %Broadcast{topic: "token_instances:" <> token_contract_address_hash, event: event, payload: payload},
        %Phoenix.Socket{topic: "token_instances_old:" <> token_contract_address_hash} = socket
      ) do
    push(socket, event, payload)
    {:noreply, socket}
  end
end
