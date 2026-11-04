defmodule BlockScoutWeb.Admin.TaskController do
  use BlockScoutWeb, :controller

  require Logger

  alias Explorer.Chain.ContractMethod

  @ok_resp %{status: "success"}
  @not_ok_resp %{status: "failure"}

  def create_contract_methods(conn, _) do
    contract_methods_response(conn, ContractMethod.import_all())
  end

  @doc false
  def contract_methods_response(conn, result) do
    {status, response} =
      case result do
        :ok ->
          {200, @ok_resp}

        {:error, error} ->
          Logger.error(fn -> ["Something went wrong while creating contract methods: ", inspect(error)] end)

          {500, @not_ok_resp}
      end

    conn
    |> put_status(status)
    |> json(response)
  end
end
