defmodule Explorer.Repair.Repair2231409Test do
  @moduledoc """
  Repair record for audit 22.30.155, finding transaction-type-union.
  Proved by the mixed creation/call, token-creation union, and paginated union tests.
  No out-of-path wiring changes are required.
  Attempt 1 at c2c6fd7079f7093bdb5cb5d8e7d4d58e1a87a1eb failed all three union
  tests because the HTTP schema returned 422 before the chain filter ran.
  This continuation uses that recorded baseline; qualification runs after implementation.
  """
  use Explorer.DataCase

  import Phoenix.ConnTest

  alias EthereumJSONRPC.Transaction, as: JSONRPCTransaction
  alias Explorer.Chain.{Address, Transaction}

  @endpoint BlockScoutWeb.Endpoint
  @recordings Path.expand("../support/fixture/vcr_cassettes", __DIR__)

  setup_all do
    {:ok, _} = Application.ensure_all_started(:block_scout_web)
    :ok
  end

  setup do
    [trace_recording] = recording("internal_transaction_importer_import_1_with_contract_creation.json")
    %{"result" => %{"trace" => [trace]}} = Jason.decode!(trace_recording["response"]["body"])
    code = trace["result"]["code"]

    creation = persist_recording("transaction_importer_import_saves_the_transaction.json", code)
    token_creation = persist_recording("transaction_importer_creates_a_to_address.json", code)
    token_address = Repo.get!(Address, token_creation.created_contract_address_hash)
    insert(:token, contract_address: token_address)

    call = persist_recording("transaction_importer_binds_internal_transactions.json", code)
    sender = Repo.get!(Address, call.from_address_hash)
    coin_transfer = persist_recording("transaction_importer_saves_the_association.json", nil, sender)
    insert(:transaction, value: 0, input: "0x") |> with_block()
    insert(:transaction, to_address: nil, value: 0)

    assert creation.to_address_hash == nil
    assert token_creation.to_address_hash == nil
    assert Repo.get!(Address, call.to_address_hash).contract_code != nil
    assert Repo.get!(Address, coin_transfer.to_address_hash).contract_code == nil

    %{creation: creation, token_creation: token_creation, call: call, coin_transfer: coin_transfer}
  end

  test "transaction-type-union: mixed creation/call filters retain null destinations in either order", context do
    creations = [context.creation, context.token_creation]
    assert_items("contract_creation", creations)
    assert_items("contract_call", [context.call])
    assert_items("coin_transfer", [context.call, context.coin_transfer])
    assert_items("token_creation", [context.token_creation])

    for filter <- [
          "contract_creation,contract_call",
          "contract_call,contract_creation",
          "contract_creation,contract_call,contract_creation"
        ] do
      items = assert_items(filter, creations ++ [context.call])
      creation = Enum.find(items, &(&1["hash"] == to_string(context.creation.hash)))
      assert creation["to"] == nil
      assert creation["created_contract"]["hash"] == Address.checksum(context.creation.created_contract_address_hash)
      assert "contract_creation" in creation["transaction_types"]
    end
  end

  test "transaction-type-union: token creation joins preserve calls and coin transfers", context do
    for filter <- ["contract_call,token_creation", "token_creation,contract_call"] do
      assert_items(filter, [context.call, context.token_creation])
    end

    for filter <- ["coin_transfer,token_creation", "token_creation,coin_transfer"] do
      assert_items(filter, [context.call, context.coin_transfer, context.token_creation])
    end

    assert_items("contract_creation,token_creation", [context.creation, context.token_creation])

    assert_items("contract_creation,contract_call,coin_transfer,token_creation", [
      context.creation,
      context.token_creation,
      context.call,
      context.coin_transfer
    ])
  end

  test "transaction-type-union: paginated unions retain ordering and return each transaction once", context do
    extra_calls =
      for _ <- 1..50 do
        insert(:transaction, to_address: Repo.get!(Address, context.call.to_address_hash), value: 0)
        |> with_block()
      end

    expected = [context.creation, context.token_creation, context.call] ++ extra_calls
    filter = "token_creation,contract_call,contract_creation"
    first = request(%{"type" => filter})
    assert length(first["items"]) == 50
    assert is_map(first["next_page_params"])

    second = request(Map.put(first["next_page_params"], "type", filter))
    assert length(second["items"]) == 3
    assert second["next_page_params"] == nil

    items = first["items"] ++ second["items"]
    assert_hashes(items, expected)

    ordered = Enum.sort_by(expected, &{&1.block_number, &1.index}, :desc)
    assert Enum.map(items, & &1["hash"]) == Enum.map(ordered, &to_string(&1.hash))
  end

  test "transaction-type-union: HTTP validation rejects unsupported types in single and mixed filters" do
    for filter <- ["unsupported", "contract_creation,unsupported", "unsupported,contract_call"] do
      response = build_conn() |> get("/api/v2/transactions", %{"type" => filter}) |> json_response(422)
      assert Enum.any?(response["errors"], &(&1["source"]["pointer"] == "/type"))
    end
  end

  defp persist_recording(name, code, sender \\ :new) do
    [recording] = recording(name)
    %{"result" => transaction} = Jason.decode!(recording["response"]["body"])
    params = transaction |> JSONRPCTransaction.to_elixir() |> JSONRPCTransaction.elixir_to_params()

    case sender do
      :new -> insert(:address, hash: params.from_address_hash)
      %Address{hash: hash} -> assert to_string(hash) == params.from_address_hash
    end

    for hash <- [params.to_address_hash, params[:created_contract_address_hash]], not is_nil(hash) do
      insert(:address, hash: hash, contract_code: code)
    end

    block = insert(:block, hash: params.block_hash, number: params.block_number)

    params =
      Map.merge(params, %{
        cumulative_gas_used: params.gas,
        gas_used: params.gas,
        status: :ok,
        block_consensus: true,
        block_timestamp: block.timestamp
      })

    %Transaction{} |> Transaction.changeset(params) |> Repo.insert!()
  end

  defp recording(name), do: @recordings |> Path.join(name) |> File.read!() |> Jason.decode!()

  defp request(params) do
    build_conn() |> get("/api/v2/transactions", params) |> json_response(200)
  end

  defp assert_items(filter, transactions) do
    response = request(%{"type" => filter})
    assert response["next_page_params"] == nil
    assert_hashes(response["items"], transactions)
    response["items"]
  end

  defp assert_hashes(items, transactions) do
    hashes = Enum.map(items, & &1["hash"])
    assert length(hashes) == length(transactions)
    assert MapSet.new(hashes) == MapSet.new(transactions, &to_string(&1.hash))
  end
end
