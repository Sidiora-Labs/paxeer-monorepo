Code.require_file("../block_scout_web/controllers/api/v2/paxeer_x/committed_evidence.exs", __DIR__)
Code.require_file("../block_scout_web/controllers/api/v2/paxeer_x/transaction_status_committed_test.exs", __DIR__)
Code.require_file("../block_scout_web/controllers/api/v2/paxeer_x/transaction_signature_committed_test.exs", __DIR__)
Code.require_file("../block_scout_web/controllers/api/v2/paxeer_x/identity_search_test.exs", __DIR__)

defmodule BlockScoutWeb.Repair.Repair2231396Test do
  use BlockScoutWeb.ConnCase

  alias BlockScoutWeb.API.V2.PaxeerX.{CommittedEvidence, IdentitySearchTest}
  alias Explorer.Chain.{Address, Import}
  alias Explorer.Chain.PaxeerX.{AccountBinding, CometCommit, Identity, ReceiptInclusion, SequencerReceipt}
  alias Explorer.Repo

  @pax "pax1005qwm6w5jj26zq8tsjs3eyp6my5d5fthrlqk3"
  @key "3f1a9b0c5d2e4f60718293a4b5c6d7e8f90a1b2c3d4e5f60718293a4b5c6d7e8"

  setup do
    original_config = Application.get_env(:explorer, SequencerReceipt)
    original_adapter = Application.get_env(:tesla, :adapter)
    Application.put_env(:tesla, :adapter, Tesla.Adapter.Mint)
    Application.put_env(:explorer, SequencerReceipt, CommittedEvidence.authority())

    on_exit(fn ->
      CommittedEvidence.restore_env(:explorer, SequencerReceipt, original_config)
      CommittedEvidence.restore_env(:tesla, :adapter, original_adapter)
    end)

    {:ok, settlement: CommittedEvidence.settlement()}
  end

  test "recorded_settlement_authenticates_execution_and_successor", %{settlement: settlement} do
    assert CommittedEvidence.fixture_digest() == "fc3fe1494c91ff8720a8d8fced398ea6c483c7aa70e3b655212e2b68de75eaab"
    hash = settlement["raw_transaction"] |> CommittedEvidence.bytes() |> ExKeccak.hash_256() |> CommittedEvidence.hex()
    assert hash == settlement["receipt"]["txHash"]
    assert CommittedEvidence.vector()["positive"]["receipt"] == settlement["receipt"]["receipt"]
    assert_authenticated_headers(settlement)

    assert {:ok, keys} = SequencerReceipt.decode_receipt_keys([CommittedEvidence.receipt_key()])

    assert {:ok, receipt} =
             SequencerReceipt.verify_signature(
               CommittedEvidence.bytes(settlement["receipt"]["receipt"]),
               CommittedEvidence.bytes(CommittedEvidence.receipt_key()["publicKey"])
             )

    assert receipt.global_sequence == 9

    assert {:committed, _, 10} =
             SequencerReceipt.verify(
               hash,
               settlement["receipt"],
               keys,
               CommittedEvidence.verification_options(settlement)
             )
  end

  for mutation <- ~w(missing-batch-proof missing-transaction-proof missing-result-proof missing-execution-header
                    missing-successor-header batch-proof transaction-proof result-proof result-bytes
                    previous-root resulting-root transaction-root batch-key batch-signature
                    unrelated-execution-header unlinked-successor insufficient-successor-power) do
    test "status_committed_inclusion_rejects_#{mutation}", %{settlement: settlement} do
      assert_authenticated_headers(settlement)
      assert {:ok, keys} = SequencerReceipt.decode_receipt_keys([CommittedEvidence.receipt_key()])
      options = CommittedEvidence.verification_options(settlement)
      mutated = mutate_settlement(settlement, unquote(mutation))
      options = Keyword.put(options, :settlement, mutated)

      assert {:error, _step} =
               SequencerReceipt.verify(settlement["receipt"]["txHash"], settlement["receipt"], keys, options)
    end
  end

  for route <- ~w(status signature),
      mutation <- ~w(missing-proof changed-proof unrelated-one-leaf-proof changed-root changed-signature) do
    test "#{route}_controller_refuses_#{mutation}", %{conn: conn, settlement: settlement} do
      batch = CommittedEvidence.gateway_batch(settlement)

      invalid_batch =
        case unquote(mutation) do
          "missing-proof" ->
            Map.delete(batch, "proof")

          "changed-proof" ->
            put_in(batch, ["proof", "leafHash"], CommittedEvidence.flip_hex(batch["proof"]["leafHash"]))

          "unrelated-one-leaf-proof" ->
            Map.put(batch, "proof", %{
              "total" => 1,
              "index" => 0,
              "leafHash" => batch["proof"]["leafHash"],
              "aunts" => []
            })

          "changed-root" ->
            Map.put(batch, "tx", flip_byte(batch["tx"], 49))

          "changed-signature" ->
            Map.put(batch, "tx", flip_byte(batch["tx"], 177))
        end

      calls =
        settlement
        |> CommittedEvidence.committed_calls()
        |> CommittedEvidence.replace_call("px_getSequenceBatch", invalid_batch)

      CommittedEvidence.serve_rpc(calls)
      response = json_response(get(conn, receipt_path(settlement, unquote(route))), 200)
      assert response["rung"] == "receipted"
      refute response["committed_height"]
      CommittedEvidence.assert_calls(calls)
    end
  end

  for route <- ~w(status signature) do
    test "#{route}_rejects_correctly_signed_foreign_chain", %{conn: conn, settlement: settlement} do
      assert_authenticated_headers(settlement)
      authority = Keyword.put(CommittedEvidence.authority(), :chain_id, "independently-configured-network")
      CommittedEvidence.serve_rpc(CommittedEvidence.committed_calls(settlement), authority)

      assert %{"message" => "Not found"} =
               json_response(get(conn, receipt_path(settlement, unquote(route))), 404)
    end

    test "#{route}_rejects_correctly_signed_unauthorized_validators", %{conn: conn, settlement: settlement} do
      assert_authenticated_headers(settlement)

      authority =
        Keyword.update!(CommittedEvidence.authority(), :validator_checkpoints, fn checkpoints ->
          put_in(checkpoints, [10, :validators_hash], <<1::256>>)
        end)

      CommittedEvidence.serve_rpc(CommittedEvidence.committed_calls(settlement), authority)

      assert %{"message" => "Not found"} =
               json_response(get(conn, receipt_path(settlement, unquote(route))), 404)
    end

    test "#{route}_does_not_discover_its_own_receipt_authority", %{conn: conn, settlement: settlement} do
      authority = Keyword.put(CommittedEvidence.authority(), :receipt_keys, [])
      CommittedEvidence.serve_rpc(receipted_calls(settlement), authority)

      assert %{"message" => "Not found"} =
               json_response(get(conn, receipt_path(settlement, unquote(route))), 404)
    end

    test "#{route}_governed_key_rotation_survives_restart", %{conn: conn, settlement: settlement} do
      # Advertise a retired key alongside the governed key; discovery is transport only.
      {foreign_answer, foreign_key} = sign_foreign_receipt(settlement)
      current = CommittedEvidence.receipt_key()
      retired = Map.put(foreign_key, "lastHeight", 9)
      {:ok, pins} = SequencerReceipt.decode_receipt_keys([retired, current])
      authority = Keyword.put(CommittedEvidence.authority(), :receipt_keys, pins)

      calls =
        CommittedEvidence.replace_call(receipted_calls(settlement), "px_getReceiptKeys", %{"keys" => [retired, current]})

      CommittedEvidence.serve_rpc(calls, authority)

      assert %{"rung" => "receipted"} = json_response(get(conn, receipt_path(settlement, unquote(route))), 200)
      CommittedEvidence.assert_calls(calls)
      CommittedEvidence.restart_endpoint()
      assert Application.get_env(:explorer, SequencerReceipt)[:receipt_keys] == pins
      assert %{"rung" => "receipted"} = json_response(get(conn, receipt_path(settlement, unquote(route))), 200)
      CommittedEvidence.assert_calls(calls)

      # The foreign receipt is correctly signed, but the advertised wider range is not governed.
      assert {:ok, _} =
               SequencerReceipt.verify_signature(
                 CommittedEvidence.bytes(foreign_answer["receipt"]),
                 CommittedEvidence.bytes(foreign_key["publicKey"])
               )

      forged_calls =
        receipted_calls(settlement)
        |> CommittedEvidence.replace_call("pax_sequencerReceipt", foreign_answer)
        |> CommittedEvidence.replace_call("px_getReceiptKeys", %{"keys" => [foreign_key]})

      CommittedEvidence.serve_rpc(forged_calls, authority)
      assert %{"message" => "Not found"} = json_response(get(conn, receipt_path(settlement, unquote(route))), 404)
    end
  end

  test "validator_checkpoint_history_is_authenticated_after_restart", %{settlement: settlement} do
    execution = %{
      "signed_header" => %{"header" => settlement["execution_header"], "commit" => settlement["execution_commit"]},
      "validators" => CommittedEvidence.validators(settlement, "10")
    }

    authority = Keyword.put(CommittedEvidence.authority(), :validator_history, [execution])
    Application.put_env(:explorer, SequencerReceipt, authority)
    successor = %{"header" => settlement["header"], "commit" => settlement["commit"]}
    validators = CommittedEvidence.validators(settlement, "11")
    assert :ok = ReceiptInclusion.authenticate(successor, validators, 11)
    CommittedEvidence.restart_endpoint()
    assert Application.get_env(:explorer, SequencerReceipt)[:validator_history] == [execution]
    assert :ok = ReceiptInclusion.authenticate(successor, validators, 11)

    altered = put_in(execution, ["signed_header", "header", "next_validators_hash"], String.duplicate("01", 32))
    Application.put_env(:explorer, SequencerReceipt, Keyword.put(authority, :validator_history, [altered]))
    refute :ok == ReceiptInclusion.authenticate(successor, validators, 11)
    Application.put_env(:explorer, SequencerReceipt, Keyword.put(authority, :validator_history, []))
    refute :ok == ReceiptInclusion.authenticate(successor, validators, 11)
  end

  for route <- ~w(status signature),
      mutation <- ~w(transaction result-data result-gas successor-link successor-power) do
    test "#{route}_authenticates_native_transport_#{mutation}", %{conn: conn, settlement: settlement} do
      calls = CommittedEvidence.committed_calls(settlement)

      calls =
        case unquote(mutation) do
          "transaction" ->
            Enum.map(calls, fn
              {"block", params, result} ->
                [batch, tx] = result["block"]["data"]["txs"]
                # The changed transaction remains valid protobuf but cannot match the signed batch root.
                bytes = Base.decode64!(tx)
                changed = :binary.replace(bytes, "73700000", "73700001")
                assert bytes != changed
                {"block", params, put_in(result, ["block", "data", "txs"], [batch, Base.encode64(changed)])}

              call ->
                call
            end)

          "result-data" ->
            Enum.map(calls, fn
              {"block_results", params, result} ->
                [batch, tx] = result["txs_results"]
                tx = Map.update!(tx, "data", &flip_base64/1)
                {"block_results", params, Map.put(result, "txs_results", [batch, tx])}

              call ->
                call
            end)

          "result-gas" ->
            Enum.map(calls, fn
              {"block_results", params, result} ->
                [batch, tx] = result["txs_results"]
                {"block_results", params, Map.put(result, "txs_results", [batch, Map.put(tx, "gas_used", "1")])}

              call ->
                call
            end)

          mutation ->
            Enum.map(calls, fn
              {"px_getCommit", ["11"], result} ->
                result =
                  if mutation == "successor-link",
                    do: put_in(result, ["signedHeader", "header", "last_block_id", "hash"], String.duplicate("04", 32)),
                    else: put_in(result, ["signedHeader", "commit"], settlement["commit_at_most_two_thirds"])

                {"px_getCommit", ["11"], result}

              call ->
                call
            end)
        end

      CommittedEvidence.serve_rpc(calls)

      if unquote(mutation) in ["successor-link", "successor-power"] do
        assert %{"message" => "Not found"} = json_response(get(conn, receipt_path(settlement, unquote(route))), 404)
      else
        assert %{"rung" => "receipted"} = json_response(get(conn, receipt_path(settlement, unquote(route))), 200)
      end

      CommittedEvidence.assert_calls(calls)
    end
  end

  test "pax_identity_round_trips_import_restart_replay_and_reorg", %{conn: conn} do
    {:ok, native_bytes} = Identity.pax_address_bytes(@pax)
    address = insert(:address, hash: native_bytes, fetched_coin_balance: 0)
    did = Identity.did(@key)
    account = Identity.kernel_account(@key)
    %{row: row, block: bound_block} = IdentitySearchTest.import_binding(address, did, account, true, 0)

    assert_native_identity(conn, address, did)
    CommittedEvidence.restart_endpoint()
    assert {:ok, %{insert_paxeer_x_account_bindings: []}} = Import.all(%{paxeer_x_account_bindings: %{params: [row]}})
    assert Repo.aggregate(AccountBinding, :count) == 1
    assert_native_identity(conn, address, did)

    %{block: unbound_block} = IdentitySearchTest.import_binding(address, did, account, false, 1)
    assert_unbound(conn, address, did)
    unbound_block |> Ecto.Changeset.change(consensus: false) |> Repo.update!()
    assert_native_identity(conn, address, did)
    bound_block |> Ecto.Changeset.change(consensus: false) |> Repo.update!()
    assert_unbound(conn, address, did)
  end

  test "pax_identity_replay_enriches_an_existing_unenriched_row", %{conn: conn} do
    {:ok, native_bytes} = Identity.pax_address_bytes(@pax)
    address = insert(:address, hash: native_bytes, fetched_coin_balance: 0)
    did = Identity.did(@key)
    %{row: row} = IdentitySearchTest.import_binding(address, did, Identity.kernel_account(@key), true, 0)
    [stored] = Repo.all(AccountBinding)
    stored |> Ecto.Changeset.change(pax_address: nil, layerx_account: nil) |> Repo.update!()
    assert {:ok, _imported} = Import.all(%{paxeer_x_account_bindings: %{params: [row]}})
    assert Repo.aggregate(AccountBinding, :count) == 1
    assert_native_identity(conn, address, did)
  end

  defp assert_authenticated_headers(settlement) do
    for {header, commit} <- [
          {settlement["execution_header"], settlement["execution_commit"]},
          {settlement["header"], settlement["commit"]}
        ] do
      height = String.to_integer(header["height"])

      assert {:ok, ^height} =
               CometCommit.verify(
                 CommittedEvidence.chain_id(),
                 %{"header" => header, "commit" => commit},
                 CommittedEvidence.validators(settlement, header["height"])
               )
    end

    assert String.to_integer(settlement["header"]["height"]) ==
             String.to_integer(settlement["execution_header"]["height"]) + 1

    assert settlement["header"]["last_block_id"] == settlement["execution_commit"]["block_id"]
    assert settlement["execution_header"]["data_hash"] == settlement["sequence_batch_proof"]["root_hash"]
    assert settlement["execution_header"]["data_hash"] == settlement["transaction_proof"]["root_hash"]
  end

  defp mutate_settlement(settlement, mutation) do
    case mutation do
      "missing-batch-proof" ->
        Map.delete(settlement, "sequence_batch_proof")

      "missing-transaction-proof" ->
        Map.delete(settlement, "transaction_proof")

      "missing-result-proof" ->
        Map.delete(settlement, "exec_tx_result_proof")

      "missing-execution-header" ->
        Map.delete(settlement, "execution_header")

      "missing-successor-header" ->
        Map.delete(settlement, "header")

      "batch-proof" ->
        update_in(settlement, ["sequence_batch_proof", "proof", "aunts"], &flip_aunt/1)

      "transaction-proof" ->
        update_in(settlement, ["transaction_proof", "proof", "aunts"], &flip_aunt/1)

      "result-proof" ->
        update_in(settlement, ["exec_tx_result_proof", "aunts"], &flip_aunt/1)

      "result-bytes" ->
        update_in(settlement, ["exec_tx_result_bytes"], &CommittedEvidence.flip_hex/1)

      "previous-root" ->
        update_in(settlement, ["sequence_batch", "PrevSeqHash"], &flip_base64/1)

      "resulting-root" ->
        update_in(settlement, ["sequence_batch", "LastSeqHash"], &flip_base64/1)

      "transaction-root" ->
        update_in(settlement, ["sequence_batch", "TxRoot"], &flip_base64/1)

      "batch-key" ->
        update_in(settlement, ["sequence_batch", "KeyId"], &flip_base64/1)

      "batch-signature" ->
        update_in(settlement, ["sequence_batch", "Sig"], &flip_base64/1)

      "unrelated-execution-header" ->
        update_in(settlement, ["execution_header", "data_hash"], fn _ -> String.duplicate("04", 32) end)

      "unlinked-successor" ->
        update_in(settlement, ["header", "last_block_id", "hash"], fn _ -> String.duplicate("04", 32) end)

      "insufficient-successor-power" ->
        Map.put(settlement, "commit", settlement["commit_at_most_two_thirds"])
    end
  end

  defp receipted_calls(settlement) do
    CommittedEvidence.committed_calls(settlement)
    |> Enum.take(3)
    |> CommittedEvidence.replace_call("eth_getTransactionReceipt", nil)
  end

  defp receipt_path(settlement, route), do: "/api/v2/transactions/#{settlement["receipt"]["txHash"]}/#{route}"

  defp sign_foreign_receipt(settlement) do
    {public_key, private_key} = :crypto.generate_key(:eddsa, :ed25519, :binary.copy(<<0xA7>>, 32))
    key_id = :crypto.hash(:sha256, public_key)
    {:ok, receipt} = settlement["receipt"]["receipt"] |> CommittedEvidence.bytes() |> SequencerReceipt.decode()
    unsigned = %{receipt | context_hash: key_id, signature: nil}
    signature = :crypto.sign(:eddsa, :none, SequencerReceipt.digest(unsigned), [private_key, :ed25519])
    receipt = SequencerReceipt.encode(%{unsigned | signature: signature})

    answer =
      settlement["receipt"]
      |> Map.put("keyId", CommittedEvidence.hex(key_id))
      |> Map.put("signingKey", CommittedEvidence.hex(public_key))
      |> Map.put("receipt", CommittedEvidence.hex(receipt))

    key =
      CommittedEvidence.receipt_key()
      |> Map.put("keyId", CommittedEvidence.hex(key_id))
      |> Map.put("publicKey", CommittedEvidence.hex(public_key))

    {answer, key}
  end

  defp flip_byte(encoded, offset) do
    <<prefix::binary-size(offset), byte, rest::binary>> = CommittedEvidence.bytes(encoded)
    CommittedEvidence.hex(prefix <> <<Bitwise.bxor(byte, 1)>> <> rest)
  end

  defp flip_aunt([first | rest]), do: [flip_base64(first) | rest]

  defp flip_base64(encoded) do
    <<first, rest::binary>> = Base.decode64!(encoded)
    Base.encode64(<<Bitwise.bxor(first, 1), rest::binary>>)
  end

  defp assert_native_identity(conn, address, did) do
    for spelling <- [to_string(address.hash), @pax, did, Identity.kernel_account(@key), @key] do
      response = json_response(get(conn, "/api/v2/search", %{"q" => spelling}), 200)
      assert [%{"type" => "address", "address_hash" => address_hash}] = response["items"]
      assert address_hash == Address.checksum(address.hash)

      unified = json_response(get(conn, "/api/v2/addresses/#{spelling}/unified"), 200)
      assert unified["identities"] == %{"evm" => Address.checksum(address.hash), "pax" => @pax, "did" => did}
    end

    response = json_response(get(conn, "/api/v2/addresses/#{address.hash}/unified"), 200)
    assert response["identities"] == %{"evm" => Address.checksum(address.hash), "pax" => @pax, "did" => did}
  end

  defp assert_unbound(conn, address, did) do
    for spelling <- [@pax, did, Identity.kernel_account(@key), @key] do
      assert %{"items" => []} = json_response(get(conn, "/api/v2/search", %{"q" => spelling}), 200)
    end

    response = json_response(get(conn, "/api/v2/addresses/#{address.hash}/unified"), 200)
    assert response["identities"] == %{"evm" => Address.checksum(address.hash), "pax" => nil, "did" => nil}
  end
end
