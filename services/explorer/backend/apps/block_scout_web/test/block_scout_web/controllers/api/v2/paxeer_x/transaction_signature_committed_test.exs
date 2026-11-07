Code.require_file("committed_evidence.exs", __DIR__)

defmodule BlockScoutWeb.API.V2.PaxeerX.TransactionSignatureCommittedTest do
  use BlockScoutWeb.ConnCase

  alias BlockScoutWeb.Schemas.API.V2.PaxeerX.TransactionSignature
  alias Explorer.Chain.PaxeerX.SequencerReceipt
  alias OpenApiSpex.{Components, Info, OpenApi}
  alias BlockScoutWeb.API.V2.PaxeerX.CommittedEvidence, as: Evidence

  setup do
    original_config = Application.get_env(:explorer, SequencerReceipt)
    original_adapter = Application.get_env(:tesla, :adapter)
    Application.put_env(:tesla, :adapter, Tesla.Adapter.Mint)

    on_exit(fn ->
      restore_env(:explorer, SequencerReceipt, original_config)
      restore_env(:tesla, :adapter, original_adapter)
    end)

    {:ok, vector: Evidence.settlement()}
  end

  test "signature_route_reports_committed_rung", context do
    signature_route_reports_committed_rung(context)
  end

  defp signature_route_reports_committed_rung(%{conn: conn, vector: vector}) do
    hash = tx_hash(vector)
    calls = committed_calls(vector, hash)
    serve_rpc(calls)

    response = json_response(get(conn, "/api/v2/transactions/#{hash}/signature"), 200)
    assert {:ok, receipt} = vector["receipt"]["receipt"] |> bytes() |> SequencerReceipt.decode()
    key = Evidence.receipt_key()

    assert response == %{
             "rung" => "committed",
             "committed_height" => 10,
             "receipt" => vector["receipt"]["receipt"],
             "receipt_key" => %{"key_id" => key["keyId"], "public_key" => key["publicKey"]},
             "signature" => hex(receipt.signature)
           }

    assert {:ok, ^receipt} = SequencerReceipt.verify_signature(bytes(response["receipt"]), bytes(key["publicKey"]))

    spec =
      OpenApiSpex.resolve_schema_modules(%OpenApi{
        info: %Info{title: "Committed transaction signature response test", version: "1"},
        paths: %{},
        components: %Components{schemas: %{"PaxeerXTransactionSignature" => TransactionSignature.schema()}}
      })

    schema = spec.components.schemas["PaxeerXTransactionSignature"]
    assert schema.properties.committed_height.type == :integer
    assert schema.properties.committed_height.nullable
    assert {:ok, _body} = OpenApiSpex.cast_value(response, schema, spec)
    assert_calls(calls)

    for {id, calls} <- [
          {"r7-height-below-receipt", below_receipt_calls(vector)},
          {"r8-commit-height-mismatch",
           Enum.take(committed_calls(Map.put(vector, "execution_commit", vector["commit"]), hash), 6)}
        ] do
      serve_rpc(calls)

      response = get(conn, "/api/v2/transactions/#{hash}/signature")
      assert response.status == 404, id
      assert %{"message" => "Not found"} = json_response(response, 404)
      assert_calls(calls)
    end

    calls = [
      {"pax_sequencerReceipt", [hash], vector["receipt"]},
      {"px_getReceiptKeys", [], %{"keys" => [key]}},
      {"eth_getTransactionReceipt", [hash], nil}
    ]

    serve_rpc(calls)
    receipted = json_response(get(conn, "/api/v2/transactions/#{hash}/signature"), 200)
    assert receipted == response |> Map.put("rung", "receipted") |> Map.delete("committed_height")
    assert {:ok, _body} = OpenApiSpex.cast_value(Map.put(receipted, "committed_height", nil), schema, spec)
    assert_calls(calls)
  end

  defp below_receipt_calls(vector) do
    batch = gateway_batch(vector)
    <<domain::binary-size(25), _height::64, rest::binary>> = bytes(batch["tx"])
    invalid_batch = batch |> Map.put("height", "9") |> Map.put("tx", hex(domain <> <<9::64>> <> rest))

    committed_calls(vector, tx_hash(vector))
    |> Enum.take(6)
    |> Enum.map(fn
      {"eth_getTransactionReceipt", params, receipt} ->
        {"eth_getTransactionReceipt", params, Map.put(receipt, "blockNumber", "0x9")}

      {"px_getCommit", _params, commit} ->
        {"px_getCommit", ["9"], Map.put(commit, "height", "9")}

      {"px_getValidators", _params, validators} ->
        {"px_getValidators", ["9"], Map.put(validators, "height", "9")}

      {"px_getSequenceBatch", _params, _batch} ->
        {"px_getSequenceBatch", ["9"], invalid_batch}

      call ->
        call
    end)
  end

  test "signature_route_reports_receipted_for_malformed_gateway_batch", context do
    signature_route_reports_receipted_for_malformed_gateway_batch(context)
  end

  defp signature_route_reports_receipted_for_malformed_gateway_batch(%{conn: conn, vector: vector}) do
    hash = tx_hash(vector)
    batch = gateway_batch(vector)
    tx = bytes(batch["tx"])
    <<domain::binary-size(25), _height::64, rest::binary>> = tx
    {:ok, receipt} = vector["receipt"]["receipt"] |> bytes() |> SequencerReceipt.decode()
    key = Evidence.receipt_key()

    for malformed_batch <- [
          Map.put(batch, "height", "11"),
          Map.put(batch, "tx", hex(domain <> <<11::64>> <> rest)),
          Map.put(batch, "tx", hex(binary_part(tx, 0, byte_size(tx) - 1))),
          Map.put(batch, "height", 10)
        ] do
      calls =
        List.keyreplace(
          committed_calls(vector, hash),
          "px_getSequenceBatch",
          0,
          {"px_getSequenceBatch", ["10"], malformed_batch}
        )

      serve_rpc(calls)
      response = json_response(get(conn, "/api/v2/transactions/#{hash}/signature"), 200)

      assert response == %{
               "rung" => "receipted",
               "receipt" => vector["receipt"]["receipt"],
               "receipt_key" => %{"key_id" => key["keyId"], "public_key" => key["publicKey"]},
               "signature" => hex(receipt.signature)
             }

      assert {:ok, ^receipt} = SequencerReceipt.verify_signature(bytes(response["receipt"]), bytes(key["publicKey"]))
      assert_calls(calls)
    end
  end

  test "signature route rejects a bare receipt key list", %{conn: conn, vector: vector} do
    hash = tx_hash(vector)

    calls = [
      {"pax_sequencerReceipt", [hash], vector["receipt"]},
      {"px_getReceiptKeys", [], [Evidence.receipt_key()]}
    ]

    serve_rpc(calls)
    assert %{"message" => "Not found"} = json_response(get(conn, "/api/v2/transactions/#{hash}/signature"), 404)
    assert_calls(calls)
  end

  defp committed_calls(vector, _hash), do: Evidence.committed_calls(vector)
  defp gateway_batch(vector), do: Evidence.gateway_batch(vector)
  defp assert_calls(calls), do: Evidence.assert_calls(calls)
  defp serve_rpc(calls), do: Evidence.serve_rpc(calls)
  defp tx_hash(vector), do: vector["raw_transaction"] |> bytes() |> ExKeccak.hash_256() |> hex()
  defp bytes(encoded), do: Evidence.bytes(encoded)
  defp hex(bytes), do: Evidence.hex(bytes)
  defp restore_env(app, key, value), do: Evidence.restore_env(app, key, value)
end
