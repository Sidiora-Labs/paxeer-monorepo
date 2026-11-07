defmodule BlockScoutWeb.Account.WatchlistController do
  use BlockScoutWeb, :controller

  import BlockScoutWeb.Account.AuthController, only: [authenticate: 2]
  import Ecto.Query, only: [from: 2]

  alias Explorer.Account.{Watchlist, WatchlistAddress}
  alias Explorer.Repo

  plug(:authenticate)

  def show(conn, _params) do
    current_user = conn.assigns.current_user

    render(
      conn,
      "show.html",
      watchlist: watchlist_with_addresses(current_user)
    )
  end

  defp watchlist_with_addresses(user) do
    Watchlist
    |> Repo.account_repo().get(user.watchlist_id)
    |> Repo.account_repo().preload(watchlist_addresses: from(wa in WatchlistAddress, order_by: [desc: wa.id]))
    |> WatchlistAddress.preload_address_fetched_coin_balance()
  end
end
