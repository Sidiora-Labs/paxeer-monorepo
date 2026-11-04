defmodule Explorer.Repair2231411Test do
  use ExUnit.Case, async: false

  alias Explorer.Chain.PaxeerX.CometCommit
  alias Explorer.Chain.PaxeerX.Finality.Heights
  alias Explorer.Chain.PaxeerX.SequencerReceipt
  alias Explorer.Chain.PaxeerX.Status
  alias Explorer.HttpClient

  @vectors Path.expand("../support/fixture/paxeer_x/committed_vectors.json", __DIR__)
  @chain_id "committed-fixture"
  @validator_hash Base.decode16!("65CEAF405AF4643DA85D93D6CAA8CA8A47F80A4174D4541D7D3F58401981ACA2")
  @block_hash Base.decode16!("066B3B86EB6F62437B659C1D075F28EB22246D233FE86B3D5CACBDB5E18216E9")
  @receipt_key %{
    key_id: Base.decode16!("BA3A5A0BBA18E7EAD9FD281C793DAAEC55218C5D084483346E45C959A92BF699"),
    public_key: Base.decode16!("AED3D83B59B735683EDB641862D13D6855F37284C190B7B501A46CA29085B239"),
    first_height: 10,
    last_height: 10
  }

  defmodule RecordedRPC do
    @behaviour Plug

    def init(options), do: options

    def call(conn, {responses, observer}) do
      {:ok, body, conn} = Plug.Conn.read_body(conn)
      %{"jsonrpc" => "2.0", "id" => id, "method" => method, "params" => params} = Jason.decode!(body)
      result = Map.fetch!(responses, {method, params})
      send(observer, {:recorded_rpc, method, params})

      conn
      |> Plug.Conn.put_resp_content_type("application/json")
      |> Plug.Conn.send_resp(200, Jason.encode!(%{jsonrpc: "2.0", id: id, result: result}))
    end
  end

  setup do
    old_config = Application.get_env(:explorer, SequencerReceipt)
    old_adapter = Application.get_env(:tesla, :adapter)
    Application.put_env(:tesla, :adapter, Tesla.Adapter.Mint)

    Application.put_env(:explorer, SequencerReceipt,
      chain_id: @chain_id,
      receipt_keys: [@receipt_key],
      validator_checkpoints: %{10 => %{block_hash: @block_hash, validators_hash: @validator_hash}}
    )

    {:ok, _} = Application.ensure_all_started(:plug_cowboy)

    on_exit(fn ->
      restore_env(:explorer, SequencerReceipt, old_config)
      restore_env(:tesla, :adapter, old_adapter)
    end)

    corpus = @vectors |> File.read!() |> Jason.decode!()
    [positive | _] = corpus["cases"]
    {:ok, corpus: corpus, positive: positive}
  end

  test "committed_without_inclusion_same_execution_reaches_status", %{positive: vector} do
    receipt = vector["receiptJson"]
    settlement = vector["settlement"]
    {:ok, [key]} = SequencerReceipt.decode_receipt_keys([vector["pinnedKey"]])
    {:ok, decoded} = receipt["receipt"] |> bytes() |> SequencerReceipt.decode()

    assert decoded.activity_id == transaction_hash(vector)
    assert SequencerReceipt.encode(decoded) == bytes(receipt["receipt"])

    assert SequencerReceipt.verify(transaction_hash(vector), receipt, [key], prev_seq_hash: vector["prevSeqHash"]) ==
             {:receipted, key}

    assert receipt == settlement["receipt"]

    assert Base.decode64!(vector["receiptInclusion"]["Transaction"]) ==
             Base.decode64!(settlement["transaction_proof"]["data"])

    assert receipt["blockNumber"] == "0xa"
    assert receipt["blockHash"] |> bytes() == Base.decode16!(settlement["execution_commit"]["block_id"]["hash"])
    assert {:ok, 10} = CometCommit.verify(vector["chain_id"], vector["commit"], vector["validators"])
    assert {:ok, 11} = CometCommit.verify(vector["chain_id"], vector["results_commit"], vector["results_validators"])
    assert settlement["header"]["last_block_id"] == settlement["execution_commit"]["block_id"]

    for name <- ["sequence_batch_proof", "transaction_proof"] do
      proof = settlement[name]
      root = Base.decode16!(settlement["execution_header"]["data_hash"])
      assert Base.decode16!(proof["root_hash"]) == root
      tx_hash = :crypto.hash(:sha256, Base.decode64!(proof["data"]))
      assert_two_leaf_proof(tx_hash, proof["proof"], root)
    end

    result_root = Base.decode16!(settlement["header"]["last_results_hash"])
    assert_two_leaf_proof(bytes(settlement["exec_tx_result_bytes"]), settlement["exec_tx_result_proof"], result_root)
    assert settlement["transaction_proof"]["proof"]["index"] == "1"
    assert settlement["exec_tx_result_proof"]["index"] == "1"
    assert settlement["sequence_batch"]["FromSeq"] == decoded.global_sequence
    assert settlement["sequence_batch"]["ToSeq"] == decoded.global_sequence
    assert settlement["exec_tx_result"]["gas_used"] == 39_092
    assert receipt["gasUsed"] == "0x98b4"

    calls = recorded_calls(vector)
    url = replay(calls)
    config = Application.fetch_env!(:explorer, SequencerReceipt)
    Application.put_env(:explorer, SequencerReceipt, Keyword.put(config, :node_rpc_url, url))
    assert {:ok, ^receipt} = rpc(url, "pax_sequencerReceipt", [receipt["txHash"]])
    assert {:ok, [^key]} = SequencerReceipt.receipt_keys(url)
    assert {:ok, eth_receipt} = rpc(url, "eth_getTransactionReceipt", [receipt["txHash"]])
    assert {:ok, commit} = SequencerReceipt.commit(10, url)
    assert {:ok, validators} = SequencerReceipt.validators(10, url)
    assert {:ok, batch} = SequencerReceipt.sequence_batch(10, url)
    assert eth_receipt == vector["eth_receipt"]
    assert commit == vector["commit"]
    assert validators == vector["validators"]
    assert batch == vector["batch"]
    assert bytes(batch["tx"]) == Base.decode64!(settlement["sequence_batch_proof"]["data"])
    assert {:ok, evidence_opts} = Explorer.Chain.PaxeerX.ReceiptEvidence.options(url, receipt["txHash"], receipt)
    assert SequencerReceipt.verify(transaction_hash(vector), receipt, [key], evidence_opts) == {:committed, key, 10}

    for {method, params, _result} <- calls do
      assert_received {:recorded_rpc, ^method, ^params}
    end

    verification =
      verify(%{
        vector
        | "receiptJson" => receipt,
          "eth_receipt" => eth_receipt,
          "commit" => commit,
          "validators" => validators,
          "batch" => batch
      })

    assert verification == {:committed, key, 10}

    assert Status.of(nil, %Heights{committed_height: nil}, verification) == %Status{
             rung: :committed,
             reason: :sequencer_receipt,
             block_number: nil,
             committed_height: 10
           }
  end

  test "committed_without_inclusion_controller_preserves_authenticated_evidence", %{positive: vector} do
    :ok = Ecto.Adapters.SQL.Sandbox.checkout(Explorer.Repo)
    :ok = Ecto.Adapters.SQL.Sandbox.checkout(Explorer.Repo.Account)
    calls = recorded_calls(vector)
    url = replay(calls)
    config = Application.fetch_env!(:explorer, SequencerReceipt)
    Application.put_env(:explorer, SequencerReceipt, Keyword.merge(config, router_url: url, node_rpc_url: url))
    hash = vector["receiptJson"]["txHash"]
    {:ok, full_hash} = Explorer.Chain.string_to_full_hash(hash)
    assert {:error, :not_found} = Explorer.Chain.hash_to_transaction(full_hash, api?: true)
    path = "/api/v2/transactions/#{hash}/status"

    response =
      BlockScoutWeb.API.V2.PaxeerX.TransactionStatusController.status(
        Plug.Test.conn(:get, path),
        %{"transaction_hash_param" => hash}
      )

    for {method, params, _result} <- calls do
      assert_received {:recorded_rpc, ^method, ^params}
    end

    assert %Plug.Conn{status: 200, resp_body: body} = response
    assert Jason.decode!(body) == %{"rung" => "committed", "block_number" => nil, "committed_height" => 10}
    assert {:error, :not_found} = Explorer.Chain.hash_to_transaction(full_hash, api?: true)
  end

  test "committed_without_inclusion_controller_rejects_altered_recorded_result", %{positive: vector} do
    :ok = Ecto.Adapters.SQL.Sandbox.checkout(Explorer.Repo)
    :ok = Ecto.Adapters.SQL.Sandbox.checkout(Explorer.Repo.Account)
    calls = recorded_calls(vector)

    calls =
      Enum.map(calls, fn
        {"block_results", params, result} ->
          result = update_in(result, ["txs_results", Access.at(1), "gas_used"], &(&1 + 1))
          {"block_results", params, result}

        call ->
          call
      end)

    url = replay(calls)
    config = Application.fetch_env!(:explorer, SequencerReceipt)
    Application.put_env(:explorer, SequencerReceipt, Keyword.merge(config, router_url: url, node_rpc_url: url))
    hash = vector["receiptJson"]["txHash"]

    assert {:error, :not_found} =
             BlockScoutWeb.API.V2.PaxeerX.TransactionStatusController.status(
               Plug.Test.conn(:get, "/api/v2/transactions/#{hash}/status"),
               %{"transaction_hash_param" => hash}
             )

    for {method, params, _result} <- calls do
      assert_received {:recorded_rpc, ^method, ^params}
    end
  end

  test "committed_without_inclusion_rejects_untrusted_chain_and_validator_history", %{positive: vector} do
    config = Application.fetch_env!(:explorer, SequencerReceipt)

    for change <- [
          [chain_id: "another-chain"],
          [validator_checkpoints: %{}],
          [validator_checkpoints: %{10 => %{block_hash: <<0::256>>, validators_hash: @validator_hash}}],
          [validator_checkpoints: %{10 => %{block_hash: @block_hash, validators_hash: <<0::256>>}}]
        ] do
      Application.put_env(:explorer, SequencerReceipt, Keyword.merge(config, change))
      assert verify(vector) == {:error, :r8}
    end
  end

  test "committed_without_inclusion_preserves_r7_and_r8_rejections", %{corpus: corpus} do
    [_positive, r7, r8] = corpus["cases"]
    assert r7["id"] == "r7-height-below-receipt"
    assert r8["id"] == "r8-commit-height-mismatch"
    assert verify(r7) == {:error, :r7}
    assert verify(r8) == {:error, :r8}
  end

  test "committed_without_inclusion_rejects_unrelated_valid_header", %{corpus: corpus} do
    [unrelated] = corpus["refusals"]
    assert {:ok, 1_000} = CometCommit.verify(unrelated["chain_id"], unrelated["commit"], unrelated["validators"])
    {:ok, [key]} = SequencerReceipt.decode_receipt_keys([unrelated["pinnedKey"]])

    assert SequencerReceipt.verify(transaction_hash(unrelated), unrelated["receiptJson"], [key]) == {:receipted, key}
    assert verify(unrelated) == {:error, :r8}
  end

  test "committed_without_inclusion_rejects_altered_covering_ranges", %{positive: vector} do
    for batch <- [%{"from_seq" => 8, "to_seq" => 9}, %{"from_seq" => 9, "to_seq" => 10}] do
      assert verify(%{vector | "batch" => batch}) == {:error, :r7}
      assert verify(%{vector | "batch" => Map.merge(vector["batch"], batch)}) == {:error, :r7}
    end
  end

  test "committed_without_inclusion_rejects_wrong_execution_block_identity", %{positive: vector} do
    wrong_hash = "0x" <> String.downcase(vector["settlement"]["commit"]["block_id"]["hash"])
    refute wrong_hash == vector["eth_receipt"]["blockHash"]
    altered = put_in(vector, ["eth_receipt", "blockHash"], wrong_hash)
    altered = put_in(altered, ["receiptJson", "blockHash"], wrong_hash)
    assert verify(altered) == {:error, :r8}
  end

  test "committed_without_inclusion_rejects_changed_transaction_and_result_witnesses", %{positive: vector} do
    paths = [
      ["Transaction"],
      ["Result", "Data"],
      ["BatchTransaction"],
      ["NextHeader", "LastBlockID", "Hash"],
      ["NextHeader", "LastResultsHash"],
      ["NextCommit", "BlockID", "Hash"]
    ]

    results =
      Enum.map(paths, fn path ->
        <<first, rest::binary>> = vector["receiptInclusion"] |> get_in(path) |> Base.decode64!()
        altered_data = Base.encode64(<<Bitwise.bxor(first, 1), rest::binary>>)
        altered = put_in(vector, ["receiptInclusion" | path], altered_data)
        {path, verify(altered)}
      end)

    assert results == Enum.map(paths, &{&1, {:error, :r8}})
  end

  test "committed_without_inclusion_reconstructs_only_with_authenticated_evidence", %{positive: vector} do
    reconstructed = vector |> Jason.encode!() |> Jason.decode!()
    assert verify(reconstructed) == verify(vector)
    assert {:committed, _, 10} = verify(reconstructed)
    assert verify(Map.delete(reconstructed, "receiptInclusion")) == {:error, :r8}
  end

  test "committed_without_inclusion_requires_every_witness", %{positive: vector} do
    fields = [
      "Transaction",
      "TransactionProof",
      "Result",
      "ResultProof",
      "BatchTransaction",
      "BatchProof",
      "NextHeader",
      "NextCommit",
      "NextValidators"
    ]

    results =
      Enum.map(fields, fn field ->
        altered = Map.update!(vector, "receiptInclusion", &Map.delete(&1, field))
        {field, verify(altered)}
      end)

    assert results == Enum.map(fields, &{&1, {:error, :r8}})
  end

  test "committed_without_inclusion_rejects_changed_result_header_and_proofs", %{positive: vector} do
    paths = [
      ["TransactionProof", "Index"],
      ["ResultProof", "Index"],
      ["BatchProof", "Index"],
      ["NextHeader", "Height"],
      ["NextCommit", "Height"]
    ]

    results =
      Enum.map(paths, fn path ->
        altered = update_in(vector, ["receiptInclusion" | path], &(&1 + 1))
        {path, verify(altered)}
      end)

    assert results == Enum.map(paths, &{&1, {:error, :r8}})
  end

  test "committed_without_inclusion_rejects_changed_aunts_and_validator_set", %{positive: vector} do
    for field <- ["TransactionProof", "ResultProof", "BatchProof"] do
      altered =
        update_in(vector, ["receiptInclusion", field, "Aunts", Access.at(0), Access.at(0)], &Bitwise.bxor(&1, 1))

      assert verify(altered) == {:error, :r8}
    end

    altered = update_in(vector, ["receiptInclusion", "NextValidators", Access.at(0), "VotingPower"], &(&1 + 1))
    assert verify(altered) == {:error, :r8}

    altered =
      update_in(
        vector,
        ["receiptInclusion", "NextValidators", Access.at(0), "PublicKey", Access.at(0)],
        &Bitwise.bxor(&1, 1)
      )

    assert verify(altered) == {:error, :r8}

    for field <- Map.keys(vector["receiptInclusion"]) do
      assert verify(put_in(vector, ["receiptInclusion", field], nil)) == {:error, :r8}
    end
  end

  test "committed_without_inclusion_never_promotes_unproven_unindexed_hash", %{corpus: corpus} do
    [unrelated] = corpus["refusals"]
    verification = verify(unrelated)

    for head <- [nil, 0, 999, 1_000, 1_001] do
      status = Status.of(nil, %Heights{committed_height: head}, verification)
      assert status.rung == :submitted
      assert status.committed_height == head
    end
  end

  defp verify(vector) do
    {:ok, keys} = SequencerReceipt.decode_receipt_keys([vector["pinnedKey"]])

    SequencerReceipt.verify(transaction_hash(vector), vector["receiptJson"], keys,
      prev_seq_hash: vector["prevSeqHash"],
      eth_receipt: vector["eth_receipt"],
      batch: vector["batch"],
      batch_activity_ids: [transaction_hash(vector)],
      chain_id: vector["chain_id"],
      commit: vector["commit"],
      validators: vector["validators"],
      receipt_inclusion: vector["receiptInclusion"]
    )
  end

  defp recorded_calls(vector) do
    height = vector["commit"]["signed_header"]["header"]["height"]

    validators = %{
      "height" => height,
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

    gateway_calls = [
      {"pax_sequencerReceipt", [vector["receiptJson"]["txHash"]], vector["receiptJson"]},
      {"px_getReceiptKeys", [], %{"keys" => [vector["pinnedKey"]]}},
      {"eth_getTransactionReceipt", [vector["receiptJson"]["txHash"]], vector["eth_receipt"]},
      {"px_getCommit", [height], %{"height" => height, "signedHeader" => vector["commit"]["signed_header"]}},
      {"px_getValidators", [height], validators},
      {"px_getSequenceBatch", [height], vector["batch"]}
    ]

    settlement = vector["settlement"]
    next_height = vector["results_commit"]["signed_header"]["header"]["height"]

    next_validators = %{
      "height" => next_height,
      "total" => vector["results_validators"]["total"],
      "validators" =>
        Enum.map(vector["results_validators"]["validators"], fn validator ->
          %{
            "address" => validator["address"],
            "pubKey" => validator["pub_key"],
            "votingPower" => validator["voting_power"]
          }
        end)
    }

    gateway_calls ++
      [
        {"block", %{"height" => height},
         %{
           "block" => %{
             "header" => settlement["execution_header"],
             "data" => %{"txs" => [settlement["sequence_batch_proof"]["data"], settlement["transaction_proof"]["data"]]}
           }
         }},
        {"block_results", %{"height" => height},
         %{
           "height" => height,
           "txs_results" => [
             %{"code" => 0, "data" => "", "gas_wanted" => 0, "gas_used" => 0},
             settlement["exec_tx_result"]
           ]
         }},
        {"px_getCommit", [next_height],
         %{"height" => next_height, "signedHeader" => vector["results_commit"]["signed_header"]}},
        {"px_getValidators", [next_height], next_validators}
      ]
  end

  defp replay(calls) do
    responses = Map.new(calls, fn {method, params, result} -> {{method, params}, result} end)
    ref = {:receipt_recording_2231411, make_ref()}

    start_supervised!(
      {Plug.Cowboy, scheme: :http, plug: {RecordedRPC, {responses, self()}}, options: [port: 0, ref: ref]}
    )

    "http://127.0.0.1:#{:ranch.get_port(ref)}/"
  end

  defp rpc(url, method, params) do
    body = Jason.encode!(%{jsonrpc: "2.0", id: 1, method: method, params: params})
    {:ok, %{status_code: 200, body: response}} = HttpClient.post(url, body, [{"Content-Type", "application/json"}])
    %{"result" => result} = Jason.decode!(response)
    {:ok, result}
  end

  defp restore_env(app, key, nil), do: Application.delete_env(app, key)
  defp restore_env(app, key, value), do: Application.put_env(app, key, value)

  defp assert_two_leaf_proof(data, proof, root) do
    assert proof["total"] == "2"
    assert proof["index"] in ["0", "1"]
    [aunt] = proof["aunts"]
    leaf = :crypto.hash(:sha256, <<0>> <> data)
    assert leaf == Base.decode64!(proof["leaf_hash"])
    sibling = Base.decode64!(aunt)
    assert byte_size(sibling) == 32
    {left, right} = if proof["index"] == "0", do: {leaf, sibling}, else: {sibling, leaf}
    assert :crypto.hash(:sha256, <<1>> <> left <> right) == root
  end

  defp transaction_hash(vector), do: vector["rawTransaction"] |> bytes() |> ExKeccak.hash_256()
  defp bytes("0x" <> value), do: Base.decode16!(value, case: :mixed)
end
