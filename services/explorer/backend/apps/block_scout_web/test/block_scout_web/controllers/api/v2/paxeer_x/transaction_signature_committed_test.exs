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
      {"px_getSequenceBatch", [height], vector["batch"]}
    ]
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
