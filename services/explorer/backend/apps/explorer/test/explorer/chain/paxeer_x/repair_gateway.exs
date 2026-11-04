Code.require_file("repair_transport.exs", __DIR__)

defmodule Explorer.Chain.PaxeerX.Repair2233523.Gateway do
  import ExUnit.Assertions
  alias Explorer.Chain.PaxeerX.{CometCommit, SequencerReceipt}
  alias Explorer.Chain.PaxeerX.Repair2233523.Transport

  @fixtures Path.expand("../../../support/fixture/paxeer_x", __DIR__)

  def receipt_gateway_transport_proof do
    corpus = Path.join(@fixtures, "committed_vectors.json") |> File.read!() |> Jason.decode!()
    vector = hd(corpus["cases"])
    old_vectors = Path.join(@fixtures, "receipt_vectors.json") |> File.read!() |> Jason.decode!()
    rotated = Enum.find(old_vectors["sequencer"]["cases"], &(&1["id"] == "V1-pax-transfer"))
    {:ok, [key]} = SequencerReceipt.decode_receipt_keys([vector["pinnedKey"]])
    {:ok, [legacy_key]} = SequencerReceipt.decode_receipt_keys([rotated["pinnedKey"]])
    next_key = %{legacy_key | key_id: :crypto.hash(:sha256, legacy_key.public_key)}
    next_wire_key = Map.put(rotated["pinnedKey"], "keyId", "0x" <> Base.encode16(next_key.key_id, case: :lower))
    refute key.key_id == next_key.key_id
    config = Application.get_env(:explorer, SequencerReceipt)
    adapter = Application.get_env(:tesla, :adapter)
    Application.put_env(:tesla, :adapter, Tesla.Adapter.Mint)
    height = vector["commit"]["signed_header"]["header"]["height"]

    checkpoint = %{
      block_hash: Base.decode16!(vector["commit"]["signed_header"]["commit"]["block_id"]["hash"], case: :mixed),
      validators_hash: Base.decode16!(vector["commit"]["signed_header"]["header"]["validators_hash"], case: :mixed)
    }

    Application.put_env(:explorer, SequencerReceipt,
      chain_id: vector["chain_id"],
      receipt_keys: [key],
      validator_checkpoints: %{String.to_integer(height) => checkpoint}
    )

    {:ok, state} = Agent.start_link(fn -> :original end)
    server = Bypass.open()
    observer = self()

    Bypass.expect(server, "POST", "/", fn conn ->
      {:ok, body, conn} = Plug.Conn.read_body(conn)
      request = Jason.decode!(body)
      assert request["jsonrpc"] == "2.0"
      method = request["method"]
      assert request["params"] == if(method == "px_getReceiptKeys", do: [], else: [height])
      send(observer, {:gateway_request, method})

      response =
        case Agent.get(state, & &1) do
          :refused ->
            %{
              "error" => %{
                "code" => -32003,
                "message" => "consensus_unavailable",
                "data" => %{"code" => "consensus_unavailable"}
              }
            }

          :malformed ->
            %{"result" => %{}}

          :bad_json ->
            :bad_json

          mode ->
            result =
              case method do
                "px_getReceiptKeys" ->
                  keys =
                    case mode do
                      :replacement -> [next_wire_key]
                      :rotated -> [vector["pinnedKey"], next_wire_key]
                      :original -> [vector["pinnedKey"]]
                    end

                  %{"keys" => keys}

                "px_getCommit" ->
                  %{"height" => height, "signedHeader" => vector["commit"]["signed_header"]}

                "px_getValidators" ->
                  gateway_validators(vector["validators"], height)
              end

            %{"result" => result}
        end

      body =
        if response == :bad_json,
          do: "{",
          else: Jason.encode!(Map.merge(response, %{"jsonrpc" => "2.0", "id" => request["id"]}))

      Plug.Conn.resp(conn, 200, body)
    end)

    url = "http://127.0.0.1:#{server.port}/"

    try do
      consumer = Task.async(fn -> read_and_verify(url, vector) end)
      assert {:committed, ^key, 10} = Task.await(consumer, 10_000)
      refute Process.alive?(consumer.pid)

      for method <- ["px_getReceiptKeys", "px_getCommit", "px_getValidators"] do
        assert_received {:gateway_request, ^method}
      end

      reopened = Task.async(fn -> read_and_verify(url, vector) end)
      assert Task.await(reopened, 10_000) == {:committed, key, 10}
      refute Process.alive?(reopened.pid)

      for {mode, expected} <- [
            {:malformed, [:malformed_receipt_keys, :malformed_commit, :malformed_validators]},
            {:bad_json, [:malformed_response, :malformed_response, :malformed_response]}
          ] do
        Agent.update(state, fn _ -> mode end)

        results = [
          SequencerReceipt.receipt_keys(url),
          SequencerReceipt.commit(10, url),
          SequencerReceipt.validators(10, url)
        ]

        assert results == Enum.map(expected, &{:error, &1})
      end

      Agent.update(state, fn _ -> :refused end)

      for result <- [
            SequencerReceipt.receipt_keys(url),
            SequencerReceipt.commit(10, url),
            SequencerReceipt.validators(10, url)
          ] do
        assert {:error, {:rpc_error, %{"code" => -32003, "data" => %{"code" => "consensus_unavailable"}}}} = result
      end

      Agent.update(state, fn _ -> :replacement end)
      assert SequencerReceipt.receipt_keys(url) == {:error, :untrusted_receipt_keys}
      current = Application.fetch_env!(:explorer, SequencerReceipt)
      Application.put_env(:explorer, SequencerReceipt, Keyword.put(current, :receipt_keys, [key, next_key]))
      Agent.update(state, fn _ -> :rotated end)

      rotated_consumer =
        Task.async(fn ->
          assert {:ok, [^key, ^next_key]} = SequencerReceipt.receipt_keys(url)

          SequencerReceipt.verify(transaction_hash(vector), vector["receiptJson"], [key, next_key],
            prev_seq_hash: vector["prevSeqHash"]
          )
        end)

      assert Task.await(rotated_consumer, 10_000) == {:receipted, key}
      refute Process.alive?(rotated_consumer.pid)
      assert {:ok, [^key, ^next_key]} = SequencerReceipt.receipt_keys(url)
    after
      Agent.stop(state)
      Transport.restore(:explorer, SequencerReceipt, config)
      Transport.restore(:tesla, :adapter, adapter)
    end
  end

  defp read_and_verify(url, vector) do
    assert {:ok, [key]} = SequencerReceipt.receipt_keys(url)
    assert {:ok, commit} = SequencerReceipt.commit(10, url)
    assert {:ok, validators} = SequencerReceipt.validators(10, url)
    assert commit == vector["commit"]
    assert validators == vector["validators"]
    assert {:ok, 10} = CometCommit.verify(vector["chain_id"], commit, validators)

    assert SequencerReceipt.verify(transaction_hash(vector), vector["receiptJson"], [key],
             prev_seq_hash: vector["prevSeqHash"]
           ) == {:receipted, key}

    SequencerReceipt.verify(transaction_hash(vector), vector["receiptJson"], [key],
      prev_seq_hash: vector["prevSeqHash"],
      eth_receipt: vector["eth_receipt"],
      batch: vector["batch"],
      batch_activity_ids: [transaction_hash(vector)],
      chain_id: vector["chain_id"],
      commit: commit,
      validators: validators,
      receipt_inclusion: vector["receiptInclusion"]
    )
  end

  defp gateway_validators(validators, height) do
    %{
      "height" => height,
      "total" => validators["total"],
      "validators" =>
        Enum.map(validators["validators"], fn entry ->
          %{"address" => entry["address"], "pubKey" => entry["pub_key"], "votingPower" => entry["voting_power"]}
        end)
    }
  end

  defp transaction_hash(vector) do
    "0x" <> raw = vector["rawTransaction"]
    raw |> Base.decode16!(case: :mixed) |> ExKeccak.hash_256()
  end
end
