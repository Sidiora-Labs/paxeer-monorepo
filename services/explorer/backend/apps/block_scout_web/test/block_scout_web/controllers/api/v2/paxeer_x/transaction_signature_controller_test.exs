defmodule BlockScoutWeb.API.V2.PaxeerX.TransactionSignatureControllerTest do
  use BlockScoutWeb.ConnCase

  alias BlockScoutWeb.Schemas.API.V2.PaxeerX.TransactionSignature
  alias Explorer.Chain.PaxeerX.SequencerReceipt
  alias OpenApiSpex.{Components, Info, OpenApi}
  alias Plug.Conn

  @vectors Path.expand(
             "../../../../../../../explorer/test/support/fixture/paxeer_x/receipt_vectors.json",
             __DIR__
           )

  setup do
    original_config = Application.get_env(:explorer, SequencerReceipt)
    original_adapter = Application.get_env(:tesla, :adapter)
    Application.put_env(:explorer, SequencerReceipt, router_url: nil)
    Application.put_env(:tesla, :adapter, Tesla.Adapter.Mint)

    on_exit(fn ->
      restore_env(:explorer, SequencerReceipt, original_config)
      restore_env(:tesla, :adapter, original_adapter)
    end)

    vectors = @vectors |> File.read!() |> Jason.decode!()
    {:ok, cases: Map.new(vectors["sequencer"]["cases"], &{&1["id"], &1})}
  end

  test "signature_route_returns_verified_receipt", context do
    signature_route_returns_verified_receipt(context)
  end

  defp signature_route_returns_verified_receipt(%{conn: conn, cases: cases}) do
    spec =
      OpenApiSpex.resolve_schema_modules(%OpenApi{
        info: %Info{title: "Transaction signature response test", version: "1"},
        paths: %{},
        components: %Components{schemas: %{"PaxeerXTransactionSignature" => TransactionSignature.schema()}}
      })

    schema = spec.components.schemas["PaxeerXTransactionSignature"]
    accepted = cases |> Map.values() |> Enum.filter(&(&1["vector"] in ["V1", "V2"]))
    assert length(accepted) == 5

    for vector <- accepted do
      hash = tx_hash(vector)
      answer = vector["receiptJson"]
      key = vector["pinnedKey"]
      serve_rpc(hash, answer, [key])

      response = json_response(get(conn, "/api/v2/transactions/#{hash}/signature"), 200)
      assert {:ok, receipt} = vector["signedReceipt"] |> bytes() |> SequencerReceipt.decode()

      assert response == %{
               "rung" => "receipted",
               "receipt" => vector["signedReceipt"],
               "receipt_key" => %{"key_id" => key["keyId"], "public_key" => key["publicKey"]},
               "signature" => hex(receipt.signature)
             }

      assert byte_size(bytes(response["signature"])) == 64
      assert {:ok, ^receipt} = SequencerReceipt.verify_signature(bytes(response["receipt"]), bytes(key["publicKey"]))
      assert {:ok, _body} = OpenApiSpex.cast_value(response, schema, spec)
      assert_received {:rpc_call, "pax_sequencerReceipt", [^hash]}
      assert_received {:rpc_call, "px_getReceiptKeys", []}
      refute_received {:rpc_call, _method, _params}
    end

    vector = cases["V1-pax-transfer"]
    hash = tx_hash(vector)

    answer =
      Map.merge(vector["receiptJson"], %{
        "rung" => "committed",
        "blockNumber" => "0x3e8",
        "blockHash" => hex(<<1::256>>)
      })

    serve_rpc(hash, answer, [vector["pinnedKey"]])
    response = json_response(get(conn, "/api/v2/transactions/#{hash}/signature"), 200)
    assert response["rung"] == "receipted"
  end

  test "signature_route_404_when_unverified", context do
    signature_route_404_when_unverified(context)
  end

  defp signature_route_404_when_unverified(%{conn: conn, cases: cases}) do
    vector = cases["V1-pax-transfer"]
    hash = tx_hash(vector)
    assert %{"message" => "Not found"} = json_response(get(conn, "/api/v2/transactions/#{hash}/signature"), 404)
    assert %{"message" => "Invalid parameter(s)"} = json_response(get(conn, "/api/v2/transactions/0x01/signature"), 422)

    rejected = [
      {:r1, vector, Map.put(vector["receiptJson"], "rung", "submitted"), vector["pinnedKey"]},
      {:r2, cases["V3-non-canonical-bytes"]},
      invalid_signature(vector),
      {:r3, cases["V3-key-not-pinned"]},
      {:r3, cases["V3-height-outside-key-range"]},
      {:r4, cases["V3-activity-id-not-raw-hash"]},
      {:r5, cases["V3-tampered-log"]},
      invalid_sequence(vector)
    ]

    for rejection <- rejected do
      {step, vector, answer, key} = rejection_inputs(rejection)
      hash = tx_hash(vector)
      assert {:ok, keys} = SequencerReceipt.decode_receipt_keys([key])
      assert SequencerReceipt.verify(hash, answer, keys, []) == {:error, step}
      serve_rpc(hash, answer, [key])

      assert %{"message" => "Not found"} = json_response(get(conn, "/api/v2/transactions/#{hash}/signature"), 404)
      assert_received {:rpc_call, "pax_sequencerReceipt", [^hash]}
      assert_received {:rpc_call, "px_getReceiptKeys", []}
      refute_received {:rpc_call, _method, _params}
    end

    for {answer, keys} <- [{nil, []}, {vector["receiptJson"], []}, {vector["receiptJson"], [%{}]}] do
      serve_rpc(hash, answer, keys)
      assert %{"message" => "Not found"} = json_response(get(conn, "/api/v2/transactions/#{hash}/signature"), 404)
    end
  end

  defp rejection_inputs({step, vector}), do: {step, vector, vector["receiptJson"], vector["pinnedKey"]}
  defp rejection_inputs({step, vector, answer, key}), do: {step, vector, answer, key}

  defp invalid_signature(vector) do
    encoded = bytes(vector["signedReceipt"])
    prefix_size = byte_size(encoded) - 1
    <<prefix::binary-size(prefix_size), last>> = encoded
    answer = Map.put(vector["receiptJson"], "receipt", hex(prefix <> <<Bitwise.bxor(last, 1)>>))
    {:r2, vector, answer, vector["pinnedKey"]}
  end

  defp invalid_sequence(vector) do
    {:ok, receipt} = vector["signedReceipt"] |> bytes() |> SequencerReceipt.decode()
    {public_key, private_key} = :crypto.generate_key(:eddsa, :ed25519)
    receipt = %{receipt | resulting_state_root: <<1::256>>}
    signature = :crypto.sign(:eddsa, :none, SequencerReceipt.digest(receipt), [private_key, :ed25519])
    encoded = SequencerReceipt.encode(%{receipt | signature: signature})
    answer = Map.put(vector["receiptJson"], "receipt", hex(encoded))
    key = Map.put(vector["pinnedKey"], "publicKey", hex(public_key))
    {:r6, vector, answer, key}
  end

  defp serve_rpc(hash, answer, keys) do
    bypass = Bypass.open()
    test_pid = self()
    results = %{"pax_sequencerReceipt" => {[hash], answer}, "px_getReceiptKeys" => {[], keys}}

    Bypass.expect(bypass, "POST", "/", fn conn ->
      {:ok, body, conn} = Conn.read_body(conn)
      assert %{"jsonrpc" => "2.0", "id" => id, "method" => method, "params" => params} = Jason.decode!(body)
      assert {^params, result} = Map.fetch!(results, method)
      send(test_pid, {:rpc_call, method, params})

      conn
      |> Conn.put_resp_content_type("application/json")
      |> Conn.resp(200, Jason.encode!(%{jsonrpc: "2.0", id: id, result: result}))
    end)

    Application.put_env(:explorer, SequencerReceipt, router_url: "http://localhost:#{bypass.port}/")
  end

  defp tx_hash(vector), do: vector["rawTransaction"] |> bytes() |> ExKeccak.hash_256() |> hex()
  defp bytes("0x" <> encoded), do: Base.decode16!(encoded, case: :mixed)
  defp hex(bytes), do: "0x" <> Base.encode16(bytes, case: :lower)
  defp restore_env(app, key, nil), do: Application.delete_env(app, key)
  defp restore_env(app, key, value), do: Application.put_env(app, key, value)
end
