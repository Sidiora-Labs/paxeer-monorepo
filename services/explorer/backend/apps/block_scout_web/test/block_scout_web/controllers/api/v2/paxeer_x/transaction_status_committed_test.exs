defmodule BlockScoutWeb.API.V2.PaxeerX.TransactionStatusCommittedTest do
  use BlockScoutWeb.ConnCase

  alias Explorer.Chain
  alias Explorer.Chain.PaxeerX.SequencerReceipt
  alias Plug.Conn

  @vectors Path.expand(
             "../../../../../../../explorer/test/support/fixture/paxeer_x/committed_vectors.json",
             __DIR__
           )

  setup do
    original_receipt_config = Application.get_env(:explorer, SequencerReceipt)
    original_adapter = Application.get_env(:tesla, :adapter)
    Application.put_env(:tesla, :adapter, Tesla.Adapter.Mint)

    on_exit(fn ->
      restore_env(:explorer, SequencerReceipt, original_receipt_config)
      restore_env(:tesla, :adapter, original_adapter)
    end)

    %{"cases" => cases} = @vectors |> File.read!() |> Jason.decode!()
    {:ok, vector: Enum.find(cases, &(&1["id"] == "committed-ok"))}
  end

  test "status_answers_committed_for_router_verified_hash", context do
    status_answers_committed_for_router_verified_hash(context)
  end

  defp status_answers_committed_for_router_verified_hash(%{conn: conn, vector: vector}) do
    hash = transaction_hash(vector)
    {:ok, transaction_hash} = Chain.string_to_full_hash(hash)
    assert {:error, :not_found} = Chain.hash_to_transaction(transaction_hash, api?: true)

    batch = gateway_batch(vector)

    calls = [
      {"pax_sequencerReceipt", [hash], vector["receiptJson"]},
      {"px_getReceiptKeys", [], %{"keys" => [vector["pinnedKey"]]}},
      {"eth_getTransactionReceipt", [hash], vector["eth_receipt"]},
      {"px_getCommit", ["1000"], %{"height" => "1000", "signedHeader" => vector["commit"]["signed_header"]}},
      {"px_getValidators", ["1000"], gateway_validators(vector["validators"])},
      {"px_getSequenceBatch", ["1000"], batch}
    ]

    response = status_response(conn, hash, calls)

    assert response == %{"rung" => "committed", "block_number" => nil, "committed_height" => 1_000}
    assert {:error, :not_found} = Chain.hash_to_transaction(transaction_hash, api?: true)

    "0x" <> tx_hex = batch["tx"]
    tx = Base.decode16!(tx_hex, case: :mixed)
    <<domain::binary-size(25), _height::64, rest::binary>> = tx

    for invalid_tx <- [
          binary_part(tx, 0, 240),
          tx <> <<0>>,
          <<0>> <> binary_part(tx, 1, 240),
          domain <> <<1_001::64>> <> rest
        ] do
      invalid_batch = Map.put(batch, "tx", hex(invalid_tx))
      invalid_calls = List.keyreplace(calls, "px_getSequenceBatch", 0, {"px_getSequenceBatch", ["1000"], invalid_batch})

      assert status_response(conn, hash, invalid_calls) ==
               %{"rung" => "receipted", "block_number" => nil, "committed_height" => nil}
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
      {"pax_sequencerReceipt", [hash], vector["receiptJson"]},
      {"px_getReceiptKeys", [], %{"keys" => [vector["pinnedKey"]]}},
      {"eth_getTransactionReceipt", [hash], vector["eth_receipt"]},
      {"px_getCommit", ["1000"], nil}
    ]

    assert status_response(conn, hash, calls) ==
             %{"rung" => "receipted", "block_number" => nil, "committed_height" => nil}

    assert {:error, :not_found} = Chain.hash_to_transaction(transaction_hash, api?: true)

    for malformed <- [[vector["pinnedKey"]], %{}, %{"keys" => nil}, %{"keys" => %{}}] do
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

  defp transaction_hash(vector) do
    "0x" <> raw_transaction = vector["rawTransaction"]
    raw_transaction |> Base.decode16!(case: :mixed) |> ExKeccak.hash_256() |> hex()
  end

  defp gateway_validators(validators) do
    %{
      "height" => validators["block_height"],
      "total" => validators["total"],
      "validators" =>
        Enum.map(validators["validators"], fn validator ->
          %{
            "address" => validator["address"],
            "pubKey" => validator["pub_key"],
            "votingPower" => validator["voting_power"]
          }
        end)
    }
  end

  defp gateway_batch(vector) do
    "0x" <> receipt_hex = vector["receiptJson"]["receipt"]
    {:ok, receipt} = receipt_hex |> Base.decode16!(case: :mixed) |> SequencerReceipt.decode()
    domain = "PAXEER/v1/sequence-batch\0"

    fields =
      <<1_000::64, vector["batch"]["from_seq"]::64, vector["batch"]["to_seq"]::64>> <>
        receipt.previous_state_root <> receipt.resulting_state_root <> receipt.activity_root

    seed = :binary.copy(<<0x5E>>, 32)
    {public_key, private_key} = :crypto.generate_key(:eddsa, :ed25519, seed)
    assert hex(public_key) == vector["pinnedKey"]["publicKey"]
    digest = :crypto.hash(:sha256, domain <> vector["chain_id"] <> fields)
    signature = :crypto.sign(:eddsa, :none, digest, [private_key, :ed25519])
    tx = domain <> fields <> receipt.context_hash <> signature
    assert byte_size(tx) == 241

    %{
      "height" => "1000",
      "tx" => hex(tx),
      "proof" => %{
        "total" => 1,
        "index" => 0,
        "leafHash" => hex(:crypto.hash(:sha256, <<0>> <> :crypto.hash(:sha256, tx))),
        "aunts" => []
      }
    }
  end

  defp hex(bytes), do: "0x" <> Base.encode16(bytes, case: :lower)

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

  defp restore_env(app, key, nil), do: Application.delete_env(app, key)
  defp restore_env(app, key, value), do: Application.put_env(app, key, value)
end
