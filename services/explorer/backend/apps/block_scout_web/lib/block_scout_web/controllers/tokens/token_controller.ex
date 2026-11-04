defmodule BlockScoutWeb.Tokens.TokenController do
  use BlockScoutWeb, :controller

  require Logger

  alias BlockScoutWeb.AccessHelper
  alias Explorer.Chain
  alias Explorer.Chain.Token

  def show(conn, %{"id" => address_hash_string}) do
    redirect(conn, to: AccessHelper.get_path(conn, :token_transfer_path, :index, address_hash_string))
  end

  def token_counters(conn, %{"id" => address_hash_string} = params) do
    with {:ok, address_hash} <- Chain.string_to_address_hash(address_hash_string),
         {:ok, false} <- AccessHelper.restricted_access?(address_hash_string, params) do
      {transfers_count, holders_count} = Token.fetch_token_counters(address_hash, 30_000)

      json(conn, %{transfer_count: transfers_count, token_holder_count: holders_count})
    else
      :error ->
        not_found(conn)

      {:restricted_access, true} ->
        not_found(conn)
    end
  end
end
