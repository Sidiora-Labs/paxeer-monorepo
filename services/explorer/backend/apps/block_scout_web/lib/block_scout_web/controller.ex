defmodule BlockScoutWeb.Controller do
  @moduledoc """
  Common controller error responses
  """

  import Phoenix.Controller
  import Plug.Conn

  @doc """
  Renders HTML Not Found error
  """
  def not_found(conn) do
    conn
    |> put_status(:not_found)
    |> put_view(BlockScoutWeb.PageNotFoundView)
    |> render(:index, token: nil)
    |> halt()
  end

  def unprocessable_entity(conn) do
    conn
    |> put_status(:unprocessable_entity)
    |> put_view(BlockScoutWeb.Error422View)
    |> render(:index)
    |> halt()
  end

  @doc """
  Checks if the request is AJAX or not.
  """
  def ajax?(conn) do
    case get_req_header(conn, "x-requested-with") do
      [value] -> String.downcase(value) == "xmlhttprequest"
      _ -> false
    end
  end

  def current_full_path(conn) do
    current_path = current_path(conn)

    full_path(current_path)
  end

  def full_path(path) do
    url_params = Application.get_env(:block_scout_web, BlockScoutWeb.Endpoint)[:url]
    network_path = url_params[:path]

    if network_path do
      network_path = "/" <> String.trim(network_path, "/")
      path = "/" <> String.trim_leading(path, "/")
      uri_path = URI.parse(path).path || "/"

      if network_path == "/" or uri_path == network_path or String.starts_with?(uri_path, network_path <> "/") do
        path
      else
        network_path <> path
      end
    else
      path
    end
  end
end
