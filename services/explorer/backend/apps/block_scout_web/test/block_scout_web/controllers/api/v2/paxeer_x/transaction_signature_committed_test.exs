defmodule BlockScoutWeb.API.V2.PaxeerX.TransactionSignatureCommittedTest do
  use BlockScoutWeb.ConnCase

  alias BlockScoutWeb.Schemas.API.V2.PaxeerX.TransactionSignature
  alias Explorer.Chain.PaxeerX.SequencerReceipt
  alias OpenApiSpex.{Components, Info, OpenApi}
  alias Plug.Conn

  @vectors Path.expand(
             "../../../../../../../explorer/test/support/fixture/paxeer_x/committed_vectors.json",
             __DIR__
           )

  setup do
    original_config = Application.get_env(:explorer, SequencerReceipt)
    original_adapter = Application.get_env(:tesla, :adapter)
    Application.put_env(:tesla, :adapter, Tesla.Adapter.Mint)

    on_exit(fn ->
      restore_env(:explorer, SequencerReceipt, original_config)
      restore_env(:tesla, :adapter, original_adapter)
    end)

    %{"cases" => cases} = @vectors |> File.read!() |> Jason.decode!()
    {:ok, cases: Map.new(cases, &{&1["id"], &1})}
  end

  test "signature_route_reports_committed_rung", context do
    signature_route_reports_committed_rung(context)
  end

  defp signature_route_reports_committed_rung(%{conn: conn, cases: cases}) do
    vector = Map.fetch!(cases, "committed-ok")
    hash = tx_hash(vector)
    calls = committed_calls(vector, hash)
    serve_rpc(calls)

    response = json_response(get(conn, "/api/v2/transactions/#{hash}/signature"), 200)
    assert {:ok, receipt} = vector["receiptJson"]["receipt"] |> bytes() |> SequencerReceipt.decode()
    key = vector["pinnedKey"]

    assert response == %{
             "rung" => "committed",
             "committed_height" => 1_000,
             "receipt" => vector["receiptJson"]["receipt"],
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

    for id <- ["r7-height-below-receipt", "r8-commit-height-mismatch"] do
      rejected = Map.fetch!(cases, id)
      rejected_hash = tx_hash(rejected)
      calls = committed_calls(rejected, rejected_hash)
      serve_rpc(calls)

      assert %{"message" => "Not found"} =
               json_response(get(conn, "/api/v2/transactions/#{rejected_hash}/signature"), 404)

      assert_calls(calls)
    end

    calls = [
      {"pax_sequencerReceipt", [hash], vector["receiptJson"]},
      {"px_getReceiptKeys", [], %{"keys" => [key]}},
      {"eth_getTransactionReceipt", [hash], nil}
    ]

    serve_rpc(calls)
    receipted = json_response(get(conn, "/api/v2/transactions/#{hash}/signature"), 200)
    assert receipted == response |> Map.put("rung", "receipted") |> Map.delete("committed_height")
    assert {:ok, _body} = OpenApiSpex.cast_value(Map.put(receipted, "committed_height", nil), schema, spec)
    assert_calls(calls)
  end

  test "signature_route_reports_receipted_for_malformed_gateway_batch", context do
    signature_route_reports_receipted_for_malformed_gateway_batch(context)
  end

  defp signature_route_reports_receipted_for_malformed_gateway_batch(%{conn: conn, cases: cases}) do
    vector = Map.fetch!(cases, "committed-ok")
    hash = tx_hash(vector)
    batch = gateway_batch(vector)
    tx = bytes(batch["tx"])
    <<domain::binary-size(25), _height::64, rest::binary>> = tx
    {:ok, receipt} = vector["receiptJson"]["receipt"] |> bytes() |> SequencerReceipt.decode()
    key = vector["pinnedKey"]

    for malformed_batch <- [
          Map.put(batch, "height", "1001"),
          Map.put(batch, "tx", hex(domain <> <<1_001::64>> <> rest)),
          Map.put(batch, "tx", hex(binary_part(tx, 0, byte_size(tx) - 1))),
          Map.put(batch, "height", 1_000)
        ] do
      calls =
        List.keyreplace(
          committed_calls(vector, hash),
          "px_getSequenceBatch",
          0,
          {"px_getSequenceBatch", ["1000"], malformed_batch}
        )

      serve_rpc(calls)
      response = json_response(get(conn, "/api/v2/transactions/#{hash}/signature"), 200)

      assert response == %{
               "rung" => "receipted",
               "receipt" => vector["receiptJson"]["receipt"],
               "receipt_key" => %{"key_id" => key["keyId"], "public_key" => key["publicKey"]},
               "signature" => hex(receipt.signature)
             }

      assert {:ok, ^receipt} = SequencerReceipt.verify_signature(bytes(response["receipt"]), bytes(key["publicKey"]))
      assert_calls(calls)
    end
  end

  test "signature route rejects a bare receipt key list", %{conn: conn, cases: cases} do
    vector = Map.fetch!(cases, "committed-ok")
    hash = tx_hash(vector)

    calls = [
      {"pax_sequencerReceipt", [hash], vector["receiptJson"]},
      {"px_getReceiptKeys", [], [vector["pinnedKey"]]}
    ]

    serve_rpc(calls)
    assert %{"message" => "Not found"} = json_response(get(conn, "/api/v2/transactions/#{hash}/signature"), 404)
    assert_calls(calls)
  end

  defp committed_calls(vector, hash) do
    "0x" <> height = vector["eth_receipt"]["blockNumber"]
    height = String.to_integer(height, 16)
    decimal_height = Integer.to_string(height)

    validators = %{
      "height" => decimal_height,
      "total" => vector["validators"]["total"],
      "validators" =>
        Enum.map(vector["validators"]["validators"], fn validator ->
          %{
            "address" => validator["address"],
            "pubKey" => validator["pub_key"],
            "votingPower" => validator["voting_power"]
          }
        end)
    }

    [
      {"pax_sequencerReceipt", [hash], vector["receiptJson"]},
      {"px_getReceiptKeys", [], %{"keys" => [vector["pinnedKey"]]}},
      {"eth_getTransactionReceipt", [hash], vector["eth_receipt"]},
      {"px_getCommit", [decimal_height],
       %{"height" => decimal_height, "signedHeader" => vector["commit"]["signed_header"]}},
      {"px_getValidators", [decimal_height], validators},
      {"px_getSequenceBatch", [decimal_height], gateway_batch(vector)}
    ]
  end

  defp gateway_batch(vector) do
    "0x" <> receipt_hex = vector["receiptJson"]["receipt"]
    {:ok, receipt} = receipt_hex |> Base.decode16!(case: :mixed) |> SequencerReceipt.decode()
    domain = "PAXEER/v1/sequence-batch\0"

    "0x" <> height = vector["eth_receipt"]["blockNumber"]
    height = String.to_integer(height, 16)

    fields =
      <<height::64, vector["batch"]["from_seq"]::64, vector["batch"]["to_seq"]::64>> <>
        receipt.previous_state_root <> receipt.resulting_state_root <> receipt.activity_root

    seed = :binary.copy(<<0x5E>>, 32)
    {public_key, private_key} = :crypto.generate_key(:eddsa, :ed25519, seed)
    assert hex(public_key) == vector["pinnedKey"]["publicKey"]
    digest = :crypto.hash(:sha256, domain <> vector["chain_id"] <> fields)
    signature = :crypto.sign(:eddsa, :none, digest, [private_key, :ed25519])
    tx = domain <> fields <> receipt.context_hash <> signature
    assert byte_size(tx) == 241

    %{
      "height" => Integer.to_string(height),
      "tx" => hex(tx),
      "proof" => %{
        "total" => 1,
        "index" => 0,
        "leafHash" => hex(:crypto.hash(:sha256, <<0>> <> :crypto.hash(:sha256, tx))),
        "aunts" => []
      }
    }
  end

  defp assert_calls(calls) do
    for {method, params, _result} <- calls do
      assert_received {:rpc_call, ^method, ^params}
    end

    refute_received {:rpc_call, _method, _params}
  end

  defp serve_rpc(calls) do
    bypass = Bypass.open()
    test_pid = self()
    answers = Map.new(calls, fn {method, params, result} -> {method, {params, result}} end)

    Bypass.expect(bypass, "POST", "/", fn conn ->
      {:ok, body, conn} = Conn.read_body(conn)
      assert %{"jsonrpc" => "2.0", "id" => id, "method" => method, "params" => params} = Jason.decode!(body)
      assert {^params, result} = Map.fetch!(answers, method)
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
