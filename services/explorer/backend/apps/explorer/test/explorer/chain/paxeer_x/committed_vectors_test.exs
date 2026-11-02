defmodule Explorer.Chain.PaxeerX.CommittedVectorsTest do
  use ExUnit.Case, async: true

  alias Explorer.Chain.PaxeerX.SequencerReceipt

  @vectors Path.expand("../../../support/fixture/paxeer_x/committed_vectors.json", __DIR__)

  test "committed_vectors_drive_verify" do
    %{"cases" => cases} = @vectors |> File.read!() |> Jason.decode!()

    assert Enum.map(cases, & &1["id"]) == [
             "committed-ok",
             "r7-height-below-receipt",
             "r8-commit-height-mismatch"
           ]

    {:ok, [key]} = SequencerReceipt.decode_receipt_keys([hd(cases)["pinnedKey"]])

    assert Enum.map(cases, &committed_vectors_drive_verify/1) == [
             {:committed, key, 1_000},
             {:error, :r7},
             {:error, :r8}
           ]
  end

  defp committed_vectors_drive_verify(test_case) do
    "0x" <> raw_transaction = test_case["rawTransaction"]
    tx_hash = raw_transaction |> Base.decode16!(case: :mixed) |> ExKeccak.hash_256()
    {:ok, keys} = SequencerReceipt.decode_receipt_keys([test_case["pinnedKey"]])
    %{"from_seq" => from_seq, "to_seq" => to_seq} = test_case["batch"]

    SequencerReceipt.verify(tx_hash, test_case["receiptJson"], keys,
      prev_seq_hash: test_case["prevSeqHash"],
      eth_receipt: test_case["eth_receipt"],
      batch: %{from_seq: from_seq, to_seq: to_seq},
      chain_id: test_case["chain_id"],
      commit: test_case["commit"],
      validators: test_case["validators"]
    )
  end
end
