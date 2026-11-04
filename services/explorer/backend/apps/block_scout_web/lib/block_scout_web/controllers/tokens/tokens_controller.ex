defmodule BlockScoutWeb.TokensController do
  use BlockScoutWeb, :controller

  import BlockScoutWeb.Chain, only: [paging_options: 1, next_page_params: 3, split_list_by_page: 1]
  alias BlockScoutWeb.{Controller, TokensView}
  alias Explorer.Chain.Token
  alias Explorer.Helper
  alias Phoenix.View

  def index(conn, %{"type" => "JSON"} = params) do
    with {:ok, items_count} <- parse_items_count(Map.get(params, "items_count", "0")) do
      filter =
        if Map.has_key?(params, "filter") do
          Map.get(params, "filter")
        else
          nil
        end

      paging_params =
        params
        |> paging_options()

      tokens = Token.list_top(filter, paging_params)

      {tokens_page, next_page} = split_list_by_page(tokens)

      next_page_path =
        case next_page_params(next_page, tokens_page, params) do
          nil ->
            nil

          next_page_params ->
            tokens_path(
              conn,
              :index,
              Map.delete(next_page_params, "type")
            )
        end

      items =
        tokens_page
        |> Enum.with_index(1)
        |> Enum.map(fn {token, index} ->
          View.render_to_string(
            TokensView,
            "_tile.html",
            token: token,
            index: items_count + index,
            conn: conn
          )
        end)

      json(
        conn,
        %{
          items: items,
          next_page_path: next_page_path
        }
      )
    else
      {:error, _} -> unprocessable_entity(conn)
    end
  end

  def index(conn, _params) do
    render(conn, "index.html", current_path: Controller.current_full_path(conn))
  end

  defp parse_items_count(value) when is_binary(value), do: Helper.safe_parse_non_negative_integer(value)
  defp parse_items_count(_value), do: {:error, :invalid_integer}
end
