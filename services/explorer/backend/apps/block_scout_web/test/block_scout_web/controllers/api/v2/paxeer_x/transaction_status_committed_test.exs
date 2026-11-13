Code.require_file("committed_evidence.exs", __DIR__)

defmodule BlockScoutWeb.API.V2.PaxeerX.TransactionStatusCommittedTest do
  use BlockScoutWeb.ConnCase

  alias Explorer.Chain
  alias Explorer.Chain.PaxeerX.SequencerReceipt
  alias BlockScoutWeb.API.V2.PaxeerX.CommittedEvidence, as: Evidence

  setup do
    original_receipt_config = Application.get_env(:explorer, SequencerReceipt)
    original_adapter = Application.get_env(:tesla, :adapter)
    Application.put_env(:tesla, :adapter, Tesla.Adapter.Mint)

    on_exit(fn ->
      restore_env(:explorer, SequencerReceipt, original_receipt_config)
      restore_env(:tesla, :adapter, original_adapter)
    end)

    {:ok, vector: Evidence.settlement()}
  end

  test "status_answers_committed_for_router_verified_hash", context do
    status_answers_committed_for_router_verified_hash(context)
  end

  defp status_answers_committed_for_router_verified_hash(%{conn: conn, vector: vector}) do
    hash = transaction_hash(vector)
    {:ok, transaction_hash} = Chain.string_to_full_hash(hash)
    assert {:error, :not_found} = Chain.hash_to_transaction(transaction_hash, api?: true)

    batch = gateway_batch(vector)

    calls = Evidence.committed_calls(vector)

    response = status_response(conn, hash, calls)

    assert response == %{"rung" => "committed", "block_number" => nil, "committed_height" => 10}
    assert {:error, :not_found} = Chain.hash_to_transaction(transaction_hash, api?: true)

    "0x" <> tx_hex = batch["tx"]
    tx = Base.decode16!(tx_hex, case: :mixed)
    <<domain::binary-size(25), _height::64, rest::binary>> = tx

    for invalid_tx <- [
          binary_part(tx, 0, 240),
          tx <> <<0>>,
          <<0>> <> binary_part(tx, 1, 240),
          domain <> <<11::64>> <> rest
        ] do
      invalid_batch = Map.put(batch, "tx", hex(invalid_tx))
      invalid_calls = List.keyreplace(calls, "px_getSequenceBatch", 0, {"px_getSequenceBatch", ["10"], invalid_batch})

      serve_rpc(invalid_calls)
      response = get(conn, "/api/v2/transactions/#{hash}/status")
      assert response.status == 404
      assert json_response(response, 404) == %{"message" => "Not found"}
      Evidence.assert_calls(invalid_calls)
    end
  end

  test "status_answers_receipted_for_gateway_receipt_keys_envelope", context do
    status_answers_receipted_for_gateway_receipt_keys_envelope(context)
  end

  defp status_answers_receipted_for_gateway_receipt_keys_envelope(%{conn: conn, vector: vector}) do
    hash = transaction_hash(vector)
    {:ok, transaction_hash} = Chain.string_to_full_hash(hash)
    assert {:error, :not_found} = Chain.hash_to_transaction(transaction_hash, api?: true)

    calls = [
      {"pax_sequencerReceipt", [hash], vector["receipt"]},
      {"px_getReceiptKeys", [], %{"keys" => [Evidence.receipt_key()]}},
      {"eth_getTransactionReceipt", [hash], Evidence.eth_receipt(vector)},
      {"px_getCommit", ["10"], nil}
    ]

    assert status_response(conn, hash, calls) ==
             %{"rung" => "receipted", "block_number" => nil, "committed_height" => nil}

    assert {:error, :not_found} = Chain.hash_to_transaction(transaction_hash, api?: true)

    for malformed <- [[Evidence.receipt_key()], %{}, %{"keys" => nil}, %{"keys" => %{}}] do
      serve_rpc([{"px_getReceiptKeys", [], malformed}])
      assert {:error, :malformed_receipt_keys} = SequencerReceipt.receipt_keys()
      assert_received {:rpc_call, "px_getReceiptKeys", []}
      refute_received {:rpc_call, _method, _params}
    end
  end

  defp status_response(conn, hash, calls) do
    serve_rpc(calls)
    response = json_response(get(conn, "/api/v2/transactions/#{hash}/status"), 200)

    for {method, params, _result} <- calls do
      assert_received {:rpc_call, ^method, ^params}
    end

    refute_received {:rpc_call, _method, _params}
    response
  end

  defp transaction_hash(vector), do: vector["raw_transaction"] |> Evidence.bytes() |> ExKeccak.hash_256() |> hex()
  defp gateway_batch(vector), do: Evidence.gateway_batch(vector)
  defp hex(bytes), do: Evidence.hex(bytes)
  defp serve_rpc(calls), do: Evidence.serve_rpc(calls)
  defp restore_env(app, key, value), do: Evidence.restore_env(app, key, value)
end
