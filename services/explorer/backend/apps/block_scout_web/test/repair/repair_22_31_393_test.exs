defmodule BlockScoutWeb.Repair.Repair2231393Test do
  @moduledoc """
  Repair record for audit 22.30.143:
  transaction-create-recipient is proved by the REST and homepage contract-creation tests.
  internal-create-recipient is proved by the internal CREATE trace test.
  No out-of-path wiring changes are required.
  Baseline verification is not run: the bounded gate order requires implementation before verification.
  """
  use BlockScoutWeb.ConnCase

  alias EthereumJSONRPC.Receipt
  alias EthereumJSONRPC.Transaction, as: JSONRPCTransaction
  alias EthereumJSONRPC.Nethermind.Trace
  alias Explorer.Chain.{Address, InternalTransaction, Transaction}
  alias Explorer.Repo

  @recordings Path.expand("../../../explorer/test/support/fixture/vcr_cassettes", __DIR__)

  test "transaction-create-recipient: REST contract creation casts with a null recipient", %{conn: conn} do
    transaction = persist_creation_transaction()

    response = json_response(get(conn, "/api/v2/transactions/#{transaction.hash}"), 200)
    assert response["hash"] == to_string(transaction.hash)
    assert_creation(response, transaction.created_contract_address_hash)
    assert "contract_creation" in response["transaction_types"]

    spec = BlockScoutWeb.Specs.Public.spec()
    assert {:ok, cast} = OpenApiSpex.cast_value(response, Map.fetch!(spec.components.schemas, "Transaction"), spec)
    assert is_nil(cast.to)
    assert_creation_contract_remains_strict(response, "Transaction", spec)
    assert_endpoint_cast(response, "/v2/transactions/{transaction_hash_param}", spec)
  end

  test "transaction-create-recipient: homepage contract creation casts with a null recipient", %{conn: conn} do
    transaction = persist_creation_transaction()

    response = json_response(get(conn, "/api/v2/main-page/transactions"), 200)
    assert [item] = response
    assert item["hash"] == to_string(transaction.hash)
    assert_creation(item, transaction.created_contract_address_hash)
    assert "contract_creation" in item["transaction_types"]

    spec = BlockScoutWeb.Specs.Public.spec()
    assert_endpoint_cast(response, "/v2/main-page/transactions", spec)
  end

  test "internal-create-recipient: internal CREATE trace casts with a null recipient", %{conn: conn} do
    [recording] = recording("internal_transaction_importer_import_1_with_contract_creation.json")
    %{"params" => [hash, ["trace"]]} = Jason.decode!(recording["request"]["body"])
    %{"result" => %{"trace" => [trace]}} = Jason.decode!(recording["response"]["body"])
    assert trace["type"] == "create"
    refute Map.has_key?(trace["action"], "to")

    sender = insert(:address, hash: trace["action"]["from"])
    created = insert(:address, hash: trace["result"]["address"], contract_code: trace["result"]["code"])

    transaction =
      insert(:transaction, hash: hash, from_address: sender, to_address: nil, created_contract_address: created)
      |> with_block(insert(:block), status: :ok)

    params =
      trace
      |> Map.merge(%{
        "blockNumber" => transaction.block_number,
        "index" => 0,
        "transactionHash" => hash,
        "transactionIndex" => transaction.index
      })
      |> Trace.to_elixir()
      |> Trace.elixir_to_params()
      |> Map.merge(%{block_hash: transaction.block_hash, block_index: 0})

    internal = %InternalTransaction{} |> InternalTransaction.changeset(params) |> Repo.insert!()
    assert is_nil(internal.to_address_hash)

    response = json_response(get(conn, "/api/v2/transactions/#{hash}/internal-transactions"), 200)
    assert %{"items" => [item], "next_page_params" => nil} = response
    assert item["transaction_hash"] == hash
    assert item["type"] == "create"
    assert item["success"] == true
    assert item["index"] == 0
    assert item["block_number"] == transaction.block_number
    assert item["from"]["hash"] == Address.checksum(sender.hash)
    assert_creation(item, created.hash)

    spec = BlockScoutWeb.Specs.Public.spec()
    assert {:ok, cast} = OpenApiSpex.cast_value(item, Map.fetch!(spec.components.schemas, "InternalTransaction"), spec)
    assert is_nil(cast.to)
    assert_creation_contract_remains_strict(item, "InternalTransaction", spec)
    assert_endpoint_cast(response, "/v2/transactions/{transaction_hash_param}/internal-transactions", spec)
  end

  defp persist_creation_transaction do
    [transaction_recording] = recording("transaction_importer_import_saves_the_transaction.json")
    [receipt_recording] = recording("transaction_importer_import_1_receipt.json")
    %{"result" => recorded_transaction} = Jason.decode!(transaction_recording["response"]["body"])
    %{"result" => recorded_receipt} = Jason.decode!(receipt_recording["response"]["body"])
    [trace_recording] = recording("internal_transaction_importer_import_1_with_contract_creation.json")
    %{"result" => %{"trace" => [trace]}} = Jason.decode!(trace_recording["response"]["body"])
    assert recorded_transaction["to"] == nil
    assert recorded_receipt["transactionHash"] == recorded_transaction["hash"]
    assert recorded_transaction["input"] == trace["action"]["init"]

    params = recorded_transaction |> JSONRPCTransaction.to_elixir() |> JSONRPCTransaction.elixir_to_params()
    receipt_params = recorded_receipt |> Receipt.to_elixir() |> Receipt.elixir_to_params()
    insert(:address, hash: params.from_address_hash)
    insert(:address, hash: receipt_params.created_contract_address_hash, contract_code: trace["result"]["code"])
    block = insert(:block, hash: params.block_hash, number: params.block_number)

    params =
      params
      |> Map.merge(receipt_params)
      |> Map.merge(%{block_consensus: true, block_timestamp: block.timestamp})

    transaction = %Transaction{} |> Transaction.changeset(params) |> Repo.insert!()
    assert is_nil(transaction.to_address_hash)
    transaction
  end

  defp recording(name) do
    @recordings |> Path.join(name) |> File.read!() |> Jason.decode!()
  end

  defp assert_creation(item, created_hash) do
    assert Map.has_key?(item, "to")
    assert is_nil(item["to"])
    assert item["created_contract"]["hash"] == Address.checksum(created_hash)
    assert item["created_contract"]["is_contract"] == true
  end

  defp assert_creation_contract_remains_strict(item, schema_name, spec) do
    schema = Map.fetch!(spec.components.schemas, schema_name)
    assert {:error, _} = OpenApiSpex.cast_value(Map.delete(item, "to"), schema, spec)
    assert {:error, _} = OpenApiSpex.cast_value(Map.put(item, "to", %{"hash" => "invalid"}), schema, spec)
    assert {:error, _} = OpenApiSpex.cast_value(Map.put(item, "from", nil), schema, spec)
  end

  defp assert_endpoint_cast(response, path, spec) do
    operation = Map.fetch!(spec.paths, path).get
    schema = Map.fetch!(operation.responses, 200).content["application/json"].schema
    assert {:ok, _} = OpenApiSpex.cast_value(response, schema, spec)
  end
end
