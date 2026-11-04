defmodule BlockScoutWeb.API.V2.PaxeerX.CommittedEvidence do
  @moduledoc false

  import ExUnit.Assertions

  alias Explorer.Chain.PaxeerX.SequencerReceipt
  alias Plug.Conn

  # The builder mounts apps/, not daemon/. Keep a byte-exact source snapshot here.
  @fixture Path.join(__DIR__, "fixtures/vector-v4-committed.json")
  @external_resource @fixture

  # These are operator inputs, never values copied from an RPC answer at runtime.
  @chain_id "committed-fixture"
  @validators_hash "65CEAF405AF4643DA85D93D6CAA8CA8A47F80A4174D4541D7D3F58401981ACA2"
  @execution_hash "066B3B86EB6F62437B659C1D075F28EB22246D233FE86B3D5CACBDB5E18216E9"
  @receipt_key %{
    "keyId" => "0xba3a5a0bba18e7ead9fd281c793daaec55218c5d084483346e45c959a92bf699",
    "publicKey" => "0xaed3d83b59b735683edb641862d13d6855f37284c190b7b501a46ca29085b239",
    "firstHeight" => 1,
    "lastHeight" => 9_223_372_036_854_775_807
  }

  def vector, do: @fixture |> File.read!() |> Jason.decode!()
  def fixture_digest, do: :crypto.hash(:sha256, File.read!(@fixture)) |> Base.encode16(case: :lower)
  def settlement, do: vector()["settlement"]
  def receipt_key, do: @receipt_key
  def chain_id, do: @chain_id

  def authority do
    {:ok, keys} = SequencerReceipt.decode_receipt_keys([@receipt_key])

    [
      chain_id: @chain_id,
      receipt_keys: keys,
      trusted_validator_hashes: %{10 => Base.decode16!(@validators_hash)},
      validator_checkpoints: %{
        10 => %{block_hash: Base.decode16!(@execution_hash), validators_hash: Base.decode16!(@validators_hash)}
      }
    ]
  end

  def gateway_batch(settlement) do
    proof = settlement["sequence_batch_proof"]["proof"]

    %{
      "height" => settlement["execution_header"]["height"],
      "tx" => settlement["sequence_batch_proof"]["data"] |> Base.decode64!() |> hex(),
      "proof" => %{
        "total" => String.to_integer(proof["total"]),
        "index" => String.to_integer(proof["index"]),
        "leafHash" => proof["leaf_hash"] |> Base.decode64!() |> hex(),
        "aunts" => Enum.map(proof["aunts"], &(&1 |> Base.decode64!() |> hex()))
      }
    }
  end

  def validators(settlement, height) do
    %{
      "block_height" => height,
      "count" => Integer.to_string(length(settlement["validators"])),
      "total" => Integer.to_string(length(settlement["validators"])),
      "validators" => settlement["validators"]
    }
  end

  def gateway_validators(settlement, height) do
    %{
      "height" => height,
      "total" => Integer.to_string(length(settlement["validators"])),
      "validators" =>
        Enum.map(settlement["validators"], fn validator ->
          %{
            "address" => validator["address"],
            "pubKey" => validator["pub_key"],
            "votingPower" => validator["voting_power"]
          }
        end)
    }
  end

  def eth_receipt(settlement) do
    settlement["receipt"]
    |> Map.take(
      ~w(blockNumber blockHash status gasUsed cumulativeGasUsed effectiveGasPrice logs from to contractAddress)
    )
    |> Map.put("transactionHash", settlement["receipt"]["txHash"])
    |> Map.put("transactionIndex", settlement["receipt"]["index"])
  end

  def committed_calls(settlement) do
    hash = settlement["receipt"]["txHash"]
    height = settlement["execution_header"]["height"]

    [
      {"pax_sequencerReceipt", [hash], settlement["receipt"]},
      {"px_getReceiptKeys", [], %{"keys" => [@receipt_key]}},
      {"eth_getTransactionReceipt", [hash], eth_receipt(settlement)},
      {"px_getCommit", [height],
       %{
         "height" => height,
         "signedHeader" => %{"header" => settlement["execution_header"], "commit" => settlement["execution_commit"]}
       }},
      {"px_getValidators", [height], gateway_validators(settlement, height)},
      {"px_getSequenceBatch", [height], gateway_batch(settlement)}
    ] ++ native_calls(settlement) ++ successor_calls(settlement)
  end

  def native_calls(settlement) do
    height = settlement["execution_header"]["height"]

    [
      {"block", %{"height" => height},
       %{
         "block_id" => settlement["execution_commit"]["block_id"],
         "block" => %{
           "header" => settlement["execution_header"],
           "data" => %{
             "txs" => [
               settlement["sequence_batch_proof"]["data"],
               settlement["transaction_proof"]["data"]
             ]
           }
         }
       }},
      {"block_results", %{"height" => height},
       %{"height" => height, "txs_results" => [%{}, settlement["exec_tx_result"]]}}
    ]
  end

  def successor_calls(settlement) do
    height = settlement["header"]["height"]

    [
      {"px_getCommit", [height],
       %{"height" => height, "signedHeader" => %{"header" => settlement["header"], "commit" => settlement["commit"]}}},
      {"px_getValidators", [height], gateway_validators(settlement, height)}
    ]
  end

  def verification_options(settlement) do
    batch = settlement["sequence_batch"]
    assert is_integer(batch["FromSeq"]) and is_integer(batch["ToSeq"])

    [
      eth_receipt: eth_receipt(settlement),
      batch:
        Map.merge(gateway_batch(settlement), %{
          from_seq: batch["FromSeq"],
          to_seq: batch["ToSeq"]
        }),
      batch_activity_ids: [bytes(settlement["receipt"]["txHash"])],
      chain_id: @chain_id,
      commit: %{"header" => settlement["execution_header"], "commit" => settlement["execution_commit"]},
      validators: validators(settlement, settlement["execution_header"]["height"]),
      settlement: settlement,
      authority: authority()
    ]
  end

  def replace_call(calls, method, result) do
    Enum.map(calls, fn
      {^method, params, _old} -> {method, params, result}
      call -> call
    end)
  end

  def serve_rpc(calls, authority \\ authority()) do
    bypass = Bypass.open()
    test_pid = self()
    answers = Map.new(calls, fn {method, params, result} -> {{method, params}, result} end)

    Bypass.expect(bypass, "POST", "/", fn conn ->
      {:ok, body, conn} = Conn.read_body(conn)
      assert %{"jsonrpc" => "2.0", "id" => id, "method" => method, "params" => params} = Jason.decode!(body)
      result = Map.fetch!(answers, {method, params})
      send(test_pid, {:rpc_call, method, params})

      conn
      |> Conn.put_resp_content_type("application/json")
      |> Conn.resp(200, Jason.encode!(%{jsonrpc: "2.0", id: id, result: result}))
    end)

    Application.put_env(
      :explorer,
      SequencerReceipt,
      authority
      |> Keyword.put(:router_url, "http://localhost:#{bypass.port}/")
      |> Keyword.put(:node_rpc_url, "http://localhost:#{bypass.port}/")
    )

    bypass
  end

  def assert_calls(calls) do
    for {method, params, _result} <- calls do
      assert_received {:rpc_call, ^method, ^params}
    end

    refute_received {:rpc_call, _method, _params}
  end

  def restart_endpoint do
    previous = Process.whereis(BlockScoutWeb.Endpoint)
    assert is_pid(previous)
    assert :ok = Supervisor.terminate_child(BlockScoutWeb.Supervisor, BlockScoutWeb.Endpoint)
    refute Process.alive?(previous)
    assert {:ok, reopened} = Supervisor.restart_child(BlockScoutWeb.Supervisor, BlockScoutWeb.Endpoint)
    assert reopened != previous
    reopened
  end

  def restore_env(app, key, nil), do: Application.delete_env(app, key)
  def restore_env(app, key, value), do: Application.put_env(app, key, value)
  def bytes("0x" <> encoded), do: Base.decode16!(encoded, case: :mixed)
  def hex(bytes), do: "0x" <> Base.encode16(bytes, case: :lower)

  def flip_hex(encoded) do
    <<first, rest::binary>> = bytes(encoded)
    hex(<<Bitwise.bxor(first, 1), rest::binary>>)
  end
end
