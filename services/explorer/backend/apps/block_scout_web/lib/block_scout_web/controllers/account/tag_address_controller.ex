defmodule BlockScoutWeb.Account.TagAddressController do
  use BlockScoutWeb, :controller

  alias Explorer.Account.TagAddress

  import BlockScoutWeb.Account.AuthController, only: [authenticate: 2]

  plug(:authenticate)

  def index(conn, _params) do
    current_user = conn.assigns.current_user

    render(conn, "index.html", address_tags: TagAddress.get_tags_address_by_identity_id(current_user.id))
  end

  def new(conn, _params) do
    render(conn, "form.html", tag_address: new_tag())
  end

  def create(conn, %{"tag_address" => tag_address}) do
    current_user = conn.assigns.current_user

    case TagAddress.create(%{
           name: tag_address["name"],
           address_hash: tag_address["address_hash"],
           identity_id: current_user.id
         }) do
      {:ok, _} ->
        redirect(conn, to: tag_address_path(conn, :index))

      {:error, invalid_tag_address} ->
        render(conn, "form.html", tag_address: invalid_tag_address)
    end
  end

  def create(conn, _) do
    redirect(conn, to: tag_address_path(conn, :index))
  end

  def delete(conn, %{"id" => id}) do
    current_user = conn.assigns.current_user

    TagAddress.delete(id, current_user.id)

    redirect(conn, to: tag_address_path(conn, :index))
  end

  defp new_tag, do: TagAddress.changeset()
end
