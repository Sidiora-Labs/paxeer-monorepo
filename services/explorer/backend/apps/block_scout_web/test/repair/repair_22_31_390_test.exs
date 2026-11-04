defmodule BlockScoutWeb.Repair22_31_390Test do
  @moduledoc """
  Repair record for audit 22.30.142, finding 32.

  Proving cases: "finding 32: routed cursor pages preserve order without overlap"
  and "finding 32: previously emitted id cursors advance and become canonical",
  plus "finding 32: search alias and language filter survive routed pagination".
  All use the persisted SimpleStorage bytecode, source and ABI from Explorer.Factory.
  Requests run through the real WebRouter because this profile disables its outer mount.
  Baseline verification was not run: the task gate order requires implementation first.
  Prior attempt: the routed cursor pages case failed because next_page_path lost filter;
  the legacy id cursor case passed. This repair preserves the listing query in every cursor.
  Wiring request: domain 14 owns test/block_scout_web/controllers/verified_contracts_controller_test.exs
  under services/explorer/backend/apps/block_scout_web; its next_page_path expectation must use smart_contract_id.
  """

  use BlockScoutWeb.ConnCase

  @endpoint BlockScoutWeb.Routers.WebRouter

  setup %{conn: conn} do
    conn =
      conn
      |> Plug.Test.init_test_session(%{})
      |> put_private(:phoenix_endpoint, BlockScoutWeb.Endpoint)
      |> Map.put(:secret_key_base, BlockScoutWeb.Endpoint.config(:secret_key_base))

    contracts =
      110
      |> insert_list(:smart_contract, language: :solidity, is_vyper_contract: false)
      |> Enum.sort_by(& &1.id, :desc)

    {:ok, conn: conn, contracts: contracts}
  end

  test "finding 32: routed cursor pages preserve order without overlap", %{conn: conn, contracts: contracts} do
    insert(:smart_contract, language: :vyper, is_vyper_contract: true)
    insert(:smart_contract, name: "OtherContract", language: :solidity, is_vyper_contract: false)

    first =
      conn
      |> get(verified_contracts_path(conn, :index), %{
        "type" => "JSON",
        "filter" => "solidity",
        "search" => "SimpleStorage"
      })
      |> json_response(200)

    assert length(first["items"]) == 50
    assert item_hashes(first) == contract_hashes(Enum.take(contracts, 50))

    first_cursor = first["next_page_path"] |> URI.parse() |> Map.fetch!(:query) |> URI.decode_query()
    assert first_cursor["smart_contract_id"] == to_string(Enum.at(contracts, 49).id)
    assert first_cursor["filter"] == "solidity"
    assert first_cursor["search"] == "SimpleStorage"
    assert first_cursor["items_count"] == "50"
    refute Map.has_key?(first_cursor, "id")
    refute Map.has_key?(first_cursor, "type")

    second = conn |> get(first["next_page_path"], %{"type" => "JSON"}) |> json_response(200)

    assert length(second["items"]) == 50
    assert item_hashes(second) == contract_hashes(Enum.slice(contracts, 50, 50))
    assert MapSet.disjoint?(MapSet.new(item_hashes(first)), MapSet.new(item_hashes(second)))

    second_cursor = second["next_page_path"] |> URI.parse() |> Map.fetch!(:query) |> URI.decode_query()
    assert second_cursor["smart_contract_id"] == to_string(Enum.at(contracts, 99).id)
    assert second_cursor["filter"] == "solidity"
    assert second_cursor["search"] == "SimpleStorage"

    third = conn |> get(second["next_page_path"], %{"type" => "JSON"}) |> json_response(200)

    assert length(third["items"]) == 10
    assert item_hashes(third) == contract_hashes(Enum.drop(contracts, 100))
    assert item_hashes(first) ++ item_hashes(second) ++ item_hashes(third) == contract_hashes(contracts)
    assert is_nil(third["next_page_path"])
  end

  test "finding 32: previously emitted id cursors advance and become canonical", %{conn: conn, contracts: contracts} do
    first =
      conn
      |> get(verified_contracts_path(conn, :index), %{
        "type" => "JSON",
        "id" => to_string(Enum.at(contracts, 9).id)
      })
      |> json_response(200)

    assert length(first["items"]) == 50
    assert item_hashes(first) == contract_hashes(Enum.slice(contracts, 10, 50))

    cursor = first["next_page_path"] |> URI.parse() |> Map.fetch!(:query) |> URI.decode_query()
    assert cursor["smart_contract_id"] == to_string(Enum.at(contracts, 59).id)
    refute Map.has_key?(cursor, "id")

    second = conn |> get(first["next_page_path"], %{"type" => "JSON"}) |> json_response(200)

    assert length(second["items"]) == 50
    assert item_hashes(second) == contract_hashes(Enum.drop(contracts, 60))
    assert MapSet.disjoint?(MapSet.new(item_hashes(first)), MapSet.new(item_hashes(second)))
    assert is_nil(second["next_page_path"])
  end

  test "finding 32: search alias and language filter survive routed pagination", %{conn: conn, contracts: contracts} do
    first =
      conn
      |> get(verified_contracts_path(conn, :index), %{
        "type" => "JSON",
        "filter" => "solidity",
        "q" => "SimpleStorage"
      })
      |> json_response(200)

    assert item_hashes(first) == contract_hashes(Enum.take(contracts, 50))

    cursor = first["next_page_path"] |> URI.parse() |> Map.fetch!(:query) |> URI.decode_query()
    assert cursor["filter"] == "solidity"
    assert cursor["q"] == "SimpleStorage"
    assert cursor["smart_contract_id"] == to_string(Enum.at(contracts, 49).id)

    second = conn |> get(first["next_page_path"], %{"type" => "JSON"}) |> json_response(200)

    assert item_hashes(second) == contract_hashes(Enum.slice(contracts, 50, 50))
    assert MapSet.disjoint?(MapSet.new(item_hashes(first)), MapSet.new(item_hashes(second)))
  end

  defp item_hashes(response) do
    Enum.map(response["items"], fn item ->
      [_, hash] = Regex.run(~r/data-identifier-hash="([^"]+)"/, item)
      String.downcase(hash)
    end)
  end

  defp contract_hashes(contracts) do
    Enum.map(contracts, &String.downcase(to_string(&1.address_hash)))
  end
end
