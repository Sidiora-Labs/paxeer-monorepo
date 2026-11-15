defmodule BlockScoutWeb.AbiImportResponseTest do
  @moduledoc "Abi import response."

  use BlockScoutWeb.ConnCase

  import Ecto.Query, only: [from: 2]

  alias BlockScoutWeb.Admin.TaskController
  alias Ecto.Adapters.SQL.Sandbox
  alias Explorer.Chain.{Address, ContractMethod}
  alias Explorer.Repo

  @endpoint BlockScoutWeb.Routers.AdminRouter

  setup %{conn: conn} = tags do
    if tags[:unboxed_import] do
      :ok = Sandbox.checkin(Repo)
      :ok = Sandbox.mode(Repo, :auto)
      on_exit(fn -> Sandbox.mode(Repo, :manual) end)
    end

    admin = insert(:administrator)

    if tags[:unboxed_import] do
      on_exit(fn ->
        Repo.delete!(admin)
        Repo.delete!(admin.user)
      end)
    end

    conn =
      conn
      |> Plug.Test.init_test_session(%{user_id: admin.user.id})
      |> put_private(:phoenix_endpoint, BlockScoutWeb.Endpoint)
      |> Map.put(:secret_key_base, BlockScoutWeb.Endpoint.config(:secret_key_base))

    {:ok, conn: conn}
  end

  @tag :unboxed_import
  test "finding 13: routed import returns a JSON success object and persists ABI methods", %{
    conn: conn
  } do
    contract = insert(:smart_contract, language: :solidity)

    on_exit(fn ->
      Enum.each(contract.abi, fn abi ->
        Repo.delete_all(from(method in ContractMethod, where: method.abi == ^abi))
      end)

      Repo.delete!(contract)
      Repo.delete_all(from(address in Address, where: address.hash == ^contract.address_hash))
    end)

    assert Repo.all(ContractMethod) == []

    response = get(conn, AdminRoutes.create_contract_methods_path(conn, :create_contract_methods))

    assert get_resp_header(response, "content-type") == ["application/json; charset=utf-8"]
    body = Phoenix.ConnTest.json_response(response, 200)
    assert is_map(body)
    assert body == %{"status" => "success"}

    methods = Repo.all(ContractMethod)
    expected_abi = Enum.reject(contract.abi, &(&1["type"] in ["fallback", "constructor"]))
    assert expected_abi != []
    assert Enum.sort(Enum.map(methods, & &1.abi)) == Enum.sort(expected_abi)

    repeated = get(conn, AdminRoutes.create_contract_methods_path(conn, :create_contract_methods))
    assert Phoenix.ConnTest.json_response(repeated, 200) == %{"status" => "success"}
    assert get_resp_header(repeated, "content-type") == ["application/json; charset=utf-8"]
    assert Repo.aggregate(ContractMethod, :count) == length(methods)
  end

  test "finding 13: transaction rollback returns a JSON failure object", %{conn: conn} do
    result = Repo.transaction(fn -> Repo.rollback(:contract_method_import_failed) end)
    assert result == {:error, :contract_method_import_failed}

    log =
      ExUnit.CaptureLog.capture_log(fn ->
        response = TaskController.contract_methods_response(conn, result)

        assert get_resp_header(response, "content-type") == ["application/json; charset=utf-8"]
        body = Phoenix.ConnTest.json_response(response, 500)
        assert is_map(body)
        assert body == %{"status" => "failure"}
        refute Map.has_key?(body, "error")
      end)

    assert log =~
             "Something went wrong while creating contract methods: :contract_method_import_failed"

    assert Repo.all(ContractMethod) == []
  end
end
